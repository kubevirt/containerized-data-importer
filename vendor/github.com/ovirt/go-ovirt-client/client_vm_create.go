package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) CreateVM(name string, clusterID string, templateID string, params OptionalVMParameters, retries ...RetryStrategy) (result VM, err error) {
	if err := validateVMCreationParameters(name, clusterID, templateID, params); err != nil {
		return nil, err
	}

	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("creating VM %s", name),
		o.logger,
		retries,
		func() error {
			builder := ovirtsdk.NewVmBuilder()
			builder.Cluster(ovirtsdk.NewClusterBuilder().Id(clusterID).MustBuild())
			builder.Template(ovirtsdk.NewTemplateBuilder().Id(templateID).MustBuild())
			builder.Name(name)
			vm, err := builder.Build()
			if err != nil {
				return wrap(err, EBug, "failed to build VM")
			}

			response, err := o.conn.SystemService().VmsService().Add().Vm(vm).Send()
			if err != nil {
				return wrap(err, EUnidentified, "failed to create VM")
			}
			vm, ok := response.Vm()
			if !ok {
				return newError(EFieldMissing, "missing VM in VM create response")
			}
			result, err = convertSDKVM(vm, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert VM",
				)
			}
			return nil
		},
	)
	return result, err
}

func validateVMCreationParameters(name string, clusterID string, templateID string, _ OptionalVMParameters) error {
	if name == "" {
		return newError(EBadArgument, "name cannot be empty for VM creation")
	}
	if clusterID == "" {
		return newError(EBadArgument, "cluster ID cannot be empty for VM creation")
	}
	if templateID == "" {
		return newError(EBadArgument, "template ID cannot be empty for VM creation")
	}
	return nil
}
