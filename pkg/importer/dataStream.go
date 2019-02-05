/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package importer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/pkg/errors"
	"github.com/ulikunitz/xz"
	"k8s.io/klog"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

// DataStreamInterface provides our interface definition required to fulfill a DataStream
type DataStreamInterface interface {
	dataStreamSelector() error
	fileFormatSelector(h *image.Header) error
	parseDataPath() (string, string)
	Read(p []byte) (int, error)
	Close() error
}

var _ DataStreamInterface = &DataStream{}

var qemuOperations = image.NewQEMUOperations()

// DataStream implements the ReadCloser interface
type DataStream struct {
	*DataStreamOptions
	url        *url.URL
	Readers    []reader
	buf        []byte // holds file headers
	qemu       bool
	archived   bool
	Size       int64
	isIsoImage bool
	dataCtxt   StreamContext
}

type reader struct {
	rdrType int
	rdr     io.ReadCloser
}

// DataStreamOptions contains all the values needed for importing from a DataStream.
type DataStreamOptions struct {
	// Dest is the destination path of the contents of the stream.
	Dest string
	// DataDir is the destination path where data is stored
	DataDir string
	// Endpoint is the endpoint to get the data from for various Sources.
	Endpoint string
	// AccessKey is the access key for the endpoint, can be blank. This needs to be a base64 encoded string.
	AccessKey string
	// SecKey is the security key needed for the endpoint, can be blank. This needs to be a base64 encoded string.
	SecKey string
	// Source is the source type of the data.
	Source string
	// ContentType is the content type of the data.
	ContentType string
	// ImageSize is the size we want the resulting image to be.
	ImageSize string
	// Available space is the available space before downloading the image
	AvailableSpace int64
	// CertDir is a directory containing tls certs
	CertDir string
	// InsecureTLS is it okay to skip TLS verification
	InsecureTLS bool
	// ScratchDataDir is the path to the scratch space inside the container.
	ScratchDataDir string
}

const (
	rdrHTTP = iota
	rdrS3
	rdrFile
	rdrGz
	rdrMulti
	rdrTar
	rdrXz
	rdrStream

	//ContainerDiskImageDir - Expected disk image location in container image as described in
	//https://github.com/kubevirt/kubevirt/blob/master/docs/container-register-disks.md
	ContainerDiskImageDir = "disk"
)

// map scheme and format to rdrType
var rdrTypM = map[string]int{
	"gz":     rdrGz,
	"http":   rdrHTTP,
	"https":  rdrHTTP,
	"local":  rdrFile,
	"s3":     rdrS3,
	"tar":    rdrTar,
	"xz":     rdrXz,
	"stream": rdrStream,
}

// ErrRequiresScratchSpace indicates that we require scratch space.
var ErrRequiresScratchSpace = fmt.Errorf("Scratch space required and none found")

// NewDataStream returns a DataStream object after validating the endpoint and constructing the reader/closer chain.
// Note: the caller must close the `Readers` in reverse order. See Close().
func NewDataStream(dso *DataStreamOptions) (*DataStream, error) {
	return newDataStream(dso, nil)
}

func newDataStream(dso *DataStreamOptions, stream io.ReadCloser) (*DataStream, error) {
	if len(dso.AccessKey) == 0 || len(dso.SecKey) == 0 {
		klog.V(2).Infof("%s and/or %s are empty\n", common.ImporterAccessKeyID, common.ImporterSecretKey)
	}
	var ep *url.URL
	var err error
	if dso.Source == controller.SourceHTTP || dso.Source == controller.SourceS3 ||
		dso.Source == controller.SourceGlance || dso.Source == controller.SourceRegistry {
		ep, err = ParseEndpoint(dso.Endpoint)
		if err != nil {
			return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", dso.Endpoint))
		}
	}

	ds := &DataStream{
		DataStreamOptions: dso,
		url:               ep,
		buf:               make([]byte, image.MaxExpectedHdrSize),
		qemu:              false,
		archived:          false,
	}

	// establish readers for endpoint's formats and do initial calc of size of raw endpt
	err = ds.constructReaders(stream)
	if err != nil {
		ds.Close()
		return nil, errors.Wrapf(err, "unable to construct readers")
	}

	// if the endpoint's file size is zero and it's an iso file then compute its orig size
	if ds.Size == 0 {
		ds.Size, err = ds.isoSize()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to calculate iso file size")
		}
		// at that point, only if ds.size != 0 we know for sure that this is an iso file
		if ds.Size != 0 {
			ds.isIsoImage = true
		}
	}
	klog.V(3).Infof("NewDataStream: endpoint %q's computed byte size: %d", ep, ds.Size)
	return ds, nil
}

