package ovirtclient

import (
	"github.com/google/uuid"
)

func (m *mockClient) CreateNIC(vmid string, name string, vnicProfileID string, _ ...RetryStrategy) (NIC, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if err := validateNICCreationParameters(vmid, name); err != nil {
		return nil, err
	}
	if _, ok := m.vms[vmid]; !ok {
		return nil, newError(ENotFound, "VM with ID %s not found for NIC creation", vmid)
	}

	id := uuid.Must(uuid.NewUUID()).String()

	nic := &nic{
		client:        m,
		id:            id,
		name:          name,
		vmid:          vmid,
		vnicProfileID: vnicProfileID,
	}
	m.nics[id] = nic

	return nic, nil
}
