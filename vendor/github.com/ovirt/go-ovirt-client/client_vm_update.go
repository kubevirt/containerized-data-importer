package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) UpdateVM(
	id string,
	params UpdateVMParameters,
	retries ...RetryStrategy,
) (result VM, err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())

	vm := &ovirtsdk.Vm{}
	vm.SetId(id)
	if name := params.Name(); name != nil {
		if *name == "" {
			return nil, newError(EBadArgument, "name must not be empty for VM update")
		}
		vm.SetName(*name)
	}
	if comment := params.Comment(); comment != nil {
		vm.SetComment(*comment)
	}

	err = retry(
		fmt.Sprintf("updating vm %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().VmsService().VmService(id).Update().Vm(vm).Send()
			if err != nil {
				return wrap(err, EUnidentified, "failed to update VM")
			}
			vm, ok := response.Vm()
			if !ok {
				return newError(EFieldMissing, "missing VM in VM update response")
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
		})
	return result, err
}
