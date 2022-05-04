package ovirtclient

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

//go:generate go run scripts/rest/rest.go -i "Vm" -n "vm" -o "VM"

// VMClient includes the methods required to deal with virtual machines.
type VMClient interface {
	// CreateVM creates a virtual machine.
	CreateVM(clusterID ClusterID, templateID TemplateID, name string, optional OptionalVMParameters, retries ...RetryStrategy) (VM, error)
	// GetVM returns a single virtual machine based on an ID.
	GetVM(id string, retries ...RetryStrategy) (VM, error)
	// GetVMByName returns a single virtual machine based on a Name.
	GetVMByName(name string, retries ...RetryStrategy) (VM, error)
	// UpdateVM updates the virtual machine with the given parameters.
	// Use UpdateVMParams to obtain a builder for the params.
	UpdateVM(id string, params UpdateVMParameters, retries ...RetryStrategy) (VM, error)
	// AutoOptimizeVMCPUPinningSettings sets the CPU settings to optimized.
	AutoOptimizeVMCPUPinningSettings(id string, optimize bool, retries ...RetryStrategy) error
	// StartVM triggers a VM start. The actual VM startup will take time and should be waited for via the
	// WaitForVMStatus call.
	StartVM(id string, retries ...RetryStrategy) error
	// StopVM triggers a VM power-off. The actual VM stop will take time and should be waited for via the
	// WaitForVMStatus call. The force parameter will cause the shutdown to proceed even if a backup is currently
	// running.
	StopVM(id string, force bool, retries ...RetryStrategy) error
	// ShutdownVM triggers a VM shutdown. The actual VM shutdown will take time and should be waited for via the
	// WaitForVMStatus call. The force parameter will cause the shutdown to proceed even if a backup is currently
	// running.
	ShutdownVM(id string, force bool, retries ...RetryStrategy) error
	// WaitForVMStatus waits for the VM to reach the desired status.
	WaitForVMStatus(id string, status VMStatus, retries ...RetryStrategy) (VM, error)
	// ListVMs returns a list of all virtual machines.
	ListVMs(retries ...RetryStrategy) ([]VM, error)
	// SearchVMs lists all virtual machines matching a certain criteria specified in params.
	SearchVMs(params VMSearchParameters, retries ...RetryStrategy) ([]VM, error)
	// RemoveVM removes a virtual machine specified by id.
	RemoveVM(id string, retries ...RetryStrategy) error
	// AddTagToVM Add tag specified by id to a VM.
	AddTagToVM(id string, tagID string, retries ...RetryStrategy) error
	// AddTagToVMByName Add tag specified by Name to a VM.
	AddTagToVMByName(id string, tagName string, retries ...RetryStrategy) error
	// GetVMIPAddresses fetches the IP addresses reported by the guest agent in the VM.
	// Optional parameters can be passed to filter the result list.
	//
	// The returned result will be a map of network interface names and the list of IP addresses assigned to them,
	// excluding any IP addresses in the specified parameters.
	GetVMIPAddresses(id string, params VMIPSearchParams, retries ...RetryStrategy) (map[string][]net.IP, error)
	// WaitForVMIPAddresses waits for at least one IP address to be reported that is not in specified ranges.
	//
	// The returned result will be a map of network interface names and the list of IP addresses assigned to them,
	// excluding any IP addresses in the specified parameters.
	WaitForVMIPAddresses(id string, params VMIPSearchParams, retries ...RetryStrategy) (map[string][]net.IP, error)
	// WaitForNonLocalVMIPAddress waits for at least one IP address to be reported that is not in the following ranges:
	//
	// - 0.0.0.0/32
	// - 127.0.0.0/8
	// - 169.254.0.0/15
	// - 224.0.0.0/4
	// - 255.255.255.255/32
	// - ::/128
	// - ::1/128
	// - fe80::/64
	// - ff00::/8
	//
	// It also excludes the following interface names:
	//
	// - lo
	// - dummy*
	//
	// The returned result will be a map of network interface names and the list of non-local IP addresses assigned to
	// them.
	WaitForNonLocalVMIPAddress(id string, retries ...RetryStrategy) (map[string][]net.IP, error)
}

// VMIPSearchParams contains the parameters for searching or waiting for IP addresses on a VM.
type VMIPSearchParams interface {
	// GetIncludedRanges returns a list of network ranges that the returned IP address must match.
	GetIncludedRanges() []net.IPNet
	// GetExcludedRanges returns a list of IP ranges that should not be taken into consideration when returning IP
	// addresses.
	GetExcludedRanges() []net.IPNet
	// GetIncludedInterfaces returns a list of interface names of which the interface name must match at least
	// one.
	GetIncludedInterfaces() []string
	// GetExcludedInterfaces returns a list of interface names that should be excluded from the search.
	GetExcludedInterfaces() []string
	// GetIncludedInterfacePatterns returns a list of regular expressions of which at least one must match
	// the interface name.
	GetIncludedInterfacePatterns() []*regexp.Regexp
	// GetExcludedInterfacePatterns returns a list of regular expressions that match interface names needing to be
	// excluded from the IP address search.
	GetExcludedInterfacePatterns() []*regexp.Regexp
}

// BuildableVMIPSearchParams is a buildable version of VMIPSearchParams.
type BuildableVMIPSearchParams interface {
	VMIPSearchParams

	WithIncludedRange(ipRange net.IPNet) BuildableVMIPSearchParams
	WithExcludedRange(ipRange net.IPNet) BuildableVMIPSearchParams
	WithIncludedInterface(interfaceName string) BuildableVMIPSearchParams
	WithExcludedInterface(interfaceName string) BuildableVMIPSearchParams
	WithIncludedInterfacePattern(interfaceNamePattern *regexp.Regexp) BuildableVMIPSearchParams
	WithExcludedInterfacePattern(interfaceNamePattern *regexp.Regexp) BuildableVMIPSearchParams
}

// NewVMIPSearchParams returns a buildable parameter set for VM IP searches.
func NewVMIPSearchParams() BuildableVMIPSearchParams {
	return &vmIPSearchParams{}
}

type vmIPSearchParams struct {
	excludedRanges                []net.IPNet
	includedRanges                []net.IPNet
	excludedInterfaceNames        []string
	includedInterfaceNames        []string
	excludedInterfaceNamePatterns []*regexp.Regexp
	includedInterfaceNamePatterns []*regexp.Regexp
}

func (v *vmIPSearchParams) GetIncludedRanges() []net.IPNet {
	return v.includedRanges
}

func (v *vmIPSearchParams) GetIncludedInterfaces() []string {
	return v.includedInterfaceNames
}

func (v *vmIPSearchParams) GetIncludedInterfacePatterns() []*regexp.Regexp {
	return v.includedInterfaceNamePatterns
}

func (v *vmIPSearchParams) WithIncludedRange(ipRange net.IPNet) BuildableVMIPSearchParams {
	v.includedRanges = append(v.includedRanges, ipRange)
	return v
}

func (v *vmIPSearchParams) WithIncludedInterface(interfaceName string) BuildableVMIPSearchParams {
	v.includedInterfaceNames = append(v.includedInterfaceNames, interfaceName)
	return v
}

func (v *vmIPSearchParams) WithIncludedInterfacePattern(interfaceNamePattern *regexp.Regexp) BuildableVMIPSearchParams {
	v.includedInterfaceNamePatterns = append(v.includedInterfaceNamePatterns, interfaceNamePattern)
	return v
}

func (v *vmIPSearchParams) GetExcludedRanges() []net.IPNet {
	return v.excludedRanges
}

func (v *vmIPSearchParams) GetExcludedInterfaces() []string {
	return v.excludedInterfaceNames
}