// Read from top-most reader. Note: ReadFull is needed since there may be intermediate,
// smaller multi-readers in the reader stack, and we need to be able to fill buf.
func (d *DataStream) Read(buf []byte) (int, error) {
	return io.ReadFull(d.topReader(), buf)
}

// Close all readers.
func (d *DataStream) Close() error {
	err := closeReaders(d.Readers)
	if d.dataCtxt != nil {
		dataCtxtErr := d.dataCtxt.cleanup()
		if dataCtxtErr != nil {
			klog.Errorf(dataCtxtErr.Error(), fmt.Sprintf("DataCtxt cleanup failed"))
		}
		d.dataCtxt = nil
	}
	return err
}

// Based on the endpoint scheme, append the scheme-specific reader to the receiver's
// reader stack.
func (d *DataStream) dataStreamSelector() (err error) {
	var r io.Reader
	scheme := d.url.Scheme
	r, d.dataCtxt, err = GetDataStream(scheme, d.url, d.SecKey, d.AccessKey, d.CertDir, d.InsecureTLS, d.ScratchDataDir)
	if err == nil && r != nil {
		d.appendReader(rdrTypM[scheme], r)
	}
	return err
}

func (d *DataStream) addStream(reader io.ReadCloser) {
	d.appendReader(rdrTypM["stream"], reader)
}

// CopyData copies the source endpoint (vm image) to the provided destination path.
func CopyData(dso *DataStreamOptions) error {
	klog.V(1).Infof("copying %q to %q...\n", dso.Endpoint, dso.Dest)
	ds, err := NewDataStream(dso)
	if err != nil {
		return errors.Wrap(err, "unable to create data stream")
	}
	defer ds.Close()
	if dso.ContentType == string(cdiv1.DataVolumeArchive) {
		if err := util.UnArchiveTar(ds.topReader(), dso.Dest); err != nil {
			return errors.Wrap(err, "unable to untar files from endpoint")
		}
		return nil
	}
	return ds.copy(dso.Dest)
}

// SaveStream reads from a stream and saves data to dest
func SaveStream(stream io.ReadCloser, dest string, diskImageFileName, dataPath, scratchPath string) (int64, error) {
	klog.V(1).Infof("Saving stream to %q...\n", dest)
	ds, err := newDataStream(&DataStreamOptions{
		Dest:           diskImageFileName,
		DataDir:        dataPath,
		Endpoint:       "stream://data",
		AccessKey:      "",
		SecKey:         "",
		Source:         controller.SourceHTTP,
		ContentType:    string(cdiv1.DataVolumeKubeVirt),
		ImageSize:      "", // Blank means don't resize
		AvailableSpace: util.GetAvailableSpace(dataPath),
		CertDir:        "",
		InsecureTLS:    false,
		ScratchDataDir: scratchPath,
	}, stream)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to create data stream from stream")
	}
	defer ds.Close()
	err = ds.copy(dest)
	if err != nil {
		return 0, errors.Wrap(err, "data stream copy failed")
	}
	return ds.Size, nil
}

// DefaultSaveStream reads from a stream and saves data to dest using the default disk image/data/scratch paths
func DefaultSaveStream(stream io.ReadCloser, dest string) (int64, error) {
	return SaveStream(stream, dest, common.ImporterWritePath, common.ImporterVolumePath, common.ScratchDataDir)
}

