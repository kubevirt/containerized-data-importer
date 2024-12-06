//go:build amd64
// +build amd64

/*
Copyright 2020 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package importer

import (
	"bufio"
	"bytes"
	"container/ring"
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/sys/unix"
	libnbd "libguestfs.org/libnbd"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-importer"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

// May be overridden in tests
var newVddkDataSource = createVddkDataSource
var newVddkDataSink = createVddkDataSink
var newVMwareClient = createVMwareClient
var newNbdKitWrapper = createNbdKitWrapper
var newNbdKitLogWatcher = createNbdKitLogWatcher

/* Section: nbdkit */

const (
	nbdUnixSocket = "/tmp/nbd.sock"
	nbdPidFile    = "/tmp/nbd.pid"
	maxLogLines   = 1000
)

var vddkVersion string
var vddkHost string

// NbdKitWrapper keeps track of one nbdkit process
type NbdKitWrapper struct {
	n      image.NbdkitOperation
	Socket *url.URL
	Handle NbdOperations
}

// NbdKitLogWatcherVddk implements VDDK-specific nbdkit log handling
type NbdKitLogWatcherVddk struct {
	stopChannel chan struct{}
	output      *bufio.Reader
}

// createNbdKitWrapper starts nbdkit and returns a process handle for further management
func createNbdKitWrapper(vmware *VMwareClient, diskFileName, snapshot string) (*NbdKitWrapper, error) {
	args := image.NbdKitVddkPluginArgs{
		Server:     vmware.url.Host,
		Username:   vmware.username,
		Password:   vmware.password,
		Thumbprint: vmware.thumbprint,
		Moref:      vmware.moref,
		Snapshot:   snapshot,
	}
	n, err := image.NewNbdkitVddk(nbdPidFile, nbdUnixSocket, args)
	if err != nil {
		klog.Errorf("Error validating nbdkit plugins: %v", err)
		return nil, err
	}
	watcher := newNbdKitLogWatcher()
	n.(*image.Nbdkit).LogWatcher = watcher
	err = n.StartNbdkit(diskFileName)
	if err != nil {
		klog.Errorf("Unable to start nbdkit: %v", err)
		return nil, err
	}

	handle, err := libnbd.Create()
	if err != nil {
		klog.Errorf("Unable to create libnbd handle: %v", err)
		errKillNbdkit := n.KillNbdkit()
		if errKillNbdkit != nil {
			klog.Errorf("can't kill Nbdkit; %v", err)
		}
		return nil, err
	}

	err = handle.AddMetaContext("base:allocation")
	if err != nil {
		klog.Errorf("Error adding base:allocation context to libnbd handle: %v", err)
	}

	socket, _ := url.Parse("nbd://" + nbdUnixSocket)
	err = handle.ConnectUri("nbd+unix://?socket=" + nbdUnixSocket)
	if err != nil {
		klog.Errorf("Unable to connect to socket %s: %v", socket, err)
		errKillNbdkit := n.KillNbdkit()
		if errKillNbdkit != nil {
			klog.Errorf("can't kill Nbdkit; %v", err)
		}
		return nil, err
	}

	source := &NbdKitWrapper{
		n:      n,
		Socket: socket,
		Handle: handle,
	}
	return source, nil
}

// createNbdKitLogWatcher creates a channel to use as a log watcher stop signal.
func createNbdKitLogWatcher() *NbdKitLogWatcherVddk {
	stopper := make(chan struct{})
	return &NbdKitLogWatcherVddk{
		stopChannel: stopper,
		output:      nil,
	}
}

// Start runs the nbdkit log watcher in the background.
func (watcher NbdKitLogWatcherVddk) Start(output *bufio.Reader) {
	watcher.output = output
	go watcher.watchNbdLog()
}

// Stop waits for the log watcher to stop. Needs something else to stop nbdkit first.
func (watcher NbdKitLogWatcherVddk) Stop() {
	klog.Infof("Waiting for VDDK nbdkit log watcher to stop.")
	<-watcher.stopChannel
	klog.Infof("Stopped VDDK nbdkit log watcher.")
}

// watchNbdLog reads lines from the nbdkit output. It picks out useful pieces
// of information (VDDK version, final ESX host) and records the last few lines
// to help debug errors (the whole log is otherwise too verbose to save).
func (watcher NbdKitLogWatcherVddk) watchNbdLog() {
	// Only log the last few lines of nbdkit output, there can be a lot
	logRing := ring.New(maxLogLines)
	// Fetch VDDK version from "VMware VixDiskLib (7.0.0) Release build-15832853"
	versionMatch := regexp.MustCompile(`\((?P<version>.*)\).*build-(?P<build>.*)`)
	// Fetch ESX host from "Opened 'vpxa-nfcssl://[iSCSI_Datastore] test/test.vmdk@esx12.test.local:902' (0xa): custom, 50331648 sectors / 24 GB."
	hostMatch := regexp.MustCompile(`Opened '.*@(?P<host>.*):.*' \(0x`)

	scanner := bufio.NewScanner(watcher.output)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "nbdkit: debug: VMware VixDiskLib") {
			if version, found := findMatch(versionMatch, line, "version"); found {
				klog.Infof("VDDK version in-use: %s", version)
				vddkVersion = version
			}
		} else if strings.HasPrefix(line, "nbdkit: vddk[1]: debug: DISKLIB-LINK  : Opened ") {
			if host, found := findMatch(hostMatch, line, "host"); found {
				klog.Infof("VDDK connected to host: %s", vddkHost)
				vddkHost = host
			}
		}

		logRing.Value = line
		logRing = logRing.Next()
	}

	if err := scanner.Err(); err != nil {
		klog.Errorf("Error watching nbdkit log: %v", err)
	}

	klog.Infof("Stopped watching nbdkit log. Last lines follow:")
	logRing.Do(dumpLogs)
	klog.Infof("End of nbdkit log.")

	watcher.stopChannel <- struct{}{}
}

