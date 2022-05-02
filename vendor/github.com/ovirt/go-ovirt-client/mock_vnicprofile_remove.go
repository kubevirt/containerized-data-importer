package ovirtclient

func (m *mockClient) RemoveVNICProfile(id string, _ ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.vnicProfiles[id]; !ok {
		return newError(ENotFound, "VNIC profile not found")
	}

	delete(m.vnicProfiles, id)

	return nil
}
