// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListHosts(_ ...RetryStrategy) ([]Host, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Host, len(m.hosts))
	i := 0
	for _, item := range m.hosts {
		result[i] = item
		i++
	}
	return result, nil
}
