package ovirtclient

import (
	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

//go:generate go run scripts/rest.go -i "DataCenter" -n "datacenter" -o "Datacenter"

// DatacenterClient contains the functions related to handling datacenter objects in oVirt. Datacenters bind together
// resources of an environment (clusters, storage domains).
// See https://www.ovirt.org/documentation/administration_guide/#chap-Data_Centers for details.
type DatacenterClient interface {
	// GetDatacenter returns a single datacenter by its ID.
	GetDatacenter(id string, retries ...RetryStrategy) (Datacenter, error)
	// ListDatacenters lists all datacenters in the oVirt engine.
	ListDatacenters(retries ...RetryStrategy) ([]Datacenter, error)
	// ListDatacenterClusters lists all clusters in the specified datacenter.
	ListDatacenterClusters(id string, retries ...RetryStrategy) ([]Cluster, error)
}

// Datacenter is a logical entity that defines the set of resources used in a specific environment.
// See https://www.ovirt.org/documentation/administration_guide/#chap-Data_Centers for details.
type Datacenter interface {
	ID() string
	Name() string

	// Clusters lists the clusters for this datacenter. This is a network call and may be slow.
	Clusters(retries ...RetryStrategy) ([]Cluster, error)
	// HasCluster returns true if the cluster is in the datacenter. This is a network call and may be slow.
	HasCluster(clusterID string, retries ...RetryStrategy) (bool, error)
}

func convertSDKDatacenter(sdkObject *ovirtsdk4.DataCenter, client *oVirtClient) (Datacenter, error) {
	id, ok := sdkObject.Id()
	if !ok {
		return nil, newFieldNotFound("datacenter", "id")
	}
	name, ok := sdkObject.Name()
	if !ok {
		return nil, newFieldNotFound("datacenter", "name")
	}

	return &datacenter{
		client: client,
		id:     id,
		name:   name,
	}, nil
}

type datacenter struct {
	client Client

	id   string
	name string
}

func (d datacenter) Clusters(retries ...RetryStrategy) ([]Cluster, error) {
	return d.client.ListDatacenterClusters(d.id, retries...)
}

func (d datacenter) HasCluster(clusterID string, retries ...RetryStrategy) (bool, error) {
	clusters, err := d.client.ListDatacenterClusters(d.id, retries...)
	if err != nil {
		return false, err
	}
	for _, cluster := range clusters {
		if cluster.ID() == clusterID {
			return true, nil
		}
	}
	return false, nil
}

func (d datacenter) ID() string {
	return d.id
}

func (d datacenter) Name() string {
	return d.name
}