// Find the first match of the given regex in the given line, if it matches the given group name.
func findMatch(regex *regexp.Regexp, line string, name string) (string, bool) {
	matches := regex.FindAllStringSubmatch(line, -1)
	for index, matchName := range regex.SubexpNames() {
		if matchName == name && len(matches) > 0 {
			return matches[0][index], true
		}
	}
	return "", false
}

// Record log lines from the nbdkit log ring buffer, hiding passwords.
func dumpLogs(ringEntry interface{}) {
	var ok bool
	var line string
	if line, ok = ringEntry.(string); !ok {
		return
	}
	if strings.Contains(line, "vddk: config key=password") {
		// Do not log passwords
		return
	}
	klog.Infof("Log line from nbdkit: %s", line)
}

/* Section: VMware API manipulations */

// VMwareConnectionOperations provides a mockable interface for the things needed from VMware client objects.
type VMwareConnectionOperations interface {
	Logout(context.Context) error
	IsVC() bool
}

// VMwareVMOperations provides a mockable interface for the things needed from VMware VM objects.
type VMwareVMOperations interface {
	Properties(context.Context, types.ManagedObjectReference, []string, interface{}) error
	Reference() types.ManagedObjectReference
	FindSnapshot(context.Context, string) (*types.ManagedObjectReference, error)
	QueryChangedDiskAreas(context.Context, *types.ManagedObjectReference, *types.ManagedObjectReference, *types.VirtualDisk, int64) (types.DiskChangeInfo, error)
	Client() *vim25.Client
}

// QueryChangedDiskAreas mocks the underlying QueryChangedDiskAreas for unit test, distinct from the one in VMwareVMOperations
var QueryChangedDiskAreas = methods.QueryChangedDiskAreas

// VMwareClient holds a connection to the VMware API with pre-filled information about one VM
type VMwareClient struct {
	conn       VMwareConnectionOperations // *govmomi.Client
	cancel     context.CancelFunc
	context    context.Context
	moref      string
	thumbprint string
	username   string
	password   string
	url        *url.URL
	vm         VMwareVMOperations // *object.VirtualMachine
}

// createVMwareClient creates a govmomi handle and finds the VM with the given UUID
func createVMwareClient(endpoint string, accessKey string, secKey string, thumbprint string, uuid string) (*VMwareClient, error) {
	vmwURL, err := url.Parse(endpoint)
	if err != nil {
		klog.Errorf("Unable to parse endpoint: %v", endpoint)
		return nil, err
	}

	vmwURL.User = url.UserPassword(accessKey, secKey)
	vmwURL.Path = "sdk"

	// Log in to vCenter
	ctx, cancel := context.WithCancel(context.Background())
	conn, err := govmomi.NewClient(ctx, vmwURL, true)
	if err != nil {
		klog.Errorf("Unable to connect to vCenter: %v", err)
		cancel()
		return nil, err
	}

	moref, vm, err := FindVM(ctx, conn, uuid)
	if err != nil {
		klog.Errorf("Unable to find MORef for VM with UUID %s!", uuid)
		cancel()
		return nil, err
	}

	// Log VM power status to help with debug
	state, err := vm.PowerState(ctx)
	if err != nil {
		klog.Warningf("Unable to get current VM power state: %v", err)
	} else {
		klog.Infof("Current VM power state: %s", state)
	}

	vmware := &VMwareClient{
		conn:       conn,
		cancel:     cancel,
		context:    ctx,
		moref:      moref,
		thumbprint: thumbprint,
		username:   accessKey,
		password:   secKey,
		url:        vmwURL,
		vm:         vm,
	}
	return vmware, nil
}

// Close disconnects from VMware
func (vmware *VMwareClient) Close() error {
	vmware.cancel()
	if err := vmware.conn.Logout(vmware.context); err != nil {
		return err
	}

	klog.Info("Logged out of VMware.")
	return nil
}

// getDiskFileName returns the name of a disk's backing file
func getDiskFileName(disk *types.VirtualDisk) string {
	device := disk.GetVirtualDevice()
	backing := device.Backing.(types.BaseVirtualDeviceFileBackingInfo)
	info := backing.GetVirtualDeviceFileBackingInfo()
	return info.FileName
}

// FindDiskInSnapshot looks through a snapshot's device list for the given backing file name
func (vmware *VMwareClient) FindDiskInSnapshot(snapshotRef types.ManagedObjectReference, fileName string) *types.VirtualDisk {
	var snapshot mo.VirtualMachineSnapshot
	err := vmware.vm.Properties(vmware.context, snapshotRef, []string{"config.hardware.device"}, &snapshot)
	if err != nil {
		klog.Errorf("Unable to get snapshot properties: %s", err)
		return nil
	}

	for _, device := range snapshot.Config.Hardware.Device {
		switch disk := device.(type) {
		case *types.VirtualDisk:
			name := getDiskFileName(disk)
			if name == fileName {
				return disk
			}
		}
	}
	return nil
}

