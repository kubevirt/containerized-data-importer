// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetDisk(id string, _ ...RetryStrategy) (Disk, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.disks[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "disk with ID %s not found", id)
}
