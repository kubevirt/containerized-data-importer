package importer

import (
	"io"
	"net/url"
	"path/filepath"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

// UploadDataSource contains all the information need to upload data into a data volume.
// Sequence of phases:
// 1a. ProcessingPhaseInfo -> ProcessingPhaseTransferScratch (In Info phase the format readers are configured) In case the readers don't contain a raw file.
// 1b. ProcessingPhaseInfo -> ProcessingPhaseTransferDataFile, in the case the readers contain a raw file.
// 2a. ProcessingPhaseTransferScratch -> ProcessingPhaseConvert
// 2b. ProcessingPhaseTransferDataFile -> ProcessingPhaseResize
type UploadDataSource struct {
	// Data strean
	stream io.ReadCloser
	// stack of readers
	readers *FormatReaders
	// url to a file in scratch space.
	url *url.URL
}

// NewUploadDataSource creates a new instance of an UploadDataSource
func NewUploadDataSource(stream io.ReadCloser) *UploadDataSource {
	return &UploadDataSource{
		stream: stream,
	}
}

// Info is called to get initial information about the data.
func (ud *UploadDataSource) Info() (ProcessingPhase, error) {
	var err error
	// Hardcoded to only accept kubevirt content type.
	ud.readers, err = NewFormatReaders(ud.stream, uint64(0))
	if err != nil {
		klog.Errorf("Error creating readers: %v", err)
		return ProcessingPhaseError, err
	}
	if !ud.readers.Convert {
		// Uploading a raw file, we can write that directly to the target.
		return ProcessingPhaseTransferDataFile, nil
	}
	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source to the passed in path.
func (ud *UploadDataSource) Transfer(path string) (ProcessingPhase, error) {
	size, err := util.GetAvailableSpace(path)
	if err != nil {
		return ProcessingPhaseError, err
	}
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}
	file := filepath.Join(path, tempFile)
	err = util.StreamDataToFile(ud.readers.TopReader(), file)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If we successfully wrote to the file, then the parse will succeed.
	ud.url, _ = url.Parse(file)
	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (ud *UploadDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	err := util.StreamDataToFile(ud.readers.TopReader(), fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If we successfully wrote to the file, then the parse will succeed.
	ud.url, _ = url.Parse(fileName)
	return ProcessingPhaseResize, nil
}

// GetURL returns the url that the data processor can use when converting the data.
func (ud *UploadDataSource) GetURL() *url.URL {
	return ud.url
}

// Close closes any readers or other open resources.
func (ud *UploadDataSource) Close() error {
	if ud.stream != nil {
		return ud.stream.Close()
	}
	return nil
}

// AsyncUploadDataSource is an asynchronouse version of an upload data source, that returns finished phase instead
// of going to post upload processing phases.
type AsyncUploadDataSource struct {
	uploadDataSource UploadDataSource
	// Next Phase indicates what the next Processing Phase should be after the transfer completes.
	ResumePhase ProcessingPhase
}

// NewAsyncUploadDataSource creates a new instance of an UploadDataSource
func NewAsyncUploadDataSource(stream io.ReadCloser) *AsyncUploadDataSource {
	return &AsyncUploadDataSource{
		uploadDataSource: UploadDataSource{
			stream: stream,
		},
		ResumePhase: ProcessingPhaseInfo,
	}
}

// Info is called to get initial information about the data.
func (aud *AsyncUploadDataSource) Info() (ProcessingPhase, error) {
	return aud.uploadDataSource.Info()
}

// Transfer is called to transfer the data from the source to the passed in path.
func (aud *AsyncUploadDataSource) Transfer(path string) (ProcessingPhase, error) {
	size, err := util.GetAvailableSpace(path)
	if err != nil {
		return ProcessingPhaseError, err
	}
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}
	file := filepath.Join(path, tempFile)
	err = util.StreamDataToFile(aud.uploadDataSource.readers.TopReader(), file)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If we successfully wrote to the file, then the parse will succeed.
	aud.uploadDataSource.url, _ = url.Parse(file)
	aud.ResumePhase = ProcessingPhaseConvert
	return ProcessingPhaseValidatePause, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (aud *AsyncUploadDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	err := util.StreamDataToFile(aud.uploadDataSource.readers.TopReader(), fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If we successfully wrote to the file, then the parse will succeed.
	aud.uploadDataSource.url, _ = url.Parse(fileName)
	aud.ResumePhase = ProcessingPhaseResize
	return ProcessingPhaseValidatePause, nil
}

// Close closes any readers or other open resources.
func (aud *AsyncUploadDataSource) Close() error {
	return aud.uploadDataSource.Close()
}

// GetURL returns the url that the data processor can use when converting the data.
func (aud *AsyncUploadDataSource) GetURL() *url.URL {
	return aud.uploadDataSource.GetURL()
}

// GetResumePhase returns the next phase to process when resuming
func (aud *AsyncUploadDataSource) GetResumePhase() ProcessingPhase {
	return aud.ResumePhase
}
