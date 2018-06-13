package image

import (
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
)

const (
	ExtImg   = ".img"
	ExtIso   = ".iso"
	ExtGz    = ".gz"
	ExtQcow2 = ".qcow2"
	ExtTar   = ".tar"
	ExtXz    = ".xz"
	ExtTarXz = ExtTar + ExtXz
	ExtTarGz = ExtTar + ExtGz
)

var SupportedNestedExtensions = []string{
	ExtTarGz, ExtTarXz,
}

var SupportedCompressionExtensions = []string{
	ExtGz, ExtXz,
}

var SupportedArchiveExtensions = []string{
	ExtTar,
}

var SupportedCompressArchiveExtensions = append(
	SupportedCompressionExtensions,
	SupportedArchiveExtensions...,
)

var SupportedImageFormats = []string{
	ExtImg, ExtIso, ExtQcow2,
}

var SupportedFileExtensions = append(
	SupportedImageFormats, append(
		SupportedCompressionExtensions, append(
			SupportedArchiveExtensions,
			SupportedNestedExtensions...,
		)...,
	)...,
)

func IsSupportedType(fn string, exts []string) bool {
	fn = TrimString(fn)
	for _, ext := range exts {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}

func IsSupportedFileType(fn string) bool {
	return IsSupportedType(fn, SupportedFileExtensions)
}

func IsSupportedCompressionType(fn string) bool {
	return IsSupportedType(fn, SupportedCompressionExtensions)
}

func IsSupportedArchiveType(fn string) bool {
	return IsSupportedType(fn, SupportedArchiveExtensions)
}

func IsSupportedCompressArchiveType(fn string) bool {
	return IsSupportedType(fn, SupportedCompressArchiveExtensions)
}

// Return string as lowercase with all spaces removed.
func TrimString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// supported file format magic strings;
var (
	GzMagicStr    = []byte{0x1F, 0x8B}
	Qcow2MagicStr = []byte{'Q', 'F', 'I', 0xFB}
	TarMagicStr   = []byte{0x75, 0x73, 0x74, 0x61, 0x72, 0x00, 0x30, 0x30} // ustar.00  OR just 6 bytes
	XzMagicStr    = []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00, 0x00}
)

type offSizeT struct{
	off int
	size int
}
type magicMapT map[string]offSizeT
var magicMap = magicMapT {
	ExtImg:   offSizeT{off: 0, size: 0},
	ExtGz:    offSizeT{off: 0, size: len(GzMagicStr)},
	ExtQcow2: offSizeT{off: 0, size: len(Qcow2MagicStr)},
	ExtTar:   offSizeT{off: 257, size: len(TarMagicStr)},
	ExtXz:    offSizeT{off: 0, size: len(XzMagicStr)},
}

func FileFormat(ext string, r io.ReadCloser) (string, io.ReadCloser, error) {
	offSize, ok := magicMap[ext]
	if !ok || offSize.size == 0 {
		return "", r, nil
	}
	magic, rdr, err := GetMagicString(r, offSize.off, offSize.size)
	if err != nil {
		return "", rdr, err
	}
	return magic, rdr, nil
}


// Return the magic string defined in `r`, starting at `offset` for `size` bytes. A MultiReader is
// wrapped around the passed in reader so that the bytes read here are not lost. Zero is returned 
// for all errors and cases where the magic string cannot be found.
func GetMagicString(r io.ReadCloser, offset, size int) (string, io.ReadCloser, error) {
	bufSize := offset + size
	buf := make([]byte, bufSize)
	cnt, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF {
		return "", r, errors.Wrap(err, "unable to read byte buffer")
	}
	multiR := ioutil.NopCloser(io.MultiReader(bytes.NewReader(buf), r)) // don't lose bytes just read
	if cnt < bufSize {
		return "", multiR, errors.Errorf("only %d bytes read to find magic num at offset %d of size %d", cnt, offset, size)
	}
	magicStr := hex.EncodeToString(buf[offset:])
	return magicStr, multiR, nil
}
