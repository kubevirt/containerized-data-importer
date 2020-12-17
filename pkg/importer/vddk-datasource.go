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
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	libnbd "github.com/mrnold/go-libnbd"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

// May be overridden in tests
var newVddkDataSource = createVddkDataSource
var newVddkDataSink = createVddkDataSink
var vddkPluginPath = getVddkPluginPath

/* Section: nbdkit */

const (
	mockPlugin            = "/opt/testing/libvddk-test-plugin.so"
	nbdUnixSocket         = "/var/run/nbd.sock"
	nbdPidFile            = "/var/run/nbd.pid"
	nbdLibraryPath        = "/opt/vmware-vix-disklib-distrib/lib64"
	startupTimeoutSeconds = 15
)

// NbdKitWrapper keeps track of one nbdkit process
type NbdKitWrapper struct {
	Command *exec.Cmd
	Socket  *url.URL
	Handle  NbdOperations
}

// getVddkPluginPath checks for the existence of a fake VDDK plugin for tests
func getVddkPluginPath() string {
	_, err := os.Stat(mockPlugin)
	if !os.IsNotExist(err) {
		return mockPlugin
	}
	return "vddk"
}

// waitForNbd waits for nbdkit to start by watching for the existence of the given PID file.
func waitForNbd(pidfile string) error {
	nbdCheck := make(chan bool, 1)
	go func() {
		klog.Infoln("Waiting for nbdkit PID.")
		for {
			select {
			case <-nbdCheck:
				return
			case <-time.After(500 * time.Millisecond):
				_, err := os.Stat(pidfile)
				if err != nil {
					if !os.IsNotExist(err) {
						klog.Warningf("Error checking for nbdkit PID: %v", err)
					}
				} else {
					nbdCheck <- true
					return
				}
			}
		}
	}()

	select {
	case <-nbdCheck:
		klog.Infoln("nbdkit ready.")
		return nil
	case <-time.After(startupTimeoutSeconds * time.Second):
		nbdCheck <- true
		return errors.New("timed out waiting for nbdkit to be ready")
	}
}

// validatePlugins tests VDDK and any other plugins before starting nbdkit for real
func validatePlugins() error {
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		klog.Infof("%s: %d %s", path, info.Size(), info.Mode())
		return nil
	}

	klog.Infof("Checking nbdkit plugin directory tree:")
	err := filepath.Walk("/usr/lib64/nbdkit", walker)
	if err != nil {
		klog.Warningf("Unable to get nbdkit plugin directory tree: %v", err)
	}

	klog.Infof("Checking VDDK library directory tree:")
	err = filepath.Walk("/opt/vmware-vix-disklib-distrib", walker)
	if err != nil {
		klog.Warningf("Unable to get VDDK library directory tree: %v", err)
	}

	args := []string{
		"--dump-plugin",
		vddkPluginPath(),
	}
	nbdkit := exec.Command("nbdkit", args...)
	env := os.Environ()
	env = append(env, "LD_LIBRARY_PATH="+nbdLibraryPath)
	nbdkit.Env = env
	out, err := nbdkit.CombinedOutput()
	if out != nil {
		klog.Infof("Output from nbdkit --dump-plugin %s: %s", vddkPluginPath(), out)
	}
	if err != nil {
		return err
	}

	return nil
}

