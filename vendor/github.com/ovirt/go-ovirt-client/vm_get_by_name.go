package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetVMByName(name string, retries ...RetryStrategy) (result VM, err error) {

	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting vm name %s", name),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().VmsService().List().Search("name=" + name).Send()
			if err != nil {
				return err
			}
			for _, sdkObject := range response.MustVms().Slice() {
				if mName, ok := sdkObject.Name(); ok {
					if name == mName {
						result, err = convertSDKVM(sdkObject, o)
						if err != nil {
							return wrap(
								err,
								EBug,
								"failed to convert vm %s",
								name,
							)
						}

					}
				}
			}
			return nil
		})
	return result, err
}

func (m *mockClient) GetVMByName(name string, _ ...RetryStrategy) (result VM, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, vm := range m.vms {
		if vm.name == name {
			return vm, nil
		}
	}
	return nil, newError(ENotFound, "vm with Name %s not found", name)
}
