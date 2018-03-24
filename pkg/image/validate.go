package image

import (
	"strings"
)

const (
	ExtImg   = ".img"
	ExtIso   = ".iso"
	ExtQcow2 = ".qcow2"
	ExtGz    = ".gz"
	ExtTar   = ".tar"
)

var SupportedCompressionExtensions = []string{
	ExtGz,
}

var SupportedArchiveExtensions = []string{
	ExtTar,
}

var SupportedImageFormats = []string{
	ExtImg, ExtIso, ExtQcow2,
}

var SupportedFileExtensions = append(SupportedImageFormats, append(SupportedCompressionExtensions, SupportedArchiveExtensions...)...)

func IsSupporedFileType(fn string) bool {
	fn = TrimString(fn)
	for _, ext := range SupportedFileExtensions {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}

func IsSupporedCompressionType(fn string) bool {
	fn = TrimString(fn)
	for _, ext := range SupportedCompressionExtensions {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}

func IsSupporedArchiveType(fn string) bool {
	fn = TrimString(fn)
	for _, ext := range SupportedArchiveExtensions {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}
