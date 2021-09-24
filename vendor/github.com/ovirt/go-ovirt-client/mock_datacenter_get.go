// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetDatacenter(id string, _ ...RetryStrategy) (Datacenter, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.dataCenters[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "datacenter with ID %s not found", id)
}