func (v *vmIPSearchParams) GetExcludedInterfacePatterns() []*regexp.Regexp {
	return v.excludedInterfaceNamePatterns
}

func (v *vmIPSearchParams) WithExcludedRange(ipRange net.IPNet) BuildableVMIPSearchParams {
	v.excludedRanges = append(v.excludedRanges, ipRange)
	return v
}

func (v *vmIPSearchParams) WithExcludedInterface(interfaceName string) BuildableVMIPSearchParams {
	v.excludedInterfaceNames = append(v.excludedInterfaceNames, interfaceName)
	return v
}

func (v *vmIPSearchParams) WithExcludedInterfacePattern(interfaceNamePattern *regexp.Regexp) BuildableVMIPSearchParams {
	v.excludedInterfaceNamePatterns = append(v.excludedInterfaceNamePatterns, interfaceNamePattern)
	return v
}

// VMData is the core of VM providing only data access functions.
type VMData interface {
	// ID returns the unique identifier (UUID) of the current virtual machine.
	ID() string
	// Name is the user-defined name of the virtual machine.
	Name() string
	// Comment is the comment added to the VM.
	Comment() string
	// ClusterID returns the cluster this machine belongs to.
	ClusterID() ClusterID
	// TemplateID returns the ID of the base template for this machine.
	TemplateID() TemplateID
	// Status returns the current status of the VM.
	Status() VMStatus
	// CPU returns the CPU structure of a VM.
	CPU() VMCPU
	// Memory return the Memory of a VM in Bytes.
	Memory() int64
	// MemoryPolicy returns the memory policy set on the VM, if any. The second parameter returned is true if the
	// memory policy is set.
	MemoryPolicy() (MemoryPolicy, bool)
	// TagIDs returns a list of tags for this VM.
	TagIDs() []string
	// HugePages returns the hugepage settings for the VM, if any.
	HugePages() *VMHugePages
	// Initialization returns the virtual machine’s initialization configuration.
	Initialization() Initialization
	// HostID returns the ID of the host if available.
	HostID() *string
	// PlacementPolicy returns placement policy applied to this VM, if any. It may be nil if no placement policy is set.
	// The second returned value will be false if no placement policy exists.
	PlacementPolicy() (placementPolicy VMPlacementPolicy, ok bool)
}

// VMPlacementPolicy is the structure that holds the rules for VM migration to other hosts.
type VMPlacementPolicy interface {
	Affinity() *VMAffinity
	HostIDs() []string
}

// VMAffinity is the affinity used in the placement policy on determining if a VM can be migrated to a different host.
type VMAffinity string

const (
	// VMAffinityMigratable allows automatic and manual VM migrations to other hosts. This is the default.
	VMAffinityMigratable VMAffinity = "migratable"
	// VMAffinityPinned disallows migrating to other hosts.
	VMAffinityPinned VMAffinity = "pinned"
	// VMAffinityUserMigratable allows only manual migrations to different hosts by a user.
	VMAffinityUserMigratable VMAffinity = "user_migratable"
)

// Validate checks the VM affinity for a valid value.
func (v VMAffinity) Validate() error {
	switch v {
	case VMAffinityMigratable:
		return nil
	case VMAffinityPinned:
		return nil
	case VMAffinityUserMigratable:
		return nil
	default:
		return newError(EBadArgument, "invalud value for VMAffinity: %s", v)
	}
}

// VMAffinityValues returns a list of all valid VMAffinity values.
func VMAffinityValues() []VMAffinity {
	return []VMAffinity{
		VMAffinityMigratable,
		VMAffinityPinned,
		VMAffinityUserMigratable,
	}
}

// VMCPU is the CPU configuration of a VM.
type VMCPU interface {
	// Topo is the desired CPU topology for this VM.
	Topo() VMCPUTopo
}

type vmCPU struct {
	topo *vmCPUTopo
}

func (v vmCPU) Topo() VMCPUTopo {
	return v.topo
}

func (v *vmCPU) clone() *vmCPU {
	if v == nil {
		return nil
	}
	return &vmCPU{
		topo: v.topo.clone(),
	}
}

// VMHugePages is the hugepages setting of the VM in bytes.
type VMHugePages uint64

// Validate returns an error if the VM hugepages doesn't have a valid value.
func (h VMHugePages) Validate() error {
	for _, hugePages := range VMHugePagesValues() {
		if hugePages == h {
			return nil
		}
	}
	return newError(
		EBadArgument,
		"Invalid value for VM huge pages: %d must be one of: %s",
		h,
		strings.Join(VMHugePagesValues().Strings(), ", "),
	)
}

const (
	// VMHugePages2M represents the small value of supported huge pages setting which is 2048 Kib.
	VMHugePages2M VMHugePages = 2048
	// VMHugePages1G represents the large value of supported huge pages setting which is 1048576 Kib.
	VMHugePages1G VMHugePages = 1048576
)

// VMHugePagesList is a list of VMHugePages.
type VMHugePagesList []VMHugePages

// Strings creates a string list of the values.
func (l VMHugePagesList) Strings() []string {
	result := make([]string, len(l))
	for i, hugepage := range l {
		result[i] = fmt.Sprint(hugepage)
	}
	return result
}

// VMHugePagesValues returns all possible VMHugepages values.
func VMHugePagesValues() VMHugePagesList {
	return []VMHugePages{
		VMHugePages2M,
		VMHugePages1G,
	}
}

// Initialization defines to the virtual machine’s initialization configuration.
type Initialization interface {
	CustomScript() string
	HostName() string
}

// BuildableInitialization is a buildable version of Initialization.
type BuildableInitialization interface {
	Initialization
	WithCustomScript(customScript string) BuildableInitialization
	WithHostname(hostname string) BuildableInitialization
}

// initialization defines to the virtual machine’s initialization configuration.
// customScript - Cloud-init script which will be executed on Virtual Machine when deployed.
// hostname - Hostname to be set to Virtual Machine when deployed.
type initialization struct {
	customScript string
	hostname     string
}

// NewInitialization creates a new Initialization from the specified parameters.
func NewInitialization(customScript, hostname string) Initialization {
	return &initialization{
		customScript: customScript,
		hostname:     hostname,
	}
}

func (i *initialization) CustomScript() string {
	return i.customScript
}

func (i *initialization) HostName() string {
	return i.hostname
}

func (i *initialization) WithCustomScript(customScript string) BuildableInitialization {
	i.customScript = customScript
	return i
}

func (i *initialization) WithHostname(hostname string) BuildableInitialization {
	i.hostname = hostname
	return i
}

// convertSDKInitialization converts the initialization of a VM. We keep the error return in case we need it later
// as errors may happen as we extend this function and we don't want to touch other functions.
func convertSDKInitialization(sdkObject *ovirtsdk.Vm) (*initialization, error) { //nolint:unparam
	initializationSDK, ok := sdkObject.Initialization()
	if !ok {
		// This happens for some, but not all API calls if the initialization is not set.
		return &initialization{}, nil
	}

	init := initialization{}
	customScript, ok := initializationSDK.CustomScript()
	if ok {
		init.customScript = customScript
	}
	hostname, ok := initializationSDK.HostName()
	if ok {
		init.hostname = hostname
	}
	return &init, nil
}

