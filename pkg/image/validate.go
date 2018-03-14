package image

import (
	"strings"
)

const (
	ExtQcow2 = ".qcow2"
	ExtGz	 = ".gz"
	ExtXz	 = ".xz"
	ExtTar	 = ".tar" // note: uppercase Tar to not conflict with tar pkg
	ExtTarGz = ExtTar + ExtGz
	ExtTarXz = ExtTar + ExtXz
)

var SupportedFileExtensions = []string{
	ExtQcow2, ExtGz, ExtTar, ExtTarGz, ExtTarXz,
}

var SupportedCompressionExtensions = []string{
	ExtTar, ExtTarGz, ExtTarXz,
}

var SupportedArchiveExtentions = []string{
        ExtTar, ExtTarGz,
}

func IsSupporedFileType(filename string) bool {
	fn := strings.ToLower(strings.TrimSpace(filename))
	for _, ext := range SupportedFileExtensions {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}
