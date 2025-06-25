//go:build amd64
// +build amd64

package importer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // This is test code
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/url"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	libnbd "libguestfs.org/libnbd"

	v1 "k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
)

const (
	socketPath = "nbd://nbdtest.sock"
)

type mockNbdExport struct {
	Size func() (uint64, error)
	Read func(uint64) ([]byte, error)
}

func defaultMockNbdExport() mockNbdExport {
	export := &mockNbdExport{}
	export.Size = func() (uint64, error) {
		return 12345, nil
	}
	export.Read = func(uint64) ([]byte, error) {
		return bytes.Repeat([]byte{0}, 512), nil
	}
	return *export
}

var currentExport mockNbdExport

var _ = Describe("VDDK data source", func() {
	BeforeEach(func() {
		mockSinkBuffer = bytes.Repeat([]byte{0x00}, 512)
		newVddkDataSource = createMockVddkDataSource
		newVddkDataSink = createMockVddkDataSink
		newVMwareClient = createMockVMwareClient
		newNbdKitWrapper = createMockNbdKitWrapper
		newTerminationChannel = createMockTerminationChannel
		currentExport = defaultMockNbdExport()
		currentVMwareFunctions = defaultMockVMwareFunctions()
		currentMockNbdFunctions = defaultMockNbdFunctions()
		currentMockSinkFunctions = defaultMockSinkFunctions()
	})

	AfterEach(func() {
		newVddkDataSource = createVddkDataSource
	})

	It("NewVDDKDataSource should fail when called with an invalid endpoint", func() {
		newVddkDataSource = createVddkDataSource
		newVMwareClient = createVMwareClient
		_, err := NewVDDKDataSource("httpx://-------", "", "", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).To(HaveOccurred())
	})

	It("NewVDDKDataSource should not fail on credentials with special characters", func() {
		newVddkDataSource = createVddkDataSource
		newVMwareClient = createVMwareClient
		_, err := NewVDDKDataSource("http://--------", "test#user@vsphere.local", "Test#password", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no such host"))
		Expect(err.Error()).ToNot(ContainSubstring("Test#password"))
		Expect(err.Error()).ToNot(ContainSubstring(url.PathEscape("Test#password")))
	})

	It("VDDK data source GetURL should pass through NBD socket information", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		socket := dp.GetURL()
		path := socket.String()
		Expect(path).To(Equal(socketPath))
	})

	It("VDDK data source should move to transfer data phase after Info", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
	})

	It("VDDK data source should move to preallocate phase after TransferFile", func() {
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return 512, nil
		}
		replaceExport.Read = func(uint64) ([]byte, error) {
			return bytes.Repeat([]byte{0x55}, 512), nil
		}
		MockableStat = func(string) (fs.FileInfo, error) {
			return nil, nil
		}
		currentExport = replaceExport
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
		phase, err = dp.TransferFile("", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseResize))
	})

	It("VDDK data source should fail if TransferFile fails", func() {
		newVddkDataSink = createVddkDataSink
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		phase, err := dp.Info()
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseTransferDataFile))
		phase, err = dp.TransferFile("", false)
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
	})

	It("VDDK data source should know if it is a delta copy", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "checkpoint-1", "checkpoint-2", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.IsDeltaCopy()).To(BeTrue())
	})

	It("VDDK data source should know if it is not a delta copy", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.IsDeltaCopy()).To(BeFalse())
	})

	It("VDDK delta copy should return immediately if there are no changed blocks", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "testdisk.vmdk", "snapshot-1", "snapshot-2", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		snapshots := createSnapshots("snapshot-1", "snapshot-2")
		snapshotList := []*types.ManagedObjectReference{
			&snapshots.RootSnapshotList[0].Snapshot,
			&snapshots.RootSnapshotList[0].ChildSnapshotList[0].Snapshot,
		}
		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfig("testdisk.vmdk", 12345)
				} else if property[0] == "snapshot" {
					out.Snapshot = createSnapshots("snapshot-1", "snapshot-2")
				}
			case *mo.VirtualMachineSnapshot:
				out.Config = *createVirtualDiskConfig("testdisk-00001.vmdk", 123456)
			}
			return nil
		}
		currentVMwareFunctions.FindSnapshot = func(ctx context.Context, nameOrID string) (*types.ManagedObjectReference, error) {
			for _, snap := range snapshotList {
				if snap.Value == nameOrID {
					return snap, nil
				}
			}
			return nil, errors.New("could not find snapshot")
		}
		currentVMwareFunctions.QueryChangedDiskAreas = func(context.Context, *types.ManagedObjectReference, *types.ManagedObjectReference, *types.VirtualDisk, int64) (types.DiskChangeInfo, error) {
			return types.DiskChangeInfo{
				StartOffset: 0,
				Length:      0,
				ChangedArea: []types.DiskChangeExtent{},
			}, nil
		}
		currentMockNbdFunctions.AioBlockStatus = func(uint64, uint64, libnbd.ExtentCallback, *libnbd.AioBlockStatusOptargs) (uint64, error) {
			return 0, fmt.Errorf("this should not be called on zero-change test")
		}
		MockableStat = func(string) (fs.FileInfo, error) {
			return nil, nil
		}
		phase, err := dp.TransferFile("", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseComplete))
	})

	It("VDDK full copy should successfully copy the same bytes passed in", func() {
		dp, err := NewVDDKDataSource("", "", "", "", "", "", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		dp.Size = 40 << 20
		sourceBytes := bytes.Repeat([]byte{0x55}, int(dp.Size))
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return dp.Size, nil
		}
		replaceExport.Read = func(offset uint64) ([]byte, error) {
			return sourceBytes, nil
		}
		currentExport = replaceExport

		MockableStat = func(string) (fs.FileInfo, error) {
			return nil, nil
		}
		mockSinkBuffer = bytes.Repeat([]byte{0x00}, int(dp.Size))
		phase, err := dp.TransferFile("target", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseResize))

		sourceSum := md5.Sum(sourceBytes)  //nolint:gosec // This is test code
		destSum := md5.Sum(mockSinkBuffer) //nolint:gosec // This is test code
		Expect(sourceSum).To(Equal(destSum))
	})

	It("VDDK delta copy should successfully apply a delta to a base disk image", func() {

		// Copy base disk ("snapshot 1")
		snap1, err := NewVDDKDataSource("", "", "", "", "", "testdisk.vmdk", "checkpoint-1", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		snap1.Size = 40 << 20
		sourceBytes := bytes.Repeat([]byte{0x55}, int(snap1.Size))
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return snap1.Size, nil
		}
		replaceExport.Read = func(uint64) ([]byte, error) {
			return sourceBytes, nil
		}
		currentExport = replaceExport

		mockSinkBuffer = bytes.Repeat([]byte{0x00}, int(snap1.Size))

		MockableStat = func(string) (fs.FileInfo, error) {
			return nil, nil
		}
		phase, err := snap1.TransferFile("target", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseResize))

		sourceSum := md5.Sum(sourceBytes)  //nolint:gosec // This is test code
		destSum := md5.Sum(mockSinkBuffer) //nolint:gosec // This is test code
		Expect(sourceSum).To(Equal(destSum))

		// Write some data to the first snapshot, then copy the delta from difference between the two snapshots
		snap2, err := NewVDDKDataSource("", "", "", "", "", "testdisk.vmdk", "checkpoint-1", "checkpoint-2", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		snap2.Size = 40 << 20
		copy(sourceBytes[1024:2048], bytes.Repeat([]byte{0xAA}, 1024))
		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfigWithSize("testdisk.vmdk", 12345, int64(snap1.Size))
				} else if property[0] == "snapshot" {
					out.Snapshot = createSnapshots("checkpoint-1", "checkpoint-2")
				}
			case *mo.VirtualMachineSnapshot:
				out.Config = *createVirtualDiskConfigWithSize("testdisk-00001.vmdk", 123456, int64(snap1.Size))
			}
			return nil
		}
		callcount := 0
		currentVMwareFunctions.QueryChangedDiskAreas = func(context.Context, *types.ManagedObjectReference, *types.ManagedObjectReference, *types.VirtualDisk, int64) (types.DiskChangeInfo, error) {
			if callcount > 0 {
				return types.DiskChangeInfo{
					StartOffset: 1024,
					Length:      0,
					ChangedArea: []types.DiskChangeExtent{},
				}, nil
			}
			callcount++
			return types.DiskChangeInfo{
				StartOffset: 1024,
				Length:      1024,
				ChangedArea: []types.DiskChangeExtent{{
					Start:  1024,
					Length: 1024,
				}},
			}, nil
		}
		snapshots := createSnapshots("checkpoint-1", "checkpoint-2")
		snapshotList := []*types.ManagedObjectReference{
			&snapshots.RootSnapshotList[0].Snapshot,
			&snapshots.RootSnapshotList[0].ChildSnapshotList[0].Snapshot,
		}
		currentVMwareFunctions.FindSnapshot = func(ctx context.Context, nameOrID string) (*types.ManagedObjectReference, error) {
			for _, snap := range snapshotList {
				if snap.Value == nameOrID {
					return snap, nil
				}
			}
			return nil, errors.New("could not find snapshot")
		}
		changedSourceSum := md5.Sum(sourceBytes) //nolint:gosec // This is test code

		phase, err = snap2.TransferFile(".", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseComplete))

		deltaSum := md5.Sum(mockSinkBuffer) //nolint:gosec // This is test code
		Expect(changedSourceSum).To(Equal(deltaSum))
	})

	It("VDDK delta copy should accept a change ID as a checkpoint", func() {
		diskName := "disk"
		snapshotName := "checkpoint-2"
		changeID := "52 de c0 d9 b9 43 9d 10-61 d5 4c 1b e9 7b 65 63/81"
		newVddkDataSource = createVddkDataSource

		snapshots := createSnapshots(snapshotName, "")
		currentVMwareFunctions.FindSnapshot = func(ctx context.Context, nameOrID string) (*types.ManagedObjectReference, error) {
			return &snapshots.RootSnapshotList[0].Snapshot, nil
		}
		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfig(diskName, 12345)
				} else if property[0] == "snapshot" {
					out.Snapshot = snapshots
				}
			case *mo.VirtualMachineSnapshot:
				out.Config = *createVirtualDiskConfig("snapshotdisk", 123456)
			}
			return nil
		}

		counter := 0
		changeInfo := types.DiskChangeInfo{
			StartOffset: 0,
			Length:      1000,
			ChangedArea: []types.DiskChangeExtent{
				{
					Start:  0,
					Length: 1000,
				},
			},
		}
		QueryChangedDiskAreas = func(ctx context.Context, r soap.RoundTripper, req *types.QueryChangedDiskAreas) (*types.QueryChangedDiskAreasResponse, error) {
			if counter > 0 {
				return &types.QueryChangedDiskAreasResponse{
					Returnval: types.DiskChangeInfo{},
				}, nil
			}
			counter++
			return &types.QueryChangedDiskAreasResponse{
				Returnval: changeInfo,
			}, nil
		}

		//ds, err := NewVDDKDataSource("", "", "", "", "", diskName, snapshotName, changeID, "", v1.PersistentVolumeFilesystem)
		_, err := NewVDDKDataSource("", "", "", "", "", diskName, snapshotName, changeID, "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		//Expect(ds.ChangedBlocks).To(Equal(&changeInfo))
	})

	DescribeTable("disk name lookup", func(targetDiskName, diskName, snapshotDiskName, rootSnapshotParentName string, expectedSuccess bool) {
		var returnedDiskName string

		newVddkDataSource = createVddkDataSource
		newNbdKitWrapper = func(vmware *VMwareClient, fileName, snapshot string) (*NbdKitWrapper, error) {
			returnedDiskName = fileName
			return createMockNbdKitWrapper(vmware, fileName, snapshot)
		}
		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfig(diskName, 12345)
				} else if property[0] == "snapshot" {
					out.Snapshot = createSnapshots("snapshot-1", "snapshot-2")
				}
			case *mo.VirtualMachineSnapshot:
				out.Config = *createVirtualDiskConfig(snapshotDiskName, 123456)
				disk := out.Config.Hardware.Device[0].(*types.VirtualDisk)
				parent := disk.Backing.(*types.VirtualDiskFlatVer1BackingInfo).Parent
				parent.FileName = rootSnapshotParentName
			}
			return nil
		}

		_, err := NewVDDKDataSource("http://vcenter.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", targetDiskName, "", "", "", v1.PersistentVolumeFilesystem)
		if expectedSuccess {
			Expect(err).ToNot(HaveOccurred())
			Expect(returnedDiskName).To(Equal(targetDiskName))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(returnedDiskName).ToNot(Equal(targetDiskName))
		}
	},
		Entry("should find backing file on a VM", "[teststore] testvm/testfile.vmdk", "[teststore] testvm/testfile.vmdk", "", "", true),
		Entry("should find backing file on a snapshot", "[teststore] testvm/testfile.vmdk", "wrong disk.vmdk", "[teststore] testvm/testfile.vmdk", "", true),
		Entry("should find base backing file even if not listed as first snapshot", "[teststore] testvm/testfile.vmdk", "[teststore] testvm/testfile-000001.vmdk", "[teststore] testvm/testfile-000002.vmdk", "[teststore] testvm/testfile.vmdk", true),
		Entry("should fail if backing file is not found in snapshot tree", "[teststore] testvm/testfile.vmdk", "wrong disk 1.vmdk", "wrong disk 2.vmdk", "wrong disk 1.vmdk", false),
	)

	It("should pass snapshot reference through to nbdkit", func() {
		expectedSnapshot := "snapshot-10"

		var receivedSnapshotRef string
		newVddkDataSource = createVddkDataSource
		newNbdKitWrapper = func(vmware *VMwareClient, fileName, snapshot string) (*NbdKitWrapper, error) {
			receivedSnapshotRef = snapshot
			return createMockNbdKitWrapper(vmware, fileName, snapshot)
		}

		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfig("disk1", 12345)
				} else if property[0] == "snapshot" {
					out.Snapshot = createSnapshots(expectedSnapshot, "snapshot-11")
				}
			case *mo.VirtualMachineSnapshot:
				out.Config = *createVirtualDiskConfig("snapshot-disk", 123456)
				disk := out.Config.Hardware.Device[0].(*types.VirtualDisk)
				parent := disk.Backing.(*types.VirtualDiskFlatVer1BackingInfo).Parent
				parent.FileName = "snapshot-root"
			}
			return nil
		}

		_, err := NewVDDKDataSource("http://vcenter.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", "disk1", expectedSnapshot, "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		Expect(receivedSnapshotRef).To(Equal(expectedSnapshot))
	})

	It("should not crash when the disk is not found and has no snapshots", func() {
		newVddkDataSource = createVddkDataSource
		diskName := "testdisk.vmdk"
		wrongDiskName := "wrong.vmdk"

		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfig(wrongDiskName, 12345)
				}
			}
			return nil
		}

		_, err := NewVDDKDataSource("http://vcenter.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", diskName, "", "", "false", v1.PersistentVolumeFilesystem)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("disk 'testdisk.vmdk' is not present in VM hardware config or snapshot list"))
	})

	It("should cancel transfer on SIGTERM", func() {
		newVddkDataSource = createVddkDataSource
		diskName := "testdisk.vmdk"

		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfig(diskName, 12345)
				}
			}
			return nil
		}
		_, err := NewVDDKDataSource("http://vcenter.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", diskName, "snapshot-1", "snapshot-2", "false", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		mockTerminationChannel <- os.Interrupt
		Expect(err).ToNot(HaveOccurred())
	})

	It("should reduce pread length for vCenter endpoints", func() {
		newVddkDataSource = createVddkDataSource
		diskName := "testdisk.vmdk"

		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfig(diskName, 12345)
				}
			}
			return nil
		}

		ds, err := NewVDDKDataSource("http://esx.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", diskName, "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		Expect(ds.MaxPreadLength).To(Equal(MaxPreadLengthESX))

		ds, err = NewVDDKDataSource("http://vcenter.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", diskName, "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		Expect(ds.MaxPreadLength).To(Equal(MaxPreadLengthVC))
	})

	It("GetTerminationMessage should contain VDDK connection information", func() {
		const testVersion = "testVersion"
		const testHost = "testHost"

		source, err := NewVDDKDataSource("http://esx.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", "testdisk.vmdk", "", "", "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())

		vddkVersion = testVersion
		vddkHost = testHost
		Expect(*source.GetTerminationMessage()).To(Equal(common.TerminationMessage{VddkInfo: &common.VddkInfo{Version: testVersion, Host: testHost}}))
	})

	It("VDDK return more than 2000 changed block area", func() {
		diskName := "disk"
		snapshotName := "checkpoint-2"
		changeID := "52 de c0 d9 b9 43 9d 10-61 d5 4c 1b e9 7b 65 63/81"
		newVddkDataSource = createVddkDataSource

		snapshots := createSnapshots(snapshotName, "")
		currentVMwareFunctions.FindSnapshot = func(ctx context.Context, nameOrID string) (*types.ManagedObjectReference, error) {
			return &snapshots.RootSnapshotList[0].Snapshot, nil
		}
		currentVMwareFunctions.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
			switch out := result.(type) {
			case *mo.VirtualMachine:
				if property[0] == "config.hardware.device" {
					out.Config = createVirtualDiskConfigWithSize(diskName, 12345, 9563078656)
				} else if property[0] == "snapshot" {
					out.Snapshot = snapshots
				}
			case *mo.VirtualMachineSnapshot:
				out.Config = *createVirtualDiskConfigWithSize("snapshotdisk", 123456, 9563078656)
			}
			return nil
		}

		expectedInfo := types.DiskChangeInfo{
			StartOffset: 0,
			Length:      9563078656,
			ChangedArea: []types.DiskChangeExtent{},
		}
		for i := 0; i < 4500; i++ {
			expectedInfo.ChangedArea = append(expectedInfo.ChangedArea, types.DiskChangeExtent{})
		}
		counter := 0
		queries := [4]bool{}
		QueryChangedDiskAreas = func(ctx context.Context, r soap.RoundTripper, req *types.QueryChangedDiskAreas) (*types.QueryChangedDiskAreasResponse, error) {
			var resp *types.QueryChangedDiskAreasResponse
			if counter == 0 {
				respInfo := types.DiskChangeInfo{
					StartOffset: 0,
					Length:      4194369536,
					ChangedArea: []types.DiskChangeExtent{},
				}
				for i := 0; i < 2000; i++ {
					respInfo.ChangedArea = append(respInfo.ChangedArea, types.DiskChangeExtent{})
				}
				resp = &types.QueryChangedDiskAreasResponse{
					Returnval: respInfo,
				}
			} else if counter == 1 {
				respInfo := types.DiskChangeInfo{
					StartOffset: 4194369536,
					Length:      4194369536,
					ChangedArea: []types.DiskChangeExtent{},
				}
				for i := 0; i < 2000; i++ {
					respInfo.ChangedArea = append(respInfo.ChangedArea, types.DiskChangeExtent{})
				}
				resp = &types.QueryChangedDiskAreasResponse{
					Returnval: respInfo,
				}
			} else if counter == 2 {
				respInfo := types.DiskChangeInfo{
					StartOffset: 8388739072,
					Length:      1174339584,
					ChangedArea: []types.DiskChangeExtent{},
				}
				for i := 0; i < 500; i++ {
					respInfo.ChangedArea = append(respInfo.ChangedArea, types.DiskChangeExtent{})
				}
				resp = &types.QueryChangedDiskAreasResponse{
					Returnval: respInfo,
				}
			} else {
				respInfo := types.DiskChangeInfo{
					StartOffset: 9563078656,
					Length:      0,
					ChangedArea: []types.DiskChangeExtent{},
				}
				resp = &types.QueryChangedDiskAreasResponse{
					Returnval: respInfo,
				}
			}
			queries[counter] = true
			counter++
			return resp, nil
		}
		MockableStat = func(string) (fs.FileInfo, error) {
			return nil, nil
		}

		ds, err := NewVDDKDataSource("http://vcenter.test", "user", "pass", "aa:bb:cc:dd", "1-2-3-4", diskName, snapshotName, changeID, "", v1.PersistentVolumeFilesystem)
		Expect(err).ToNot(HaveOccurred())
		_, err = ds.TransferFile("", false)
		Expect(err).ToNot(HaveOccurred())
		Expect(queries[0]).To(BeTrue())
		Expect(queries[1]).To(BeTrue())
		Expect(queries[2]).To(BeTrue())
	})
})

