package importer

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func TestImporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Importer Suite", reporters.NewReporters())
}
