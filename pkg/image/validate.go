package image

import (
	"strings"
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

var SupportedImageFormats = []string{
	ExtImg, ExtIso, ExtQcow2,
}

var SupportedFileExtensions = append(SupportedImageFormats,
	append(SupportedCompressionExtensions,
	append(SupportedArchiveExtensions, SupportedNestedExtensions...)...)...)

func IsSupporedFileType(fn string) bool {
	fn = TrimString(fn)
	for _, ext := range SupportedFileExtensions {
		if strings.HasSuffix(fn, string(ext)) {
			return true
		}
	}
	return false
}

func IsSupporedCompressionType(fn string) bool {
	fn = TrimString(fn)
	for _, ext := range SupportedCompressionExtensions {
		if strings.HasSuffix(fn, string(ext)) {
			return true
		}
	}
	return false
}

func IsSupporedArchiveType(fn string) bool {
	fn = TrimString(fn)
	for _, ext := range SupportedArchiveExtensions {
		if strings.HasSuffix(fn, string(ext)) {
			return true
		}
	}
	return false
}
