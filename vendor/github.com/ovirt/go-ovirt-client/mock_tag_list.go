// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListTags(_ ...RetryStrategy) ([]Tag, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Tag, len(m.tags))
	i := 0
	for _, item := range m.tags {
		result[i] = item
		i++
	}
	return result, nil
}
