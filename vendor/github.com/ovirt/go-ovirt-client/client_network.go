package ovirtclient

import (
	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

//go:generate go run scripts/rest/rest.go -i "Network" -n "network"

// NetworkClient describes the functions related to oVirt networks.
//
// See https://www.ovirt.org/documentation/administration_guide/#chap-Logical_Networks for details.
type NetworkClient interface {
	// GetNetwork returns a single network based on its ID.
	GetNetwork(id string, retries ...RetryStrategy) (Network, error)
	// ListNetworks returns all networks on the oVirt engine.
	ListNetworks(retries ...RetryStrategy) ([]Network, error)
}

// NetworkData is the core of Network, providing only the data access functions, but not the client
// functions.
type NetworkData interface {
	// ID returns the auto-generated identifier for this network.
	ID() string
	// Name returns the user-give nname for this network.
	Name() string
	// DatacenterID is the identifier of the datacenter object.
	DatacenterID() string
}

// Network is the interface defining the fields for networks.
type Network interface {
	NetworkData

	// Datacenter fetches the datacenter associated with this network. This is a network call and may be slow.
	Datacenter(retries ...RetryStrategy) (Datacenter, error)
}

func convertSDKNetwork(sdkObject *ovirtsdk4.Network, client *oVirtClient) (Network, error) {
	id, ok := sdkObject.Id()
	if !ok {
		return nil, newFieldNotFound("network", "id")
	}
	name, ok := sdkObject.Name()
	if !ok {
		return nil, newFieldNotFound("network", name)
	}
	dc, ok := sdkObject.DataCenter()
	if !ok {
		return nil, newFieldNotFound("network", "datacenter")
	}
	dcID, ok := dc.Id()
	if !ok {
		return nil, newFieldNotFound("datacenter on network", "ID")
	}
	return &network{
		client: client,
		id:     id,
		name:   name,
		dcID:   dcID,
	}, nil
}

type network struct {
	client Client

	id   string
	name string
	dcID string
}

func (n network) ID() string {
	return n.id
}

func (n network) Name() string {
	return n.name
}

func (n network) DatacenterID() string {
	return n.dcID
}

func (n network) Datacenter(retries ...RetryStrategy) (Datacenter, error) {
	return n.client.GetDatacenter(n.dcID, retries...)
}
