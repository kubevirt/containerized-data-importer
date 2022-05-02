// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetTag(id string, _ ...RetryStrategy) (Tag, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.tags[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "tag with ID %s not found", id)
}
