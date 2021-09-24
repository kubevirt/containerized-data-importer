// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListStorageDomains(_ ...RetryStrategy) ([]StorageDomain, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]StorageDomain, len(m.storageDomains))
	i := 0
	for _, item := range m.storageDomains {
		result[i] = item
		i++
	}
	return result, nil
}
