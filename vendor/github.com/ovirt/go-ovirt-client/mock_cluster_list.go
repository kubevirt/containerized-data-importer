// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) ListClusters(_ ...RetryStrategy) ([]Cluster, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	result := make([]Cluster, len(m.clusters))
	i := 0
	for _, item := range m.clusters {
		result[i] = item
		i++
	}
	return result, nil
}
