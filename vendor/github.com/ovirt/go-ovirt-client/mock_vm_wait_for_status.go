package ovirtclient

import (
	"fmt"
)

func (m *mockClient) WaitForVMStatus(id string, status VMStatus, retries ...RetryStrategy) (vm VM, err error) {
	retries = defaultRetries(retries, defaultLongTimeouts())
	err = retry(
		fmt.Sprintf("waiting for VM %s status %s", id, status),
		m.logger,
		retries,
		func() error {
			vm, err = m.GetVM(id, retries...)
			if err != nil {
				return err
			}
			if vm.Status() != status {
				return newError(EPending, "VM status is %s, not %s", vm.Status(), status)
			}
			return nil
		})
	return
}