// VM is the implementation of the virtual machine in oVirt.
type VM interface {
	VMData

	// Update updates the virtual machine with the given parameters. Use UpdateVMParams to
	// get a builder for the parameters.
	Update(params UpdateVMParameters, retries ...RetryStrategy) (VM, error)
	// Remove removes the current VM. This involves an API call and may be slow.
	Remove(retries ...RetryStrategy) error

	// Start will cause a VM to start. The actual start process takes some time and should be checked via WaitForStatus.
	Start(retries ...RetryStrategy) error
	// Stop will cause the VM to power-off. The force parameter will cause the VM to stop even if a backup is currently
	// running.
	Stop(force bool, retries ...RetryStrategy) error
	// Shutdown will cause the VM to shut down. The force parameter will cause the VM to shut down even if a backup
	// is currently running.
	Shutdown(force bool, retries ...RetryStrategy) error
	// WaitForStatus will wait until the VM reaches the desired status. If the status is not reached within the
	// specified amount of retries, an error will be returned. If the VM enters the desired state, an updated VM
	// object will be returned.
	WaitForStatus(status VMStatus, retries ...RetryStrategy) (VM, error)

	// CreateNIC creates a network interface on the current VM. This involves an API call and may be slow.
	CreateNIC(name string, vnicProfileID string, params OptionalNICParameters, retries ...RetryStrategy) (NIC, error)
	// GetNIC fetches a NIC with a specific ID on the current VM. This involves an API call and may be slow.
	GetNIC(id string, retries ...RetryStrategy) (NIC, error)
	// ListNICs fetches a list of network interfaces attached to this VM. This involves an API call and may be slow.
	ListNICs(retries ...RetryStrategy) ([]NIC, error)

	// AttachDisk attaches a disk to this VM.
	AttachDisk(
		diskID string,
		diskInterface DiskInterface,
		params CreateDiskAttachmentOptionalParams,
		retries ...RetryStrategy,
	) (DiskAttachment, error)
	// GetDiskAttachment returns a specific disk attachment for the current VM by ID.
	GetDiskAttachment(diskAttachmentID string, retries ...RetryStrategy) (DiskAttachment, error)
	// ListDiskAttachments lists all disk attachments for the current VM.
	ListDiskAttachments(retries ...RetryStrategy) ([]DiskAttachment, error)
	// DetachDisk removes a specific disk attachment by the disk attachment ID.
	DetachDisk(
		diskAttachmentID string,
		retries ...RetryStrategy,
	) error
	// Tags list all tags for the current VM
	Tags(retries ...RetryStrategy) ([]Tag, error)

	// GetHost retrieves the host object for the current VM. If the VM is not running, nil will be returned.
	GetHost(retries ...RetryStrategy) (Host, error)

	// GetIPAddresses fetches the IP addresses and returns a map of the interface name and list of IP addresses.
	//
	// The optional parameters let you filter the returned interfaces and IP addresses.
	GetIPAddresses(params VMIPSearchParams, retries ...RetryStrategy) (map[string][]net.IP, error)
	// WaitForIPAddresses waits for at least one IP address to be reported that is not in specified ranges.
	//
	// The returned result will be a map of network interface names and the list of IP addresses assigned to them,
	// excluding any IP addresses and interfaces in the specified parameters.
	WaitForIPAddresses(params VMIPSearchParams, retries ...RetryStrategy) (map[string][]net.IP, error)
	// WaitForNonLocalIPAddress waits for at least one IP address to be reported that is not in the following ranges:
	//
	// - 0.0.0.0/32
	// - 127.0.0.0/8
	// - 169.254.0.0/15
	// - 224.0.0.0/4
	// - 255.255.255.255/32
	// - ::/128
	// - ::1/128
	// - fe80::/64
	// - ff00::/8
	//
	// It also excludes the following interface names:
	//
	// - lo
	// - dummy*
	//
	// The returned result will be a map of network interface names and the list of non-local IP addresses assigned to
	// them.
	WaitForNonLocalIPAddress(retries ...RetryStrategy) (map[string][]net.IP, error)
}

// VMSearchParameters declares the parameters that can be passed to a VM search. Each parameter
// is declared as a pointer, where a nil value will mean that parameter will not be searched for.
// All parameters are used together as an AND filter.
type VMSearchParameters interface {
	// Name will match the name of the virtual machine exactly.
	Name() *string
	// Tag will match the tag of the virtual machine.
	Tag() *string
	// Statuses will return a list of acceptable statuses for this VM search.
	Statuses() *VMStatusList
	// NotStatuses will return a list of not acceptable statuses for this VM search.
	NotStatuses() *VMStatusList
}

// BuildableVMSearchParameters is a buildable version of VMSearchParameters.
type BuildableVMSearchParameters interface {
	VMSearchParameters

	// WithName sets the name to search for.
	WithName(name string) BuildableVMSearchParameters
	// WithTag sets the tag to search for.
	WithTag(name string) BuildableVMSearchParameters
	// WithStatus adds a single status to the filter.
	WithStatus(status VMStatus) BuildableVMSearchParameters
	// WithNotStatus excludes a VM status from the search.
	WithNotStatus(status VMStatus) BuildableVMSearchParameters
	// WithStatuses will return the statuses the returned VMs should be in.
	WithStatuses(list VMStatusList) BuildableVMSearchParameters
	// WithNotStatuses will return the statuses the returned VMs should not be in.
	WithNotStatuses(list VMStatusList) BuildableVMSearchParameters
}

// VMSearchParams creates a buildable set of search parameters for easier use.
func VMSearchParams() BuildableVMSearchParameters {
	return &vmSearchParams{
		lock: &sync.Mutex{},
	}
}

type vmSearchParams struct {
	lock *sync.Mutex

	name        *string
	tag         *string
	statuses    *VMStatusList
	notStatuses *VMStatusList
}

func (v *vmSearchParams) WithStatus(status VMStatus) BuildableVMSearchParameters {
	v.lock.Lock()
	defer v.lock.Unlock()
	newStatuses := append(*v.statuses, status)
	v.statuses = &newStatuses
	return v
}

func (v *vmSearchParams) WithNotStatus(status VMStatus) BuildableVMSearchParameters {
	v.lock.Lock()
	defer v.lock.Unlock()
	newNotStatuses := append(*v.notStatuses, status)
	v.statuses = &newNotStatuses
	return v
}

func (v *vmSearchParams) Tag() *string {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.tag
}

func (v *vmSearchParams) Name() *string {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.name
}

func (v *vmSearchParams) Statuses() *VMStatusList {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.statuses
}

func (v *vmSearchParams) NotStatuses() *VMStatusList {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.notStatuses
}

func (v *vmSearchParams) WithName(name string) BuildableVMSearchParameters {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.name = &name
	return v
}

func (v *vmSearchParams) WithTag(tag string) BuildableVMSearchParameters {
	v.lock.Lock()
	defer v.lock.Unlock()
	v.tag = &tag
	return v
}

func (v *vmSearchParams) WithStatuses(list VMStatusList) BuildableVMSearchParameters {
	v.lock.Lock()
	defer v.lock.Unlock()
	newStatuses := list.Copy()
	v.statuses = &newStatuses
	return v
}

func (v *vmSearchParams) WithNotStatuses(list VMStatusList) BuildableVMSearchParameters {
	v.lock.Lock()
	defer v.lock.Unlock()
	newNotStatuses := list.Copy()
	v.notStatuses = &newNotStatuses
	return v
}

// OptionalVMParameters are a list of parameters that can be, but must not necessarily be added on VM creation. This
// interface is expected to be extended in the future.
type OptionalVMParameters interface {
	// Comment returns the comment for the VM.
	Comment() string

	// CPU contains the CPU topology, if any.
	CPU() VMCPUTopo

	// HugePages returns the optional value for the HugePages setting for VMs.
	HugePages() *VMHugePages

	// Initialization defines the virtual machine’s initialization configuration.
	Initialization() Initialization

	// Clone should return true if the VM should be cloned from the template instead of linking it. This means that the
	// template can be removed while the VM still exists.
	Clone() *bool

	// Memory returns the VM memory in Bytes.
	Memory() *int64

	// MemoryPolicy returns the memory policy configuration for this VM, if any.
	MemoryPolicy() *MemoryPolicyParameters

	// Disks returns a list of disks that are to be changed from the template.
	Disks() []OptionalVMDiskParameters

	// PlacementPolicy returns a VM placement policy to apply, if any.
	PlacementPolicy() *VMPlacementPolicyParameters
}

