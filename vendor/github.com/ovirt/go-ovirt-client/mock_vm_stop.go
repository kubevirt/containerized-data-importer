package ovirtclient

import (
	"net"
	"time"
)

func (m *mockClient) StopVM(id string, force bool, _ ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.vms[id]; ok {
		if (item.status == VMStatusSavingState || item.status == VMStatusRestoringState) && !force {
			return newError(EConflict, "VM is currently backing up or restoring.")
		}
		m.vmIPs[id] = map[string][]net.IP{}
		if item.status != VMStatusDown {
			item.status = VMStatusPoweringDown
			go func() {
				time.Sleep(2 * time.Second)
				m.lock.Lock()
				defer m.lock.Unlock()
				if item.status != VMStatusPoweringDown {
					return
				}
				item.status = VMStatusDown
				item.hostID = nil
			}()
		}
		return nil
	}
	return newError(ENotFound, "vm with ID %s not found", id)
}