// FindDiskInSnapshotTree looks through a VM's snapshot tree for a disk with the given file name
func (vmware *VMwareClient) FindDiskInSnapshotTree(snapshots []types.VirtualMachineSnapshotTree, fileName string) *types.VirtualDisk {
	for _, snapshot := range snapshots {
		if disk := vmware.FindDiskInSnapshot(snapshot.Snapshot, fileName); disk != nil {
			return disk
		}
		if disk := vmware.FindDiskInSnapshotTree(snapshot.ChildSnapshotList, fileName); disk != nil {
			return disk
		}
	}
	return nil
}

// FindDiskInRootSnapshotParent checks if the parent of the very first snapshot has the target disk name.
// There are cases where the first listed disk is a delta, so other search methods can't find the right disk.
func (vmware *VMwareClient) FindDiskInRootSnapshotParent(snapshots []types.VirtualMachineSnapshotTree, fileName string) *types.VirtualDisk {
	if len(snapshots) > 0 {
		first := snapshots[0].Snapshot
		var snapshot mo.VirtualMachineSnapshot
		err := vmware.vm.Properties(vmware.context, first, []string{"config.hardware.device"}, &snapshot)
		if err == nil {
			for _, device := range snapshot.Config.Hardware.Device {
				switch disk := device.(type) {
				case *types.VirtualDisk:
					var parent *types.VirtualDeviceFileBackingInfo
					switch disk.Backing.(type) {
					case *types.VirtualDiskFlatVer1BackingInfo:
						parent = &disk.Backing.(*types.VirtualDiskFlatVer1BackingInfo).Parent.VirtualDeviceFileBackingInfo
					case *types.VirtualDiskFlatVer2BackingInfo:
						parent = &disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo).Parent.VirtualDeviceFileBackingInfo
					case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
						parent = &disk.Backing.(*types.VirtualDiskRawDiskMappingVer1BackingInfo).Parent.VirtualDeviceFileBackingInfo
					}
					if parent != nil && parent.FileName == fileName {
						return disk
					}
				}
			}
		}
	}

	return nil
}

// FindDiskFromName finds a disk object with the given file name, usable by QueryChangedDiskAreas.
// Looks at the current VM disk as well as any snapshots.
func (vmware *VMwareClient) FindDiskFromName(fileName string) (*types.VirtualDisk, error) {
	// Check current VM disk for given backing file path
	var vm mo.VirtualMachine
	err := vmware.vm.Properties(vmware.context, vmware.vm.Reference(), []string{"config.hardware.device"}, &vm)
	if err != nil {
		return nil, err
	}
	for _, device := range vm.Config.Hardware.Device {
		switch disk := device.(type) {
		case *types.VirtualDisk:
			diskName := getDiskFileName(disk)
			if diskName == fileName {
				return disk, nil
			}
		}
	}

	var snapshot mo.VirtualMachine
	err = vmware.vm.Properties(vmware.context, vmware.vm.Reference(), []string{"snapshot"}, &snapshot)
	if err != nil {
		klog.Errorf("Unable to list snapshots: %s\n", err)
		return nil, err
	}
	if snapshot.Snapshot == nil {
		klog.Errorf("No snapshots on this virtual machine.")
	} else {
		if disk := vmware.FindDiskInSnapshotTree(snapshot.Snapshot.RootSnapshotList, fileName); disk != nil {
			return disk, nil
		}
		if disk := vmware.FindDiskInRootSnapshotParent(snapshot.Snapshot.RootSnapshotList, fileName); disk != nil {
			return disk, nil
		}
	}

	return nil, fmt.Errorf("disk '%s' is not present in VM hardware config or snapshot list", fileName)
}

// FindSnapshotDiskName finds the name of the given disk at the time the snapshot was taken
func (vmware *VMwareClient) FindSnapshotDiskName(snapshotRef *types.ManagedObjectReference, diskID string) (string, error) {
	disk, err := vmware.FindSnapshotDisk(snapshotRef, diskID)
	if err != nil {
		return "", err
	}
	device := disk.GetVirtualDevice()
	backing := device.Backing.(types.BaseVirtualDeviceFileBackingInfo)
	info := backing.GetVirtualDeviceFileBackingInfo()
	return info.FileName, nil
}

// FindSnapshotDisk finds the name of the given disk at the time the snapshot was taken
func (vmware *VMwareClient) FindSnapshotDisk(snapshotRef *types.ManagedObjectReference, diskID string) (*types.VirtualDisk, error) {
	var snapshot mo.VirtualMachineSnapshot
	err := vmware.vm.Properties(vmware.context, *snapshotRef, []string{"config.hardware.device"}, &snapshot)
	if err != nil {
		klog.Errorf("Unable to get snapshot properties: %v", err)
		return nil, err
	}

	for _, device := range snapshot.Config.Hardware.Device {
		switch disk := device.(type) {
		case *types.VirtualDisk:
			if disk.DiskObjectId == diskID {
				return disk, nil
			}
		}
	}
	return nil, fmt.Errorf("Could not find disk image with ID %s in snapshot %s", diskID, snapshotRef.Value)
}

