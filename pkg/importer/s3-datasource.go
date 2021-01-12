package importer

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

const s3FolderSep = "/"

// S3Client is the interface to the used S3 client.
type S3Client interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
}

// may be overridden in tests
var newClientFunc = getS3Client

// S3DataSource is the struct containing the information needed to import from an S3 data source.
// Sequence of phases:
// 1. Info -> Transfer
// 2. Transfer -> Convert
type S3DataSource struct {
	// S3 end point
	ep *url.URL
	// User name
	accessKey string
	// Password
	secKey string
	// Reader
	s3Reader io.ReadCloser
	// stack of readers
	readers *FormatReaders
	// The image file in scratch space.
	url *url.URL
}

// NewS3DataSource creates a new instance of the S3DataSource
func NewS3DataSource(endpoint, accessKey, secKey string) (*S3DataSource, error) {
	ep, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", endpoint))
	}
	s3Reader, err := createS3Reader(ep, accessKey, secKey)
	if err != nil {
		return nil, err
	}
	return &S3DataSource{
		ep:        ep,
		accessKey: accessKey,
		secKey:    secKey,
		s3Reader:  s3Reader,
	}, nil
}

// Info is called to get initial information about the data.
func (sd *S3DataSource) Info() (ProcessingPhase, error) {
	var err error
	sd.readers, err = NewFormatReaders(sd.s3Reader, uint64(0))
	if err != nil {
		klog.Errorf("Error creating readers: %v", err)
		return ProcessingPhaseError, err
	}
	if !sd.readers.Convert {
		// Downloading a raw file, we can write that directly to the target.
		return ProcessingPhaseTransferDataFile, nil
	}

	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source to a temporary location.
func (sd *S3DataSource) Transfer(path string) (ProcessingPhase, error) {
	size, err := util.GetAvailableSpace(path)
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}
	file := filepath.Join(path, tempFile)
	err = util.StreamDataToFile(sd.readers.TopReader(), file)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If streaming succeeded, then parsing the file into URL will also succeed, no need to check error status
	sd.url, _ = url.Parse(file)
	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (sd *S3DataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	err := util.StreamDataToFile(sd.readers.TopReader(), fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseResize, nil
}

// GetURL returns the url that the data processor can use when converting the data.
func (sd *S3DataSource) GetURL() *url.URL {
	return sd.url
}

// Close closes any readers or other open resources.
func (sd *S3DataSource) Close() error {
	var err error
	if sd.readers != nil {
		err = sd.readers.Close()
	}
	return err
}

func createS3Reader(ep *url.URL, accessKey, secKey string) (io.ReadCloser, error) {
	klog.V(3).Infoln("Using S3 client to get data")

	endpoint := ep.Host
	klog.Infof("Endpoint %s", endpoint)
	path := strings.Trim(ep.Path, "/")
	bucket, object := extractBucketAndObject(path)

	klog.V(1).Infof("bucket %s", bucket)
	klog.V(1).Infof("object %s", object)
	svc, err := newClientFunc(endpoint, accessKey, secKey)
	if err != nil {
		return nil, errors.Wrapf(err, "could not build s3 client for %q", ep.Host)
	}

	objInput := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	}
	objOutput, err := svc.GetObject(objInput)
	if err != nil {
		return nil, errors.Wrapf(err, "could not get s3 object: \"%s/%s\"", bucket, object)
	}
	objectReader := objOutput.Body
	return objectReader, nil
}

func getS3Client(endpoint, accessKey, secKey string) (S3Client, error) {
	creds := credentials.NewStaticCredentials(accessKey, secKey, "")
	region := extractRegion(endpoint)
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		Credentials:      creds,
		S3ForcePathStyle: aws.Bool(true),
	},
	)
	if err != nil {
		return nil, err
	}

	svc := s3.New(sess)
	return svc, nil
}

func extractRegion(s string) string {
	var region string
	r, _ := regexp.Compile("s3\\.(.+)\\.amazonaws\\.com")
	if matches := r.FindStringSubmatch(s); matches != nil {
		region = matches[1]
	} else {
		region = strings.Split(s, ".")[0]
	}

	return region
}

func extractBucketAndObject(s string) (string, string) {
	pathSplit := strings.Split(s, s3FolderSep)
	bucket := pathSplit[0]
	object := strings.Join(pathSplit[1:], s3FolderSep)
	return bucket, object
}
