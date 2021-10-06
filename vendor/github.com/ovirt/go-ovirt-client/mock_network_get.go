// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetNetwork(id string, _ ...RetryStrategy) (Network, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.networks[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "network with ID %s not found", id)
}
