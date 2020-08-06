/*
Copyright 2018 The CDI Authors.

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
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"

	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

var qemuOperations = image.NewQEMUOperations()

// ProcessingPhase is the current phase being processed.
type ProcessingPhase string

const (
	// ProcessingPhaseInfo is the first phase, during this phase the source obtains information needed to determine which phase to go to next.
	ProcessingPhaseInfo ProcessingPhase = "Info"
	// ProcessingPhaseTransferScratch is the phase in which the data source writes data to the scratch space.
	ProcessingPhaseTransferScratch ProcessingPhase = "TransferScratch"
	// ProcessingPhaseTransferDataDir is the phase in which the data source writes data directly to the target path without conversion.
	ProcessingPhaseTransferDataDir ProcessingPhase = "TransferDataDir"
	// ProcessingPhaseTransferDataFile is the phase in which the data source writes data directly to the target file without conversion.
	ProcessingPhaseTransferDataFile ProcessingPhase = "TransferDataFile"
	// ProcessingPhaseValidatePause is the phase in which the data processor should validate and then pause.
	ProcessingPhaseValidatePause ProcessingPhase = "ValidatePause"
	// ProcessingPhaseProcess is the phase in which the data source processes the data just written to the scratch space.
	ProcessingPhaseProcess ProcessingPhase = "Process"
	// ProcessingPhaseConvert is the phase in which the data is taken from the url provided by the source, and it is converted to the target RAW disk image format.
	// The url can be an http end point or file system end point.
	ProcessingPhaseConvert ProcessingPhase = "Convert"
	// ProcessingPhaseResize the disk image, this is only needed when the target contains a file system (block device do not need a resize)
	ProcessingPhaseResize ProcessingPhase = "Resize"
	// ProcessingPhaseComplete is the phase where the entire process completed successfully and we can exit gracefully.
	ProcessingPhaseComplete ProcessingPhase = "Complete"
	// ProcessingPhasePause is the phase where we pause processing and end the loop, and expect something to call the process loop again.
	ProcessingPhasePause ProcessingPhase = "Pause"
	// ProcessingPhaseError is the phase in which we encountered an error and need to exit ungracefully.
	ProcessingPhaseError ProcessingPhase = "Error"
)

// ValidationSizeError is an error indication size validation failure.
type ValidationSizeError struct {
	err error
}

func (e ValidationSizeError) Error() string { return e.err.Error() }

// ErrRequiresScratchSpace indicates that we require scratch space.
var ErrRequiresScratchSpace = fmt.Errorf("scratch space required and none found")

// ErrInvalidPath indicates that the path is invalid.
var ErrInvalidPath = fmt.Errorf("invalid transfer path")

// may be overridden in tests
var getAvailableSpaceBlockFunc = util.GetAvailableSpaceBlock
var getAvailableSpaceFunc = util.GetAvailableSpace

// DataSourceInterface is the interface all data sources should implement.
type DataSourceInterface interface {
	// Info is called to get initial information about the data.
	Info() (ProcessingPhase, error)
	// Transfer is called to transfer the data from the source to the path passed in.
	Transfer(path string) (ProcessingPhase, error)
	// TransferFile is called to transfer the data from the source to the file passed in.
	TransferFile(fileName string) (ProcessingPhase, error)
	// Process is called to do any special processing before giving the url to the data back to the processor
	Process() (ProcessingPhase, error)
	// Geturl returns the url that the data processor can use when converting the data.
	GetURL() *url.URL
	// Close closes any readers or other open resources.
	Close() error
}

//ResumableDataSource is the interface all resumeable data sources should implement
type ResumableDataSource interface {
	DataSourceInterface
	GetResumePhase() ProcessingPhase
}

// DataProcessor holds the fields needed to process data from a data provider.
type DataProcessor struct {
	// currentPhase is the phase the processing is in currently.
	currentPhase ProcessingPhase
	// provider provides the data for processing.
	source DataSourceInterface
	// destination file. will be DataDir/disk.img if file system, or a block device (if a block device, then DataDir will not exist).
	dataFile string
	// dataDir path to target directory if it contains a file system.
	dataDir string
	// scratchDataDir path to the scratch space.
	scratchDataDir string
	// requestImageSize is the size we want the resulting image to be.
	requestImageSize string
	// available space is the available space before downloading the image
	availableSpace int64
}

// NewDataProcessor create a new instance of a data processor using the passed in data provider.
func NewDataProcessor(dataSource DataSourceInterface, dataFile, dataDir, scratchDataDir, requestImageSize string) *DataProcessor {
	dp := &DataProcessor{
		currentPhase:     ProcessingPhaseInfo,
		source:           dataSource,
		dataFile:         dataFile,
		dataDir:          dataDir,
		scratchDataDir:   scratchDataDir,
		requestImageSize: requestImageSize,
	}
	// Calculate available space before doing anything.
	dp.availableSpace = dp.calculateTargetSize()
	return dp
}

// ProcessData is the main synchronous processing loop
func (dp *DataProcessor) ProcessData() error {
	if size, _ := util.GetAvailableSpace(dp.scratchDataDir); size > int64(0) {
		// Clean up before trying to write, in case a previous attempt left a mess. Note the deferred cleanup is intentional.
		if err := CleanDir(dp.scratchDataDir); err != nil {
			return errors.Wrap(err, "Failure cleaning up temporary scratch space")
		}
		// Attempt to be a good citizen and clean up my mess at the end.
		defer CleanDir(dp.scratchDataDir)
	}

	if size, _ := util.GetAvailableSpace(dp.dataDir); size > int64(0) {
		// Clean up data dir before trying to write in case a previous attempt failed and left some stuff behind.
		if err := CleanDir(dp.dataDir); err != nil {
			return errors.Wrap(err, "Failure cleaning up target space")
		}
	}
	return dp.ProcessDataWithPause()
}

// ProcessDataResume Resume a paused processor, assumes the provided data source is ResumableDataSource
func (dp *DataProcessor) ProcessDataResume() error {
	rds, ok := dp.source.(ResumableDataSource)
	if !ok {
		return errors.New("Datasource not resumable")
	}
	klog.Infof("Resuming processing at phase %s", rds.GetResumePhase())
	dp.currentPhase = rds.GetResumePhase()
	return dp.ProcessDataWithPause()
}

// ProcessDataWithPause is the main processing loop.
func (dp *DataProcessor) ProcessDataWithPause() error {
	var err error
	for dp.currentPhase != ProcessingPhaseComplete && dp.currentPhase != ProcessingPhasePause {
		switch dp.currentPhase {
		case ProcessingPhaseInfo:
			dp.currentPhase, err = dp.source.Info()
			if err != nil {
				err = errors.Wrap(err, "Unable to obtain information about data source")
			}
		case ProcessingPhaseTransferScratch:
			dp.currentPhase, err = dp.source.Transfer(dp.scratchDataDir)
			if err == ErrInvalidPath {
				// Passed in invalid scratch space path, return scratch space needed error.
				err = ErrRequiresScratchSpace
			} else if err != nil {
				err = errors.Wrap(err, "Unable to transfer source data to scratch space")
			}
		case ProcessingPhaseTransferDataDir:
			dp.currentPhase, err = dp.source.Transfer(dp.dataDir)
			if err != nil {
				err = errors.Wrap(err, "Unable to transfer source data to target directory")
			}
		case ProcessingPhaseTransferDataFile:
			dp.currentPhase, err = dp.source.TransferFile(dp.dataFile)
			if err != nil {
				err = errors.Wrap(err, "Unable to transfer source data to target file")
			}
		case ProcessingPhaseValidatePause:
			validateErr := dp.validate(dp.source.GetURL())
			if validateErr != nil {
				dp.currentPhase = ProcessingPhaseError
				err = validateErr
			}
			dp.currentPhase = ProcessingPhasePause
		case ProcessingPhaseProcess:
			dp.currentPhase, err = dp.source.Process()
			if err != nil {
				err = errors.Wrap(err, "Unable to process source data to intermediate state before transferring to target")
			}
		case ProcessingPhaseConvert:
			dp.currentPhase, err = dp.convert(dp.source.GetURL())
			if err != nil {
				err = errors.Wrap(err, "Unable to convert source data to target format")
			}
		case ProcessingPhaseResize:
			dp.currentPhase, err = dp.resize()
			if err != nil {
				err = errors.Wrap(err, "Unable to resize disk image to requested size")
			}
		default:
			return errors.Errorf("Unknown processing phase %s", dp.currentPhase)
		}
		if err != nil {
			klog.Errorf("%+v", err)
			return err
		}
		klog.V(1).Infof("New phase: %s\n", dp.currentPhase)
	}
	return err
}

func (dp *DataProcessor) validate(url *url.URL) error {
	klog.V(1).Infoln("Validating image")
	err := qemuOperations.Validate(url, dp.availableSpace)
	if err != nil {
		return ValidationSizeError{err: err}
	}
	return nil
}

// convert is called when convert the image from the url to a RAW disk image. Source formats include RAW/QCOW2 (Raw to raw conversion is a copy)
func (dp *DataProcessor) convert(url *url.URL) (ProcessingPhase, error) {
	err := dp.validate(url)
	if err != nil {
		return ProcessingPhaseError, err
	}
	klog.V(3).Infoln("Converting to Raw")
	err = qemuOperations.ConvertToRawStream(url, dp.dataFile)
	if err != nil {
		return ProcessingPhaseError, errors.Wrap(err, "Conversion to Raw failed")
	}

	return ProcessingPhaseResize, nil
}

func (dp *DataProcessor) resize() (ProcessingPhase, error) {
	// Resize only if we have a resize request, and if the image is on a file system pvc.
	size, _ := getAvailableSpaceBlockFunc(dp.dataFile)
	klog.V(3).Infof("Available space in dataFile: %d", size)
	if dp.requestImageSize != "" && size < int64(0) {
		klog.V(3).Infoln("Resizing image")
		err := ResizeImage(dp.dataFile, dp.requestImageSize, dp.availableSpace)
		if err != nil {
			return ProcessingPhaseError, errors.Wrap(err, "Resize of image failed")
		}
	}
	if dp.dataFile != "" {
		// Change permissions to 0660
		err := os.Chmod(dp.dataFile, 0660)
		if err != nil {
			err = errors.Wrap(err, "Unable to change permissions of target file")
		}
	}
	return ProcessingPhaseComplete, nil
}

// ResizeImage resizes the images to match the requested size. Sometimes provisioners misbehave and the available space
// is not the same as the requested space. For those situations we compare the available space to the requested space and
// use the smallest of the two values.
func ResizeImage(dataFile, imageSize string, totalTargetSpace int64) error {
	dataFileURL, _ := url.Parse(dataFile)
	info, err := qemuOperations.Info(dataFileURL)
	if err != nil {
		return err
	}
	if imageSize != "" {
		currentImageSizeQuantity := resource.NewScaledQuantity(info.VirtualSize, 0)
		newImageSizeQuantity := resource.MustParse(imageSize)
		minSizeQuantity := util.MinQuantity(resource.NewScaledQuantity(totalTargetSpace, 0), &newImageSizeQuantity)
		if minSizeQuantity.Cmp(newImageSizeQuantity) != 0 {
			// Available destination space is smaller than the size we want to resize to
			klog.Warningf("Available space less than requested size, resizing image to available space %s.\n", minSizeQuantity.String())
		}
		if currentImageSizeQuantity.Cmp(minSizeQuantity) == 0 {
			klog.V(1).Infof("No need to resize image. Requested size: %s, Image size: %d.\n", imageSize, info.VirtualSize)
			return nil
		}
		klog.V(1).Infof("Expanding image size to: %s\n", minSizeQuantity.String())
		return qemuOperations.Resize(dataFile, minSizeQuantity)
	}
	return errors.New("Image resize called with blank resize")
}

func (dp *DataProcessor) calculateTargetSize() int64 {
	klog.V(1).Infof("Calculating available size\n")
	var targetQuantity *resource.Quantity
	size, err := getAvailableSpaceBlockFunc(dp.dataFile)
	if err != nil {
		klog.Error(err)
	}
	if size >= int64(0) {
		// Block volume.
		klog.V(1).Infof("Checking out block volume size.\n")
		targetQuantity = resource.NewScaledQuantity(size, 0)
	} else {
		// File system volume.
		klog.V(1).Infof("Checking out file system volume size.\n")
		size, err := getAvailableSpaceFunc(dp.dataDir)
		if err != nil {
			klog.Error(err)
		}
		targetQuantity = resource.NewScaledQuantity(size, 0)
	}
	if dp.requestImageSize != "" {
		klog.V(1).Infof("Request image size not empty.\n")
		newImageSizeQuantity := resource.MustParse(dp.requestImageSize)
		minQuantity := util.MinQuantity(targetQuantity, &newImageSizeQuantity)
		targetQuantity = &minQuantity
	}
	klog.V(1).Infof("Target size %s.\n", targetQuantity.String())
	targetSize, _ := targetQuantity.AsInt64()
	return targetSize
}
