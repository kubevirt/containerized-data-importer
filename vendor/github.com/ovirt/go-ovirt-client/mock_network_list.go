// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListNetworks(_ ...RetryStrategy) ([]Network, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Network, len(m.networks))
	i := 0
	for _, item := range m.networks {
		result[i] = item
		i++
	}
	return result, nil
}
