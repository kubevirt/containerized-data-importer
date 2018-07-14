package importer_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Known lib.Size() exceptations due to:
//   1) .gz and .xz not supporting size in their headers (but that's ok if they are wrapped by tar
//      or the underlying file is an iso or qcow2 file), and
//   2) in tinyCore.iso where the sector count being less than what is in the file (sparseness?)
// All other variations (iso.tar, iso.gz, iso.xz, iso.tar.gz, qcow2, qcow2.gz, qcow2.tar.gz...)
// should have the exact byte size returned. Exceptions are skipped but reported.
//
var sizeExceptions = map[string]struct{}{
	"tinyCore.iso": struct{}{},
	//"tinyCore.iso.gz": struct{}{},
	//"tinyCore.iso.xz": struct{}{},
}


func TestDatastream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datastream Suite")
}

var _ = AfterSuite(func() {
	fmt.Fprintf(GinkgoWriter, "\nINFO: the following file formats are skipped in the `lib.Size()` tests:\n")
	for ex := range sizeExceptions {
		fmt.Fprintf(GinkgoWriter, "\t%s\n", ex)
	}
})
