package image

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
)

var SupportedCompressionExtensions = []string{
	Gz,
}

var SupportedArchiveExtentions = []string{
	TarArch, TarArch + Gz,
}

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
	fn := filepath.Ext(strings.ToLower(strings.TrimSpace(filename)))
	glog.Infof("DecompressData: checking ext: %v", fn)
	switch fn {
	case Gz:
		glog.Infof("DecompressData: detected %v is of %v format", fn, Gz)
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
	glog.Infoln("DearchiveData: filename: %v, src: %v", )
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

func IsCompressed(filename string) bool {
	fExt := filepath.Ext(filename)
	if fExt == "" {
		return false
	}
	for _, ext := range SupportedCompressionExtensions {
		if ext == fExt {
			return true
		}
	}
	return false
}

func IsArchived(filename string) bool {
	for _, v := range SupportedArchiveExtentions {
		if strings.HasSuffix(filename, v) {
			return true
		}
	}
	return false
}

// TODO: generalize for all compression formats. This just handles tar!
func Unarchive(srcFile string, f io.Reader) (io.Reader, error) {
	glog.Infof("Unarchive: srcfile: %v, f: %T", srcFile, f)
	//var fn string
	tr := tar.NewReader(f)
	glog.Infof("Unarchive: new tar reader: %T", tr)
	//for {
	//	hdr, err := tr.Next()
	//	if err == io.EOF {
	//		break
	//	}
	//	if err != nil {
	//		glog.Errorf("Unarchive: unexpected tar read error on %q: %v\n", srcFile, err)
	//		return nil, fmt.Errorf("Unarchive: unexpected tar read error on %q: %v\n", srcFile, err)
	//	}
	//	if fn != "" {
	//		glog.Errorf("Unarchive: excpect only 1 file in archive %q\n", srcFile)
	//		return nil, fmt.Errorf("Unarchive: excpect only 1 file in archive %q\n", srcFile)
	//	}
	//	fn = hdr.Name
	//	fmt.Printf("\n**** archived filename=%q\n", fn)
	//}
	//if fn == "" {
	//	return nil, fmt.Errorf("Unarchive: excpect 1 file in archive %q\n", srcFile)
	//}
	return tr, nil
}
