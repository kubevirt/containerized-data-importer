// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListTemplates(_ ...RetryStrategy) ([]Template, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Template, len(m.templates))
	i := 0
	for _, item := range m.templates {
		result[i] = item
		i++
	}
	return result, nil
}
