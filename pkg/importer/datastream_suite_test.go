package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Known size.Size() exceptions due to:
//   1) .gz and .xz not supporting size in their headers (but that's ok if they are wrapped by tar
//      or the underlying file is a qcow2 file), and
//   2) in tinyCore.iso where the returned size is smaller than the original. Note: this is not
//      the case for larger iso files such as windows.
var sizeExceptions = map[string]struct{}{
	".iso":    {},
	".iso.gz": {},
	".iso.xz": {},
}

func TestDatastream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datastream Suite")
}

var _ = AfterSuite(func() {
	for _, filename := range testfiles {
		os.Remove(filepath.Join(os.TempDir(), filename))
	}
	fmt.Fprintf(GinkgoWriter, "\nINFO: the following file formats are skipped in the `size.Size()` tests:\n")
	for ex := range sizeExceptions {
		fmt.Fprintf(GinkgoWriter, "\t%s\n", ex)
	}
})