// FindVM takes the UUID of the VM to migrate and finds its MOref
func FindVM(context context.Context, conn *govmomi.Client, uuid string) (string, *object.VirtualMachine, error) {
	// Get the list of datacenters to search for VM UUID
	finder := find.NewFinder(conn.Client, true)
	datacenters, err := finder.DatacenterList(context, "*")
	if err != nil {
		klog.Errorf("Unable to retrieve datacenter list: %v", err)
		return "", nil, err
	}

	// Search for VM matching given UUID, and save the MOref
	var moref string
	var instanceUUID bool
	var vm *object.VirtualMachine
	searcher := object.NewSearchIndex(conn.Client)
	for _, datacenter := range datacenters {
		ref, err := searcher.FindByUuid(context, datacenter, uuid, true, &instanceUUID)
		if err != nil || ref == nil {
			klog.Infof("VM %s not found in datacenter %s.", uuid, datacenter)
		} else {
			moref = ref.Reference().Value
			klog.Infof("VM %s found in datacenter %s: %s", uuid, datacenter, moref)
			vm = object.NewVirtualMachine(conn.Client, ref.Reference())
			return moref, vm, nil
		}
	}

	return "", nil, errors.New("unable to locate VM in any datacenter")
}

/* Section: remote source file operations (libnbd) */

// MaxBlockStatusLength limits the maximum block status request size to 2GB
const MaxBlockStatusLength = (2 << 30)

// MaxPreadLengthESX limits individual VDDK data block transfers to 23MB.
// Larger block sizes fail immediately.
const MaxPreadLengthESX = (23 << 20)

// MaxPreadLengthVC limits indidivual VDDK data block transfers to 2MB only when
// connecting to vCenter. With vCenter endpoints, multiple simultaneous importer
// pods with larger read sizes cause allocation failures on the server, and the
// imports start to fail:
//
//	"NfcFssrvrProcessErrorMsg: received NFC error 5 from server:
//	 Failed to allocate the requested 24117272 bytes"
const MaxPreadLengthVC = (2 << 20)

// MaxPreadLength is the maxmimum read size to request from VMware. Default to
// the larger option, and reduce it in createVddkDataSource when connecting to
// vCenter endpoints.
var MaxPreadLength = MaxPreadLengthESX

// NbdOperations provides a mockable interface for the things needed from libnbd.
type NbdOperations interface {
	GetSize() (uint64, error)
	Pread([]byte, uint64, *libnbd.PreadOptargs) error
	Close() *libnbd.LibnbdError
	BlockStatus(uint64, uint64, libnbd.ExtentCallback, *libnbd.BlockStatusOptargs) error
}

// BlockStatusData holds zero/hole status for one block of data
type BlockStatusData struct {
	Offset int64
	Length int64
	Flags  uint32
}

// Request blocks one at a time from libnbd
var fixedOptArgs = libnbd.BlockStatusOptargs{
	Flags:    libnbd.CMD_FLAG_REQ_ONE,
	FlagsSet: true,
}

// GetBlockStatus runs libnbd.BlockStatus on a given disk range.
// Translated from IMS v2v-conversion-host.
func GetBlockStatus(handle NbdOperations, extent types.DiskChangeExtent) []*BlockStatusData {
	var blocks []*BlockStatusData

	// Callback for libnbd.BlockStatus. Needs to modify blocks list above.
	updateBlocksCallback := func(metacontext string, nbdOffset uint64, extents []uint32, err *int) int {
		if nbdOffset > math.MaxInt64 {
			klog.Errorf("Block status offset too big for conversion: 0x%x", nbdOffset)
			return -2
		}
		offset := int64(nbdOffset)

		if *err != 0 {
			klog.Errorf("Block status callback error at offset %d: error code %d", offset, *err)
			return *err
		}
		if metacontext != "base:allocation" {
			klog.Infof("Offset %d not base:allocation, ignoring", offset)
			return 0
		}
		if (len(extents) % 2) != 0 {
			klog.Errorf("Block status entry at offset %d has unexpected length %d!", offset, len(extents))
			return -1
		}
		for i := 0; i < len(extents); i += 2 {
			length, flags := int64(extents[i]), extents[i+1]
			if blocks != nil {
				last := len(blocks) - 1
				lastBlock := blocks[last]
				lastFlags := lastBlock.Flags
				lastOffset := lastBlock.Offset + lastBlock.Length
				if lastFlags == flags && lastOffset == offset {
					// Merge with previous block
					blocks[last] = &BlockStatusData{
						Offset: lastBlock.Offset,
						Length: lastBlock.Length + length,
						Flags:  lastFlags,
					}
				} else {
					blocks = append(blocks, &BlockStatusData{Offset: offset, Length: length, Flags: flags})
				}
			} else {
				blocks = append(blocks, &BlockStatusData{Offset: offset, Length: length, Flags: flags})
			}
			offset += length
		}
		return 0
	}

	if extent.Length < 1024*1024 {
		blocks = append(blocks, &BlockStatusData{
			Offset: extent.Start,
			Length: extent.Length,
			Flags:  0})
		return blocks
	}

	lastOffset := extent.Start
	endOffset := extent.Start + extent.Length
	for lastOffset < endOffset {
		var length int64
		missingLength := endOffset - lastOffset
		if missingLength > (MaxBlockStatusLength) {
			length = MaxBlockStatusLength
		} else {
			length = missingLength
		}
		createWholeBlock := func() []*BlockStatusData {
			block := &BlockStatusData{
				Offset: extent.Start,
				Length: extent.Length,
				Flags:  0,
			}
			blocks = []*BlockStatusData{block}
			return blocks
		}
		err := handle.BlockStatus(uint64(length), uint64(lastOffset), updateBlocksCallback, &fixedOptArgs)
		if err != nil {
			klog.Errorf("Error getting block status at offset %d, returning whole block instead. Error was: %v", lastOffset, err)
			return createWholeBlock()
		}
		last := len(blocks) - 1
		newOffset := blocks[last].Offset + blocks[last].Length
		if lastOffset == newOffset {
			klog.Infof("No new block status data at offset %d, returning whole block.", newOffset)
			return createWholeBlock()
		}
		lastOffset = newOffset
	}

	return blocks
}

