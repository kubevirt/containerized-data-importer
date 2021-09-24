// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetHost(id string, _ ...RetryStrategy) (Host, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.hosts[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "host with ID %s not found", id)
}