// BuildableVMParameters is a variant of OptionalVMParameters that can be changed using the supplied
// builder functions. This is placed here for future use.
type BuildableVMParameters interface {
	OptionalVMParameters

	// WithComment adds a common to the VM.
	WithComment(comment string) (BuildableVMParameters, error)
	// MustWithComment is identical to WithComment, but panics instead of returning an error.
	MustWithComment(comment string) BuildableVMParameters

	// WithCPU adds a VMCPUTopo to the VM.
	WithCPU(cpu VMCPUTopo) (BuildableVMParameters, error)
	// MustWithCPU adds a VMCPUTopo and panics if an error happens.
	MustWithCPU(cpu VMCPUTopo) BuildableVMParameters
	// WithCPUParameters is a simplified function that calls NewVMCPUTopo and adds the CPU topology to
	// the VM.
	WithCPUParameters(cores, threads, sockets uint) (BuildableVMParameters, error)
	// MustWithCPUParameters is a simplified function that calls MustNewVMCPUTopo and adds the CPU topology to
	// the VM.
	MustWithCPUParameters(cores, threads, sockets uint) BuildableVMParameters

	// WithHugePages sets the HugePages setting for the VM.
	WithHugePages(hugePages VMHugePages) (BuildableVMParameters, error)
	// MustWithHugePages is identical to WithHugePages, but panics instead of returning an error.
	MustWithHugePages(hugePages VMHugePages) BuildableVMParameters
	// WithMemory sets the Memory setting for the VM.
	WithMemory(memory int64) (BuildableVMParameters, error)
	// MustWithMemory is identical to WithMemory, but panics instead of returning an error.
	MustWithMemory(memory int64) BuildableVMParameters
	// WithMemoryPolicy sets the memory policy parameters for the VM.
	WithMemoryPolicy(memory MemoryPolicyParameters) BuildableVMParameters

	// WithInitialization sets the virtual machine’s initialization configuration.
	WithInitialization(initialization Initialization) (BuildableVMParameters, error)
	// MustWithInitialization is identical to WithInitialization, but panics instead of returning an error.
	MustWithInitialization(initialization Initialization) BuildableVMParameters
	// MustWithInitializationParameters is a simplified function that calls MustNewInitialization and adds customScript
	MustWithInitializationParameters(customScript, hostname string) BuildableVMParameters

	// WithClone sets the clone flag. If the clone flag is true the VM is cloned from the template instead of linking to
	// it. This means the template can be deleted while the VM still exists.
	WithClone(clone bool) (BuildableVMParameters, error)
	// MustWithClone is identical to WithClone, but panics instead of returning an error.
	MustWithClone(clone bool) BuildableVMParameters

	// WithDisks adds disk configurations to the VM creation to manipulate the disks inherited from templates.
	WithDisks(disks []OptionalVMDiskParameters) (BuildableVMParameters, error)
	// MustWithDisks is identical to WithDisks, but panics instead of returning an error.
	MustWithDisks(disks []OptionalVMDiskParameters) BuildableVMParameters

	// WithPlacementPolicy adds a placement policy dictating which hosts the VM can be migrated to.
	WithPlacementPolicy(placementPolicy VMPlacementPolicyParameters) BuildableVMParameters
}

// VMPlacementPolicyParameters contains the optional parameters on VM placement.
type VMPlacementPolicyParameters interface {
	// Affinity dictates how a VM can be migrated to a different host. This can be nil, in which case the engine
	// default is to set the policy to migratable.
	Affinity() *VMAffinity
	// HostIDs returns a list of host IDs to apply as possible migration targets. The default is an empty list,
	// which means the VM can be migrated to any host.
	HostIDs() []string
}

// BuildableVMPlacementPolicyParameters is a buildable version of the VMPlacementPolicyParameters.
type BuildableVMPlacementPolicyParameters interface {
	VMPlacementPolicyParameters

	// WithAffinity sets the way VMs can be migrated to other hosts.
	WithAffinity(affinity VMAffinity) (BuildableVMPlacementPolicyParameters, error)
	// MustWithAffinity is identical to WithAffinity, but panics instead of returning an error.
	MustWithAffinity(affinity VMAffinity) BuildableVMPlacementPolicyParameters

	// WithHostIDs sets the list of hosts this VM can be migrated to.
	WithHostIDs(hostIDs []string) (BuildableVMPlacementPolicyParameters, error)
	// MustWithHostIDs is identical to WithHostIDs, but panics instead of returning an error.
	MustWithHostIDs(hostIDs []string) BuildableVMPlacementPolicyParameters
}

// NewVMPlacementPolicyParameters creates a new BuildableVMPlacementPolicyParameters for use on VM creation.
func NewVMPlacementPolicyParameters() BuildableVMPlacementPolicyParameters {
	return &vmPlacementPolicyParameters{}
}

type vmPlacementPolicyParameters struct {
	affinity *VMAffinity
	hostIDs  []string
}

func (v vmPlacementPolicyParameters) Affinity() *VMAffinity {
	return v.affinity
}

func (v vmPlacementPolicyParameters) HostIDs() []string {
	return v.hostIDs
}

func (v vmPlacementPolicyParameters) WithAffinity(affinity VMAffinity) (BuildableVMPlacementPolicyParameters, error) {
	if err := affinity.Validate(); err != nil {
		return nil, err
	}
	v.affinity = &affinity
	return v, nil
}

func (v vmPlacementPolicyParameters) MustWithAffinity(affinity VMAffinity) BuildableVMPlacementPolicyParameters {
	builder, err := v.WithAffinity(affinity)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v vmPlacementPolicyParameters) WithHostIDs(hostIDs []string) (BuildableVMPlacementPolicyParameters, error) {
	v.hostIDs = hostIDs
	return v, nil
}

func (v vmPlacementPolicyParameters) MustWithHostIDs(hostIDs []string) BuildableVMPlacementPolicyParameters {
	builder, err := v.WithHostIDs(hostIDs)
	if err != nil {
		panic(err)
	}
	return builder
}

// MemoryPolicyParameters contain the parameters for the memory policy setting on the VM.
type MemoryPolicyParameters interface {
	Guaranteed() *int64
}

// BuildableMemoryPolicyParameters is a buildable version of MemoryPolicyParameters.
type BuildableMemoryPolicyParameters interface {
	MemoryPolicyParameters

	WithGuaranteed(guaranteed int64) (BuildableMemoryPolicyParameters, error)
	MustWithGuaranteed(guaranteed int64) BuildableMemoryPolicyParameters
}

// NewMemoryPolicyParameters creates a new instance of BuildableMemoryPolicyParameters.
func NewMemoryPolicyParameters() BuildableMemoryPolicyParameters {
	return &memoryPolicyParameters{}
}

type memoryPolicyParameters struct {
	guaranteed *int64
}

func (m *memoryPolicyParameters) MustWithGuaranteed(guaranteed int64) BuildableMemoryPolicyParameters {
	builder, err := m.WithGuaranteed(guaranteed)
	if err != nil {
		panic(err)
	}
	return builder
}

func (m *memoryPolicyParameters) Guaranteed() *int64 {
	return m.guaranteed
}

