package importer

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/minio/minio-go"
)

type DataStreamInterface interface {
	DataStreamSelector() (io.ReadCloser, error)
	s3() (io.ReadCloser, error)
	http() (io.ReadCloser, error)
	parseDataPath() (string, string, error)
	Error() error
}

var _ DataStreamInterface = &dataStream{}

type dataStream struct {
	url         *url.URL
	accessKeyId string
	secretKey   string
	err         error
}

func (d *dataStream) Error() error {
	return d.err
}

// NewDataStream: construct a new dataStream object from params.
func NewDataStream(ep *url.URL, accKey, secKey string) *dataStream {
	if len(accKey) == 0 || len(secKey) == 0 {
		glog.Warningf("NewDataStream: %s and/or %s env variables are empty\n", common.IMPORTER_ACCESS_KEY_ID, common.IMPORTER_SECRET_KEY)
	}
	return &dataStream{
		url:         ep,
		accessKeyId: accKey,
		secretKey:   secKey,
	}
}

func (d *dataStream) DataStreamSelector() (io.ReadCloser, error) {
	switch d.url.Scheme {
	case "s3":
		return d.s3()
	case "http", "https":
		return d.http()
	default:
		return nil, fmt.Errorf("DataStreamSelector: invalid url scheme: %s", d.url.Scheme)
	}
}

func (d *dataStream) s3() (io.ReadCloser, error) {
	glog.Infoln("Using S3 client to get data")
	bucket := d.url.Host
	object := strings.Trim(d.url.Path, "/")
	mc, err := minio.NewV4(common.IMPORTER_S3_HOST, d.accessKeyId, d.secretKey, false)
	if err != nil {
		return nil, fmt.Errorf("getDataWithS3Client: error building minio client for %q\n", d.url.Host)
	}
	glog.Infof("Attempting to get object %q via S3 client\n", d.url.String())
	objectReader, err := mc.GetObject(bucket, object, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3: failed getting s3 object: \"%s/%s\": %v\n", bucket, object, err)
	}
	return objectReader, nil
}

func (d *dataStream) http() (io.ReadCloser, error) {
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.SetBasicAuth(d.accessKeyId, d.secretKey) // Redirects will lose basic auth, so reset them manually
			return nil
		},
	}
	req, err := http.NewRequest("GET", d.url.String(), nil)
	if len(d.accessKeyId) > 0 && len(d.secretKey) > 0 {
		req.SetBasicAuth(d.accessKeyId, d.secretKey)
	}
	glog.Infoln("Using HTTP GET to fetch data.")
	resp, err := client.Do(req)
	if err != nil {
		glog.Fatalf("http: response body error: %v\n", err)
	}
	if resp.StatusCode != 200 {
		glog.Errorf("http: expected status code 200, got %d", resp.StatusCode)
		return nil, fmt.Errorf("http: expected status code 200, got %d. Status: %s", resp.StatusCode, resp.Status)
	}
	return resp.Body, nil
}

// parseDataPath only used for debugging
func (d *dataStream) parseDataPath() (string, string, error) {
	pathSlice := strings.Split(strings.Trim(d.url.EscapedPath(), "/"), "/")
	glog.Infof("DEBUG -- %v", pathSlice)
	return pathSlice[0], strings.Join(pathSlice[1:], "/"), nil
}
