package ovirtclient

import (
	ovirtsdk "github.com/ovirt/go-ovirt"
)

// NICClient defines the methods related to dealing with network interfaces.
type NICClient interface {
	// CreateNIC adds a new NIC to a VM specified in vmid.
	CreateNIC(vmid string, name string, vnicProfileID string, retries ...RetryStrategy) (NIC, error)
	// GetNIC returns one specific NIC with the ID specified in id, attached to a VM with the ID specified in vmid.
	GetNIC(vmid string, id string, retries ...RetryStrategy) (NIC, error)
	// ListNICs lists all NICs attached to the VM specified in vmid.
	ListNICs(vmid string, retries ...RetryStrategy) ([]NIC, error)
	// RemoveNIC removes the network interface specified.
	RemoveNIC(vmid string, id string, retries ...RetryStrategy) error
}

// NIC represents a network interface.
type NIC interface {
	// ID is the identifier for this network interface.
	ID() string
	// Name is the user-given name of the network interface.
	Name() string
	// VMID is the identified of the VM this NIC is attached to. May be nil if the NIC is not attached.
	VMID() string
	// VNICProfileID returns the ID of the VNIC profile in use by the NIC.
	VNICProfileID() string

	// GetVM fetches an up to date copy of the virtual machine this NIC is attached to. This involves an API call and
	// may be slow.
	GetVM(retries ...RetryStrategy) (VM, error)
	// GetVNICProfile retrieves the VNIC profile associated with this NIC. This involves an API call and may be slow.
	GetVNICProfile(retries ...RetryStrategy) (VNICProfile, error)
	// Remove removes the current network interface. This involves an API call and may be slow.
	Remove(retries ...RetryStrategy) error
}

func convertSDKNIC(sdkObject *ovirtsdk.Nic, cli Client) (NIC, error) {
	id, ok := sdkObject.Id()
	if !ok {
		return nil, newFieldNotFound("id", "NIC")
	}
	name, ok := sdkObject.Name()
	if !ok {
		return nil, newFieldNotFound("name", "NIC")
	}
	vm, ok := sdkObject.Vm()
	if !ok {
		return nil, newFieldNotFound("vm", "NIC")
	}
	vmid, ok := vm.Id()
	if !ok {
		return nil, newFieldNotFound("VM in NIC", "ID")
	}
	vnicProfile, ok := sdkObject.VnicProfile()
	if !ok {
		return nil, newFieldNotFound("VM", "vNIC Profile")
	}
	vnicProfileID, ok := vnicProfile.Id()
	if !ok {
		return nil, newFieldNotFound("vNIC Profile on VM", "ID")
	}
	return &nic{
		cli,
		id,
		name,
		vmid,
		vnicProfileID,
	}, nil
}

type nic struct {
	client Client

	id            string
	name          string
	vmid          string
	vnicProfileID string
}

func (n nic) GetVM(retries ...RetryStrategy) (VM, error) {
	return n.client.GetVM(n.vmid, retries...)
}

func (n nic) GetVNICProfile(retries ...RetryStrategy) (VNICProfile, error) {
	return n.client.GetVNICProfile(n.vnicProfileID, retries...)
}

func (n nic) VNICProfileID() string {
	return n.vnicProfileID
}

func (n nic) ID() string {
	return n.id
}

func (n nic) Name() string {
	return n.name
}

func (n nic) VMID() string {
	return n.vmid
}

func (n nic) Remove(retries ...RetryStrategy) error {
	return n.client.RemoveNIC(n.vmid, n.id, retries...)
}
