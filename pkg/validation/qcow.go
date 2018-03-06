package validation

import (
	"path/filepath"
	"strings"
)

const (
	qcow2Ext = "qcow2"
	tar      = "tar"
	gzip     = "gz"
	xz       = "xz"
)

func IsQcow2(file string) bool {
	return filepath.Ext(file) == qcow2Ext
}

func IsTarBall(file string) bool {
	return strings.Contains(file, tar) && filepath.Ext(file) == gzip || filepath.Ext(file) == xz
}

func IsValidImageFile(file string) bool {
	return IsQcow2(file) || IsTarBall(file)
}
