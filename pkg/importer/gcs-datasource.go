package importer

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

type GCSDataSource struct {
	// GCS endpoint
	ep *url.URL
	// Stack of readers
	readers *FormatReaders
	// Reader
	gcsReader *storage.Reader
	// The image file in scratch space
	url *url.URL
}

func NewGCSDataSource(endpoint string, saKey string) (*GCSDataSource, error) {
	ep, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", endpoint))
	}

	var client *storage.Client
	ctx := context.Background()
	if len(saKey) > 0 {
		client, err = storage.NewClient(ctx, option.WithCredentialsJSON([]byte(saKey)))
	} else {
		client, err = storage.NewClient(ctx)
	}
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to create GCS client: %v", err))
	}

	bucket := ep.Host
	object := ep.Path[1:]
	reader, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, errors.Wrapf(
			err, fmt.Sprintf("unable to create reader for bucket %s and object %s: %v", bucket, object, err))
	}

	return &GCSDataSource{
		ep:        ep,
		gcsReader: reader,
	}, nil
}

// Info is called to get initial information about the data.
func (gd *GCSDataSource) Info() (ProcessingPhase, error) {
	var err error
	gd.readers, err = NewFormatReaders(gd.gcsReader, uint64(0))
	if err != nil {
		klog.Errorf("Error creating readers: %v", err)
		return ProcessingPhaseError, err
	}
	if !gd.readers.Convert {
		// Downloading a raw file, we can write that directly to the target.
		return ProcessingPhaseTransferDataFile, nil
	}

	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source to the path passed in.
func (gd *GCSDataSource) Transfer(path string) (ProcessingPhase, error) {
	size, _ := util.GetAvailableSpace(path)
	if size <= int64(0) {
		// Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}
	file := filepath.Join(path, tempFile)
	err := util.StreamDataToFile(gd.readers.TopReader(), file)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If streaming succeeded, then parsing the file into URL will also succeed, no need to check error status
	gd.url, _ = url.Parse(file)
	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the file passed in.
func (gd *GCSDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	err := util.StreamDataToFile(gd.readers.TopReader(), fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseResize, nil
}

// Geturl returns the url that the data processor can use when converting the data.
func (gd *GCSDataSource) GetURL() *url.URL {
	return gd.url
}

// Close closes any readers or other open resources.
func (gd *GCSDataSource) Close() error {
	var err error
	if gd.readers != nil {
		err = gd.readers.Close()
	}
	return err
}
