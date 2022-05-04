package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveVMFromAffinityGroup(
	clusterID ClusterID,
	vmID string,
	agID AffinityGroupID,
	retries ...RetryStrategy,
) error {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	return retry(
		fmt.Sprintf("adding VM %s to affinity group %s", vmID, agID),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.
				SystemService().
				ClustersService().
				ClusterService(string(clusterID)).
				AffinityGroupsService().
				GroupService(string(agID)).
				VmsService().
				VmService(vmID).
				Remove().
				Send()
			return err
		},
	)
}

func (m *mockClient) RemoveVMFromAffinityGroup(
	clusterID ClusterID,
	vmID string,
	agID AffinityGroupID,
	_ ...RetryStrategy,
) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	clusterAGs, ok := m.affinityGroups[clusterID]
	if !ok {
		return newError(ENotFound, "Cluster %s not found", clusterID)
	}

	ag, ok := clusterAGs[agID]
	if !ok {
		return newError(ENotFound, "Affinity group %s not found", agID)
	}
	for i, agVMID := range ag.vmids {
		if vmID == agVMID {
			ag.vmids = append(ag.vmids[0:i], ag.vmids[i+1:]...)
			return nil
		}
	}
	return newError(ENotFound, "VM %s is not in affinity group %s.", vmID, agID)
}
