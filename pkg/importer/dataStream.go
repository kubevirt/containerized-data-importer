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
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/minio/minio-go"
	"github.com/pkg/errors"
	"github.com/xi2/xz"
	. "kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
)

type DataStreamInterface interface {
	dataStreamSelector() error
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
	buf         []byte // holds file headers
	Qemu        bool
	Size        int64
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
	RdrFile
	RdrGz
	RdrMulti
	RdrQcow2
	RdrTar
	RdrXz
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
		buf:         make([]byte, image.MaxExpectedHdrSize),
		accessKeyId: accKey,
		secretKey:   secKey,
	}
	// establish readers for each nested format types in the endpoint
	err = ds.constructReaders()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to construct readers")
	}
	return ds, nil
}

// Read from top-most reader.
func (d *dataStream) Read(buf []byte) (int, error) {
	return d.topReader().Read(buf)
}

// Close all readers.
func (d *dataStream) Close() error {
	return closeReaders(d.Readers)
}

func (d *dataStream) dataStreamSelector() (err error) {
	var r Reader
	switch d.Url.Scheme {
	case "s3":
		r, err = d.s3()
	case "http", "https":
		r, err = d.http()
	case "file":
		r, err = d.local()
	default:
		return errors.Errorf("invalid url scheme: %q", d.Url.Scheme)
	}
	// append source reader to datastream reader stack
	d.Readers = append(d.Readers, r)
	return
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

// Copy the source endpoint (vm image) to the provided destination path.
func CopyImage(dest, endpoint, accessKey, secKey string) error {
	glog.V(Vuser).Infof("copying %q to %q...\n", endpoint, dest)
	ds, err := NewDataStream(endpoint, accessKey, secKey)
	if err != nil {
		return errors.Wrapf(err, "unable to create data stream")
	}
	defer ds.Close()
	return ds.copy(dest)
}

// Read the endpoint and determine the file composition (eg. .iso.tar.gz) based on the magic number in
// each known file format header. Set the Reader slice in the receiver and set the Size field to each
// reader's original size. Note: the reader processed last defines the final Size.
// The reader order starts with the lowest level reader, eg. http, used to read file content. The next
// readers are combinations of decompression/archive readers and bytes multi-readers. The multi-readers
// are created so that header data (interpreted by the current reader) is present for the next reader.
// Thus, the last reader in the reader stack is always a multi-reader. Readers are closed in reverse order,
// see the Close method. If a format doesn't natively support Close() a no-op Closer is wrapped around
// the native Reader so that all Readers can be consider ReadClosers.
// Examples:
//   Filename                    Readers (mr == multi-reader)
//   --------                    ----------------------------
//   "https:/foo.tar.gz"         [http, mr, gz, mr, tar, mr]
//   "file:/foo.tar.xz"          [file, mr, xz, mr, tar, mr]
//   "s3://foo-iso"              [s3, mr]
//   "https://foo.qcow2"         [http, mr]		     note: there is no qcow2 reader
//   "https://foo.qcow2.tar.gz"  [http, mr, gz, mr, tar, mr] note: there is no qcow2 reader
// Note: file extensions are ignored.
// Note: readers are not closed here, see dataStream.Close().
// Assumption: a particular header format only appears once in the data stream. Eg. foo.gz.gz is not supported.
func (d *dataStream) constructReaders() error {
	glog.V(Vadmin).Infof("create the initial Reader based on the url's %q scheme", d.Url.Scheme)
	err := d.dataStreamSelector()
	if err != nil {
		return errors.WithMessage(err, "could not get data reader")
	}

	// loop through all supported file formats until we do not find a header we recognize
	knownHdrs := image.CopyKnownHdrs() // need local copy since keys are removed
	glog.V(Vdebug).Infof("constructReaders: checking compression and archive formats: %s\n", d.Url.Path)
	for {
		hdr, err := d.matchHeader(&knownHdrs)
		if err != nil {
			return errors.WithMessage(err, "could not process image header")
		}
		if hdr == nil {
			break // done processing headers, we have the orig source file
		}
		glog.V(Vadmin).Infof("found header of type %q\n", hdr.Format)

		switch hdr.Format {
		case "gz":
			err = d.gzReader()
		case "qcow2":
			err = d.qcow2NopReader(hdr)
		case "tar":
			err = d.tarReader()
		case "xz":
			err = d.xzReader()
		default:
			return errors.Errorf("mismatch between supported file formats and this header type: %q", hdr.Format)
		}
		if err != nil {
			return errors.WithMessage(err, "could not create compression/unarchive reader")
		}
	}

	if len(d.Readers) <= 2 { // 1st rdr is source, 2nd rdr is multi-rdr, >2 means we have additional formats
		glog.V(Vdebug).Infof("constructReaders: no headers found for file %q\n", d.Url.Path)
	}
	glog.V(Vadmin).Infof("done processing %q headers\n", d.Url.Path)
	return nil
}

func (d *dataStream) topReader() io.ReadCloser {
	return d.Readers[len(d.Readers)-1].Rdr
}

//NOTE: size in gz is stored in the last 4 bytes of the file. This requires the file to be decompressed in
//  order to get its original size. For now 0 is returned.
// Assumes a single file was gzipped.
//TODO: support gz size.
func (d *dataStream) gzReader() error {
	gz, err := gzip.NewReader(d.topReader())
	if err != nil {
		return errors.Wrap(err, "could not create gzip reader")
	}
	glog.V(Vadmin).Infof("gzip: extracting %q\n", gz.Name)
	d.Readers = append(d.Readers, Reader{RdrType: RdrGz, Rdr: gz})
	d.Size = 0 //TODO: implement size
	return nil
}

// Note: size is stored at offset 24 in the qcow2 header.
// Note: there is no qcow2 reader.
func (d *dataStream) qcow2NopReader(h *image.Header) error {
	s := hex.EncodeToString(d.buf[h.SizeOff : h.SizeOff+h.SizeLen])
	size, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return errors.Wrapf(err, "unable to determine original qcow2 file size from %+v", s)
	}
	d.Qemu = true
	d.Size = size
	return nil
}