var _ = Describe("VDDK log watcher", func() {
	DescribeTable("should set VDDK connection information from nbdkit log stream, ", func(messages []string, results []string) {
		vddkVersion = "not set"
		vddkHost = "not set"
		logbuffer := new(bytes.Buffer)
		logreader := bufio.NewReader(logbuffer)
		watcher := createNbdKitLogWatcher()
		for _, message := range messages {
			logbuffer.WriteString(message + "\n")
		}
		watcher.Start(logreader)
		watcher.Stop()
		Expect(vddkVersion).To(Equal(results[0]))
		Expect(vddkHost).To(Equal(results[1]))
	},
		Entry("with no data", []string{}, []string{"not set", "not set"}),
		Entry("with good version string", []string{"nbdkit: debug: VMware VixDiskLib (7.0.0) Release build-15832853"}, []string{"7.0.0", "not set"}),
		Entry("with alternate good version string", []string{"nbdkit: debug: VMware VixDiskLib (7.0) Release build-15832853"}, []string{"7.0", "not set"}),
		Entry("with bad version string", []string{"nbdkit: debug: VMware VixDiskLib 7.0 Release build-15832853"}, []string{"not set", "not set"}),
		Entry("with good host string", []string{"nbdkit: vddk[1]: debug: DISKLIB-LINK  : Opened 'vpxa-nfcssl://[iSCSI_Datastore] test/test.vmdk@esx15.test.lan:902' (0xa): custom, 50331648 sectors / 24 GB."}, []string{"not set", "esx15.test.lan"}),
		Entry("with alternate good host string", []string{"nbdkit: vddk[1]: debug: DISKLIB-LINK  : Opened 'http://disk@esx:1234' (0xa): custom, 50331648 sectors / 24 GB."}, []string{"not set", "esx"}),
		Entry("with bad host string", []string{"nbdkit: vddk[1]: debug: DISKLIB-LINK  : Opened 'vpxa-nfcssl://esx' (0xa): custom, 50331648 sectors / 24 GB."}, []string{"not set", "not set"}),
		Entry("with good version and host strings", []string{
			"nbdkit: debug: VMware VixDiskLib (7.0.0) Release build-15832853",
			"nbdkit: vddk[1]: debug: DISKLIB-LINK  : Opened 'vpxa-nfcssl://disk@esx:1234' (0xa): custom, 50331648 sectors / 24 GB."},
			[]string{"7.0.0", "esx"}),
		Entry("with bad version and host strings", []string{
			"nbdkit: debug: VMware VixDiskLib ",
			"nbdkit: vddk[1]: debug: DISKLIB-LINK  : Opened '"},
			[]string{"not set", "not set"}),
	)
})

