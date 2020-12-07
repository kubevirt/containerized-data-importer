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
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/vmware/govmomi/vim25/types"
	"k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

// May be overridden in tests
var newVddkIncrementalDataSource = createVddkIncrementalDataSource
var newVddkIncrementalDataSink = createVddkIncrementalDataSink

// VDDKIncrementalDataSource is the data provider for an incremental copy from vddk.
type VDDKIncrementalDataSource struct {
	NbdKit   *NbdKitWrapper
	VMware   *VMwareClient
	Changed  *types.DiskChangeInfo
	Blocks   []*BlockStatusData // Filled out in Info phase
	Previous string
	Current  string
	Size     uint64
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

// NewVDDKIncrementalDataSource creates a new instance of the vddk delta copy provider.
func NewVDDKIncrementalDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string) (*VDDKIncrementalDataSource, error) {
	return newVddkIncrementalDataSource(endpoint, accessKey, secKey, thumbprint, uuid, backingFile, currentCheckpoint, previousCheckpoint, finalCheckpoint)
}

func createVddkIncrementalDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string) (*VDDKIncrementalDataSource, error) {
	// Log in to VMware
	vmware, err := CreateVMwareClient(endpoint, accessKey, secKey, thumbprint, uuid)
	if err != nil {
		klog.Errorf("Unable to log in to VMware: %v", err)
		return nil, err
	}

	// Find current disk
	disk, err := vmware.FindDisk(backingFile)
	if err != nil {
		klog.Errorf("Could not find VM disk %s: %v", backingFile, err)
		return nil, err
	}

	// Find starting snapshot
	previous, err := vmware.vm.FindSnapshot(vmware.context, previousCheckpoint)
	if err != nil {
		klog.Errorf("Could not find previous snapshot %s: %v", previousCheckpoint, err)
		return nil, err
	}

	// Find current snapshot
	current, err := vmware.vm.FindSnapshot(vmware.context, currentCheckpoint)
	if err != nil {
		klog.Errorf("Could not find current snapshot %s: %v", currentCheckpoint, err)
		return nil, err
	}

	// Find changed disk areas between the two snapshots
	changed, err := vmware.vm.QueryChangedDiskAreas(vmware.context, previous, current, disk, 0)
	if err != nil {
		klog.Errorf("Unable to query changed areas: %s", err)
		return nil, err
	}

	nbdkit, err := createNbdKitWrapper(vmware, backingFile)
	klog.Infof("Checkpoints: %s %s %s", currentCheckpoint, previousCheckpoint, finalCheckpoint)
	vmware.Close()

	size := uint64(0)
	for _, change := range changed.ChangedArea {
		size += uint64(change.Length)
	}

	source := &VDDKIncrementalDataSource{
		NbdKit:   nbdkit,
		VMware:   vmware,
		Changed:  &changed,
		Previous: previous.Value,
		Current:  current.Value,
		Size:     size,
	}
	return source, err
}

func createVddkIncrementalDataSink(destinationFile string) (VDDKDataSink, error) {
	file, err := os.OpenFile(destinationFile, os.O_WRONLY, 0644)
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

// Info is called to get initial information about the data.
func (vs *VDDKIncrementalDataSource) Info() (ProcessingPhase, error) {
	klog.Infof("Getting block status information for %d-byte disk image delta...", vs.Size)
	blocks, err := GetBlockStatus(vs.NbdKit.Handle, vs.Changed.ChangedArea)
	if err != nil {
		return ProcessingPhaseError, err
	}
	vs.Blocks = blocks
	return ProcessingPhaseTransferDataFile, nil
}

// Close closes any readers or other open resources.
func (vs *VDDKIncrementalDataSource) Close() error {
	vs.NbdKit.Handle.Close()
	return vs.NbdKit.Command.Process.Kill()
}

// GetURL returns the url that the data processor can use when converting the data.
func (vs *VDDKIncrementalDataSource) GetURL() *url.URL {
	return vs.NbdKit.Socket
}

// Transfer is called to transfer the data from the source to the path passed in.
func (vs *VDDKIncrementalDataSource) Transfer(path string) (ProcessingPhase, error) {
	return ProcessingPhaseTransferDataFile, nil
}

// TransferFile is called to transfer the data from the source to the file passed in.
func (vs *VDDKIncrementalDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	if len(vs.Changed.ChangedArea) < 1 {
		klog.Infof("No changes reported between snapshot %s and snapshot %s!\n", vs.Previous, vs.Current)
		return ProcessingPhaseComplete, nil
	}

	sink, err := newVddkIncrementalDataSink(fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	defer sink.Close()

	currentProgressBytes := uint64(0)
	previousProgressBytes := uint64(0)
	previousProgressPercent := uint(0)
	previousProgressTime := time.Now()
	initialProgressTime := time.Now()

	for _, block := range vs.Blocks {
		written, err := CopyRange(vs.NbdKit.Handle, sink, block)
		if err != nil {
			klog.Errorf("Unable to copy block at offset %d: %v", block.Offset, err)
			return ProcessingPhaseError, err
		}

		// Only log progress at approximately 1% intervals.
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

	cmd := exec.Command("md5sum", fileName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		klog.Errorf("Error getting MD5 sum: %v", err)
	} else {
		klog.Infof("MD5 sum: %s", output)
	}

	return ProcessingPhaseComplete, nil
}
