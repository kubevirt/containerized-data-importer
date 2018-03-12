package image

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

var SupportedCompressionExtensions = []string{
	Gz,
}

var SupportedArchiveExtentions = []string{
	TarArch, TarArch + Gz,
}

// UnpackData combines the decompressing and unarchiving of a data stream and returns the
// end of the stream for further processing.
func UnpackData(filename string, src io.Reader) io.Reader {
	var uncompData io.ReadCloser
	var unarchData io.Reader
	/*if file is raw image{
		return io.ReadCloser (raw image stream)
	}*/
	uncompData = DecompressData(filename, src)
	defer uncompData.Close()
	unarchData = DearchiveData(filename, uncompData)

	/*if file is qcow2 {
	convert to raw image
	return io.ReadCloser
	}*/
	return unarchData
}

// DecompressData analyzes the filename (string) to decide which decompression function to call.
// Compression packages return objects that must be closed after reading,
// hence the need to return a ReadCloser.  It is up to the caller of DecompressData
// to close the returned stream.
// If not compression is detected, it is considered a 'noop' and the original stream is
// returned.
func DecompressData(filename string, src io.ReadCloser) io.Reader{
	if src == nil || filename == "" {
		return nil
	}
	fn := filepath.Ext(strings.ToLower(strings.TrimSpace(filename)))
	var stream io.ReadCloser
	switch fn {
	case Gz:
		stream = gunzip(src)
		return stream
	}
	return src //noop
}

// DearchiveData analyzes a filename (string) to decided which de-archive function to call.
// Golang archive packages return readers and do not need to be closed, so only an io.Reader
// is returned.
// If not archive format is detected, it is considered a 'noop' and the original stream is
// returned.
func DearchiveData(filename string, src io.Reader) io.Reader {
	var err error
	switch {
	case strings.HasSuffix(filename, TarArch):
		src, err = Unarchive(filename, src)
	}
	if err != nil {
		return nil
	}
	return src // noop
}

func gunzip(r io.Reader) io.ReadCloser {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil
	}
	return gzr
}

func untar(r io.Reader) io.Reader {
	tr := tar.NewReader(r)
	return tr
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
	var fn string
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Unarchive: unexpected tar read error on %q: %v\n", srcFile, err)
		}
		if fn != "" {
			return nil, fmt.Errorf("Unarchive: excpect only 1 file in archive %q\n", srcFile)
		}
		fn = hdr.Name
		fmt.Printf("\n**** archived filename=%q\n", fn)
	}
	if fn == "" {
		return nil, fmt.Errorf("Unarchive: excpect 1 file in archive %q\n", srcFile)
	}
	return tr, nil
}
