package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
)

// Return string as lowercase with all spaces removed.
func prepareString(f string) string {
	return strings.ToLower(strings.TrimSpace(f))
}

// UnpackData combines the decompressing and unarchiving of a data stream and returns the
// end of the stream for further processing.
func UnpackData(filename string, src io.Reader) interface{} {
	glog.Infof("UnpackData: checking compressed and/or archive for file %q\n", filename)
	/*if file is raw image{
		return io.ReadCloser (raw image stream)
	}*/
	var iface interface{}
	filename, iface = DecompressData(filename, src)
	r := iface.(io.ReadCloser)
	src = DearchiveData(filename, r).(io.Reader)
	/*if file is qcow2 {
		convert to raw image
		return io.ReadCloser
	}*/
	return src
}

// TODO fixup comment
// DecompressData analyzes the filename (string) to decide which decompression function to call.
// Compression packages return objects that must be closed after reading,
// hence the need to return a ReadCloser.  It is up to the caller of DecompressData
// to close the returned stream.
// If no compression is detected, it is considered a 'noop' and the original stream is returned.
// Returns trimmed filename string and gzip Reader if gzip compression was used.
func DecompressData(filename string, src io.Reader) (string, interface{}) {
	glog.Infof("DecompressData: checking if %q is compressed\n", filename)
	if src == nil || filename == "" {
		glog.Errorln("DecompressData: Nil parameter found.")
		return "", nil
	}
	ext := filepath.Ext(prepareString(filename))
	switch ext {
	case ExtGz:
		glog.Infof("DecompressData: detected %v compression format", ExtGz)
		filename = strings.TrimSuffix(filename, ext) // trim ".gz"
		src = gunzip(src)
	default:
		// noop
	}
	return filename, src
}

// TODO fixup comment
// DearchiveData analyzes a filename (string) to decided which de-archive function to call.
// Golang archive packages return readers and do not need to be closed, so only an io.Reader
// is returned.
// If not archive format is detected, it is considered a 'noop' and the original stream is
// returned.
func DearchiveData(filename string, src io.Reader) interface{} {
	var err error
	glog.Infof("DearchiveData: checking if %q is an archive file\n", filename)
	ext := filepath.Ext(prepareString(filename))
	//TODO: strip .gz ext prior to this call, if present, then check for just .tar
	var r io.Reader
	switch ext {
	case ExtTar:
		glog.Infof("DearchiveData: detected %v archive format\n", ext)
		r, err = tarPrep(filename, src)
		if err != nil {
			glog.Infof("DearchiveData: error: %v\n", err)
			return src // return passed-in reader
		}
	default:
		r = src
	}
	return r
}

func gunzip(r io.Reader) io.ReadCloser {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		glog.Errorf("gunzip: error creating new reader: %v", err)
		return nil
	}
	return gzr
}

// TODO: generalize for all compression formats. This just handles tar!
func tarPrep(srcFile string, r io.Reader) (io.Reader, error) {
	tr := tar.NewReader(r)
	hdr, err := tr.Next() // advance cursor to 1st (and only) file in tarball
	glog.Infof("tarPrep: extracting name %q\n", hdr.Name)
	if err != nil {
		return r, fmt.Errorf("tarPrep: reading tarfile %q header: %v\n", srcFile, err)
	}
	return tr, nil
}
