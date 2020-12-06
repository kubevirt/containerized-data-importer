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
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"golang.org/x/sys/unix"
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
	err = unix.Fadvise(int(file.Fd()), 0, int64(size), unix.MADV_SEQUENTIAL)
	if err != nil {
		klog.Warningf("Error with sequential fadvise: %v", err)
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

	sink, err := newVddkDataSink(destinationFile, size)
	if err != nil {
		return ProcessingPhaseError, err
	}
	defer sink.Close()

	start := uint64(0)
	lastProgressPercent := uint(0)
	lastProgressBytes := uint64(0)
	lastProgressTime := time.Now()
	initialProgressTime := time.Now()
	blocksize := uint64(23 * 1024 * 1024)
	buf := make([]byte, blocksize)
	for i := start; i < size; i += blocksize {
		if (size - i) < blocksize {
			blocksize = size - i
			buf = make([]byte, blocksize)
		}

		err = vs.NbdKit.Handle.Pread(buf, i, nil)
		if err != nil {
			klog.Errorf("Failed to read from data source at offset %d! First error was: %v", i, err)
			retryErr := vs.NbdKit.Handle.Pread(buf, i, nil)
			if retryErr != nil {
				klog.Errorf("Retry error was: %v", retryErr)
				return ProcessingPhaseError, err
			}
			klog.Infof("Retry was successful.")
		}

		written, err := sink.Write(buf)
		if err != nil {
			klog.Errorf("Failed to write source data to destination: %v", err)
			return ProcessingPhaseError, err
		}
		if uint64(written) < blocksize {
			klog.Errorf("Failed to write whole buffer to destination! Wrote %d/%d bytes.", written, blocksize)
			return ProcessingPhaseError, errors.New("failed to write whole buffer to destination")
		}

		// Only log progress at approximately 1% intervals.
		currentProgressBytes := i + uint64(written)
		currentProgressPercent := uint(100.0 * (float64(currentProgressBytes) / float64(size)))
		if currentProgressPercent > lastProgressPercent {
			progressMessage := fmt.Sprintf("Transferred %d/%d bytes (%d%%)", currentProgressBytes, size, currentProgressPercent)

			currentProgressTime := time.Now()
			overallProgressTime := uint64(time.Since(initialProgressTime).Seconds())
			if overallProgressTime > 0 {
				overallProgressRate := currentProgressBytes / overallProgressTime
				progressMessage += fmt.Sprintf(" at %d bytes/second overall", overallProgressRate)
			}

			progressTimeDifference := uint64(currentProgressTime.Sub(lastProgressTime).Seconds())
			if progressTimeDifference > 0 {
				progressSize := currentProgressBytes - lastProgressBytes
				progressRate := progressSize / progressTimeDifference
				progressMessage += fmt.Sprintf(", last 1%% was %d bytes at %d bytes/second", progressSize, progressRate)
			}

			klog.Info(progressMessage)

			lastProgressBytes = currentProgressBytes
			lastProgressTime = currentProgressTime
			lastProgressPercent = currentProgressPercent
		}
		v := float64(currentProgressPercent)
		metric := &dto.Metric{}
		err = progress.WithLabelValues(ownerUID).Write(metric)
		if err == nil && v > 0 && v > *metric.Counter.Value {
			progress.WithLabelValues(ownerUID).Add(v - *metric.Counter.Value)
		}
	}

	return ProcessingPhaseComplete, nil
}
