// +build unit_test

package importer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestImporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Importer Suite")
}
