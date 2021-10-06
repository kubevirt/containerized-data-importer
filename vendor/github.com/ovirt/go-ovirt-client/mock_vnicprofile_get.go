// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetVNICProfile(id string, _ ...RetryStrategy) (VNICProfile, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.vnicProfiles[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "VNIC profile with ID %s not found", id)
}
