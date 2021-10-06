package ovirtclient

import (
	ovirtsdk "github.com/ovirt/go-ovirt"
)

//go:generate go run scripts/rest.go -i "VnicProfile" -n "VNIC profile" -o "VNICProfile" -s "Profile"

// VNICProfileClient defines the methods related to dealing with virtual NIC profiles.
type VNICProfileClient interface {
	// GetVNICProfile returns a single VNIC Profile based on the ID
	GetVNICProfile(id string, retries ...RetryStrategy) (VNICProfile, error)
	// ListVNICProfiles lists all VNIC Profiles.
	ListVNICProfiles(retries ...RetryStrategy) ([]VNICProfile, error)
}

// VNICProfile is a collection of settings that can be applied to individual virtual network interface cards in the
// Engine.
type VNICProfile interface {
	// ID returns the identifier of the VNICProfile.
	ID() string
	// Name returns the human-readable name of the VNIC profile.
	Name() string
	// NetworkID returns the network ID the VNICProfile is attached to.
	NetworkID() string

	// Network fetches the network object from the oVirt engine. This is an API call and may be slow.
	Network(retries ...RetryStrategy) (Network, error)
}

func convertSDKVNICProfile(sdkObject *ovirtsdk.VnicProfile, client Client) (VNICProfile, error) {
	id, ok := sdkObject.Id()
	if !ok {
		return nil, newFieldNotFound("VNICProfile", "ID")
	}
	name, ok := sdkObject.Name()
	if !ok {
		return nil, newFieldNotFound("VNICProfile", "name")
	}
	network, ok := sdkObject.Network()
	if !ok {
		return nil, newFieldNotFound("VNICProfile", "network")
	}
	networkID, ok := network.Id()
	if !ok {
		return nil, newFieldNotFound("Network on VNICProfile", "ID")
	}

	return &vnicProfile{
		client: client,

		id:        id,
		name:      name,
		networkID: networkID,
	}, nil
}

type vnicProfile struct {
	id        string
	client    Client
	networkID string
	name      string
}

func (v vnicProfile) Network(retries ...RetryStrategy) (Network, error) {
	return v.client.GetNetwork(v.networkID, retries...)
}

func (v vnicProfile) Name() string {
	return v.name
}

func (v vnicProfile) NetworkID() string {
	return v.networkID
}

func (v vnicProfile) ID() string {
	return v.id
}
