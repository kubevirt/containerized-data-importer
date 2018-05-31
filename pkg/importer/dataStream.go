package importer

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/pkg/errors"
)

type DataStreamInterface interface {
	dataStreamSelector() (io.ReadCloser, error)
	s3() (io.ReadCloser, error)
	http() (io.ReadCloser, error)
	local() (io.ReadCloser, error)
	parseDataPath() (string, string)
}

var _ DataStreamInterface = &dataStream{}

type dataStream struct {
	url         *url.URL
	accessKeyId string
	secretKey   string
	readers     []io.ReadCloser
	qemu        bool
}

// Return a dataStream object after validating the endpoint and constructing the reader/closer chain.
// Note: the caller must close the `readers` in reverse order. See CloseReaders().
func NewDataStream(endpt, accKey, secKey string) (*dataStream, error) {
	if len(accKey) == 0 || len(secKey) == 0 {
		glog.Warningf("%s and/or %s env variables are empty\n", IMPORTER_ACCESS_KEY_ID, IMPORTER_SECRET_KEY)
	}
	ep, err := ParseEndpoint(endpt)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", endpt))
	}
	fn := filepath.Base(ep.Path)
	if !image.IsSupporedFileType(fn) {
		return nil, errors.Errorf("unsupported source file %q. Supported types: %v\n", fn, image.SupportedFileExtensions)
	}
	ds := &dataStream{
		url:         ep,
		accessKeyId: accKey,
		secretKey:   secKey,
	}
	// establish readers for each extension type in the endpoint
	readers, err := ds.constructReaders()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to construct readers")
	}
	ds.readers = readers
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
		return nil, errors.Errorf("invalid url scheme: %q", d.url.Scheme)
	}
}

func (d *dataStream) s3() (io.ReadCloser, error) {
	glog.V(Vdebug).Infoln("Using S3 client to get data")
	bucket := d.url.Host
	object := strings.Trim(d.url.Path, "/")
	mc, err := minio.NewV4(IMPORTER_S3_HOST, d.accessKeyId, d.secretKey, false)
	if err != nil {
		return nil, errors.Wrapf(err, "could not build minio client for %q", d.url.Host)
	}
	glog.V(Vadmin).Infof("Attempting to get object %q via S3 client\n", d.url.String())
	objectReader, err := mc.GetObject(bucket, object, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "could not get s3 object: \"%s/%s\"", bucket, object)
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
	if err != nil {
		return nil, errors.Wrap(err, "could not create HTTP request")
	}
	if len(d.accessKeyId) > 0 && len(d.secretKey) > 0 {
		req.SetBasicAuth(d.accessKeyId, d.secretKey)
	}
	glog.V(Vadmin).Infof("Attempting to get object %q via http client\n", d.url.String())
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request errored")
	}
	if resp.StatusCode != 200 {
		glog.Errorf("http: expected status code 200, got %d", resp.StatusCode)
		return nil, errors.Errorf("expected status code 200, got %d. Status: %s", resp.StatusCode, resp.Status)
	}
	return resp.Body, nil
}

func (d *dataStream) local() (io.ReadCloser, error) {
	fn := d.url.Path
	f, err := os.Open(fn)
	if err != nil {
		return nil, errors.Wrapf(err, "could not open file %q", fn)
	}
	//note: if poor perf here consider wrapping this with a buffered i/o Reader
	return f, nil
}

// Copy the source file (image) to the provided destination path.
func CopyImage(dest, endpoint, accessKey, secKey string) error {
	ds, err := NewDataStream(endpoint, accessKey, secKey)
	if err != nil {
		return errors.Wrapf(err, "unable to create data stream")
	}
	defer CloseReaders(ds.readers)
	return ds.copy(dest)
}

// Return the size in bytes of the provided endpoint.
func ImageSize(endpoint, accessKey, secKey string) (int64, error) {
	ds, err := NewDataStream(endpoint, accessKey, secKey)
	if err != nil {
		return -1, errors.Wrapf(err, "unable to create data stream")
	}
	defer CloseReaders(ds.readers)
	return ds.size()
}

