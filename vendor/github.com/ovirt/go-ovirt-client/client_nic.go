package ovirtclient

import (
	ovirtsdk "github.com/ovirt/go-ovirt"
)

// NICClient defines the methods related to dealing with network interfaces.
type NICClient interface {
	// CreateNIC adds a new NIC to a VM specified in vmid.
	CreateNIC(
		vmid string,
		vnicProfileID string,
		name string,
		optional OptionalNICParameters,
		retries ...RetryStrategy,
	) (NIC, error)
	// UpdateNIC allows updating the NIC.
	UpdateNIC(
		vmid string,
		nicID string,
		params UpdateNICParameters,
		retries ...RetryStrategy,
	) (NIC, error)
	// GetNIC returns one specific NIC with the ID specified in id, attached to a VM with the ID specified in vmid.
	GetNIC(vmid string, id string, retries ...RetryStrategy) (NIC, error)
	// ListNICs lists all NICs attached to the VM specified in vmid.
	ListNICs(vmid string, retries ...RetryStrategy) ([]NIC, error)
	// RemoveNIC removes the network interface specified.
	RemoveNIC(vmid string, id string, retries ...RetryStrategy) error
}

// OptionalNICParameters is an interface that declares the source of optional parameters for NIC creation.
type OptionalNICParameters interface{}

// BuildableNICParameters is a modifiable version of OptionalNICParameters. You can use CreateNICParams() to create a
// new copy, or implement your own.
type BuildableNICParameters interface {
	OptionalNICParameters
}

// CreateNICParams returns a buildable structure of OptionalNICParameters.
func CreateNICParams() BuildableNICParameters {
	return &nicParams{}
}

type nicParams struct{}

// UpdateNICParameters is an interface that declares methods of changeable parameters for NIC's. Each
// method can return nil to leave an attribute unchanged, or a new value for the attribute.
type UpdateNICParameters interface {
	// Name potentially returns a changed name for a NIC.
	Name() *string

	// VNICProfileID potentially returns a change VNIC profile for a NIC.
	VNICProfileID() *string
}

// BuildableUpdateNICParameters is a buildable version of UpdateNICParameters.
type BuildableUpdateNICParameters interface {
	UpdateNICParameters

	// WithName sets the name of a NIC for the UpdateNIC method.
	WithName(name string) (BuildableUpdateNICParameters, error)
	// MustWithName is identical to WithName, but panics instead of returning an error.
	MustWithName(name string) BuildableUpdateNICParameters

	// WithVNICProfileID sets the VNIC profile ID of a NIC for the UpdateNIC method.
	WithVNICProfileID(id string) (BuildableUpdateNICParameters, error)
	// MustWithVNICProfileID is identical to WithVNICProfileID, but panics instead of returning an error.
	MustWithVNICProfileID(id string) BuildableUpdateNICParameters
}

// UpdateNICParams creates a buildable UpdateNICParameters.
func UpdateNICParams() BuildableUpdateNICParameters {
	return &updateNICParams{}
}

type updateNICParams struct {
	name          *string
	vnicProfileID *string
}

func (u *updateNICParams) Name() *string {
	return u.name
}

func (u *updateNICParams) VNICProfileID() *string {
	return u.vnicProfileID
}

func (u *updateNICParams) WithName(name string) (BuildableUpdateNICParameters, error) {
	u.name = &name
	return u, nil
}

func (u *updateNICParams) MustWithName(name string) BuildableUpdateNICParameters {
	b, err := u.WithName(name)
	if err != nil {
		panic(err)
	}
	return b
}

func (u *updateNICParams) WithVNICProfileID(id string) (BuildableUpdateNICParameters, error) {
	u.vnicProfileID = &id
	return u, nil
}

func (u *updateNICParams) MustWithVNICProfileID(id string) BuildableUpdateNICParameters {
	b, err := u.WithVNICProfileID(id)
	if err != nil {
		panic(err)
	}
	return b
}

// NICData is the core of NIC which only provides data-access functions.
type NICData interface {
	// ID is the identifier for this network interface.
	ID() string
	// Name is the user-given name of the network interface.
	Name() string
	// VMID is the identified of the VM this NIC is attached to. May be nil if the NIC is not attached.
	VMID() string
	// VNICProfileID returns the ID of the VNIC profile in use by the NIC.
	VNICProfileID() string
}

// NIC represents a network interface.
type NIC interface {
	NICData

	// GetVM fetches an up to date copy of the virtual machine this NIC is attached to. This involves an API call and
	// may be slow.
	GetVM(retries ...RetryStrategy) (VM, error)
	// GetVNICProfile retrieves the VNIC profile associated with this NIC. This involves an API call and may be slow.
	GetVNICProfile(retries ...RetryStrategy) (VNICProfile, error)
	// Update updates the NIC with the specified parameters. It returns the updated NIC as a response. You can use
	// UpdateNICParams() to obtain a buildable parameter structure.
	Update(params UpdateNICParameters, retries ...RetryStrategy) (NIC, error)
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

func (n nic) Update(params UpdateNICParameters, retries ...RetryStrategy) (NIC, error) {
	return n.client.UpdateNIC(n.vmid, n.id, params, retries...)
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

func (n nic) withName(name string) *nic {
	return &nic{
		client:        n.client,
		id:            n.id,
		name:          name,
		vmid:          n.vmid,
		vnicProfileID: n.vnicProfileID,
	}
}

func (n nic) withVNICProfileID(vnicProfileID string) *nic {
	return &nic{
		client:        n.client,
		id:            n.id,
		name:          n.name,
		vmid:          n.vmid,
		vnicProfileID: vnicProfileID,
	}
}