var _ = Describe("VDDK get block status", func() {
	initializeExportSize := func(size int) {
		replaceExport := currentExport
		replaceExport.Size = func() (uint64, error) {
			return uint64(size), nil
		}
		replaceExport.Read = func(uint64) ([]byte, error) {
			return bytes.Repeat([]byte{0x55}, size), nil
		}
		currentExport = replaceExport

		mockSinkBuffer = bytes.Repeat([]byte{0x00}, size)
	}

	BeforeEach(func() {
		currentExport = defaultMockNbdExport()
		currentMockNbdFunctions = defaultMockNbdFunctions()
		currentMockSinkFunctions = defaultMockSinkFunctions()

		// Do-nothing Pread, much faster than default byte copy for large block status requests
		cookie := uint64(0)
		currentMockNbdFunctions.AioPread = func(buf libnbd.AioBuffer, offset uint64, optargs *libnbd.AioPreadOptargs) (uint64, error) {
			if optargs.CompletionCallbackSet {
				optargs.CompletionCallback(new(int))
			}
			cookie++
			return cookie, nil
		}

		// Do-nothing sink functions, no need to do any buffer filling or copying for block status tests
		currentMockSinkFunctions.Pwrite = func(buf []byte, offset uint64) (int, error) {
			return len(buf), nil
		}

		currentMockSinkFunctions.Write = func(buf []byte) (int, error) {
			return len(buf), nil
		}

		currentMockSinkFunctions.ZeroRange = func(int64, int64) error {
			return nil
		}
	})

	It("should return the whole block when libnbd returns an error", func() {
		nbdCalled := false
		currentMockNbdFunctions.AioBlockStatus = func(length, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
			nbdCalled = true
			err := 1
			callback("base:allocation", offset, []uint32{}, &err)
			optargs.CompletionCallback(&err)
			return 0, nil
		}
		const size = 1024
		initializeExportSize(size)

		sink, err := createMockVddkDataSink("", size, v1.PersistentVolumeFilesystem)
		Expect(err).To(BeNil())
		err = CopyRange(&mockNbdOperations{}, sink, 0, size, nil, MaxPreadLengthVC)
		Expect(err).To(BeNil())
		Expect(nbdCalled).To(BeTrue())
	})

	It("should return the whole block when the status request returns an empty list", func() {
		currentMockNbdFunctions.AioBlockStatus = func(length uint64, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
			err := 0
			callback("base:allocation", offset, []uint32{}, &err)
			optargs.CompletionCallback(&err)
			return 0, nil
		}

		// Can't test just the block results from CopyRange, so intercept Pread and make sure it reads the whole block (total pread size should equal export size).
		readTotal := 0
		cookie := uint64(0)
		currentMockNbdFunctions.AioPread = func(buf libnbd.AioBuffer, offset uint64, optargs *libnbd.AioPreadOptargs) (uint64, error) {
			readTotal += int(buf.Size)
			cookie++
			return cookie, nil
		}

		const size = 1024
		initializeExportSize(size)
		sink, err := createMockVddkDataSink("", size, v1.PersistentVolumeFilesystem)
		Expect(err).To(BeNil())
		err = CopyRange(&mockNbdOperations{}, sink, 0, size, nil, MaxPreadLengthVC)
		Expect(err).To(BeNil())
		Expect(readTotal).To(Equal(size))
	})

	It("should chunk up block status requests when asking for more than the maximum status request length", func() {
		timesCalled := 0
		blocks := []*BlockStatusData{}
		currentMockNbdFunctions.AioBlockStatus = func(length, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
			err := 0
			if timesCalled == 0 {
				Expect(offset).To(Equal(uint64(0)))
				Expect(length).To(Equal(uint64(MaxBlockStatusLength)))
				blocks = append(blocks, &BlockStatusData{
					Extent: Extent{
						Offset: 0,
						Length: MaxBlockStatusLength,
					},
					Flags: 0,
				})
				callback("base:allocation", offset, []uint32{MaxBlockStatusLength, 0}, &err)
			}
			if timesCalled == 1 {
				Expect(offset).To(Equal(uint64(MaxBlockStatusLength)))
				Expect(length).To(Equal(uint64(1024)))
				blocks = append(blocks, &BlockStatusData{
					Extent: Extent{
						Offset: MaxBlockStatusLength,
						Length: 1024,
					},
					Flags: 0,
				})
				callback("base:allocation", offset, []uint32{1024, 0}, &err)
			}
			optargs.CompletionCallback(&err)
			timesCalled++
			return 0, nil
		}

		length := MaxBlockStatusLength + 1024
		initializeExportSize(length)

		expectedBlocks := []*BlockStatusData{
			{
				Extent: Extent{
					Offset: 0,
					Length: int64(MaxBlockStatusLength),
				},
				Flags: 0,
			},
			{
				Extent: Extent{
					Offset: int64(MaxBlockStatusLength),
					Length: 1024,
				},
				Flags: 0,
			},
		}

		sink, err := createMockVddkDataSink("", uint64(length), v1.PersistentVolumeFilesystem)
		Expect(err).To(BeNil())
		err = CopyRange(&mockNbdOperations{}, sink, 0, MaxBlockStatusLength+1024, nil, MaxPreadLengthVC)
		Expect(err).To(BeNil())
		Expect(timesCalled).To(Equal(2))
		Expect(blocks).To(Equal(expectedBlocks))
	})

	It("should return multiple blocks when data source ranges have different flags", func() {
		const blockSize = 1024 * 1024

		blocks := []*BlockStatusData{}
		currentMockNbdFunctions.AioBlockStatus = func(length, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
			err := 0
			if offset == 0 {
				callback("base:allocation", offset, []uint32{blockSize, 0}, &err)
				blocks = append(blocks, &BlockStatusData{
					Extent: Extent{
						Offset: 0,
						Length: blockSize,
					},
					Flags: 0,
				})
			}
			if offset == blockSize {
				callback("base:allocation", offset, []uint32{blockSize, 3}, &err)
				blocks = append(blocks, &BlockStatusData{
					Extent: Extent{
						Offset: blockSize,
						Length: blockSize,
					},
					Flags: 3,
				})
			}
			optargs.CompletionCallback(&err)
			return 0, nil
		}

		expectedBlocks := []*BlockStatusData{
			{
				Extent: Extent{
					Offset: 0,
					Length: blockSize,
				},
				Flags: 0,
			},
			{
				Extent: Extent{
					Offset: blockSize,
					Length: blockSize,
				},
				Flags: 3,
			},
		}

		initializeExportSize(blockSize * 2)

		sink, err := createMockVddkDataSink("", uint64(blockSize*2), v1.PersistentVolumeFilesystem)
		Expect(err).To(BeNil())
		err = CopyRange(&mockNbdOperations{}, sink, 0, blockSize*2, nil, MaxPreadLengthVC)
		Expect(err).To(BeNil())
		Expect(blocks).To(Equal(expectedBlocks))
	})

	It("should handle large blocks with the same flags", func() {
		const blockSize = math.MaxUint32
		const size = blockSize * 2
		blocks := []*BlockStatusData{}
		currentMockNbdFunctions.AioBlockStatus = func(length, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
			err := 0
			ln := uint32(min(length, MaxBlockStatusLength))
			callback("base:allocation", offset, []uint32{ln, 0}, &err)
			blocks = append(blocks, &BlockStatusData{
				Extent: Extent{
					Offset: int64(offset),
					Length: int64(ln),
				},
				Flags: 0,
			})
			optargs.CompletionCallback(&err)
			return 0, nil
		}

		initializeExportSize(size)

		expectedBlocks := []*BlockStatusData{
			{
				Extent: Extent{
					Offset: 0,
					Length: MaxBlockStatusLength,
				},
				Flags: 0,
			},
			{
				Extent: Extent{
					Offset: MaxBlockStatusLength,
					Length: MaxBlockStatusLength,
				},
				Flags: 0,
			},
			{
				Extent: Extent{
					Offset: MaxBlockStatusLength * 2,
					Length: MaxBlockStatusLength,
				},
				Flags: 0,
			},
			{
				Extent: Extent{
					Offset: MaxBlockStatusLength * 3,
					Length: MaxBlockStatusLength - 2, // Minus two to handle MaxUint32*2 block sizes
				},
				Flags: 0,
			},
		}

		sink, err := createMockVddkDataSink("", uint64(size), v1.PersistentVolumeFilesystem)
		Expect(err).To(BeNil())
		err = CopyRange(&mockNbdOperations{}, sink, 0, size, nil, MaxPreadLengthVC)
		Expect(err).To(BeNil())
		Expect(blocks).To(Equal(expectedBlocks))
	})

	It("should handle large blocks with different flags", func() {
		const blockSize = 1 << 33
		const size = blockSize * 4

		flagSetting := true // Alternate flags between blocks
		currentMockNbdFunctions.AioBlockStatus = func(length, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
			err := 0
			ln := uint32(min(length, MaxBlockStatusLength))

			if flagSetting {
				callback("base:allocation", offset, []uint32{ln, 0}, &err)
			} else {
				callback("base:allocation", offset, []uint32{ln, 3}, &err)
			}
			flagSetting = !flagSetting

			optargs.CompletionCallback(&err)
			return 0, nil
		}

		initializeExportSize(MaxPreadLengthVC)
		currentExport.Size = func() (uint64, error) {
			return uint64(size), nil
		}

		sink, err := createMockVddkDataSink("", uint64(size), v1.PersistentVolumeFilesystem)
		Expect(err).To(BeNil())
		err = CopyRange(&mockNbdOperations{}, sink, 0, size, nil, MaxPreadLengthVC)
		Expect(err).To(BeNil())
	})
})