// ResizeImage resizes the images to match the requested size. Sometimes provisioners misbehave and the available space
// is not the same as the requested space. For those situations we compare the available space to the requested space and
// use the smallest of the two values.
func ResizeImage(dest, imageSize string, totalTargetSpace int64) error {
	info, err := qemuOperations.Info(dest)
	if err != nil {
		return err
	}
	if imageSize != "" {
		currentImageSizeQuantity := resource.NewScaledQuantity(info.VirtualSize, 0)
		newImageSizeQuantity := resource.MustParse(imageSize)
		minSizeQuantity := util.MinQuantity(resource.NewScaledQuantity(totalTargetSpace, 0), &newImageSizeQuantity)
		if minSizeQuantity.Cmp(newImageSizeQuantity) != 0 {
			// Available dest space is smaller than the size we want to resize to
			klog.Warningf("Available space less than requested size, resizing image to available space %s.\n", minSizeQuantity.String())
		}
		if currentImageSizeQuantity.Cmp(minSizeQuantity) == 0 {
			klog.V(1).Infof("No need to resize image. Requested size: %s, Image size: %d.\n", imageSize, info.VirtualSize)
			return nil
		}
		klog.V(1).Infof("Expanding image size to: %s\n", minSizeQuantity.String())
		return qemuOperations.Resize(dest, minSizeQuantity)
	}
	return errors.New("Image resize called with blank resize")
}

// Read the endpoint and determine the file composition (eg. .iso.tar.gz) based on the magic number in
// each known file format header. Set the Reader slice in the receiver and set the Size field to each
// reader's original size. Note: if, when this method returns, the Size is still 0 then another method
// will compute the final size. See '*' note below.
// The reader order starts with the lowest level reader, eg. http, used to read file content. The next
// readers are combinations of decompression/archive readers and bytes multi-readers. The multi-readers
// are created so that header data (interpreted by the current reader) is present for the next reader.
// Thus, the last reader in the reader stack is always a multi-reader. Readers are closed in reverse
// order, see the Close method. If a format doesn't natively support Close() a no-op Closer is wrapped
// around the native Reader so that all Readers can be consider ReadClosers.
//
// Examples:
//   Filename                    Readers (mr == multi-reader)
//   --------                    ----------------------------
//   "https://foo.iso"           [http, mr, mr*]
//   "s3://foo.iso"              [s3, mr, mr*]
//   "https://foo.iso.tar"       [http, mr, tar, mr]
//   "https://foo.iso.gz"        [http, mr, gz, mr, mr*]
//   "https://foo.iso.tar.gz"    [http, mr, gz, mr, tar, mr]
//   "https://foo.iso.xz"        [http, mr, xz, mr, mr*]
//   "https://foo.qcow2"         [http, mr]		     note: there is no qcow2 reader
//   "https://foo.qcow2.tar.gz"  [http, mr, gz, mr, tar, mr] note: there is no qcow2 reader
//
//   * in .iso.gz and .iso.xz files (not tar'd) the size of the orig file is not available in their
//     respective headers. All tar'd and .qcow2 files have the original file size in their headers.
//     For .iso, .iso.gz and .iso.xz files the Size() func reads a much larger header structure to
//     calculate these sizes. This entails using another byte reader and thus there will be two
//     consecutive multi-readers for these file types.
//
// Assumptions:
//   A particular header format only appears once in the data stream. Eg. foo.gz.gz is not supported.
// Note: file extensions are ignored.
// Note: readers are not closed here, see dataStream.Close().
func (d *DataStream) constructReaders(stream io.ReadCloser) error {
	klog.V(2).Infof("create the initial Reader based on the endpoint's %q scheme", d.url.Scheme)

	if stream == nil {
		// create the scheme-specific source reader and append it to dataStream readers stack
		err := d.dataStreamSelector()
		if err != nil {
			klog.Errorf("failed to construct dataStream from endpoint")
			return errors.WithMessage(err, "could not get data reader")
		}
	} else {
		d.addStream(stream)
	}

	// loop through all supported file formats until we do not find a header we recognize
	// note: iso file headers are not processed here due to their much larger size and if
	//   the iso file is tar'd we can get its size via the tar hdr -- see intro comments.
	knownHdrs := image.CopyKnownHdrs() // need local copy since keys are removed
	klog.V(3).Infof("constructReaders: checking compression and archive formats: %s\n", d.url.Path)
	var isTarFile bool
	for {
		hdr, err := d.matchHeader(&knownHdrs)
		if err != nil {
			return errors.WithMessage(err, "could not process image header")
		}
		if hdr == nil {
			break // done processing headers, we have the orig source file
		}
		klog.V(2).Infof("found header of type %q\n", hdr.Format)
		// create format-specific reader and append it to dataStream readers stack
		err = d.fileFormatSelector(hdr)
		if err != nil {
			return errors.WithMessage(err, "could not create compression/unarchive reader")
		}
		isTarFile = isTarFile || hdr.Format == "tar"
		// exit loop if hdr is qcow2 since that's the equivalent of a raw (iso) file,
		// mentioned above as the orig source file
		if hdr.Format == "qcow2" {
			break
		}
	}

	if d.ContentType == string(cdiv1.DataVolumeArchive) && !isTarFile {
		return errors.Errorf("cannot process a non tar file as an archive")
	}

	if len(d.Readers) <= 2 {
		// 1st rdr is source, 2nd rdr is multi-rdr, >2 means we have additional formats
		klog.V(3).Infof("constructReaders: no headers found for file %q\n", d.url.Path)
	}
	klog.V(2).Infof("done processing %q headers\n", d.url.Path)
	return nil
}

