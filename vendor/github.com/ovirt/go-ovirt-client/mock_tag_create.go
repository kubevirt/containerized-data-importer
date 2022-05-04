package ovirtclient

import "github.com/google/uuid"

func (m *mockClient) CreateTag(name string, description string, _ ...RetryStrategy) (result Tag, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	id := uuid.Must(uuid.NewUUID()).String()
	tag := &tag{
		client:      m,
		id:          id,
		name:        name,
		description: description,
	}
	m.tags[id] = tag

	result = tag
	return
}
