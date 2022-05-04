package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) UpdateNIC(
	vmid string,
	nicID string,
	params UpdateNICParameters,
	retries ...RetryStrategy,
) (result NIC, err error) {
	req := o.conn.SystemService().VmsService().VmService(vmid).NicsService().NicService(nicID).Update()

	nicBuilder := ovirtsdk.NewNicBuilder().Id(nicID)
	if name := params.Name(); name != nil {
		nicBuilder.Name(*name)
	}
	if vnicProfileID := params.VNICProfileID(); vnicProfileID != nil {
		nicBuilder.VnicProfile(ovirtsdk.NewVnicProfileBuilder().Id(*vnicProfileID).MustBuild())
	}

	req.Nic(nicBuilder.MustBuild())

	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("updating NIC %s for VM %s", nicID, vmid),
		o.logger,
		retries,
		func() error {
			update, err := req.Send()
			if err != nil {
				return wrap(err, EUnidentified, "Failed to update NIC %s", nicID)
			}
			sdkNIC, ok := update.Nic()
			if !ok {
				return newFieldNotFound("NIC update response", "NIC")
			}
			nic, err := convertSDKNIC(sdkNIC, o)
			if err != nil {
				return err
			}
			result = nic
			return nil
		})
	return result, err
}