type mockNbdFunctions struct {
	GetSize             func() (uint64, error)
	AioPread            func(buf libnbd.AioBuffer, offset uint64, optargs *libnbd.AioPreadOptargs) (uint64, error)
	Close               func() *libnbd.LibnbdError
	AioBlockStatus      func(length uint64, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error)
	Poll                func(timeout int) (uint, error)
	AioCommandCompleted func(uint64) (bool, error)
	AioInFlight         func() (uint, error)
}

func defaultMockNbdFunctions() mockNbdFunctions {
	cookie := uint64(0)
	ops := mockNbdFunctions{}
	ops.GetSize = func() (uint64, error) {
		return currentExport.Size()
	}
	ops.AioPread = func(buf libnbd.AioBuffer, offset uint64, optargs *libnbd.AioPreadOptargs) (uint64, error) {
		fakebuf, err := currentExport.Read(offset)
		slice := fakebuf[offset : offset+uint64(buf.Size)]
		for i := range slice {
			*buf.Get(uint(i)) = slice[i]
		}
		if optargs.CompletionCallbackSet {
			errorCode := 0
			optargs.CompletionCallback(&errorCode)
		}
		cookie++
		return cookie, err
	}
	ops.Close = func() *libnbd.LibnbdError {
		return nil
	}
	ops.AioBlockStatus = func(length uint64, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
		err := 0
		callback("base:allocation", offset, []uint32{uint32(length), 0}, &err)
		if optargs.CompletionCallbackSet {
			errorCode := 0
			optargs.CompletionCallback(&errorCode)
		}
		cookie++
		return cookie, nil
	}
	ops.Poll = func(timeout int) (uint, error) {
		return 0, nil
	}
	ops.AioCommandCompleted = func(uint64) (bool, error) {
		return true, nil
	}
	ops.AioInFlight = func() (uint, error) {
		return 0, nil
	}
	return ops
}

