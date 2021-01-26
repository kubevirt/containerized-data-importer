package featuregates

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func TestFeatureGates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "FeatureGates Suite", reporters.NewReporters())
}
