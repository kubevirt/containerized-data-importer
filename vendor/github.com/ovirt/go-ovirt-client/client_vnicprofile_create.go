package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) CreateVNICProfile(
	name string,
	networkID string,
	params OptionalVNICProfileParameters,
	retries ...RetryStrategy,
) (result VNICProfile, err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())

	if err := validateVNICProfileCreationParameters(name, networkID, params); err != nil {
		return nil, err
	}

	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("creating VNIC profile %s", name),
		o.logger,
		retries,
		func() error {
			profileBuilder := ovirtsdk.NewVnicProfileBuilder()
			profileBuilder.Name(name)
			profileBuilder.Network(ovirtsdk.NewNetworkBuilder().Id(networkID).MustBuild())
			req := o.conn.SystemService().VnicProfilesService().Add()
			response, err := req.Profile(profileBuilder.MustBuild()).Send()
			if err != nil {
				return err
			}
			profile, ok := response.Profile()
			if !ok {
				return newFieldNotFound("response from VNIC profile creation", "profile")
			}
			result, err = convertSDKVNICProfile(profile, o)
			return err
		})
	return result, err
}

func validateVNICProfileCreationParameters(name string, networkID string, _ OptionalVNICProfileParameters) error {
	if name == "" {
		return newError(EBadArgument, "name cannot be empty for VNIC profile creation")
	}
	if networkID == "" {
		return newError(EBadArgument, "network ID cannot be empty for VNIC profile creation")
	}
	return nil
}
