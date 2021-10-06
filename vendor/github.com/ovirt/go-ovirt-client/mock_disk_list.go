// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListDisks(_ ...RetryStrategy) ([]Disk, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Disk, len(m.disks))
	i := 0
	for _, item := range m.disks {
		result[i] = item
		i++
	}
	return result, nil
}
