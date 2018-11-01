package keys

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func TestKeyStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Keystore Suite", reporters.NewReporters())
}
