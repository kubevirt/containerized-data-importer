package ovirtclient

func (m *mockClient) RemoveNIC(vmid string, id string, _ ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.vms[vmid]; !ok {
		return newError(ENotFound, "NIC with ID %s not found", vmid)
	}
	if _, ok := m.nics[id]; !ok {
		return newError(ENotFound, "NIC with ID %s not found on VM with ID %s", id, vmid)
	}
	delete(m.nics, id)
	return nil
}