// Append to the receiver's reader stack the passed in reader. If the reader type is multi-reader
// then wrap a multi-reader around the passed in reader. If the reader is not a Closer then wrap a
// nop closer.
func (d *DataStream) appendReader(rType int, x interface{}) {
	if x == nil {
		return
	}
	r, ok := x.(io.Reader)
	if !ok {
		klog.Errorf("internal error: unexpected reader type passed to appendReader()")
		return
	}
	if rType == rdrMulti {
		r = io.MultiReader(r, d.topReader())
	}
	if _, ok := r.(io.Closer); !ok {
		r = ioutil.NopCloser(r)
	}
	d.Readers = append(d.Readers, reader{rdrType: rType, rdr: r.(io.ReadCloser)})
}

// Return the top-level io.ReadCloser from the receiver Reader "stack".
func (d *DataStream) topReader() io.ReadCloser {
	return d.Readers[len(d.Readers)-1].rdr
}

// Based on the passed in header, append the format-specific reader to the readers stack,
// and update the receiver Size field. Note: a bool is set in the receiver for qcow2 files.
func (d *DataStream) fileFormatSelector(hdr *image.Header) (err error) {
	var r io.Reader
	fFmt := hdr.Format
	switch fFmt {
	case "gz":
		r, d.Size, err = d.gzReader()
		if err != nil {
			d.archived = true
		}
	case "qcow2":
		r, d.Size, err = d.qcow2NopReader(hdr)
		d.qemu = true
	case "tar":
		r, d.Size, err = d.tarReader()
		if err != nil {
			d.archived = true
		}
	case "xz":
		r, d.Size, err = d.xzReader()
		if err != nil {
			d.archived = true
		}
	default:
		return errors.Errorf("mismatch between supported file formats and this header type: %q", fFmt)
	}
	if err == nil && r != nil {
		d.appendReader(rdrTypM[fFmt], r)
	}
	return err
}

// Return the gz reader and the size of the endpoint "through the eye" of the previous reader.
// Assumes a single file was gzipped.
//NOTE: size in gz is stored in the last 4 bytes of the file. This probably requires the file
//  to be decompressed in order to get its original size. For now 0 is returned.
//TODO: support gz size.
func (d *DataStream) gzReader() (io.ReadCloser, int64, error) {
	gz, err := gzip.NewReader(d.topReader())
	if err != nil {
		return nil, 0, errors.Wrap(err, "could not create gzip reader")
	}
	klog.V(2).Infof("gzip: extracting %q\n", gz.Name)
	size := int64(0) //TODO: implement size
	return gz, size, nil
}

// Return the size of the endpoint "through the eye" of the previous reader. Note: there is no
// qcow2 reader so nil is returned so that nothing is appended to the reader stack.
// Note: size is stored at offset 24 in the qcow2 header.
func (d *DataStream) qcow2NopReader(h *image.Header) (io.Reader, int64, error) {
	s := hex.EncodeToString(d.buf[h.SizeOff : h.SizeOff+h.SizeLen])
	size, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "unable to determine original qcow2 file size from %+v", s)
	}
	return nil, size, nil
}

// Return the xz reader and size of the endpoint "through the eye" of the previous reader.
// Assumes a single file was compressed. Note: the xz reader is not a closer so we wrap a
// nop Closer around it.
//NOTE: size is not stored in the xz header. This may require the file to be decompressed in
//  order to get its original size. For now 0 is returned.
//TODO: support gz size.
func (d *DataStream) xzReader() (io.Reader, int64, error) {
	xz, err := xz.NewReader(d.topReader())
	if err != nil {
		return nil, 0, errors.Wrap(err, "could not create xz reader")
	}
	size := int64(0) //TODO: implement size
	return xz, size, nil
}

