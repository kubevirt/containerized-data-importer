package ovirtclient

func (m *mockClient) RemoveVM(id string, _ ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.vms[id]; !ok {
		return newError(ENotFound, "VM with ID %s not found", id)
	}
	for nicID, nic := range m.nics {
		if nic.VMID() == id {
			delete(m.nics, nicID)
		}
	}
	delete(m.vms, id)
	return nil
}
