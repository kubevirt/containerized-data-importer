package ovirtclient

func (m *mockClient) RemoveDiskFromStorageDomain(id string, diskID string, _ ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.disks[diskID]; !ok {
		return newError(ENotFound, "disk with ID %s not found", diskID)
	}

	domains := m.disks[diskID].storageDomainIDs

	// if there is only 1 domain just delete the disk
	if len(domains) == 1 {
		delete(m.disks, diskID)
		return nil
	}

	// remove the storagedomain from the disk slice
	for i, sdomain := range domains {
		if sdomain == id {
			// gocritic will complain on the following line due to appendAssign, but that's legit here
			m.disks[diskID].storageDomainIDs = append(domains[:i], domains[i+1:]...) //nolint:gocritic
			return nil
		}
	}
	return newError(ENotFound, "disk %s is not found in StorageDomain %s", diskID, id)
}
