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

type dataStreamFactory struct {
	url         *url.URL
	accessKeyId string
	secretKey   string
	err         error
}

func (d *dataStreamFactory) Error() error {
	return d.err
}

// NewDataStreamFactory: construct a new DataStreamFactory object from params.
func NewDataStreamFactory(ep, accKey, secKey string) *dataStreamFactory {
	if len(accKey) == 0 || len(secKey) == 0 {
		glog.Warningf("NewDataStreamFactory: %s and/or %s env variables are empty\n", common.IMPORTER_ACCESS_KEY_ID, common.IMPORTER_SECRET_KEY)
	}
	epUrl, err := url.Parse(ep)
	if err != nil {
		return &dataStreamFactory{err: err}
	}
	return &dataStreamFactory{
		url:         epUrl,
		accessKeyId: accKey,
		secretKey:   secKey,
	}
}

func (d *dataStreamFactory) NewDataStream() (io.ReadCloser, error) {
	if d.err != nil {
		return nil, d.err
	}
	if d.url.Scheme == "s3" { // TODO what about non-aws s3 interfaces? (minio just performs http GET on them anyway)
		return d.s3()
	}
	return d.http()
}

func (d *dataStreamFactory) s3() (io.ReadCloser, error) {
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

func (d *dataStreamFactory) http() (io.ReadCloser, error) {
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.SetBasicAuth(d.accessKeyId, d.secretKey)
			return nil
		},
	}
	req, err := http.NewRequest("GET", d.url.String(), nil)
	if len(d.accessKeyId) > 0 && len(d.secretKey) > 0 {
	}
	glog.Infoln("Using HTTP GET to fetch data.")
	resp, err := client.Do(req)
	if err != nil {
		glog.Fatalf("http: response body error: %v\n", err)
	}
	return resp.Body, nil
}

func (d *dataStreamFactory) parseDataPath() (string, string, error) {
	pathSlice := strings.Split(strings.Trim(d.url.EscapedPath(), "/"), "/")
	glog.Infof("DEBUG -- %v", pathSlice)
	return pathSlice[0], strings.Join(pathSlice[1:], "/"), nil
}
