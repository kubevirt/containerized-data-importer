package ovirtclient

func (m *mockClient) ListNICs(vmid string, _ ...RetryStrategy) ([]NIC, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	var result []NIC
	for _, item := range m.nics {
		if item.vmid == vmid {
			result = append(result, item)
		}
	}
	return result, nil
}
