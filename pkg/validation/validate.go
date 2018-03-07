package validation

import (
	"strings"
)

const (
	qcow2 = ".qcow2"
	tar   = ".tar"
	tarGz = tar + ".gz"
	tarXz = tar + ".xz"
)

func IsQcow2(file string) bool {
	return strings.HasSuffix(qcow2, file)
}

func IsTarBall(file string) bool {
	return strings.HasSuffix(file, tar) || strings.HasSuffix(file, tarGz) || strings.HasSuffix(file, tarXz)
}

func IsValidImageFile(file string) bool {
	f := strings.ToLower(strings.TrimSpace(file))
	return IsQcow2(f) || IsTarBall(f)
}
