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
var newVddkDataSource = createVddkDataSource
var newVddkDataSink = createVddkDataSink

// VDDKDataSource is the data provider for vddk.
type VDDKDataSource struct {
	NbdKit *NbdKitWrapper
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
func NewVDDKDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string) (*VDDKDataSource, error) {
	return newVddkDataSource(endpoint, accessKey, secKey, thumbprint, uuid, backingFile)
}

func createVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string) (*VDDKDataSource, error) {
	vmware, err := CreateVMwareClient(endpoint, accessKey, secKey, thumbprint, uuid)
	if err != nil {
		klog.Errorf("Unable to log in to VMware: %v", err)
		return nil, err
	}

	nbdkit, err := createNbdKitWrapper(vmware, backingFile)
	vmware.Close()

	source := &VDDKDataSource{
		NbdKit: nbdkit,
	}
	return source, err
}

func createVddkDataSink(destinationFile string, size uint64) (VDDKDataSink, error) {
	file, err := os.OpenFile(destinationFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
func (vs *VDDKDataSource) Info() (ProcessingPhase, error) {
	size, err := vs.NbdKit.Handle.GetSize()
	if err != nil {
		klog.Errorf("Unable to get size from libnbd handle: %v", err)
		return ProcessingPhaseError, err
	}
	klog.Infof("Transferring %d-byte disk image...", size)
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

// TransferFile is called to transfer the data from the source to the file passed in.
func (vs *VDDKDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	size, err := vs.NbdKit.Handle.GetSize()
	if err != nil {
		klog.Errorf("Unable to get size from libnbd handle: %v", err)
		return ProcessingPhaseError, err
	}

	sink, err := newVddkDataSink(fileName, size)
	if err != nil {
		return ProcessingPhaseError, err
	}
	defer sink.Close()

	currentProgressBytes := uint64(0)
	previousProgressBytes := uint64(0)
	previousProgressPercent := uint(0)
	previousProgressTime := time.Now()
	initialProgressTime := time.Now()

	start := uint64(0)
	blocksize := uint64(MaxBlockStatusLength)
	for i := start; i < size; i += blocksize {
		if (size - i) < blocksize {
			blocksize = size - i
		}

		extent := []types.DiskChangeExtent{{
			Length: int64(blocksize),
			Start:  int64(i),
		}}
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
			// Only log progress at approximately 1% intervals.
			currentProgressBytes += uint64(written)
			currentProgressPercent := uint(100.0 * (float64(currentProgressBytes) / float64(size)))
			if currentProgressPercent > previousProgressPercent {
				progressMessage := fmt.Sprintf("Transferred %d/%d bytes (%d%%)", currentProgressBytes, size, currentProgressPercent)

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
