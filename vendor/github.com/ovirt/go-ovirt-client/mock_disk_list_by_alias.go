package ovirtclient

func (m *mockClient) ListDisksByAlias(alias string, _ ...RetryStrategy) ([]Disk, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Disk, 0)
	for _, d := range m.disks {
		if d.alias == alias {
			result = append(result, d)
		}
	}
	return result, nil
}
