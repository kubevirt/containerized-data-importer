// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

func (m *mockClient) GetCluster(id ClusterID, _ ...RetryStrategy) (Cluster, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if item, ok := m.clusters[id]; ok {
		return item, nil
	}
	return nil, newError(ENotFound, "cluster with ID %s not found", id)
}