// Return the tar reader and size of the endpoint "through the eye" of the previous reader.
// Assumes a single file was archived.
// Note: the size stored in the header is used rather than raw metadata.
func (d *DataStream) tarReader() (io.Reader, int64, error) {
	if d.ContentType == string(cdiv1.DataVolumeArchive) {
		return d.mulFileTarReader()
	}
	tr := tar.NewReader(d.topReader())
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	if err != nil {
		return nil, 0, errors.Wrap(err, "could not read tar header")
	}
	klog.V(2).Infof("tar: extracting %q\n", hdr.Name)
	return tr, hdr.Size, nil
}

// Note - the tar file is processed in dataStream.CopyData
// directly by calling util.UnArchiveTar.
func (d *DataStream) mulFileTarReader() (io.Reader, int64, error) {
	buf, err := ioutil.ReadAll(d.topReader())
	if err != nil {
		return nil, 0, err
	}
	tr := tar.NewReader(bytes.NewReader(buf))
	var size int64
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, 0, err
		}
		size += header.Size
	}
	return bytes.NewReader(buf), size, nil
}

// If the raw endpoint is an ISO file then set the receiver's Size via the iso metadata.
// ISO reference: http://alumnus.caltech.edu/~pje/iso9660.html
// Note: no error is returned if the enpoint does not match the expected iso format.
func (d *DataStream) isoSize() (int64, error) {
	// iso id values
	const (
		id        = "CD001"
		primaryVD = 1
	)
	// primary volume descriptor sector offset in iso file
	const (
		isoSectorSize        = 2048
		primVolDescriptorOff = 16 * isoSectorSize
	)
	// single volume descriptor layout (independent of location within file)
	// note: offsets are zero-relative and lengths are in bytes
	const (
		vdTypeOff       = 0
		typeLen         = 1
		vdIDOff         = 1
		idLen           = 5
		vdNumSectorsOff = 84
		numSectorsLen   = 4
		vdSectorSizeOff = 130
		sectorSizeLen   = 2
	)
	// primary volume descriptor layout within full iso file (lengths are defined above)
	const (
		typeOff       = vdTypeOff + primVolDescriptorOff
		idOff         = vdIDOff + primVolDescriptorOff
		numSectorsOff = vdNumSectorsOff + primVolDescriptorOff
		sectorSizeOff = vdSectorSizeOff + primVolDescriptorOff // last field we care about
	)
	const bufSize = sectorSizeOff + sectorSizeLen

	buf := make([]byte, bufSize)
	_, err := d.Read(buf) // read primary volume descriptor
	if err != nil {
		return 0, errors.Wrapf(err, "attempting to read ISO primary volume descriptor")
	}
	// append multi-reader so that the iso data can be re-read by subsequent readers
	d.appendReader(rdrMulti, bytes.NewReader(buf))

	// ensure we have an iso file by checking the type and id value
	vdtyp, err := strconv.Atoi(hex.EncodeToString(buf[typeOff : typeOff+typeLen]))
	if err != nil {
		klog.Errorf("isoSize: Atoi error on endpoint %q: %v", d.url.Path, err)
		return 0, nil
	}
	if vdtyp != primaryVD && string(buf[idOff:idOff+idLen]) != id {
		klog.V(3).Infof("isoSize: endpoint %q is not an ISO file", d.url.Path)
		return 0, nil
	}

	// get the logical block/sector size (expect 2048)
	s := hex.EncodeToString(buf[sectorSizeOff : sectorSizeOff+sectorSizeLen])
	sectSize, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		klog.Errorf("isoSize: sector size ParseInt error on endpoint %q: %v", d.url.Path, err)
		return 0, nil
	}
	// get the number sectors
	s = hex.EncodeToString(buf[numSectorsOff : numSectorsOff+numSectorsLen])
	numSects, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		klog.Errorf("isoSize: sector count ParseInt error on endpoint %q: %v", d.url.Path, err)
		return 0, nil
	}
	return int64(numSects * sectSize), nil
}