var currentMockNbdFunctions mockNbdFunctions

type mockNbdOperations struct{}

func (handle *mockNbdOperations) GetSize() (uint64, error) {
	return currentMockNbdFunctions.GetSize()
}

func (handle *mockNbdOperations) AioPread(buf libnbd.AioBuffer, offset uint64, optargs *libnbd.AioPreadOptargs) (uint64, error) {
	return currentMockNbdFunctions.AioPread(buf, offset, optargs)
}

func (handle *mockNbdOperations) Close() *libnbd.LibnbdError {
	return currentMockNbdFunctions.Close()
}

func (handle *mockNbdOperations) AioBlockStatus(length, offset uint64, callback libnbd.ExtentCallback, optargs *libnbd.AioBlockStatusOptargs) (uint64, error) {
	return currentMockNbdFunctions.AioBlockStatus(length, offset, callback, optargs)
}

func (handle *mockNbdOperations) Poll(timeout int) (uint, error) {
	return currentMockNbdFunctions.Poll(timeout)
}

func (handle *mockNbdOperations) AioCommandCompleted(command uint64) (bool, error) {
	return currentMockNbdFunctions.AioCommandCompleted(command)
}

func (handle *mockNbdOperations) AioInFlight() (uint, error) {
	return currentMockNbdFunctions.AioInFlight()
}

func createMockVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string, volumeMode v1.PersistentVolumeMode) (*VDDKDataSource, error) {
	socketURL, err := url.Parse(socketPath)
	if err != nil {
		return nil, err
	}

	handle := &mockNbdOperations{}

	nbdkit := &NbdKitWrapper{
		n:      &image.Nbdkit{},
		Socket: socketURL,
		Handle: handle,
	}

	vmware, err := newVMwareClient(endpoint, accessKey, secKey, thumbprint, uuid)
	if err != nil {
		return nil, err
	}

	return &VDDKDataSource{
		VMware:           vmware,
		NbdKit:           nbdkit,
		CurrentSnapshot:  currentCheckpoint,
		PreviousSnapshot: previousCheckpoint,
		Size:             0,
		VolumeMode:       volumeMode,
		MaxPreadLength:   MaxPreadLengthVC,
	}, nil
}

type mockSinkFunctions struct {
	Close     func()
	Pwrite    func([]byte, uint64) (int, error)
	Write     func([]byte) (int, error)
	ZeroRange func(int64, int64) error
}

type mockVddkDataSink struct {
	position int
}

var currentMockSinkFunctions mockSinkFunctions
var mockSinkBuffer []byte

func defaultMockSinkFunctions() mockSinkFunctions {
	sink := mockVddkDataSink{}
	funcs := mockSinkFunctions{}

	funcs.Close = func() {}

	funcs.ZeroRange = func(offset, length int64) error {
		buf := bytes.Repeat([]byte{0x00}, int(length))
		_, err := sink.Pwrite(buf, uint64(offset))
		return err
	}

	funcs.Pwrite = func(buf []byte, offset uint64) (int, error) {
		copy(mockSinkBuffer[offset:offset+uint64(len(buf))], buf)
		if len(buf) > sink.position {
			sink.position = int(offset) + len(buf)
		}
		return len(buf), nil
	}

	funcs.Write = func(buf []byte) (int, error) {
		copy(mockSinkBuffer[sink.position:sink.position+len(buf)], buf)
		sink.position += len(buf)
		return len(buf), nil
	}

	return funcs
}

