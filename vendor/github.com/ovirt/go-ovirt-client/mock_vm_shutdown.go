package ovirtclient

import (
	"time"
)

func (m *mockClient) ShutdownVM(id string, force bool, _ ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.vms[id]; ok {
		if (item.status == VMStatusSavingState || item.status == VMStatusRestoringState) && !force {
			return newError(EConflict, "VM is currently backing up or restoring.")
		}
		if item.status != VMStatusDown {
			item.status = VMStatusPoweringDown
			go func() {
				time.Sleep(2 * time.Second)
				m.lock.Lock()
				defer m.lock.Unlock()
				item.status = VMStatusDown
			}()
		}
		return nil
	}
	return newError(ENotFound, "vm with ID %s not found", id)
}
