package importer

import (
	"context"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pkg/errors"
	"google.golang.org/api/option"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	gcsFolderSep = "/"
	gcsScheme    = "gs"
)

// Helper for unit-testing
var newReaderFunc = getGcsObjectReader

// GCSDataSource is the struct containing the information needed to import from a GCS data source.
// Sequence of phases:
// 1. Info -> Transfer
// 2. Transfer -> Convert
type GCSDataSource struct {
	// GCS end point
	ep *url.URL
	// Key File
	keyFile string
	// Reader
	gcsReader io.ReadCloser
	// stack of readers
	readers *FormatReaders
	// The image file in scratch space.
	url *url.URL
}

// NewGCSDataSource creates a new instance of the GCSDataSource
func NewGCSDataSource(endpoint, keyFile string) (*GCSDataSource, error) {
	klog.V(3).Infoln("GCS Importer: New Data Source")

	// Placeholders
	var bucket, object, host string
	var options []option.ClientOption

	// Parsing Endpoint
	ep, err := ParseEndpoint(endpoint)

	if err != nil {
		return nil, errors.Wrapf(err, "GCS Importer: unable to parse endpoint %q", endpoint)
	}

	// Getting Context
	ctx, _ := context.WithTimeout(context.Background(), time.Second*60) //nolint:govet // todo - solve this: the cancel function returned by context.WithTimeout should be called, not discarded, to avoid a context leak

	if ep.Scheme == "gs" {
		// Using gs:// endpoint and extracting bucket and object name
		bucket, object = extractGcsBucketAndObject(endpoint)
	} else if ep.Scheme == "http" || ep.Scheme == "https" {
		// Using http(s):// endpoint and extracting bucket, object name and host
		bucket, object, host = extractGcsBucketObjectAndHost(endpoint)
		options = append(options, option.WithEndpoint(host))
	}

	// Creating GCS Client
	client, err := getGcsClient(ctx, keyFile, options...)

	if err != nil {
		klog.Errorf("GCS Importer: Error creating GCS Client")
		return nil, err
	}

	// Creating GCS Reader
	gcsReader, err := newReaderFunc(ctx, client, bucket, object)
	if err != nil {
		klog.Errorf("GCS Importer: Error creating Reader")
		return nil, err
	}

	return &GCSDataSource{
		ep:        ep,
		keyFile:   keyFile,
		gcsReader: gcsReader,
	}, nil
}

// Info is called to get initial information about the data.
func (sd *GCSDataSource) Info() (ProcessingPhase, error) {
	var err error
	sd.readers, err = NewFormatReaders(sd.gcsReader, uint64(0))
	if err != nil {
		klog.Errorf("GCS Importer: Error creating readers: %v", err)
		return ProcessingPhaseError, err
	}
	if !sd.readers.Convert {
		// Downloading a raw file, we can write that directly to the target.
		return ProcessingPhaseTransferDataFile, nil
	}

	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source to a temporary location.
func (sd *GCSDataSource) Transfer(path string) (ProcessingPhase, error) {
	klog.V(3).Infoln("GCS Importer: Transfer")
	file := filepath.Join(path, tempFile)

	if err := CleanAll(file); err != nil {
		return ProcessingPhaseError, err
	}

	size, _ := GetAvailableSpace(path)

	if size <= int64(0) {
		//Path provided is invalid.
		klog.V(3).Infoln("GCS Importer: Transfer Error: ", ErrInvalidPath)
		return ProcessingPhaseError, ErrInvalidPath
	}

	_, _, err := StreamDataToFile(sd.readers.TopReader(), file, true)
	if err != nil {
		klog.V(3).Infoln("GCS Importer: Transfer Error: ", err)
		return ProcessingPhaseError, err
	}
	// If streaming succeeded, then parsing the file into URL will also succeed, no need to check error status
	sd.url, _ = url.Parse(file)
	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (sd *GCSDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	if err := CleanAll(fileName); err != nil {
		return ProcessingPhaseError, err
	}

	_, _, err := StreamDataToFile(sd.readers.TopReader(), fileName, true)
	if err != nil {
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseResize, nil
}

// GetURL returns the url that the data processor can use when converting the data.
func (sd *GCSDataSource) GetURL() *url.URL {
	return sd.url
}

// GetTerminationMessage returns data to be serialized and used as the termination message of the importer.
func (sd *GCSDataSource) GetTerminationMessage() *common.TerminationMessage {
	return nil
}

// Close closes any readers or other open resources.
func (sd *GCSDataSource) Close() error {
	var err error
	if sd.readers != nil {
		err = sd.readers.Close()
	}
	return err
}

// Create a Cloud Storage Client
func getGcsClient(ctx context.Context, keyFile string, options ...option.ClientOption) (*storage.Client, error) {
	klog.V(3).Infoln("GCS Importer: Creating Client")
	if keyFile == "" {
		options = append(options, option.WithoutAuthentication())
		klog.V(3).Infoln("GCS Importer: Authentication: Anonymous")
	}
	return storage.NewClient(ctx, options...)
}

// Create Cloud Storage Object Reader
func getGcsObjectReader(ctx context.Context, client *storage.Client, bucket, object string) (io.ReadCloser, error) {
	klog.V(3).Infoln("GCS Importer: Creating Reader for bucket:", bucket, "object:", object)
	return client.Bucket(bucket).Object(object).NewReader(ctx)
}

// Extract url in format gs://bucket/filename or gs://bucket/subdir/filename
func extractGcsBucketAndObject(s string) (string, string) {
	klog.V(3).Infoln("GCS Importer: Extracting GCS Bucket and Object")
	pathSplit := strings.Split(s, gcsFolderSep)
	bucket := pathSplit[2]
	object := strings.Join(pathSplit[3:], gcsFolderSep)
	klog.V(3).Infoln("GCS Importer: GCS Bucket:", bucket)
	klog.V(3).Infoln("GCS Importer: GCS Object:", object)
	return bucket, object
}

// Extract url in format https://storage.cloud.google.com/bucket/filename
func extractGcsBucketObjectAndHost(s string) (string, string, string) {
	klog.V(3).Infoln("GCS Importer: Extracting GCS Bucket, Object and Host")
	pathSplit := strings.Split(s, gcsFolderSep)
	host := strings.Join(pathSplit[:3], gcsFolderSep) + "/"
	bucket := pathSplit[3]
	object := strings.Join(pathSplit[4:], gcsFolderSep)
	klog.V(3).Infoln("GCS Importer: GCS Host:", host)
	klog.V(3).Infoln("GCS Importer: GCS Bucket:", bucket)
	klog.V(3).Infoln("GCS Importer: GCS Object:", object)
	return bucket, object, host
}
