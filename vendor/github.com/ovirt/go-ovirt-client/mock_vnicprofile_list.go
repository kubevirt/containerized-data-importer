// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListVNICProfiles(_ ...RetryStrategy) ([]VNICProfile, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]VNICProfile, len(m.vnicProfiles))
	i := 0
	for _, item := range m.vnicProfiles {
		result[i] = item
		i++
	}
	return result, nil
}