// Return the matching header, if one is found, from the passed-in map of known headers. After a
// successful read append a multi-reader to the receiver's reader stack.
// Note: .iso files are not detected here but rather in the Size() function.
// Note: knownHdrs is passed by reference and modified.
func (d *DataStream) matchHeader(knownHdrs *image.Headers) (*image.Header, error) {
	_, err := d.Read(d.buf) // read current header
	if err != nil {
		return nil, err
	}
	// append multi-reader so that the header data can be re-read by subsequent readers
	d.appendReader(rdrMulti, bytes.NewReader(d.buf))

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
func closeReaders(readers []reader) (rtnerr error) {
	var err error
	for i := len(readers) - 1; i >= 0; i-- {
		err = readers[i].rdr.Close()
		if err != nil {
			rtnerr = err // tracking last error
		}
	}
	return rtnerr
}

func (d *DataStream) isHTTPQcow2() bool {
	return (d.url.Scheme == "http" || d.url.Scheme == "https") &&
		d.AccessKey == "" &&
		d.SecKey == "" &&
		d.qemu &&
		len(d.Readers) == 2
}

func (d *DataStream) calculateTargetSize(dest string) int64 {
	targetQuantity := resource.NewScaledQuantity(util.GetAvailableSpace(filepath.Dir(dest)), 0)
	if d.ImageSize != "" {
		newImageSizeQuantity := resource.MustParse(d.ImageSize)
		minQuantity := util.MinQuantity(targetQuantity, &newImageSizeQuantity)
		targetQuantity = &minQuantity
	}
	targetSize, _ := targetQuantity.AsInt64()
	return targetSize
}

func (d *DataStream) convertQcow2ToRawStream(dest string) error {
	klog.V(3).Infoln("Validating qcow2 file")

	err := qemuOperations.Validate(d.url.String(), "qcow2", d.calculateTargetSize(dest))
	if err != nil {
		return errors.Wrap(err, "Streaming image validation failed")
	}
	klog.V(3).Infoln("Doing streaming qcow2 to raw conversion")
	err = qemuOperations.ConvertQcow2ToRawStream(d.url, dest)
	if err != nil {
		return errors.Wrap(err, "Streaming qcow2 to raw conversion failed")
	}

	return nil
}

func (d *DataStream) convertQcow2ToRaw(src, dest string) error {
	klog.V(3).Infoln("Validating qcow2 file")
	err := qemuOperations.Validate(src, "qcow2", d.calculateTargetSize(dest))
	if err != nil {
		return errors.Wrap(err, "Local image validation failed")
	}

	klog.V(2).Infoln("converting qcow2 image")
	err = qemuOperations.ConvertQcow2ToRaw(src, dest)
	if err != nil {
		return errors.Wrap(err, "Local qcow to raw conversion failed")
	}
	return nil
}

// Copy endpoint to dest based on passed-in reader.
func (d *DataStream) copy(dest string) error {
	if util.GetAvailableSpace(d.ScratchDataDir) > int64(0) {
		defer CleanDir(d.ScratchDataDir)
	}
	if d.isHTTPQcow2() {
		err := d.convertQcow2ToRawStream(dest)
		if err != nil {
			return err
		}
	} else {
		if util.GetAvailableSpace(d.ScratchDataDir) <= int64(0) {
			//Need scratch space but none provided.
			return ErrRequiresScratchSpace
		}
		// Replace /data/target name with scratch path/target name
		tmpDest := filepath.Join(d.ScratchDataDir, filepath.Base(dest))

		err := StreamDataToFile(d.topReader(), tmpDest)
		if err != nil {
			return err
		}

		// The actual copy
		if d.qemu {
			err = d.convertQcow2ToRaw(tmpDest, dest)
			if err != nil {
				return err
			}
		} else {
			err = util.MoveFileAcrossFs(tmpDest, dest)
			if err != nil {
				return err
			}
		}
	}

	if !d.isIsoImage && d.ImageSize != "" {
		klog.V(3).Infoln("Resizing image")
		err := ResizeImage(dest, d.ImageSize, d.AvailableSpace)
		if err != nil {
			return errors.Wrap(err, "Resize of image failed")
		}
	}

	return nil
}

// parseDataPath only used for debugging
func (d *DataStream) parseDataPath() (string, string) {
	pathSlice := strings.Split(strings.Trim(d.url.EscapedPath(), "/"), "/")
	klog.V(3).Infof("parseDataPath: url path: %v", pathSlice)
	return pathSlice[0], strings.Join(pathSlice[1:], "/")
}
