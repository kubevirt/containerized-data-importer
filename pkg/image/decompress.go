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

const (
	TarArch = ".tar"
	Gz      = ".gz"
	Qcow2   = ".qcow2"
	Img     = ".img"
)

var SupportedExtentions = []string{
	TarArch, TarArch + Gz, Gz,
}

var SupportedImageFormat = []string{
	Qcow2, Img,
}

var SupportedFileExtensions = append(SupportedImageFormat, SupportedExtentions...)

// TODO fixup comment
// UnpackData combines the decompressing and unarchiving of a data stream and returns the
// end of the stream for further processing.
func UnpackData(filename string, src io.Reader) interface{} {
	glog.Infof("UnpackData: filename: %v, src: %T\n", filename, src)
	/*if file is raw image{
		return io.ReadCloser (raw image stream)
	}*/
	src = DecompressData(filename, src).(io.ReadCloser)
	src = DearchiveData(filename, src).(io.Reader)
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
// If not compression is detected, it is considered a 'noop' and the original stream is
// returned.
func DecompressData(filename string, src io.Reader) interface{} {
	glog.Infof("DecompressData: filename: %v, src: %T", filename, src)
	if src == nil || filename == "" {
		glog.Errorln("DecompressData: Nil parameter found.")
		return nil
	}
	ext := filepath.Ext(prepareString(filename))
	glog.Infof("DecompressData: checking ext: %s", ext)
	switch ext {
	case Gz:
		glog.Infof("DecompressData: detected %s compression format", Gz)
		src = gunzip(src)
	default:
		// noop
	}
	return src
}

// TODO fixup comment
// DearchiveData analyzes a filename (string) to decided which de-archive function to call.
// Golang archive packages return readers and do not need to be closed, so only an io.Reader
// is returned.
// If not archive format is detected, it is considered a 'noop' and the original stream is
// returned.
func DearchiveData(filename string, src io.Reader) interface{} {
	glog.Infof("DearchiveData: filename: %v, src: %T", filename, src)
	filename = prepareString(filename)
	switch {
	case strings.HasSuffix(filename, TarArch), strings.HasSuffix(filename, TarArch+Gz):
		glog.Infof("")
		src, _ = Unarchive(filename, src)
	default:
		// noop
	}
	return src
}

func gunzip(r io.Reader) io.ReadCloser {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		glog.Errorf("gunzip: error creating new reader: %v", err)
		return nil
	}
	glog.Infof("gunzip: created new gzip reader: %T", gzr)
	return gzr
}

// TODO: generalize for all compression formats. This just handles tar!
func Unarchive(filename string, src io.Reader) (io.Reader, error) {
	glog.Infof("Unarchive: srcfile: %v, f: %T", filename, src)
	var fn string
	tr := tar.NewReader(src)
	glog.Infof("Unarchive: new tar reader: %T", tr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			glog.Errorf("Unarchive: unexpected tar read error on %q: %v\n", filename, err)
			return nil, fmt.Errorf("Unarchive: unexpected tar read error on %q: %v\n", filename, err)
		}
		if fn != "" {
			glog.Errorf("Unarchive: excpect only 1 file in archive %q\n", filename)
			return nil, fmt.Errorf("Unarchive: excpect only 1 file in archive %q\n", filename)
		}
		fn = hdr.Name
		fmt.Printf("\n**** archived filename=%q\n", fn)
	}
	if fn == "" {
		return nil, fmt.Errorf("Unarchive: excpect 1 file in archive %q\n", filename)
	}
	return tr, nil
}

func prepareString(f string) string {
	return strings.ToLower(strings.TrimSpace(f))
}