// CopyRange takes one data block, checks if it is a hole or filled with zeroes, and copies it to the sink
func CopyRange(handle NbdOperations, sink VDDKDataSink, block *BlockStatusData, updateProgress func(int)) error {
	skip := ""
	if (block.Flags & libnbd.STATE_HOLE) != 0 {
		skip = "hole"
	}
	if (block.Flags & libnbd.STATE_ZERO) != 0 {
		if skip != "" {
			skip += "/"
		}
		skip += "zero block"
	}

	if (block.Flags & (libnbd.STATE_ZERO | libnbd.STATE_HOLE)) != 0 {
		klog.Infof("Found a %d-byte %s at offset %d, filling destination with zeroes.", block.Length, skip, block.Offset)
		err := sink.ZeroRange(block.Offset, block.Length)
		updateProgress(int(block.Length))
		return err
	}

	buffer := bytes.Repeat([]byte{0}, MaxPreadLength)
	count := int64(0)
	for count < block.Length {
		if block.Length-count < int64(MaxPreadLength) {
			buffer = bytes.Repeat([]byte{0}, int(block.Length-count))
		}
		length := len(buffer)

		offset := block.Offset + count
		err := handle.Pread(buffer, uint64(offset), nil)
		if err != nil {
			klog.Errorf("Error reading from source at offset %d: %v", offset, err)
			return err
		}

		written, err := sink.Pwrite(buffer, uint64(offset))
		if err != nil {
			klog.Errorf("Failed to write data block at offset %d to local file: %v", block.Offset, err)
			return err
		}

		updateProgress(written)
		count += int64(length)
	}
	return nil
}

/* Section: Destination file operations */

// VDDKDataSink provides a mockable interface for saving data from the source.
type VDDKDataSink interface {
	Pwrite(buf []byte, offset uint64) (int, error)
	Write(buf []byte) (int, error)
	ZeroRange(offset int64, length int64) error
	Close()
}

// VDDKFileSink writes the source disk data to a local file.
type VDDKFileSink struct {
	file    *os.File
	writer  *bufio.Writer
	isBlock bool
}

func createVddkDataSink(destinationFile string, size uint64, volumeMode v1.PersistentVolumeMode) (VDDKDataSink, error) {
	isBlock := (volumeMode == v1.PersistentVolumeBlock)

	flags := os.O_WRONLY
	if !isBlock {
		flags |= os.O_CREATE
	}

	file, err := os.OpenFile(destinationFile, flags, 0644)
	if err != nil {
		return nil, err
	}

	writer := bufio.NewWriter(file)
	sink := &VDDKFileSink{
		file:    file,
		writer:  writer,
		isBlock: isBlock,
	}
	return sink, err
}

// Pwrite writes the given byte buffer to the sink at the given offset
func (sink *VDDKFileSink) Pwrite(buffer []byte, offset uint64) (int, error) {
	written, err := syscall.Pwrite(int(sink.file.Fd()), buffer, int64(offset))
	blocksize := len(buffer)
	if written < blocksize {
		klog.Infof("Wrote less than blocksize (%d): %d", blocksize, written)
	}
	if err != nil {
		klog.Errorf("Buffer write error: %s", err)
	}
	return written, err
}

// Write appends the given buffer to the sink
func (sink *VDDKFileSink) Write(buf []byte) (int, error) {
	written, err := sink.writer.Write(buf)
	if err != nil {
		return written, err
	}
	err = sink.writer.Flush()
	return written, err
}