// createNbdKitWrapper starts nbdkit and returns a process handle for further management
func createNbdKitWrapper(vmware *VMwareClient, diskFileName string) (*NbdKitWrapper, error) {
	err := validatePlugins()
	if err != nil {
		klog.Errorf("Error validating nbdkit plugins: %v", err)
		return nil, err
	}

	args := []string{
		"--foreground",
		"--readonly",
		"--exit-with-parent",
		"--unix", nbdUnixSocket,
		"--pidfile", nbdPidFile,
		"--filter=retry",
		vddkPluginPath(),
		"server=" + vmware.url.Host,
		"user=" + vmware.username,
		"password=" + vmware.password,
		"thumbprint=" + vmware.thumbprint,
		"vm=moref=" + vmware.moref,
		"file=" + diskFileName,
		"libdir=" + nbdLibraryPath,
	}

	nbdkit := exec.Command("nbdkit", args...)
	env := os.Environ()
	env = append(env, "LD_LIBRARY_PATH="+nbdLibraryPath)
	nbdkit.Env = env

	stdout, err := nbdkit.StdoutPipe()
	if err != nil {
		klog.Errorf("Error constructing stdout pipe: %v", err)
		return nil, err
	}
	nbdkit.Stderr = nbdkit.Stdout
	output := bufio.NewReader(stdout)
	go func() {
		for {
			line, err := output.ReadString('\n')
			if err != nil {
				break
			}
			klog.Infof("Log line from nbdkit: %s", line)
		}
		klog.Infof("Stopped watching nbdkit log.")
	}()

	err = nbdkit.Start()
	if err != nil {
		klog.Errorf("Unable to start nbdkit: %v", err)
		return nil, err
	}

	err = waitForNbd(nbdPidFile)
	if err != nil {
		klog.Errorf("Failed waiting for nbdkit to start up: %v", err)
		return nil, err
	}

	handle, err := libnbd.Create()
	if err != nil {
		klog.Errorf("Unable to create libnbd handle: %v", err)
		nbdkit.Process.Kill()
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
		nbdkit.Process.Kill()
		return nil, err
	}

	source := &NbdKitWrapper{
		Command: nbdkit,
		Socket:  socket,
		Handle:  handle,
	}
	return source, nil
}

/* Section: VMware API manipulations */

// VMwareClient holds a connection to the VMware API with pre-filled information about one VM
type VMwareClient struct {
	conn       *govmomi.Client
	cancel     context.CancelFunc
	context    context.Context
	moref      string
	thumbprint string
	username   string
	password   string
	url        *url.URL
	vm         *object.VirtualMachine
}

