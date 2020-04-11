package uploadproxy_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func TestUploadproxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Uploadproxy Suite", reporters.NewReporters())
}