func (sink *mockVddkDataSink) ZeroRange(offset int64, length int64) error {
	return currentMockSinkFunctions.ZeroRange(offset, length)
}

func (sink *mockVddkDataSink) Pwrite(buf []byte, offset uint64) (int, error) {
	return currentMockSinkFunctions.Pwrite(buf, offset)
}

func (sink *mockVddkDataSink) Write(buf []byte) (int, error) {
	return currentMockSinkFunctions.Write(buf)
}

func (sink *mockVddkDataSink) Close() {
	currentMockSinkFunctions.Close()
}

func createMockVddkDataSink(destinationFile string, size uint64, volumeMode v1.PersistentVolumeMode) (VDDKDataSink, error) {
	sink := &mockVddkDataSink{0}
	return sink, nil
}

type mockVMwareConnectionOperations struct {
	Endpoint string
}

func (ops *mockVMwareConnectionOperations) Logout(context.Context) error {
	return nil
}

func (ops *mockVMwareConnectionOperations) IsVC() bool {
	return strings.Contains(ops.Endpoint, "vcenter")
}

type mockVMwareFunctions struct {
	Properties            func(context.Context, types.ManagedObjectReference, []string, interface{}) error
	Reference             func() types.ManagedObjectReference
	FindSnapshot          func(context.Context, string) (*types.ManagedObjectReference, error)
	QueryChangedDiskAreas func(context.Context, *types.ManagedObjectReference, *types.ManagedObjectReference, *types.VirtualDisk, int64) (types.DiskChangeInfo, error)
	Client                func() *vim25.Client
}

