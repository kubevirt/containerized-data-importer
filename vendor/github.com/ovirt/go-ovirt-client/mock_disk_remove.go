package ovirtclient

func (m *mockClient) RemoveDisk(diskID string, retries ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.disks[diskID]; ok {
		delete(m.disks, diskID)
		return nil
	}
	return newError(ENotFound, "disk with ID %s not found", diskID)
}