func (m *memoryPolicyParameters) WithGuaranteed(guaranteed int64) (BuildableMemoryPolicyParameters, error) {
	m.guaranteed = &guaranteed
	return m, nil
}

// MemoryPolicy is the memory policy set on the VM.
type MemoryPolicy interface {
	// Guaranteed returns the number of guaranteed bytes to the VM.
	Guaranteed() *int64
}

type memoryPolicy struct {
	guaranteed *int64
}

func (m memoryPolicy) Guaranteed() *int64 {
	return m.guaranteed
}

// OptionalVMDiskParameters describes the disk parameters that can be given to VM creation. These manipulate the
// disks inherited from the template.
type OptionalVMDiskParameters interface {
	// DiskID returns the identifier of the disk that is being changed.
	DiskID() string
	// Sparse sets the sparse parameter if set. Note, that Sparse is only supported in oVirt on block devices with QCOW2
	// images. On NFS you MUST use raw disks to use sparse.
	Sparse() *bool
	// Format returns the image format to be used for the specified disk.
	Format() *ImageFormat
}

// BuildableVMDiskParameters is a buildable version of OptionalVMDiskParameters.
type BuildableVMDiskParameters interface {
	OptionalVMDiskParameters

	// WithSparse enables or disables sparse disk provisioning. Note, that Sparse is only supported in oVirt on block
	// devices with QCOW2 images. On NFS you MUST use raw images to use sparse. See WithFormat.
	WithSparse(sparse bool) (BuildableVMDiskParameters, error)
	// MustWithSparse is identical to WithSparse, but panics instead of returning an error.
	MustWithSparse(sparse bool) BuildableVMDiskParameters

	// WithFormat adds a disk format to the VM on creation. Note, that QCOW2 is only supported in conjunction with
	// Sparse on block devices. On NFS you MUST use raw images to use sparse. See WithSparse.
	WithFormat(format ImageFormat) (BuildableVMDiskParameters, error)
	// MustWithFormat is identical to WithFormat, but panics instead of returning an error.
	MustWithFormat(format ImageFormat) BuildableVMDiskParameters
}

// NewBuildableVMDiskParameters creates a new buildable OptionalVMDiskParameters.
func NewBuildableVMDiskParameters(diskID string) (BuildableVMDiskParameters, error) {
	return &vmDiskParameters{
		diskID,
		nil,
		nil,
	}, nil
}

// MustNewBuildableVMDiskParameters is identical to NewBuildableVMDiskParameters but panics instead of returning an
// error.
func MustNewBuildableVMDiskParameters(diskID string) BuildableVMDiskParameters {
	builder, err := NewBuildableVMDiskParameters(diskID)
	if err != nil {
		panic(err)
	}
	return builder
}

type vmDiskParameters struct {
	diskID string
	sparse *bool
	format *ImageFormat
}

func (v *vmDiskParameters) Format() *ImageFormat {
	return v.format
}

func (v *vmDiskParameters) WithFormat(format ImageFormat) (BuildableVMDiskParameters, error) {
	if err := format.Validate(); err != nil {
		return nil, err
	}
	v.format = &format
	return v, nil
}

func (v *vmDiskParameters) MustWithFormat(format ImageFormat) BuildableVMDiskParameters {
	builder, err := v.WithFormat(format)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmDiskParameters) DiskID() string {
	return v.diskID
}

func (v *vmDiskParameters) Sparse() *bool {
	return v.sparse
}

func (v *vmDiskParameters) WithSparse(sparse bool) (BuildableVMDiskParameters, error) {
	v.sparse = &sparse
	return v, nil
}

func (v *vmDiskParameters) MustWithSparse(sparse bool) BuildableVMDiskParameters {
	builder, err := v.WithSparse(sparse)
	if err != nil {
		panic(err)
	}
	return builder
}

// UpdateVMParameters returns a set of parameters to change on a VM.
type UpdateVMParameters interface {
	// Name returns the name for the VM. Return nil if the name should not be changed.
	Name() *string
	// Comment returns the comment for the VM. Return nil if the name should not be changed.
	Comment() *string
}

// VMCPUTopo contains the CPU topology information about a VM.
type VMCPUTopo interface {
	// Cores is the number of CPU cores.
	Cores() uint
	// Threads is the number of CPU threads in a core.
	Threads() uint
	// Sockets is the number of sockets.
	Sockets() uint
}

// NewVMCPUTopo creates a new VMCPUTopo from the specified parameters.
func NewVMCPUTopo(cores uint, threads uint, sockets uint) (VMCPUTopo, error) {
	if cores == 0 {
		return nil, newError(EBadArgument, "number of cores must be positive")
	}
	if threads == 0 {
		return nil, newError(EBadArgument, "number of threads must be positive")
	}
	if sockets == 0 {
		return nil, newError(EBadArgument, "number of sockets must be positive")
	}
	return &vmCPUTopo{
		cores:   cores,
		threads: threads,
		sockets: sockets,
	}, nil
}

// MustNewVMCPUTopo is equivalent to NewVMCPUTopo, but panics instead of returning an error.
func MustNewVMCPUTopo(cores uint, threads uint, sockets uint) VMCPUTopo {
	topo, err := NewVMCPUTopo(cores, threads, sockets)
	if err != nil {
		panic(err)
	}
	return topo
}

type vmCPUTopo struct {
	cores   uint
	threads uint
	sockets uint
}

func (v *vmCPUTopo) Cores() uint {
	return v.cores
}

func (v *vmCPUTopo) Threads() uint {
	return v.threads
}

func (v *vmCPUTopo) Sockets() uint {
	return v.sockets
}

func (v *vmCPUTopo) clone() *vmCPUTopo {
	if v == nil {
		return nil
	}
	return &vmCPUTopo{
		cores:   v.cores,
		threads: v.threads,
		sockets: v.sockets,
	}
}

// BuildableUpdateVMParameters is a buildable version of UpdateVMParameters.
type BuildableUpdateVMParameters interface {
	UpdateVMParameters

	// WithName adds an updated name to the request.
	WithName(name string) (BuildableUpdateVMParameters, error)

	// MustWithName is identical to WithName, but panics instead of returning an error
	MustWithName(name string) BuildableUpdateVMParameters

	// WithComment adds a comment to the request
	WithComment(comment string) (BuildableUpdateVMParameters, error)

	// MustWithComment is identical to WithComment, but panics instead of returning an error.
	MustWithComment(comment string) BuildableUpdateVMParameters
}

// UpdateVMParams returns a buildable set of update parameters.
func UpdateVMParams() BuildableUpdateVMParameters {
	return &updateVMParams{}
}

type updateVMParams struct {
	name    *string
	comment *string
}

func (u *updateVMParams) MustWithName(name string) BuildableUpdateVMParameters {
	builder, err := u.WithName(name)
	if err != nil {
		panic(err)
	}
	return builder
}

func (u *updateVMParams) MustWithComment(comment string) BuildableUpdateVMParameters {
	builder, err := u.WithComment(comment)
	if err != nil {
		panic(err)
	}
	return builder
}

func (u *updateVMParams) Name() *string {
	return u.name
}

func (u *updateVMParams) Comment() *string {
	return u.comment
}

func (u *updateVMParams) WithName(name string) (BuildableUpdateVMParameters, error) {
	if err := validateVMName(name); err != nil {
		return nil, err
	}
	u.name = &name
	return u, nil
}

func (u *updateVMParams) WithComment(comment string) (BuildableUpdateVMParameters, error) {
	u.comment = &comment
	return u, nil
}

// CreateVMParams creates a set of BuildableVMParameters that can be used to construct the optional VM parameters.
func CreateVMParams() BuildableVMParameters {
	return &vmParams{
		lock: &sync.Mutex{},
	}
}

