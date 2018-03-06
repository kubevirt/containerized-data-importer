package validation

import (
	"path/filepath"
	"strings"

	"github.com/golang/glog"
)

const (
	qcow2Ext = ".qcow2"
	tar      = ".tar"
	gzip     = ".gz"
	xz       = ".xz"
)

func IsQcow2(file string) bool {
	return filepath.Ext(file) == qcow2Ext
}

func IsTarBall(file string) bool {
	return strings.Contains(file, tar) && filepath.Ext(file) == gzip || filepath.Ext(file) == xz
}

func IsValidImageFile(file string) bool {
	var valid bool
	if valid = IsQcow2(file) || IsTarBall(file); !valid {
		glog.Errorf("IsValidImageFile: file extension is unsupported. Expected *.qcow2, *.tar.gz or *.tar.xz; Got %s", file)
	}
	return valid
}