func defaultMockVMwareFunctions() mockVMwareFunctions {
	ops := &mockVMwareFunctions{}
	ops.Properties = func(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
		return nil
	}
	ops.Reference = func() types.ManagedObjectReference {
		return types.ManagedObjectReference{}
	}
	ops.FindSnapshot = func(ctx context.Context, nameOrID string) (*types.ManagedObjectReference, error) {
		return nil, nil
	}
	ops.QueryChangedDiskAreas = func(ctx context.Context, baseSnapshot *types.ManagedObjectReference, changedSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error) {
		return types.DiskChangeInfo{}, nil
	}
	ops.Client = func() *vim25.Client {
		return &vim25.Client{}
	}
	return *ops
}

var currentVMwareFunctions mockVMwareFunctions

type mockVMwareVMOperations struct{}

func (ops *mockVMwareVMOperations) Properties(ctx context.Context, ref types.ManagedObjectReference, property []string, result interface{}) error {
	return currentVMwareFunctions.Properties(ctx, ref, property, result)
}

func (ops *mockVMwareVMOperations) Reference() types.ManagedObjectReference {
	return currentVMwareFunctions.Reference()
}

func (ops *mockVMwareVMOperations) FindSnapshot(ctx context.Context, nameOrID string) (*types.ManagedObjectReference, error) {
	return currentVMwareFunctions.FindSnapshot(ctx, nameOrID)
}

func (ops *mockVMwareVMOperations) QueryChangedDiskAreas(ctx context.Context, baseSnapshot *types.ManagedObjectReference, changedSnapshot *types.ManagedObjectReference, disk *types.VirtualDisk, offset int64) (types.DiskChangeInfo, error) {
	return currentVMwareFunctions.QueryChangedDiskAreas(ctx, baseSnapshot, changedSnapshot, disk, offset)
}

func (ops *mockVMwareVMOperations) Client() *vim25.Client {
	return currentVMwareFunctions.Client()
}

func createMockVMwareClient(endpoint string, accessKey string, secKey string, thumbprint string, uuid string) (*VMwareClient, error) {
	ep, _ := url.Parse(endpoint)
	ctx, cancel := context.WithCancel(context.Background())

	return &VMwareClient{
		conn:       &mockVMwareConnectionOperations{endpoint},
		cancel:     cancel,
		context:    ctx,
		moref:      "vm-1",
		thumbprint: thumbprint,
		username:   accessKey,
		password:   secKey,
		url:        ep,
		vm:         &mockVMwareVMOperations{},
	}, nil
}

func createMockNbdKitWrapper(vmware *VMwareClient, diskFileName, snapshot string) (*NbdKitWrapper, error) {
	u, _ := url.Parse("http://vcenter.test")
	return &NbdKitWrapper{
		n:      &image.Nbdkit{},
		Socket: u,
		Handle: &mockNbdOperations{},
	}, nil
}

func createVirtualDiskConfig(fileName string, key int32) *types.VirtualMachineConfigInfo {
	return &types.VirtualMachineConfigInfo{
		Hardware: types.VirtualHardware{
			Device: []types.BaseVirtualDevice{
				&types.VirtualDisk{
					DiskObjectId: "test-1",
					VirtualDevice: types.VirtualDevice{
						Key: key,
						Backing: &types.VirtualDiskFlatVer1BackingInfo{
							VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
								FileName: fileName,
							},
							Parent: &types.VirtualDiskFlatVer1BackingInfo{},
						},
					},
				},
			},
		},
	}
}
func createVirtualDiskConfigWithSize(fileName string, key int32, size int64) *types.VirtualMachineConfigInfo {
	return &types.VirtualMachineConfigInfo{
		Hardware: types.VirtualHardware{
			Device: []types.BaseVirtualDevice{
				&types.VirtualDisk{
					DiskObjectId:    "test-1",
					CapacityInBytes: size,
					VirtualDevice: types.VirtualDevice{
						Key: key,
						Backing: &types.VirtualDiskFlatVer1BackingInfo{
							VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
								FileName: fileName,
							},
							Parent: &types.VirtualDiskFlatVer1BackingInfo{},
						},
					},
				},
			},
		},
	}
}

func createSnapshots(name1 string, name2 string) *types.VirtualMachineSnapshotInfo {
	return &types.VirtualMachineSnapshotInfo{
		RootSnapshotList: []types.VirtualMachineSnapshotTree{
			{
				Snapshot: types.ManagedObjectReference{
					Type:  "VirtualMachineSnapshot",
					Value: name1,
				},
				ChildSnapshotList: []types.VirtualMachineSnapshotTree{
					{
						Snapshot: types.ManagedObjectReference{
							Type:  "VirtualMachineSnapshot",
							Value: name2,
						},
					},
				},
			},
		},
	}
}
