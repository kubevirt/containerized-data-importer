// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListVMs(_ ...RetryStrategy) ([]VM, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]VM, len(m.vms))
	i := 0
	for _, item := range m.vms {
		result[i] = item
		i++
	}
	return result, nil
}
