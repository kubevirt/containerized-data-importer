package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) CreateNIC(vmid string, name string, vnicProfileID string, retries ...RetryStrategy) (result NIC, err error) {
	if err := validateNICCreationParameters(vmid, name); err != nil {
		return nil, err
	}

	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("creating NIC for VM %s", vmid),
		o.logger,
		retries,
		func() error {
			nicBuilder := ovirtsdk.NewNicBuilder()
			nicBuilder.Name(name)
			nicBuilder.VnicProfile(ovirtsdk.NewVnicProfileBuilder().Id(vnicProfileID).MustBuild())
			nic := nicBuilder.MustBuild()

			response, err := o.conn.SystemService().VmsService().VmService(vmid).NicsService().Add().Nic(nic).Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Nic()
			if !ok {
				return newError(
					ENotFound,
					"no NIC returned creating NIC for VM ID %s",
					vmid,
				)
			}
			result, err = convertSDKNIC(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert newly created NIC for VM %s",
					vmid,
				)
			}
			return nil
		},
	)
	return result, err
}

func validateNICCreationParameters(vmid string, name string) error {
	if vmid == "" {
		return newError(EBadArgument, "VM ID cannot be empty")
	}
	if name == "" {
		return newError(EBadArgument, "NIC name cannot be empty")
	}
	return nil
}
