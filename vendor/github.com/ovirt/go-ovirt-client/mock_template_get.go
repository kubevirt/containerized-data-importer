// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetTemplate(id string, _ ...RetryStrategy) (Template, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.templates[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "template with ID %s not found", id)
}