// ZeroRange fills the destination range with zero bytes
func (sink *VDDKFileSink) ZeroRange(offset int64, length int64) error {
	punch := func(offset int64, length int64) error {
		klog.Infof("Punching %d-byte hole at offset %d", length, offset)
		flags := uint32(unix.FALLOC_FL_PUNCH_HOLE | unix.FALLOC_FL_KEEP_SIZE)
		return syscall.Fallocate(int(sink.file.Fd()), flags, offset, length)
	}

	var err error
	if sink.isBlock { // Try to punch a hole in block device destination
		err = punch(offset, length)
	} else {
		var info os.FileInfo
		info, err = sink.file.Stat()
		if err != nil {
			klog.Errorf("Unable to stat destination file: %v", err)
		} else { // Filesystem
			if offset+length > info.Size() { // Truncate only if extending the file
				err = syscall.Ftruncate(int(sink.file.Fd()), offset+length)
			} else { // Otherwise, try to punch a hole in the file
				err = punch(offset, length)
			}
		}
	}

	if err != nil { // Fall back to regular pwrite
		klog.Errorf("Unable to zero range %d - %d on destination, falling back to pwrite: %v", offset, offset+length, err)
		err = nil
		count := int64(0)
		const blocksize = 16 << 20
		buffer := bytes.Repeat([]byte{0}, blocksize)
		for count < length {
			remaining := length - count
			if remaining < blocksize {
				buffer = bytes.Repeat([]byte{0}, int(remaining))
			}
			written, err := sink.Pwrite(buffer, uint64(offset))
			if err != nil {
				klog.Errorf("Unable to write %d zeroes at offset %d: %v", length, offset, err)
				break
			}
			count += int64(written)
		}
	}

	return err
}

// Close closes the file after a transfer is complete.
func (sink *VDDKFileSink) Close() {
	logOnError(sink.writer.Flush())
	logOnError(sink.file.Sync())
	logOnError(sink.file.Close())
}

func logOnError(err error) {
	klog.Error(err)
}

/* Section: CDI data source */

// VDDKDataSource is the data provider for vddk.
type VDDKDataSource struct {
	VMware           *VMwareClient
	BackingFile      string
	NbdKit           *NbdKitWrapper
	CurrentSnapshot  string
	PreviousSnapshot string
	Size             uint64
	VolumeMode       v1.PersistentVolumeMode
}

func init() {
	if err := metrics.SetupMetrics(); err != nil {
		klog.Errorf("Unable to create prometheus progress counter: %v", err)
	}
	ownerUID, _ = util.ParseEnvVar(common.OwnerUID, false)
}

// NewVDDKDataSource creates a new instance of the vddk data provider.
func NewVDDKDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string, volumeMode v1.PersistentVolumeMode) (*VDDKDataSource, error) {
	return newVddkDataSource(endpoint, accessKey, secKey, thumbprint, uuid, backingFile, currentCheckpoint, previousCheckpoint, finalCheckpoint, volumeMode)
}

func createVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string, volumeMode v1.PersistentVolumeMode) (*VDDKDataSource, error) {
	klog.Infof("Creating VDDK data source: backingFile [%s], currentCheckpoint [%s], previousCheckpoint [%s], finalCheckpoint [%s]", backingFile, currentCheckpoint, previousCheckpoint, finalCheckpoint)

	if currentCheckpoint == "" && previousCheckpoint != "" {
		// Not sure what to do with just previous set by itself, return error
		return nil, errors.New("previous checkpoint set without current")
	}

	// Log in to VMware to make sure disks and snapshots are present
	vmware, err := newVMwareClient(endpoint, accessKey, secKey, thumbprint, uuid)
	if err != nil {
		klog.Errorf("Unable to log in to VMware: %v", err)
		return nil, err
	}
	defer func() { _ = vmware.Close() }()

	// Find disk object for backingFile disk image path
	backingFileObject, err := vmware.FindDiskFromName(backingFile)
	if err != nil {
		klog.Errorf("Could not find VM disk %s: %v", backingFile, err)
		return nil, err
	}

	// Find current snapshot object if requested
	var currentSnapshot *types.ManagedObjectReference
	if currentCheckpoint != "" {
		currentSnapshot, err = vmware.vm.FindSnapshot(vmware.context, currentCheckpoint)
		if err != nil {
			klog.Errorf("Could not find current snapshot %s: %v", currentCheckpoint, err)
			return nil, err
		}
	}

	diskFileName := backingFile // By default, just set the nbdkit file name to the given backingFile path
	if currentSnapshot != nil {
		// When copying from a snapshot, set the nbdkit file name to the name of the disk in the snapshot
		// that matches the ID of the given backing file, like "[iSCSI] vm/vmdisk-000001.vmdk".
		diskFileName, err = vmware.FindSnapshotDiskName(currentSnapshot, backingFileObject.DiskObjectId)
		if err != nil {
			klog.Errorf("Could not find matching disk in current snapshot: %v", err)
			return nil, err
		}
		klog.Infof("Set disk file name from current snapshot: %s", diskFileName)
	}
	nbdkit, err := newNbdKitWrapper(vmware, diskFileName, currentCheckpoint)
	if err != nil {
		klog.Errorf("Unable to start nbdkit: %v", err)
		return nil, err
	}

	// Get the total transfer size of either the disk or the delta
	var size uint64
	size, err = nbdkit.Handle.GetSize()
	if err != nil {
		klog.Errorf("Unable to get source disk size: %v", err)
		return nil, err
	}

	MaxPreadLength = MaxPreadLengthESX
	if vmware.conn.IsVC() {
		klog.Infof("Connected to vCenter, restricting read request size to %d.", MaxPreadLengthVC)
		MaxPreadLength = MaxPreadLengthVC
	}

	source := &VDDKDataSource{
		VMware:           vmware,
		BackingFile:      backingFile,
		NbdKit:           nbdkit,
		CurrentSnapshot:  currentCheckpoint,
		PreviousSnapshot: previousCheckpoint,
		Size:             size,
		VolumeMode:       volumeMode,
	}

	terminationChannel := newTerminationChannel()
	go func() {
		<-terminationChannel
		klog.Infof("Caught termination signal, closing nbdkit.")
		source.Close()
	}()

	return source, nil
}