type vmParams struct {
	lock *sync.Mutex

	name    string
	comment string
	cpu     VMCPUTopo

	hugePages *VMHugePages

	initialization Initialization
	memory         *int64
	memoryPolicy   *MemoryPolicyParameters

	clone *bool

	disks []OptionalVMDiskParameters

	placementPolicy *VMPlacementPolicyParameters
}

func (v *vmParams) WithPlacementPolicy(placementPolicy VMPlacementPolicyParameters) BuildableVMParameters {
	v.placementPolicy = &placementPolicy
	return v
}

func (v *vmParams) PlacementPolicy() *VMPlacementPolicyParameters {
	return v.placementPolicy
}

func (v *vmParams) MemoryPolicy() *MemoryPolicyParameters {
	return v.memoryPolicy
}

func (v *vmParams) WithMemoryPolicy(memory MemoryPolicyParameters) BuildableVMParameters {
	v.memoryPolicy = &memory
	return v
}

func (v *vmParams) Clone() *bool {
	return v.clone
}

func (v *vmParams) WithClone(clone bool) (BuildableVMParameters, error) {
	v.clone = &clone
	return v, nil
}

func (v *vmParams) MustWithClone(clone bool) BuildableVMParameters {
	builder, err := v.WithClone(clone)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) Disks() []OptionalVMDiskParameters {
	return v.disks
}

func (v *vmParams) WithDisks(disks []OptionalVMDiskParameters) (BuildableVMParameters, error) {
	diskIDs := map[string]int{}
	for i, d := range disks {
		if previousID, ok := diskIDs[d.DiskID()]; ok {
			return nil, newError(
				EBadArgument,
				"Disk %s appears twice, in position %d and %d.",
				d.DiskID(),
				previousID,
				i,
			)
		}
	}
	v.disks = disks
	return v, nil
}

func (v *vmParams) MustWithDisks(disks []OptionalVMDiskParameters) BuildableVMParameters {
	builder, err := v.WithDisks(disks)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) HugePages() *VMHugePages {
	return v.hugePages
}

func (v *vmParams) WithHugePages(hugePages VMHugePages) (BuildableVMParameters, error) {
	if err := hugePages.Validate(); err != nil {
		return v, err
	}
	v.hugePages = &hugePages
	return v, nil
}

func (v *vmParams) MustWithHugePages(hugePages VMHugePages) BuildableVMParameters {
	builder, err := v.WithHugePages(hugePages)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) Memory() *int64 {
	return v.memory
}

func (v *vmParams) WithMemory(memory int64) (BuildableVMParameters, error) {
	v.memory = &memory
	return v, nil
}

func (v *vmParams) MustWithMemory(memory int64) BuildableVMParameters {
	builder, err := v.WithMemory(memory)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) Initialization() Initialization {
	return v.initialization
}

func (v *vmParams) WithInitialization(initialization Initialization) (BuildableVMParameters, error) {
	v.initialization = initialization
	return v, nil
}

func (v *vmParams) MustWithInitialization(initialization Initialization) BuildableVMParameters {
	builder, err := v.WithInitialization(initialization)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) MustWithInitializationParameters(customScript, hostname string) BuildableVMParameters {
	init := NewInitialization(customScript, hostname)
	return v.MustWithInitialization(init)
}

func (v *vmParams) CPU() VMCPUTopo {
	return v.cpu
}

func (v *vmParams) WithCPU(cpu VMCPUTopo) (BuildableVMParameters, error) {
	v.cpu = cpu
	return v, nil
}

func (v *vmParams) MustWithCPU(cpu VMCPUTopo) BuildableVMParameters {
	builder, err := v.WithCPU(cpu)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) WithCPUParameters(cores, threads, sockets uint) (BuildableVMParameters, error) {
	cpu, err := NewVMCPUTopo(cores, threads, sockets)
	if err != nil {
		return nil, err
	}
	return v.WithCPU(cpu)
}

func (v *vmParams) MustWithCPUParameters(cores, threads, sockets uint) BuildableVMParameters {
	return v.MustWithCPU(MustNewVMCPUTopo(cores, threads, sockets))
}

func (v *vmParams) MustWithName(name string) BuildableVMParameters {
	builder, err := v.WithName(name)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) MustWithComment(comment string) BuildableVMParameters {
	builder, err := v.WithComment(comment)
	if err != nil {
		panic(err)
	}
	return builder
}

func (v *vmParams) WithName(name string) (BuildableVMParameters, error) {
	if err := validateVMName(name); err != nil {
		return nil, err
	}
	v.name = name
	return v, nil
}

func (v *vmParams) WithComment(comment string) (BuildableVMParameters, error) {
	v.comment = comment
	return v, nil
}

func (v vmParams) Name() string {
	return v.name
}

func (v vmParams) Comment() string {
	return v.comment
}

type vm struct {
	client Client

	id              string
	name            string
	comment         string
	clusterID       ClusterID
	templateID      TemplateID
	status          VMStatus
	cpu             *vmCPU
	memory          int64
	tagIDs          []string
	hugePages       *VMHugePages
	initialization  Initialization
	hostID          *string
	placementPolicy *vmPlacementPolicy
	memoryPolicy    *memoryPolicy
}

func (v *vm) PlacementPolicy() (VMPlacementPolicy, bool) {
	return v.placementPolicy, v.placementPolicy != nil
}

func (v *vm) MemoryPolicy() (MemoryPolicy, bool) {
	return v.memoryPolicy, v.memoryPolicy != nil
}

func (v *vm) WaitForIPAddresses(params VMIPSearchParams, retries ...RetryStrategy) (map[string][]net.IP, error) {
	return v.client.WaitForVMIPAddresses(v.id, params, retries...)
}

func (v *vm) WaitForNonLocalIPAddress(retries ...RetryStrategy) (map[string][]net.IP, error) {
	return v.client.WaitForNonLocalVMIPAddress(v.id, retries...)
}

func (v *vm) GetIPAddresses(params VMIPSearchParams, retries ...RetryStrategy) (map[string][]net.IP, error) {
	return v.client.GetVMIPAddresses(v.id, params, retries...)
}

func (v *vm) HostID() *string {
	return v.hostID
}

func (v *vm) GetHost(retries ...RetryStrategy) (Host, error) {
	hostID := v.hostID
	if hostID == nil {
		return nil, nil
	}
	return v.client.GetHost(*hostID, retries...)
}

func (v *vm) HugePages() *VMHugePages {
	return v.hugePages
}

func (v *vm) Start(retries ...RetryStrategy) error {
	return v.client.StartVM(v.id, retries...)
}

func (v *vm) Stop(force bool, retries ...RetryStrategy) error {
	return v.client.StopVM(v.id, force, retries...)
}

func (v *vm) Shutdown(force bool, retries ...RetryStrategy) error {
	return v.client.ShutdownVM(v.id, force, retries...)
}

func (v *vm) WaitForStatus(status VMStatus, retries ...RetryStrategy) (VM, error) {
	return v.client.WaitForVMStatus(v.id, status, retries...)
}

func (v *vm) CPU() VMCPU {
	return v.cpu
}

func (v *vm) Memory() int64 {
	return v.memory
}

func (v *vm) Initialization() Initialization {
	return v.initialization
}

// withName returns a copy of the VM with the new name. It does not change the original copy to avoid
// shared state issues.
func (v *vm) withName(name string) *vm {
	return &vm{
		v.client,
		v.id,
		name,
		v.comment,
		v.clusterID,
		v.templateID,
		v.status,
		v.cpu,
		v.memory,
		v.tagIDs,
		v.hugePages,
		v.initialization,
		v.hostID,
		v.placementPolicy,
		v.memoryPolicy,
	}
}

