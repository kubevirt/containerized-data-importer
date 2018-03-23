package importer

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/minio/minio-go"
)

type DataStreamInterface interface {
	dataStreamSelector() (io.ReadCloser, error)
	s3() (io.ReadCloser, error)
	http() (io.ReadCloser, error)
	local() (io.ReadCloser, error)
	parseDataPath() (string, string, error)
	Error() error
}

var _ DataStreamInterface = &dataStream{}

type dataStream struct {
	DataRdr	    io.ReadCloser
	url         *url.URL
	accessKeyId string
	secretKey   string
	err         error
}

func (d *dataStream) Error() error {
	return d.err
}

// NewDataStream: construct a new dataStream object from params.
func NewDataStream(ep *url.URL, accKey, secKey string) (*dataStream, error) {
	if len(accKey) == 0 || len(secKey) == 0 {
		glog.Warningf("NewDataStream: %s and/or %s env variables are empty\n", common.IMPORTER_ACCESS_KEY_ID, common.IMPORTER_SECRET_KEY)
	}
	ds := &dataStream{
		url:	     ep,
		accessKeyId: accKey,
		secretKey:   secKey,
	}
	rdr, err := ds.dataStreamSelector()
	if err != nil {
		return nil, fmt.Errorf("NewDataStream: %v\n", err)
	}
	ds.DataRdr = rdr
	return ds, nil
}

func (d *dataStream) dataStreamSelector() (io.ReadCloser, error) {
	switch d.url.Scheme {
	case "s3":
		return d.s3()
	case "http", "https":
		return d.http()
	case "file":
		return d.local()
	default:
		return nil, fmt.Errorf("dataStreamSelector: invalid url scheme: %s", d.url.Scheme)
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

func (d *dataStream) local() (io.ReadCloser, error) {
	fn := d.url.Path
	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("open fail on %q: %v\n", fn, err)
	}
	//note: if poor perf here consider wrapping this with a buffered i/o Reader
	return f, nil
}

// parseDataPath only used for debugging
func (d *dataStream) parseDataPath() (string, string, error) {
	pathSlice := strings.Split(strings.Trim(d.url.EscapedPath(), "/"), "/")
	glog.Infof("DEBUG -- %v", pathSlice)
	return pathSlice[0], strings.Join(pathSlice[1:], "/"), nil
}