// Info is called to get initial information about the data.
func (vs *VDDKDataSource) Info() (ProcessingPhase, error) {
	klog.Infof("Data transfer size: %d", vs.Size)
	return ProcessingPhaseTransferDataFile, nil
}

// Close closes any readers or other open resources.
func (vs *VDDKDataSource) Close() error {
	vs.NbdKit.Handle.Close()
	return vs.NbdKit.n.KillNbdkit()
}

// GetURL returns the url that the data processor can use when converting the data.
func (vs *VDDKDataSource) GetURL() *url.URL {
	return vs.NbdKit.Socket
}

// GetTerminationMessage returns data to be serialized and used as the termination message of the importer.
func (vs *VDDKDataSource) GetTerminationMessage() *common.TerminationMessage {
	return &common.TerminationMessage{
		VddkInfo: &common.VddkInfo{
			Version: vddkVersion,
			Host:    vddkHost,
		},
	}
}

// Transfer is called to transfer the data from the source to the path passed in.
func (vs *VDDKDataSource) Transfer(path string) (ProcessingPhase, error) {
	return ProcessingPhaseTransferDataFile, nil
}

// IsWarm returns true if this is a multi-stage transfer.
func (vs *VDDKDataSource) IsWarm() bool {
	return vs.CurrentSnapshot != ""
}

// IsDeltaCopy is called to determine if this is a full copy or one delta copy stage
// in a warm migration. This is different from IsWarm because the first step is
// a full copy, and subsequent steps are delta copies.
func (vs *VDDKDataSource) IsDeltaCopy() bool {
	result := vs.PreviousSnapshot != "" && vs.CurrentSnapshot != ""
	return result
}

// Mockable stat, so unit tests can run through TransferFile
var MockableStat = os.Stat