// withComment returns a copy of the VM with the new comment. It does not change the original copy to avoid
// shared state issues.
func (v *vm) withComment(comment string) *vm {
	return &vm{
		v.client,
		v.id,
		v.name,
		comment,
		v.clusterID,
		v.templateID,
		v.status,
		v.cpu,
		v.memory,
		v.tagIDs,
		v.hugePages,
		v.initialization,
		v.hostID,
		v.placementPolicy,
		v.memoryPolicy,
	}
}

func (v *vm) Update(params UpdateVMParameters, retries ...RetryStrategy) (VM, error) {
	return v.client.UpdateVM(v.id, params, retries...)
}

func (v *vm) Status() VMStatus {
	return v.status
}

func (v *vm) AttachDisk(
	diskID string,
	diskInterface DiskInterface,
	params CreateDiskAttachmentOptionalParams,
	retries ...RetryStrategy,
) (DiskAttachment, error) {
	return v.client.CreateDiskAttachment(v.id, diskID, diskInterface, params, retries...)
}

func (v *vm) GetDiskAttachment(diskAttachmentID string, retries ...RetryStrategy) (DiskAttachment, error) {
	return v.client.GetDiskAttachment(v.id, diskAttachmentID, retries...)
}

func (v *vm) ListDiskAttachments(retries ...RetryStrategy) ([]DiskAttachment, error) {
	return v.client.ListDiskAttachments(v.id, retries...)
}

func (v *vm) DetachDisk(diskAttachmentID string, retries ...RetryStrategy) error {
	return v.client.RemoveDiskAttachment(v.id, diskAttachmentID, retries...)
}

func (v *vm) Remove(retries ...RetryStrategy) error {
	return v.client.RemoveVM(v.id, retries...)
}

func (v *vm) CreateNIC(name string, vnicProfileID string, params OptionalNICParameters, retries ...RetryStrategy) (NIC, error) {
	return v.client.CreateNIC(v.id, vnicProfileID, name, params, retries...)
}

func (v *vm) GetNIC(id string, retries ...RetryStrategy) (NIC, error) {
	return v.client.GetNIC(v.id, id, retries...)
}

func (v *vm) ListNICs(retries ...RetryStrategy) ([]NIC, error) {
	return v.client.ListNICs(v.id, retries...)
}

func (v *vm) Comment() string {
	return v.comment
}

func (v *vm) ClusterID() ClusterID {
	return v.clusterID
}

func (v *vm) TemplateID() TemplateID {
	return v.templateID
}

func (v *vm) ID() string {
	return v.id
}

func (v *vm) Name() string {
	return v.name
}

func (v *vm) TagIDs() []string {
	return v.tagIDs
}

func (v *vm) Tags(retries ...RetryStrategy) ([]Tag, error) {
	tags := make([]Tag, len(v.tagIDs))
	for i, id := range v.tagIDs {
		tag, err := v.client.GetTag(id, retries...)
		if err != nil {
			return nil, err
		}
		tags[i] = tag
	}
	return tags, nil
}

func (v *vm) AddTagToVM(tagID string, retries ...RetryStrategy) error {
	return v.client.AddTagToVM(v.id, tagID, retries...)
}
func (v *vm) AddTagToVMByName(tagName string, retries ...RetryStrategy) error {
	return v.client.AddTagToVMByName(v.id, tagName, retries...)
}

var vmNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_\-.]*$`)

func validateVMName(name string) error {
	if !vmNameRegexp.MatchString(name) {
		return newError(EBadArgument, "invalid VM name: %s", name)
	}
	return nil
}

func convertSDKVM(sdkObject *ovirtsdk.Vm, client Client) (VM, error) {
	vmObject := &vm{
		client: client,
	}
	vmConverters := []func(sdkObject *ovirtsdk.Vm, vm *vm) error{
		vmIDConverter,
		vmNameConverter,
		vmCommentConverter,
		vmClusterConverter,
		vmStatusConverter,
		vmTemplateConverter,
		vmCPUConverter,
		vmHugePagesConverter,
		vmTagsConverter,
		vmInitializationConverter,
		vmPlacementPolicyConverter,
		vmHostConverter,
		vmMemoryConverter,
		vmMemoryPolicyConverter,
	}
	for _, converter := range vmConverters {
		if err := converter(sdkObject, vmObject); err != nil {
			return nil, err
		}
	}

	return vmObject, nil
}

func vmMemoryPolicyConverter(object *ovirtsdk.Vm, v *vm) error {
	if memPolicy, ok := object.MemoryPolicy(); ok {
		resultMemPolicy := &memoryPolicy{}
		if guaranteed, ok := memPolicy.Guaranteed(); ok {
			if guaranteed < -1 {
				return newError(
					EBug,
					"the engine returned a negative guaranteed memory value for VM %s (%d)",
					object.MustId(),
					guaranteed,
				)
			}
			resultMemPolicy.guaranteed = &guaranteed
		}
		v.memoryPolicy = resultMemPolicy
	}
	return nil
}

func vmHostConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	if host, ok := sdkObject.Host(); ok {
		if hostID, ok := host.Id(); ok && hostID != "" {
			v.hostID = &hostID
		}
	}
	return nil
}

func vmPlacementPolicyConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	if pp, ok := sdkObject.PlacementPolicy(); ok {
		placementPolicy := &vmPlacementPolicy{}
		affinity, ok := pp.Affinity()
		if ok {
			a := VMAffinity(affinity)
			placementPolicy.affinity = &a
		}
		hosts, ok := pp.Hosts()
		if ok {
			hostIDs := make([]string, len(hosts.Slice()))
			for i, host := range hosts.Slice() {
				hostIDs[i] = host.MustId()
			}
			placementPolicy.hostIDs = hostIDs
		}
		v.placementPolicy = placementPolicy
	}
	return nil
}

func vmIDConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	id, ok := sdkObject.Id()
	if !ok {
		return newError(EFieldMissing, "id field missing from VM object")
	}
	v.id = id
	return nil
}

func vmNameConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	name, ok := sdkObject.Name()
	if !ok {
		return newError(EFieldMissing, "name field missing from VM object")
	}
	v.name = name
	return nil
}

func vmCommentConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	comment, ok := sdkObject.Comment()
	if !ok {
		return newError(EFieldMissing, "comment field missing from VM object")
	}
	v.comment = comment
	return nil
}

func vmClusterConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	cluster, ok := sdkObject.Cluster()
	if !ok {
		return newError(EFieldMissing, "cluster field missing from VM object")
	}
	clusterID, ok := cluster.Id()
	if !ok {
		return newError(EFieldMissing, "ID field missing from cluster in VM object")
	}
	v.clusterID = ClusterID(clusterID)
	return nil
}

func vmStatusConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	status, ok := sdkObject.Status()
	if !ok {
		return newFieldNotFound("vm", "status")
	}
	v.status = VMStatus(status)
	return nil
}

func vmTemplateConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	template, ok := sdkObject.Template()
	if !ok {
		return newFieldNotFound("VM", "template")
	}
	templateID, ok := template.Id()
	if !ok {
		return newFieldNotFound("template in VM", "template ID")
	}
	v.templateID = TemplateID(templateID)
	return nil
}

func vmCPUConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	cpu, err := convertSDKVMCPU(sdkObject)
	if err != nil {
		return err
	}
	v.cpu = cpu
	return nil
}

func vmHugePagesConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	hugePages, err := hugePagesFromSDKVM(sdkObject)
	if err != nil {
		return err
	}
	v.hugePages = hugePages
	return nil
}

func vmMemoryConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	memory, ok := sdkObject.Memory()
	if !ok {
		return newFieldNotFound("vm", "memory")
	}
	v.memory = memory
	return nil
}

func vmInitializationConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	var vmInitialization *initialization
	vmInitialization, err := convertSDKInitialization(sdkObject)
	if err != nil {
		return err
	}
	v.initialization = vmInitialization
	return nil
}

func vmTagsConverter(sdkObject *ovirtsdk.Vm, v *vm) error {
	var tagIDs []string
	if sdkTags, ok := sdkObject.Tags(); ok {
		for _, tag := range sdkTags.Slice() {
			tagID, _ := tag.Id()
			tagIDs = append(tagIDs, tagID)
		}
	}
	v.tagIDs = tagIDs
	return nil
}

func convertSDKVMCPU(sdkObject *ovirtsdk.Vm) (*vmCPU, error) {
	sdkCPU, ok := sdkObject.Cpu()
	if !ok {
		return nil, newFieldNotFound("VM", "CPU")
	}
	cpuTopo, ok := sdkCPU.Topology()
	if !ok {
		return nil, newFieldNotFound("CPU in VM", "CPU topo")
	}
	cores, ok := cpuTopo.Cores()
	if !ok {
		return nil, newFieldNotFound("CPU topo in CPU in VM", "cores")
	}
	threads, ok := cpuTopo.Threads()
	if !ok {
		return nil, newFieldNotFound("CPU topo in CPU in VM", "threads")
	}
	sockets, ok := cpuTopo.Sockets()
	if !ok {
		return nil, newFieldNotFound("CPU topo in CPU in VM", "sockets")
	}
	cpu := &vmCPU{
		topo: &vmCPUTopo{
			uint(cores),
			uint(threads),
			uint(sockets),
		},
	}
	return cpu, nil
}

// VMStatus represents the status of a VM.
type VMStatus string

const (
	// VMStatusDown indicates that the VM is not running.
	VMStatusDown VMStatus = "down"
	// VMStatusImageLocked indicates that the virtual machine process is not running and there is some operation on the
	// disks of the virtual machine that prevents it from being started.
	VMStatusImageLocked VMStatus = "image_locked"
	// VMStatusMigrating indicates that the virtual machine process is running and the virtual machine is being migrated
	// from one host to another.
	VMStatusMigrating VMStatus = "migrating"
	// VMStatusNotResponding indicates that the hypervisor detected that the virtual machine is not responding.
	VMStatusNotResponding VMStatus = "not_responding"
	// VMStatusPaused indicates that the virtual machine process is running and the virtual machine is paused.
	// This may happen in two cases: when running a virtual machine is paused mode and when the virtual machine is being
	// automatically paused due to an error.
	VMStatusPaused VMStatus = "paused"
	// VMStatusPoweringDown indicates that the virtual machine process is running and it is about to stop running.
	VMStatusPoweringDown VMStatus = "powering_down"
	// VMStatusPoweringUp  indicates that the virtual machine process is running and the guest operating system is being
	// loaded. Note that if no guest-agent is installed, this status is set for a predefined period of time, that is by
	// default 60 seconds, when running a virtual machine.
	VMStatusPoweringUp VMStatus = "powering_up"
	// VMStatusRebooting indicates that the virtual machine process is running and the guest operating system is being
	// rebooted.
	VMStatusRebooting VMStatus = "reboot_in_progress"
	// VMStatusRestoringState indicates that the virtual machine process is about to run and the virtual machine is
	// going to awake from hibernation. In this status, the running state of the virtual machine is being restored.
	VMStatusRestoringState VMStatus = "restoring_state"
	// VMStatusSavingState indicates that the virtual machine process is running and the virtual machine is being
	// hibernated. In this status, the running state of the virtual machine is being saved. Note that this status does
	// not mean that the guest operating system is being hibernated.
	VMStatusSavingState VMStatus = "saving_state"
	// VMStatusSuspended indicates that the virtual machine process is not running and a running state of the virtual
	// machine was saved. This status is similar to Down, but when the VM is started in this status its saved running
	// state is restored instead of being booted using the normal procedure.
	VMStatusSuspended VMStatus = "suspended"
	// VMStatusUnassigned means an invalid status was received.
	VMStatusUnassigned VMStatus = "unassigned"
	// VMStatusUnknown indicates that the system failed to determine the status of the virtual machine.
	// The virtual machine process may be running or not running in this status.
	// For instance, when host becomes non-responsive the virtual machines that ran on it are set with this status.
	VMStatusUnknown VMStatus = "unknown"
	// VMStatusUp indicates that the virtual machine process is running and the guest operating system is loaded.
	// Note that if no guest-agent is installed, this status is set after a predefined period of time, that is by
	// default 60 seconds, when running a virtual machine.
	VMStatusUp VMStatus = "up"
	// VMStatusWaitForLaunch indicates that the virtual machine process is about to run.
	// This status is set when a request to run a virtual machine arrives to the host.
	// It is possible that the virtual machine process will fail to run.
	VMStatusWaitForLaunch VMStatus = "wait_for_launch"
)

type vmPlacementPolicy struct {
	affinity *VMAffinity
	hostIDs  []string
}

func (v vmPlacementPolicy) Affinity() *VMAffinity {
	return v.affinity
}

func (v vmPlacementPolicy) HostIDs() []string {
	return v.hostIDs
}

// Validate validates if a VMStatus has a valid value.
func (s VMStatus) Validate() error {
	for _, v := range VMStatusValues() {
		if v == s {
			return nil
		}
	}
	return newError(EBadArgument, "invalid value for VM status: %s", s)
}

// VMStatusList is a list of VMStatus.
type VMStatusList []VMStatus

// Copy creates a separate copy of the current status list.
func (l VMStatusList) Copy() VMStatusList {
	result := make([]VMStatus, len(l))
	for i, s := range l {
		result[i] = s
	}
	return result
}

// Validate validates the list of statuses.
func (l VMStatusList) Validate() error {
	for _, s := range l {
		if err := s.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// VMStatusValues returns all possible VMStatus values.
func VMStatusValues() VMStatusList {
	return []VMStatus{
		VMStatusDown,
		VMStatusImageLocked,
		VMStatusMigrating,
		VMStatusNotResponding,
		VMStatusPaused,
		VMStatusPoweringDown,
		VMStatusPoweringUp,
		VMStatusRebooting,
		VMStatusRestoringState,
		VMStatusSavingState,
		VMStatusSuspended,
		VMStatusUnassigned,
		VMStatusUnknown,
		VMStatusUp,
		VMStatusWaitForLaunch,
	}
}

// Strings creates a string list of the values.
func (l VMStatusList) Strings() []string {
	result := make([]string, len(l))
	for i, status := range l {
		result[i] = string(status)
	}
	return result
}

func hugePagesFromSDKVM(vm *ovirtsdk.Vm) (*VMHugePages, error) {
	var hugePagesText string
	customProperties, ok := vm.CustomProperties()
	if !ok {
		return nil, nil
	}
	for _, c := range customProperties.Slice() {
		customPropertyName, ok := c.Name()
		if !ok {
			return nil, nil
		}
		if customPropertyName == "hugepages" {
			hugePagesText, ok = c.Value()
			if !ok {
				return nil, nil
			}
			break
		}
	}
	hugepagesUint, err := strconv.ParseUint(hugePagesText, 10, 64)
	if err != nil {
		return nil, wrap(err, EBug, "Failed to parse 'hugepages' custom property into a number: %s", hugePagesText)
	}
	hugepages := VMHugePages(hugepagesUint)
	return &hugepages, nil
}
