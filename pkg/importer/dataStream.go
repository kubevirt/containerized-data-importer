package importer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	"github.com/xi2/xz"
)

type DataStreamInterface interface {
	dataStreamSelector() (Reader, error)
	s3() (Reader, error)
	http() (Reader, error)
	local() (Reader, error)
	parseDataPath() (string, string)
	Read(p []byte) (int, error)
	Close() error
}

var _ DataStreamInterface = &dataStream{}

// implements the ReadCloser interface
type dataStream struct {
	Url         *url.URL
	Readers     []Reader
	Qemu        bool
	accessKeyId string
	secretKey   string
}

type Reader struct {
	RdrType int
	Rdr     io.ReadCloser
}

const (
	RdrHttp = iota
	RdrS3
	RdrTar
	RdrGz
	RdrXz
	RdrFile
	RdrMulti
)

// Return a dataStream object after validating the endpoint and constructing the reader/closer chain.
// Note: the caller must close the `Readers` in reverse order. See Close().
func NewDataStream(endpt, accKey, secKey string) (*dataStream, error) {
	if len(accKey) == 0 || len(secKey) == 0 {
		glog.Warningf("%s and/or %s env variables are empty\n", IMPORTER_ACCESS_KEY_ID, IMPORTER_SECRET_KEY)
	}
	ep, err := ParseEndpoint(endpt)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", endpt))
	}
	fn := filepath.Base(ep.Path)
	if !image.IsSupportedFileType(fn) {
		return nil, errors.Errorf("unsupported source file %q. Supported types: %v\n", fn, image.SupportedFileExtensions)
	}
	ds := &dataStream{
		Url:         ep,
		accessKeyId: accKey,
		secretKey:   secKey,
	}
	// establish readers for each extension type in the endpoint
	err = ds.constructReaders()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to construct readers")
	}
	return ds, nil
}

// Read from top-most reader.
func (d *dataStream) Read(buf []byte) (int, error) {
	r := d.Readers[len(d.Readers)-1].Rdr
	return r.Read(buf)
}

// Close all readers.
func (d *dataStream) Close() error {
	return closeReaders(d.Readers)
}

func (d *dataStream) dataStreamSelector() (Reader, error) {
	switch d.Url.Scheme {
	case "s3":
		return d.s3()
	case "http", "https":
		return d.http()
	case "file":
		return d.local()
	default:
		return Reader{}, errors.Errorf("invalid url scheme: %q", d.Url.Scheme)
	}
}

func (d *dataStream) s3() (Reader, error) {
	glog.V(Vdebug).Infoln("Using S3 client to get data")
	bucket := d.Url.Host
	object := strings.Trim(d.Url.Path, "/")
	mc, err := minio.NewV4(IMPORTER_S3_HOST, d.accessKeyId, d.secretKey, false)
	if err != nil {
		return Reader{}, errors.Wrapf(err, "could not build minio client for %q", d.Url.Host)
	}
	glog.V(Vadmin).Infof("Attempting to get object %q via S3 client\n", d.Url.String())
	objectReader, err := mc.GetObject(bucket, object, minio.GetObjectOptions{})
	if err != nil {
		return Reader{}, errors.Wrapf(err, "could not get s3 object: \"%s/%s\"", bucket, object)
	}
	return Reader{RdrType: RdrS3, Rdr: objectReader}, nil
}

func (d *dataStream) http() (Reader, error) {
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.SetBasicAuth(d.accessKeyId, d.secretKey) // Redirects will lose basic auth, so reset them manually
			return nil
		},
	}
	req, err := http.NewRequest("GET", d.Url.String(), nil)
	if err != nil {
		return Reader{}, errors.Wrap(err, "could not create HTTP request")
	}
	if len(d.accessKeyId) > 0 && len(d.secretKey) > 0 {
		req.SetBasicAuth(d.accessKeyId, d.secretKey)
	}
	glog.V(Vadmin).Infof("Attempting to get object %q via http client\n", d.Url.String())
	resp, err := client.Do(req)
	if err != nil {
		return Reader{}, errors.Wrap(err, "HTTP request errored")
	}
	if resp.StatusCode != 200 {
		glog.Errorf("http: expected status code 200, got %d", resp.StatusCode)
		return Reader{}, errors.Errorf("expected status code 200, got %d. Status: %s", resp.StatusCode, resp.Status)
	}
	return Reader{RdrType: RdrHttp, Rdr: resp.Body}, nil
}

func (d *dataStream) local() (Reader, error) {
	fn := d.Url.Path
	f, err := os.Open(fn)
	if err != nil {
		return Reader{}, errors.Wrapf(err, "could not open file %q", fn)
	}
	//note: if poor perf here consider wrapping this with a buffered i/o Reader
	return Reader{RdrType: RdrFile, Rdr: f}, nil
}

// Copy the source file (image) to the provided destination path.
func CopyImage(dest, endpoint, accessKey, secKey string) error {
	ds, err := NewDataStream(endpoint, accessKey, secKey)
	if err != nil {
		return errors.Wrapf(err, "unable to create data stream")
	}
	defer ds.Close()
	return ds.copy(dest)
}

