// +build unit_test

package importer

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestImporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Importer Suite")
}
