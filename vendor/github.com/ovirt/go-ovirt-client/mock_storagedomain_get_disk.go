package ovirtclient

func (m *mockClient) GetDiskFromStorageDomain(id string, diskID string, _ ...RetryStrategy) (Disk, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if disk, ok := m.disks[diskID]; ok {
		for _, domain := range disk.storageDomainIDs {
			if domain == id {
				return disk, nil
			}
		}
		return nil, newError(ENotFound, "disk %s doesnt exist in storage domain %s", diskID, id)
	}
	return nil, newError(ENotFound, "disk %s doesnt exists", diskID)
}
