package ovirtclient

import (
	"github.com/google/uuid"
)

func (m *mockClient) CreateNIC(
	vmid string,
	vnicProfileID string,
	name string,
	_ OptionalNICParameters,
	_ ...RetryStrategy,
) (NIC, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if err := validateNICCreationParameters(vmid, name); err != nil {
		return nil, err
	}
	if _, ok := m.vms[vmid]; !ok {
		return nil, newError(ENotFound, "VM with ID %s not found for NIC creation", vmid)
	}
	for _, n := range m.nics {
		if n.name == name {
			return nil, newError(ENotFound, "NIC with name %s is already in use", name)
		}
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
