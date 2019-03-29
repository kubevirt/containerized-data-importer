package importer

import (
	"io"
	"net/url"
	"path/filepath"

	"k8s.io/klog"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

// UploadDataSource contains all the information need to upload data into a data volume.
// Sequence of phases:
// 1. Info -> Transfer (In Info phase the format readers are configured)
// 2. Transfer -> Process
// 3. Process -> Convert
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
	ud.readers, err = NewFormatReaders(ud.stream, cdiv1.DataVolumeKubeVirt)
	if err != nil {
		klog.Errorf("Error creating readers: %v", err)
		return Error, err
	}
	return Transfer, nil
}

// Transfer is called to transfer the data from the source to a temporary location.
func (ud *UploadDataSource) Transfer(path string) (ProcessingPhase, error) {
	if util.GetAvailableSpace(path) <= int64(0) {
		//Path provided is invalid.
		return Error, ErrInvalidPath
	}
	file := filepath.Join(path, tempFile)
	err := StreamDataToFile(ud.readers.TopReader(), file)
	if err != nil {
		return Error, err
	}
	// If we successfully wrote to the file, then the parse will succeed.
	ud.url, _ = url.Parse(file)
	return Process, nil
}

// Process is called to do any special processing before giving the url to the data back to the processor
func (ud *UploadDataSource) Process() (ProcessingPhase, error) {
	return Convert, nil
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
