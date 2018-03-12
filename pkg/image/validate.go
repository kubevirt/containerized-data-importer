package image

import (
	"strings"
)

const (
	Qcow2   = ".qcow2"
	TarArch = ".tar"
	Gz      = ".gz"
)

var SupportedFileExtensions = []string{
	Qcow2, TarArch, Gz,
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