// TransferFile is called to transfer the data from the source to the file passed in.
func (vs *VDDKDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	if !vs.IsWarm() {
		if err := CleanAll(fileName); err != nil {
			return ProcessingPhaseError, err
		}

		// Make sure file exists before applying deltas.
		_, err := MockableStat(fileName)
		if os.IsNotExist(err) {
			klog.Infof("Disk image does not exist, cannot apply deltas for warm migration: %v", err)
			return ProcessingPhaseError, err
		}
	}

	sink, err := newVddkDataSink(fileName, vs.Size, vs.VolumeMode)
	if err != nil {
		return ProcessingPhaseError, err
	}
	defer sink.Close()

	currentProgressBytes := uint64(0)
	previousProgressBytes := uint64(0)
	previousProgressPercent := uint(0)
	previousProgressTime := time.Now()
	initialProgressTime := time.Now()
	updateProgress := func(written int) {
		// Only log progress at approximately 1% minimum intervals.
		currentProgressBytes += uint64(written)
		currentProgressPercent := uint(100.0 * (float64(currentProgressBytes) / float64(vs.Size)))
		if currentProgressPercent > previousProgressPercent {
			progressMessage := fmt.Sprintf("Transferred %d/%d bytes (%d%%)", currentProgressBytes, vs.Size, currentProgressPercent)

			currentProgressTime := time.Now()
			overallProgressTime := uint64(time.Since(initialProgressTime).Seconds())
			if overallProgressTime > 0 {
				overallProgressRate := currentProgressBytes / overallProgressTime
				progressMessage += fmt.Sprintf(" at %d bytes/second overall", overallProgressRate)
			}

			progressTimeDifference := uint64(currentProgressTime.Sub(previousProgressTime).Seconds())
			if progressTimeDifference > 0 {
				progressSize := currentProgressBytes - previousProgressBytes
				progressRate := progressSize / progressTimeDifference
				progressMessage += fmt.Sprintf(", last 1%% was %d bytes at %d bytes/second", progressSize, progressRate)
			}

			klog.Info(progressMessage)

			previousProgressBytes = currentProgressBytes
			previousProgressTime = currentProgressTime
			previousProgressPercent = currentProgressPercent
		}
		v := float64(currentProgressPercent)
		progress, err := metrics.Progress(ownerUID).Get()
		if err == nil && v > 0 && v > progress {
			metrics.Progress(ownerUID).Add(v - progress)
		}
	}

	if vs.IsDeltaCopy() { // Warm migration delta copy
		// Find disk object for backingFile disk image path
		backingFileObject, err := vs.VMware.FindDiskFromName(vs.BackingFile)
		if err != nil {
			klog.Errorf("Could not find VM disk %s: %v", vs.BackingFile, err)
			return ProcessingPhaseError, err
		}

		// Find current snapshot object if requested
		var currentSnapshot *types.ManagedObjectReference
		if vs.CurrentSnapshot != "" {
			currentSnapshot, err = vs.VMware.vm.FindSnapshot(vs.VMware.context, vs.CurrentSnapshot)
			if err != nil {
				klog.Errorf("Could not find current snapshot %s: %v", vs.CurrentSnapshot, err)
				return ProcessingPhaseError, err
			}
		}

		disk, err := vs.VMware.FindSnapshotDisk(currentSnapshot, backingFileObject.DiskObjectId)
		if err != nil {
			klog.Errorf("Could not find matching disk in current snapshot: %v", err)
			return ProcessingPhaseError, err
		}

		// Check if this is a snapshot or a change ID, and query disk areas as appropriate.
		// Change IDs look like: 52 de c0 d9 b9 43 9d 10-61 d5 4c 1b e9 7b 65 63/81
		changeIDPattern := `([0-9a-fA-F]{2}\s?)*-([0-9a-fA-F]{2}\s?)*\/([0-9a-fA-F]*)`
		isChangeID, _ := regexp.MatchString(changeIDPattern, vs.PreviousSnapshot)
		var changed types.DiskChangeInfo
		var previousSnapshot *types.ManagedObjectReference
		if !isChangeID {
			previousSnapshot, err = vs.VMware.vm.FindSnapshot(vs.VMware.context, vs.PreviousSnapshot)
			if err != nil {
				klog.Errorf("Could not find previous snapshot %s: %v", vs.PreviousSnapshot, err)
				return ProcessingPhaseError, err
			}
			if previousSnapshot == nil {
				return ProcessingPhaseError, fmt.Errorf("failed to find previous snapshot %s", vs.PreviousSnapshot)
			}
		}

		// QueryChangedDiskAreas needs to be called multiple times to get all possible disk changes.
		// Experimentation shows it returns maximally 2000 changed blocks. If the disk has more than
		// 2000 changed blocks we need to query the next chunk of the blocks starting from previous.
		// Loop until QueryChangedDiskAreas starts returning zero-length block lists.
		for {
			klog.Infof("Querying changed disk areas at offset %d", changed.Length)
			if isChangeID { // Previous checkpoint is a change ID
				request := types.QueryChangedDiskAreas{
					ChangeId:    vs.PreviousSnapshot,
					DeviceKey:   backingFileObject.Key,
					Snapshot:    currentSnapshot,
					StartOffset: changed.Length,
					This:        vs.VMware.vm.Reference(),
				}
				response, err := QueryChangedDiskAreas(vs.VMware.context, vs.VMware.vm.Client(), &request)
				if err != nil {
					klog.Errorf("Failed to query changed areas: %s", err)
					return ProcessingPhaseError, err
				}
				klog.Infof("%d changed areas reported at offset %d with data length %d", len(response.Returnval.ChangedArea), changed.Length, response.Returnval.Length)
				if len(response.Returnval.ChangedArea) == 0 { // No more changes
					break
				}
				changed.ChangedArea = append(changed.ChangedArea, response.Returnval.ChangedArea...)
				changed.Length += response.Returnval.Length
			} else { // Previous checkpoint is a snapshot
				changedAreas, err := vs.VMware.vm.QueryChangedDiskAreas(vs.VMware.context, previousSnapshot, currentSnapshot, backingFileObject, changed.Length)
				if err != nil {
					klog.Errorf("Unable to query changed areas: %s", err)
					return ProcessingPhaseError, err
				}
				klog.Infof("%d changed areas reported at offset %d with data length %d", len(changedAreas.ChangedArea), changed.Length, changedAreas.Length)
				if len(changedAreas.ChangedArea) == 0 {
					break
				}
				changed.ChangedArea = append(changed.ChangedArea, changedAreas.ChangedArea...)
				changed.Length += changedAreas.Length
			}

			// No changes? Immediately return success.
			if len(changed.ChangedArea) < 1 {
				klog.Infof("No changes reported between snapshot %s and snapshot %s, marking transfer complete.", vs.PreviousSnapshot, vs.CurrentSnapshot)
				return ProcessingPhaseComplete, nil
			}
			// The start offset should not be the size of the disk otherwise the QueryChangedDiskAreas will fail
			if changed.Length >= disk.CapacityInBytes {
				klog.Infof("the offset %d is greater or equal to disk capacity %d", changed.Length, disk.CapacityInBytes)
				break
			}
			// Copy actual data from query ranges to destination
			for _, extent := range changed.ChangedArea {
				blocks := GetBlockStatus(vs.NbdKit.Handle, extent)
				for _, block := range blocks {
					err := CopyRange(vs.NbdKit.Handle, sink, block, updateProgress)
					if err != nil {
						klog.Errorf("Unable to copy block at offset %d: %v", block.Offset, err)
						return ProcessingPhaseError, err
					}
				}
			}
		}
	} else { // Cold migration full copy
		start := uint64(0)
		blocksize := uint64(MaxBlockStatusLength)
		for i := start; i < vs.Size; i += blocksize {
			if (vs.Size - i) < blocksize {
				blocksize = vs.Size - i
			}

			extent := types.DiskChangeExtent{
				Length: int64(blocksize),
				Start:  int64(i),
			}

			blocks := GetBlockStatus(vs.NbdKit.Handle, extent)
			for _, block := range blocks {
				err := CopyRange(vs.NbdKit.Handle, sink, block, updateProgress)
				if err != nil {
					klog.Errorf("Unable to copy block at offset %d: %v", block.Offset, err)
					return ProcessingPhaseError, err
				}
			}
		}
	}

	if vs.PreviousSnapshot != "" {
		// Don't resize when applying snapshot deltas as the resize has already happened
		// when the first snapshot was imported.
		return ProcessingPhaseComplete, nil
	}
	return ProcessingPhaseResize, nil
}
