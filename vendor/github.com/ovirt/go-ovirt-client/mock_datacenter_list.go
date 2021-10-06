// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListDatacenters(_ ...RetryStrategy) ([]Datacenter, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Datacenter, len(m.dataCenters))
	i := 0
	for _, item := range m.dataCenters {
		result[i] = item
		i++
	}
	return result, nil
}
