package importer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/kubevirt/containerized-data-importer/pkg/image"
	"github.com/minio/minio-go"
)

type DataStreamInterface interface {
	dataStreamSelector() (io.ReadCloser, error)
	s3() (io.ReadCloser, error)
	http() (io.ReadCloser, error)
	local() (io.ReadCloser, error)
	parseDataPath() (string, string, error)
}

var _ DataStreamInterface = &dataStream{}

type dataStream struct {
	DataRdr     io.ReadCloser
	url         *url.URL
	accessKeyId string
	secretKey   string
}

func (d *dataStream) Read(p []byte) (int, error) {
	return d.DataRdr.Read(p)
}

func (d *dataStream) Close() error {
	return d.DataRdr.Close()
}

// NewDataStream: construct a new dataStream object from params.
func NewDataStream(ep *url.URL, accKey, secKey string) *dataStream {
	if len(accKey) == 0 || len(secKey) == 0 {
		glog.Warningf("NewDataStream: %s and/or %s env variables are empty\n", IMPORTER_ACCESS_KEY_ID, IMPORTER_SECRET_KEY)
	}
	return &dataStream{
		url:         ep,
		accessKeyId: accKey,
		secretKey:   secKey,
	}
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
	glog.V(Vdebug).Infoln("Using S3 client to get data")
	bucket := d.url.Host
	object := strings.Trim(d.url.Path, "/")
	mc, err := minio.NewV4(IMPORTER_S3_HOST, d.accessKeyId, d.secretKey, false)
	if err != nil {
		return nil, fmt.Errorf("getDataWithS3Client: error building minio client for %q\n", d.url.Host)
	}
	glog.V(Vadmin).Infof("Attempting to get object %q via S3 client\n", d.url.String())
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
	glog.V(Vadmin).Infof("Attempting to get object %q via http client\n", d.url.String())
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

// Creates the ReadClosers needed to read the endpoint URI.
// Performs the copy and closes each ReadCloser.
func (d *dataStream) Copy(outPath string) error {
	var err error
	fn := d.url.Path

	glog.V(Vdebug).Infof("Copy: create the initial Reader based on the url's %q scheme", d.url.Scheme)
	d.DataRdr, err = d.dataStreamSelector()
	if err != nil {
		return fmt.Errorf("Copy: %v\n", err)
	}
	defer func(r io.ReadCloser) {
		r.Close()
	}(d.DataRdr)

	// build slice of compression/archive extensions in right-to-left order
	exts := []string{}
	for image.IsSupporedCompressArchiveType(fn) {
		ext := strings.ToLower(filepath.Ext(fn))
		exts = append(exts, ext)
		fn = strings.TrimSuffix(fn, ext)
	}

	// create decompress/un-archive Readers
	glog.V(Vdebug).Infof("Copy: checking compressed and/or archive for file %q\n", d.url.Path)
	for _, ext := range exts {
		switch ext {
		case image.ExtGz:
			d.DataRdr, err = image.GzReader(d.DataRdr)
		case image.ExtTar:
			d.DataRdr, err = image.TarReader(d.DataRdr)
		case image.ExtXz:
			d.DataRdr, err = image.XzReader(d.DataRdr)
		}
		if err != nil {
			return fmt.Errorf("Copy: %v\n", err)
		}
		defer func(r io.ReadCloser) {
			r.Close()
		}(d.DataRdr)
	}

	// If image is qemu convert it to raw. Note .qcow2 ext is ignored
	magicStr, err := image.GetMagicNumber(d.DataRdr)
	if err != nil {
		return fmt.Errorf("Copy: %v\n", err)
	}
	qemu := image.MatchQcow2MagicNum(magicStr)
	// Don't lose bytes read reading the magic number. MultiReader reads from each
	// reader, in order, until the last reader returns eof.
	multir := io.MultiReader(bytes.NewReader(magicStr), d.DataRdr)

	// copy image file to outPath
	err = copyImage(multir, outPath, qemu)
	if err != nil {
		return fmt.Errorf("Copy: %v", err)
	}
	return nil
}

// Copy the file using its Reader (r) to the passed-in destination (`out`).
func copyImage(r io.Reader, out string, qemu bool) error {
	out = filepath.Clean(out)
	glog.V(Vadmin).Infof("copying image file to %q", out)
	dest := out
	if qemu {
		// copy to tmp; qemu conversion will write to passed-in destination
		dest = randTmpName(out)
		glog.V(Vdebug).Infof("copyImage: temp file for qcow2 conversion: %q", dest)
	}
	// actual copy
	err := StreamDataToFile(r, dest)
	if err != nil {
		return fmt.Errorf("copyImage: unable to stream data to file %q: %v\n", dest, err)
	}
	if qemu {
		glog.V(Vadmin).Infoln("converting qcow2 image")
		err = image.ConvertQcow2ToRaw(dest, out)
		if err != nil {
			return fmt.Errorf("copyImage: converting qcow2 image: %v\n", err)
		}
		err = os.Remove(dest)
		if err != nil {
			return fmt.Errorf("copyImage: error removing temp file %q: %v\n", dest, err)
		}
	}
	return nil
}

// Return a random temp path with the `src` basename as the prefix and preserving the extension.
// Eg. "/tmp/disk1d729566c74d1003.img".
func randTmpName(src string) string {
	ext := filepath.Ext(src)
	base := filepath.Base(src)
	base = base[:len(base)-len(ext)] // exclude extension
	randName := make([]byte, 8)
	rand.Read(randName)
	return filepath.Join(os.TempDir(), base+hex.EncodeToString(randName)+ext)
}

// parseDataPath only used for debugging
func (d *dataStream) parseDataPath() (string, string, error) {
	pathSlice := strings.Split(strings.Trim(d.url.EscapedPath(), "/"), "/")
	glog.V(Vdebug).Infof("parseDataPath: url path: %v", pathSlice)
	return pathSlice[0], strings.Join(pathSlice[1:], "/"), nil
}
