package image

import (
	"strings"
)

const (
	ExtImg   = ".img"
	ExtQcow2 = ".qcow2"
	ExtGz    = ".gz"
	ExtTar   = ".tar"
)

var SupportedCompressionExtensions = []string{
	ExtTar, ExtGz,
}

var SupportedImageFormats = []string{
	ExtImg, ExtQcow2,
}

var SupportedFileExtensions = append(SupportedImageFormats, SupportedCompressionExtensions...)

func IsSupporedFileType(filename string) bool {
	fn := strings.ToLower(strings.TrimSpace(filename))
	for _, ext := range SupportedFileExtensions {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}
