package image

import (
	"strings"
)



func IsSupporedFileType(filename string) bool {
	fn := strings.ToLower(strings.TrimSpace(filename))
	for _, ext := range SupportedFileExtensions {
		if strings.HasSuffix(fn, ext) {
			return true
		}
	}
	return false
}
