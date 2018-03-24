package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/xi2/xz"
)

// Return string as lowercase with all spaces removed.
func TrimString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// UnpackData combines the decompressing and unarchiving of a data stream and returns the
// end of the stream for further processing.
// Note: order of unpacking is hard-coded: 1) decompress, 2) un-archive. This can be changed
//   to be sensitive to the file's (sub-)extensions.
func UnpackData(filename string, src io.ReadCloser) (io.ReadCloser, error) {
	glog.Infof("UnpackData: checking compressed and/or archive for file %q\n", filename)
	var err error
	// see if file is compressed
	filename, src, err = DecompressData(filename, src)
	if err == nil { // see if file is an archive
		src, err = DearchiveData(filename, src)
	}
	if err != nil {
		return nil, fmt.Errorf("UnpackData: decompress/un-archive error: %v\n", err)
	}
	return src, nil
}

// DecompressData analyzes the filename extension to decide which decompression function to call.
// Compression packages return objects that must be closed after reading, hence the need to
// return a ReadCloser.  It is up to the caller of DecompressData to close the returned stream.
// If no compression is detected, it is considered a 'noop' and the original stream is returned.
// Returns trimmed filename string and gzip Reader if gzip compression was used.
func DecompressData(filename string, src io.ReadCloser) (fn string, rc io.ReadCloser, err error) {
	glog.Infof("DecompressData: checking if %q is compressed\n", filename)
	ext := filepath.Ext(TrimString(filename))
	switch ext {
	case ExtGz, ExtXz:
		glog.Infof("DecompressData: detected %v compression format", ext)
		switch ext {
		case ExtGz:
			rc, err = gunzip(src)
		case ExtXz:
			rc, err = xzDecompress(src)
		}
		if err != nil {
			return "", nil, fmt.Errorf("DecompressData: %v\n", err)
		}
		fn = strings.TrimSuffix(filename, ext) // trim compression extension
		return fn, rc, nil
	}
	return filename, src, nil // orig filename and reader
}

// DearchiveData analyzes a filename extension to decided which de-archive function to call.
// If no archive format is detected, it is considered a 'noop' and the original stream is
// returned.
func DearchiveData(filename string, src io.ReadCloser) (io.ReadCloser, error) {
	glog.Infof("DearchiveData: checking if %q is an archive file\n", filename)
	ext := filepath.Ext(TrimString(filename))
	switch ext {
	case ExtTar:
		glog.Infof("DearchiveData: detected %v archive format\n", ext)
		return tarPrep(filename, src)
	}
	return src, nil // orig reader
}

func gunzip(r io.ReadCloser) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

func xzDecompress(r io.ReadCloser) (io.ReadCloser, error) {
	rdr, err := xz.NewReader(r, 0) //note: default dict size may be too small
	if err != nil {
		return nil, fmt.Errorf("xzDecompress: error creating xz Reader: %v\n", err)
	}
	return ioutil.NopCloser(rdr), nil
}

// TODO: support other archive formats? This just handles tar.
func tarPrep(srcFile string, r io.ReadCloser) (io.ReadCloser, error) {
	tr := tar.NewReader(r)
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	if err != nil {
		return r, fmt.Errorf("tarPrep: reading tarfile %q header: %v\n", srcFile, err)
	}
	glog.Infof("tarPrep: extracting %q\n", hdr.Name)
	return ioutil.NopCloser(tr), nil
}
