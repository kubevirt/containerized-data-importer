package image

import (
	"strings"
)

const (
	Qcow2 = ".qcow2"
	Tar   = ".tar" // note: uppercase Tar to not conflict with tar pkg
	TarGz = Tar + ".gz"
	TarXz = Tar + ".xz"
)

var SupportedFileExtensions = []string{
	Qcow2, Tar, TarGz, TarXz,
}
var SupportedCompressionExtensions = []string{
	Tar, TarGz, TarXz,
}

func IsQcow2(file string) bool {
	return strings.HasSuffix(file, Qcow2)
}

func IsTarBall(file string) bool {
	return strings.HasSuffix(file, Tar) || strings.HasSuffix(file, TarGz) || strings.HasSuffix(file, TarXz)
}

func IsValidImageFile(file string) bool {
	f := strings.ToLower(strings.TrimSpace(file))
	return IsQcow2(f) || IsTarBall(f)
}