// Parse the endpoint extensions and set the Reader slice in the receiver. The reader order starts with
// the lowest level reader, eg. http, used to read file content. The last reader is a multi reader needed
// to piece together a byte reader which is always used to read the magic number of the file. In between
// are the decompression/archive readers, if any. Readers are closed in reverse (right-to-left) order,
// see the Close method. If a format doesn't natively support Close() a no-op Closer is wrapped around
// the native Reader so that all Readers can be consider ReadClosers.
// Examples:
//   Filename                         Readers
//   --------                         -------
//   "https:/somefile.tar.gz"         [http, gz, tar, multi-reader]
//   "file:/somefile.tar.xz"          [file, xz, tar, multi-reader]
//   "s3://some-iso"                  [s3, multi-reader]
//   "https://somefile.qcow2"         [http, multi-reader]
//   "https://somefile.qcow2.tar.gz"  [http, gz, tar, multi-reader]
// Note: the .qcow2 ext is ignored; qemu files are detected based on their magic number.
// Note: readers are not closed here, see dataStream.Close().
func (d *dataStream) constructReaders() error {
	glog.V(Vdebug).Infof("constructReaders: create the initial Reader based on the url's %q scheme", d.Url.Scheme)

	rdr, err := d.dataStreamSelector()
	if err != nil {
		return errors.WithMessage(err, "could not get data reader")
	}
	d.Readers = append(d.Readers, rdr)

	// build slice of compression/archive extensions in right-to-left order
	exts := []string{}
	fn := d.Url.Path
	for image.IsSupportedCompressArchiveType(fn) {
		ext := strings.ToLower(filepath.Ext(fn))
		exts = append(exts, ext)
		fn = strings.TrimSuffix(fn, ext)
	}
	// create decompress/un-archive Readers
	glog.V(Vdebug).Infof("constructReaders: checking compressed and/or archive for file %q\n", d.Url.Path)
	for _, ext := range exts {
		switch ext {
		case image.ExtGz:
			rdr, err = GzReader(rdr.Rdr)
		case image.ExtTar:
			rdr, err = TarReader(rdr.Rdr)
		case image.ExtXz:
			rdr, err = XzReader(rdr.Rdr)
		}
		if err != nil {
			return errors.WithMessage(err, "could not get compression reader")
		}
		d.Readers = append(d.Readers, rdr)
	}

	// if image is qemu convert it to raw
	glog.V(Vdebug).Infoln("constructReaders: check for qcow2/qemu file")
	magicStr, err := image.GetMagicNumber(rdr.Rdr)
	if err != nil {
		return errors.WithMessage(err, "unable to check magic number")
	}
	d.Qemu = image.MatchQcow2MagicNum(magicStr)
	glog.V(Vdebug).Infof("constructReaders: qemu file is %t\n", d.Qemu) 
	// don't lose bytes reading the magic number regardless of the file being qemu or not:
	// MultiReader reads from each reader, in order, until the last reader returns eof.
	multir := io.MultiReader(bytes.NewReader(magicStr), rdr.Rdr)
	d.Readers = append(d.Readers, Reader{RdrType: RdrMulti, Rdr: ioutil.NopCloser(multir)})
	return nil
}

func GzReader(r io.ReadCloser) (Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return Reader{}, errors.Wrap(err, "could not create gzip reader")
	}
	glog.V(Vadmin).Infof("gzip: extracting %q\n", gz.Name)
	return Reader{RdrType: RdrGz, Rdr: gz}, nil
}

func XzReader(r io.ReadCloser) (Reader, error) {
	glog.V(Vdebug).Infoln("XzReader: xz format")
	xz, err := xz.NewReader(r, 0) //note: default dict size may be too small
	if err != nil {
		return Reader{}, errors.Wrap(err, "could not create xz reader")
	}
	return Reader{RdrType: RdrXz, Rdr: ioutil.NopCloser(xz)}, nil
}

func TarReader(r io.ReadCloser) (Reader, error) {
	tr := tar.NewReader(r)
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	if err != nil {
		return Reader{}, errors.Wrap(err, "could not read tar header")
	}
	glog.V(Vadmin).Infof("tar: extracting %q\n", hdr.Name)
	return Reader{RdrType: RdrTar, Rdr: ioutil.NopCloser(tr)}, nil
}

// Close the passed-in Readers in reverse order, see constructReaders().
func closeReaders(readers []Reader) (rtnerr error) {
	var err error
	for i := len(readers) - 1; i >= 0; i-- {
		err = readers[i].Rdr.Close()
		if err != nil {
			rtnerr = err // tracking last error
		}
	}
	return rtnerr
}

// Copy endpoint to dest based on passed-in reader.
func (d *dataStream) copy(dest string) error {
	r := d.Readers[len(d.Readers)-1]
	return copy(r.Rdr, dest, d.Qemu)
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
	pathSlice := strings.Split(strings.Trim(d.Url.EscapedPath(), "/"), "/")
	glog.V(Vdebug).Infof("parseDataPath: url path: %v", pathSlice)
	return pathSlice[0], strings.Join(pathSlice[1:], "/")
}
