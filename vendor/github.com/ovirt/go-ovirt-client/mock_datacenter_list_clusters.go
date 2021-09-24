package ovirtclient

func (m *mockClient) ListDatacenterClusters(id string, _ ...RetryStrategy) ([]Cluster, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	dc, ok := m.dataCenters[id]
	if !ok {
		return nil, newError(ENotFound, "datacenter with ID %s not found", id)
	}
	clusters := make([]Cluster, len(dc.clusters))
	for i, clusterID := range dc.clusters {
		clusters[i] = m.clusters[clusterID]
	}

	return clusters, nil
}
