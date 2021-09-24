package ovirtclient

func (m *mockClient) GetNIC(vmid string, id string, _ ...RetryStrategy) (NIC, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if nic, ok := m.nics[id]; ok {
		if nic.vmid != vmid {
			return nil, newError(ENotFound, "nic with ID %s not found", id)
		}
		return nic, nil
	}
	return nil, newError(ENotFound, "nic with ID %s not found", id)
}
