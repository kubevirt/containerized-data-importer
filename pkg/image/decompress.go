package image

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func IsCompressed(srcFile string) bool {
	fExt := filepath.Ext(srcFile)
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

// TODO: generalize for all compression formats. This just handles tar!
func Decompress(srcFile string) (string, error) {
	f, err := os.Open(srcFile)
	if err != nil {
		return "", fmt.Errorf("Decompress: open error on file %q: %v\n", srcFile, err)
	}
	defer f.Close()

	var fn string
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("Decompress: unexpected tar read error on %q: %v\n", srcFile, err)
		}
		if fn != "" {
			return "", fmt.Errorf("Decompress: excpect only 1 file in archive %q\n", srcFile)
		}
		fn = hdr.Name
		fmt.Printf("\n**** archived filename=%q\n", fn)
	}
	if fn == "" {
		return "", fmt.Errorf("Decompress: excpect 1 file in archive %q\n", srcFile)
	}
	return fn, nil
}
