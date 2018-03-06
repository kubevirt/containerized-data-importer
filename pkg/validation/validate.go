package validation

import (
	"strings"

	"github.com/golang/glog"
)

const (
	qcow2 = ".qcow2"
	tar   = ".tar"
	tarGz = tar + ".gz"
	tarXz = tar + ".xz"
)

// extList organizes the supported extensions for printing. When adding or removing an extension constant, make the
// appropriate alteration in this slice.
var extList = []string{qcow2, tar, tarGz, tarXz}

func IsQcow2(file string) bool {
	return strings.HasSuffix(qcow2, file)
}

func IsTarBall(file string) bool {
	return strings.HasSuffix(file, tar) || strings.HasSuffix(file, tarGz) || strings.HasSuffix(file, tarXz)
}

func IsValidImageFile(file string) bool {
	f := strings.ToLower(strings.TrimSpace(file))
	var valid bool
	if valid = IsQcow2(f) || IsTarBall(f); !valid {
		glog.Errorf("IsValidImageFile: file extension is unsupported. Expected %s; Got %s", extList, file)
	}
	return valid
}