// CreateVMwareClient creates a govmomi handle and finds the VM with the given UUID
func CreateVMwareClient(endpoint string, accessKey string, secKey string, thumbprint string, uuid string) (*VMwareClient, error) {
	vmwURL, err := url.Parse(endpoint)
	if err != nil {
		klog.Errorf("Unable to parse endpoint: %v", endpoint)
		return nil, err
	}

	// Construct VMware SDK URL and get MOref
	sdkURL := vmwURL.Scheme + "://" + accessKey + ":" + secKey + "@" + vmwURL.Host + "/sdk"
	vmwURL, err = url.Parse(sdkURL)
	if err != nil {
		klog.Errorf("Unable to create VMware URL: %v", err)
		return nil, err
	}

	// Log in to vCenter
	ctx, cancel := context.WithCancel(context.Background())
	conn, err := govmomi.NewClient(ctx, vmwURL, true)
	if err != nil {
		klog.Errorf("Unable to connect to vCenter: %v", err)
		return nil, err
	}

	moref, vm, err := FindVM(ctx, conn, uuid)
	if err != nil {
		klog.Errorf("Unable to find MORef for VM with UUID %s!", uuid)
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
func (vmware *VMwareClient) Close() {
	vmware.cancel()
	vmware.conn.Logout(vmware.context)
	klog.Info("Logged out of VMware.")
}

// FindDisk finds a disk object with the given filename, usable by QueryChangedDiskAreas
func (vmware *VMwareClient) FindDisk(filename string) (*types.VirtualDisk, error) {
	var movm mo.VirtualMachine
	err := vmware.vm.Properties(vmware.context, vmware.vm.Reference(), []string{"config.hardware.device"}, &movm)
	if err != nil {
		return nil, err
	}
	for _, device := range movm.Config.Hardware.Device {
		switch disk := device.(type) {
		case *types.VirtualDisk:
			device := disk.GetVirtualDevice()
			backing := device.Backing.(types.BaseVirtualDeviceFileBackingInfo)
			info := backing.GetVirtualDeviceFileBackingInfo()
			if info.FileName == filename {
				klog.Infof("Found target disk: %s", info.FileName)
				return disk, nil
			}
		}
	}
	return nil, errors.New("could not find target VMware disk")
}

// FindSnapshotDiskName finds the name of the given disk at the time the snapshot was taken
func (vmware *VMwareClient) FindSnapshotDiskName(snapshotRef *types.ManagedObjectReference, diskID string) (string, error) {
	var snapshot mo.VirtualMachineSnapshot
	err := vmware.vm.Properties(vmware.context, *snapshotRef, []string{"config.hardware.device"}, &snapshot)
	if err != nil {
		klog.Errorf("Unable to get snapshot properties: %v", err)
		return "", err
	}

	for _, device := range snapshot.Config.Hardware.Device {
		switch disk := device.(type) {
		case *types.VirtualDisk:
			if disk.DiskObjectId == diskID {
				device := disk.GetVirtualDevice()
				backing := device.Backing.(types.BaseVirtualDeviceFileBackingInfo)
				info := backing.GetVirtualDeviceFileBackingInfo()
				return info.FileName, nil
			}
		}
	}
	return "", fmt.Errorf("Could not find disk image with ID %s in snapshot %s", diskID, snapshotRef.Value)
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

// MaxPreadLength limits individual data block transfers to 23MB, larger block sizes fail
const MaxPreadLength = (23 << 20)

// NbdOperations provides a mockable interface for the things needed from libnbd.
type NbdOperations interface {
	GetSize() (uint64, error)
	Pread([]byte, uint64, *libnbd.PreadOptargs) error
	Close() *libnbd.LibnbdError
	BlockStatus(uint64, uint64, libnbd.ExtentCallback, *libnbd.BlockStatusOptargs) error
}

// BlockStatusData holds zero/hole status for one block of data
type BlockStatusData struct {
	Offset uint64
	Length uint32
	Flags  uint32
}

// Request blocks one at a time from libnbd
var fixedOptArgs = libnbd.BlockStatusOptargs{
	Flags:    libnbd.CMD_FLAG_REQ_ONE,
	FlagsSet: true,
}

// GetBlockStatus runs libnbd.BlockStatus on a given disk range.
// Translated from IMS v2v-conversion-host.
func GetBlockStatus(handle NbdOperations, extent types.DiskChangeExtent) ([]*BlockStatusData, error) {
	var blocks []*BlockStatusData

	// Callback for libnbd.BlockStatus. Needs to modify blocks list above.
	updateBlocksCallback := func(metacontext string, offset uint64, extents []uint32, err *int) int {
		if *err != 0 {
			klog.Errorf("Block status callback error was: %v", *err)
			return *err
		}
		if metacontext != "base:allocation" {
			klog.Info("Not base:allocation, ignoring")
			return 0
		}
		klog.Infof("Block status callback offset: %d", offset)
		klog.Infof("Block status length of extents: %d", len(extents))
		if (len(extents) % 2) != 0 {
			klog.Error("Block status entry unexpected length!")
			return -1
		}
		for i := 0; i < len(extents); i += 2 {
			length, flags := extents[i], extents[i+1]
			if blocks != nil {
				last := len(blocks) - 1
				lastBlock := blocks[last]
				lastFlags := lastBlock.Flags
				lastOffset := lastBlock.Offset + uint64(lastBlock.Length)
				if lastFlags == flags && lastOffset == offset {
					// Merge with previous block
					blocks[last] = &BlockStatusData{
						Offset: lastBlock.Offset,
						Length: lastBlock.Length,
						Flags:  lastFlags,
					}
					klog.Infof("Block status: merging offset %d, length %d, flags 0x%x into previous offset %d, length %d, flags 0x%x", offset, length, flags, lastOffset, lastBlock.Length, lastFlags)
				} else {
					klog.Infof("Block status: offset %d, length %d, flags 0x%x", offset, length, flags)
					blocks = append(blocks, &BlockStatusData{Offset: offset, Length: length, Flags: flags})
				}
			} else {
				klog.Infof("First block: offset %d, length %d, flags 0x%x", offset, length, flags)
				blocks = append(blocks, &BlockStatusData{Offset: offset, Length: length, Flags: flags})
			}
			offset += uint64(length)
		}
		return 0
	}

	if extent.Length < 1024*1024 {
		klog.Info("Size is less than 1M, avoiding block status request.")
		blocks = append(blocks, &BlockStatusData{
			Offset: uint64(extent.Start),
			Length: uint32(extent.Length),
			Flags:  0})
		return blocks, nil
	}

	lastOffset := extent.Start
	endOffset := extent.Start + extent.Length
	for lastOffset < endOffset {
		var length uint64
		missingLength := endOffset - lastOffset
		if missingLength > (MaxBlockStatusLength) {
			length = (MaxBlockStatusLength)
		} else {
			length = uint64(missingLength)
		}
		err := handle.BlockStatus(length, uint64(lastOffset), updateBlocksCallback, &fixedOptArgs)
		if err != nil {
			klog.Errorf("Error getting block status at %d! %v", lastOffset, err)
			return nil, err
		}
		last := len(blocks) - 1
		newOffset := blocks[last].Offset + uint64(blocks[last].Length)
		if uint64(lastOffset) == newOffset {
			klog.Info("No new block status data")
		}
		lastOffset = int64(newOffset)
	}

	return blocks, nil
}

// CopyRange takes one data block, checks if it is a hole or filled with zeroes, and copies it to the sink
func CopyRange(handle NbdOperations, sink VDDKDataSink, block *BlockStatusData) (int, error) {
	if (block.Flags & libnbd.STATE_HOLE) != 0 {
		klog.Info("Found a hole, filling destination with zeroes.")
	}
	if (block.Flags & libnbd.STATE_ZERO) != 0 {
		klog.Info("Found a zero block, filling destination with zeroes.")
	}

	if (block.Flags & (libnbd.STATE_ZERO | libnbd.STATE_HOLE)) != 0 {
		size := block.Offset + uint64(block.Length)
		err := sink.Ftruncate(int64(size))
		return int(block.Length), err
	}

	buffer := make([]byte, block.Length)
	count := uint32(0)
	for count < block.Length {
		var length uint32
		if block.Length-count > MaxPreadLength {
			length = MaxPreadLength
		} else {
			length = block.Length - count
		}
		offset := block.Offset + uint64(count)
		klog.Infof("Reading %d-bytes to local %d at file %d", length, count, offset)
		err := handle.Pread(buffer[count:count+length], offset, nil)
		if err != nil {
			klog.Errorf("Error reading from source: %v", err)
			return 0, err
		}
		count += length
	}
	written, err := sink.Pwrite(buffer, uint64(block.Offset))
	if err != nil {
		klog.Errorf("Failed to write data block at offset %d to local file: %v", block.Offset, err)
	}
	return written, err
}

/* Section: Destination file operations */

// VDDKDataSink provides a mockable interface for saving data from the source.
type VDDKDataSink interface {
	Ftruncate(size int64) error
	Pwrite(buf []byte, offset uint64) (int, error)
	Write(buf []byte) (int, error)
	Close()
}

// VDDKFileSink writes the source disk data to a local file.
type VDDKFileSink struct {
	file   *os.File
	writer *bufio.Writer
}

func createVddkDataSink(destinationFile string, size uint64) (VDDKDataSink, error) {
	file, err := os.OpenFile(destinationFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	writer := bufio.NewWriter(file)
	sink := &VDDKFileSink{
		file:   file,
		writer: writer,
	}
	return sink, err
}

// Ftruncate is used to pad the sink with zeroes to the given size.
func (sink *VDDKFileSink) Ftruncate(size int64) error {
	err := syscall.Ftruncate(int(sink.file.Fd()), size)
	if err != nil {
		klog.Errorf("Unable to ftruncate file: %v", err)
	}
	return err
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

// Close closes the file after a transfer is complete.
func (sink *VDDKFileSink) Close() {
	sink.writer.Flush()
	sink.file.Sync()
	sink.file.Close()
}

/* Section: CDI data source */

// VDDKDataSource is the data provider for vddk.
type VDDKDataSource struct {
	NbdKit           *NbdKitWrapper
	ChangedBlocks    *types.DiskChangeInfo
	CurrentSnapshot  string
	PreviousSnapshot string
	Size             uint64
}

func init() {
	if err := prometheus.Register(progress); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// A counter for that metric has been registered before.
			// Use the old counter from now on.
			progress = are.ExistingCollector.(*prometheus.CounterVec)
		} else {
			klog.Errorf("Unable to create prometheus progress counter")
		}
	}
	ownerUID, _ = util.ParseEnvVar(common.OwnerUID, false)
}

// NewVDDKDataSource creates a new instance of the vddk data provider.
func NewVDDKDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string) (*VDDKDataSource, error) {
	return newVddkDataSource(endpoint, accessKey, secKey, thumbprint, uuid, backingFile, currentCheckpoint, previousCheckpoint, finalCheckpoint)
}

func createVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string) (*VDDKDataSource, error) {
	klog.Infof("Creating VDDK data source: backingFile [%s], currentCheckpoint [%s], previousCheckpoint [%s], finalCheckpoint [%s]", backingFile, currentCheckpoint, previousCheckpoint, finalCheckpoint)
	if currentCheckpoint == "" && previousCheckpoint != "" {
		// Not sure what to do with just previous set by itself, return error
		return nil, errors.New("previous checkpoint set without current")
	}

	// Log in to VMware, and get everything needed up front
	vmware, err := CreateVMwareClient(endpoint, accessKey, secKey, thumbprint, uuid)
	if err != nil {
		klog.Errorf("Unable to log in to VMware: %v", err)
		return nil, err
	}
	defer vmware.Close()

	// Find disk object for backingFile disk image path
	currentDisk, err := vmware.FindDisk(backingFile)
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

	// Find previous snapshot object if requested
	var previousSnapshot *types.ManagedObjectReference
	if previousCheckpoint != "" {
		previousSnapshot, err = vmware.vm.FindSnapshot(vmware.context, previousCheckpoint)
		if err != nil {
			klog.Errorf("Could not find previous snapshot %s: %v", previousCheckpoint, err)
			return nil, err
		}
	}

	// If this is a warm migration (current and previous checkpoints set),
	// then get the list of changed blocks from VMware for a delta copy.
	var changed *types.DiskChangeInfo
	if currentSnapshot != nil && previousSnapshot != nil {
		changedAreas, err := vmware.vm.QueryChangedDiskAreas(vmware.context, previousSnapshot, currentSnapshot, currentDisk, 0)
		if err != nil {
			klog.Errorf("Unable to query changed areas: %s", err)
			return nil, err
		}
		changed = &changedAreas
	}

	diskFileName := backingFile
	if currentSnapshot != nil && previousSnapshot == nil {
		// Cold copy snapshot: set the nbdkit file name to the name of this
		// disk in the snapshot, like "[iSCSI] vm/vmdisk-000001.vmdk"
		diskFileName, err = vmware.FindSnapshotDiskName(currentSnapshot, currentDisk.DiskObjectId)
		if err != nil {
			klog.Errorf("Could not find matching disk in current snapshot: %v", err)
			return nil, err
		}
	}
	nbdkit, err := createNbdKitWrapper(vmware, diskFileName)
	if err != nil {
		klog.Errorf("Unable to start nbdkit: %v", err)
		return nil, err
	}

	// Get the total transfer size of either the disk or the delta
	var size uint64
	if changed != nil { // Warm migration: get size of the delta
		size = 0
		for _, change := range changed.ChangedArea {
			size += uint64(change.Length)
		}
	} else { // Cold migration: get size of the whole disk
		size, err = nbdkit.Handle.GetSize()
		if err != nil {
			klog.Errorf("Unable to get source disk size: %v", err)
			return nil, err
		}
	}

	source := &VDDKDataSource{
		NbdKit:           nbdkit,
		ChangedBlocks:    changed,
		CurrentSnapshot:  currentCheckpoint,
		PreviousSnapshot: previousCheckpoint,
		Size:             size,
	}
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
	return vs.NbdKit.Command.Process.Kill()
}

// GetURL returns the url that the data processor can use when converting the data.
func (vs *VDDKDataSource) GetURL() *url.URL {
	return vs.NbdKit.Socket
}

// Transfer is called to transfer the data from the source to the path passed in.
func (vs *VDDKDataSource) Transfer(path string) (ProcessingPhase, error) {
	return ProcessingPhaseTransferDataFile, nil
}

// IsDeltaCopy is called to determine if this is a full copy or one delta copy stage
// in a warm migration.
func (vs *VDDKDataSource) IsDeltaCopy() bool {
	result := vs.PreviousSnapshot != "" && vs.CurrentSnapshot != ""
	return result
}

// TransferFile is called to transfer the data from the source to the file passed in.
func (vs *VDDKDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	if vs.ChangedBlocks != nil { // Warm migration pre-checks
		if len(vs.ChangedBlocks.ChangedArea) < 1 { // No changes? Immediately return success.
			klog.Infof("No changes reported between snapshot %s and snapshot %s, marking transfer complete.", vs.PreviousSnapshot, vs.CurrentSnapshot)
			return ProcessingPhaseComplete, nil
		}

		// Make sure file exists before applying deltas.
		_, err := os.Stat(fileName)
		if os.IsNotExist(err) {
			klog.Infof("Disk image does not exist, cannot apply deltas for warm migration: %v", err)
			return ProcessingPhaseError, err
		}
	}

	sink, err := newVddkDataSink(fileName, vs.Size)
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
		metric := &dto.Metric{}
		err = progress.WithLabelValues(ownerUID).Write(metric)
		if err == nil && v > 0 && v > *metric.Counter.Value {
			progress.WithLabelValues(ownerUID).Add(v - *metric.Counter.Value)
		}
	}

	if vs.ChangedBlocks != nil { // Warm migration delta copy
		for _, extent := range vs.ChangedBlocks.ChangedArea {
			blocks, err := GetBlockStatus(vs.NbdKit.Handle, extent)
			if err != nil {
				klog.Errorf("Unable to get block status for %d bytes at offset %d: %v", extent.Length, extent.Start, err)
				return ProcessingPhaseError, err // Could probably just copy the whole block here instead
			}

			for _, block := range blocks {
				written, err := CopyRange(vs.NbdKit.Handle, sink, block)
				if err != nil {
					klog.Errorf("Unable to copy block at offset %d: %v", block.Offset, err)
					return ProcessingPhaseError, err
				}
				updateProgress(written)
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

			blocks, err := GetBlockStatus(vs.NbdKit.Handle, extent)
			if err != nil {
				klog.Errorf("Unable to get block status for %d bytes at offset %d: %v", blocksize, i, err)
				return ProcessingPhaseError, err // Could probably just copy the whole block here instead
			}

			for _, block := range blocks {
				written, err := CopyRange(vs.NbdKit.Handle, sink, block)
				if err != nil {
					klog.Errorf("Unable to copy block at offset %d: %v", block.Offset, err)
					return ProcessingPhaseError, err
				}
				updateProgress(written)
			}
		}
	}

	return ProcessingPhaseComplete, nil
}