//NOTE: size is not stored in the xz header. This may require the file to be decompressed in
//  order to get its original size. For now 0 is returned.
//TODO: support gz size.
func (d *dataStream) xzReader() error {
	xz, err := xz.NewReader(d.topReader(), 0) //note: default dict size may be too small
	if err != nil {
		return errors.Wrap(err, "could not create xz reader")
	}
	d.Readers = append(d.Readers, Reader{RdrType: RdrXz, Rdr: ioutil.NopCloser(xz)})
	d.Size = 0 //TODO: implement size
	return nil
}

// Assumes a single file was archived.
func (d *dataStream) tarReader() error {
	tr := tar.NewReader(d.topReader())
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	if err != nil {
		return errors.Wrap(err, "could not read tar header")
	}
	glog.V(Vadmin).Infof("tar: extracting %q\n", hdr.Name)
	d.Size = hdr.Size
	d.Readers = append(d.Readers, Reader{RdrType: RdrTar, Rdr: ioutil.NopCloser(tr)})
	return nil
}

// Return the matching header, if one is found, from the passed-in map of known headers.
// After reading the header, update the receiver's slice of readers with a byte multi-reader
// so that the header data is not lost.
// Note: knownHdrs is passed by reference and modified.
func (d *dataStream) matchHeader(knownHdrs *image.Headers) (*image.Header, error) {
	_, err := d.Read(d.buf) // read current header
	if err != nil {
		return nil, err
	}
	// create a multi-reader so that the header data is re-read by subsequent readers
	hRdr := ioutil.NopCloser(io.MultiReader(bytes.NewReader(d.buf), d.topReader()))
	// append byte multi-reader to datastream reader stack
	d.Readers = append(d.Readers, Reader{RdrType: RdrMulti, Rdr: hRdr})

	// loop through known headers until a match
	for format, kh := range *knownHdrs {
		if kh.Match(d.buf) {
			// delete this header format key so that it's not processed again
			delete(*knownHdrs, format)
			return &kh, nil
		}
	}
	return nil, nil // no match
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
	return copy(d.topReader(), dest, d.Qemu)
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
		defer func(f string) {
			os.Remove(f)
		}(dest)
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