// Parse the endpoint extensions and return a slice of read-closers. The readers are in order starting with
// the lowest level reader (eg. http) with the last entry being the highest level reader (eg, gzip).
// Note: the .qcow2 ext is ignored however qemu files are detected based on their magic number.
// Note: readers are not closed here.
func (d *dataStream) constructReaders() ([]io.ReadCloser, error) {
	glog.V(Vdebug).Infof("constructReaders: create the initial Reader based on the url's %q scheme", d.url.Scheme)
	var readers []io.ReadCloser

	rdr, err := d.dataStreamSelector()
	if err != nil {
		return nil, errors.WithMessage(err, "could not get data reader")
	}
	readers = append(readers, rdr)

	// build slice of compression/archive extensions in right-to-left order
	exts := []string{}
	fn := d.url.Path
	for image.IsSupporedCompressArchiveType(fn) {
		ext := strings.ToLower(filepath.Ext(fn))
		exts = append(exts, ext)
		fn = strings.TrimSuffix(fn, ext)
	}

	// create decompress/un-archive Readers
	glog.V(Vdebug).Infof("constructReaders: checking compressed and/or archive for file %q\n", d.url.Path)
	for _, ext := range exts {
		switch ext {
		case image.ExtGz:
			rdr, err = image.GzReader(rdr)
		case image.ExtTar:
			rdr, err = image.TarReader(rdr)
		case image.ExtXz:
			rdr, err = image.XzReader(rdr)
		}
		if err != nil {
			return nil, errors.WithMessage(err, "could not get compression reader")
		}
		readers = append(readers, rdr)
	}

	// if image is qemu convert it to raw
	magicStr, err := image.GetMagicNumber(rdr)
	if err != nil {
		return nil, errors.WithMessage(err, "unable to check magic number")
	}
	d.qemu = image.MatchQcow2MagicNum(magicStr)
	// don't lose bytes reading the magic number regardless of the file being qemu or not:
	// MultiReader reads from each reader, in order, until the last reader returns eof.
	multir := io.MultiReader(bytes.NewReader(magicStr), rdr)
	readers = append(readers, ioutil.NopCloser(multir))

	return readers, nil
}

// Close the passed-in slice of readers in reverse order. See constructReaders().
func CloseReaders(readers []io.ReadCloser) {
	for i := len(readers) - 1; i >= 0; i-- {
		r := readers[i]
		r.Close()
	}
}

// Copy endpoint to dest based on passed-in reader.
func (d *dataStream) copy(dest string) error {
	rdr := d.readers[len(d.readers)-1]
	return copy(rdr, dest, d.qemu)
}

// Return the endpoint size based on passed-in reader.
func (d *dataStream) size() (int64, error) {
	rdr := d.readers[len(d.readers)-1]
	return size(rdr, d.qemu)
}

// Copy the file using its Reader (r) to the passed-in destination (`out`).
func copy(r io.Reader, out string, qemu bool) error {
	out = filepath.Clean(out)
	glog.V(Vadmin).Infof("copying image file to %q", out)
	dest := out
	if qemu {
		// copy to tmp; qemu conversion will write to passed-in destination
		dest = randTmpName(out)
		glog.V(Vdebug).Infof("Copy: temp file for qcow2 conversion: %q", dest)
	}
	// actual copy
	err := StreamDataToFile(r, dest)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("unable to stream data to file %q", dest))
	}
	if qemu {
		glog.V(Vadmin).Infoln("converting qcow2 image")
		err = image.ConvertQcow2ToRaw(dest, out)
		if err != nil {
			return errors.WithMessage(err, "unable to copy image")
		}
		err = os.Remove(dest)
		if err != nil {
			return errors.Wrapf(err, "error removing temp file %q", dest)
		}
	}
	return nil
}

// Return the size of the endpoint corresponding to the passed-in reader.
func size(rdr io.ReadCloser, qemu bool) (int64, error) {
	// TODO: figure out the size!
	return 0, nil
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
func (d *dataStream) parseDataPath() (string, string) {
	pathSlice := strings.Split(strings.Trim(d.url.EscapedPath(), "/"), "/")
	glog.V(Vdebug).Infof("parseDataPath: url path: %v", pathSlice)
	return pathSlice[0], strings.Join(pathSlice[1:], "/")
}
