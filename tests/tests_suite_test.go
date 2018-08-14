package tests_test

import (
	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"

	"testing"

	"kubevirt.io/qe-tools/pkg/ginkgo-reporters"

	"kubevirt.io/containerized-data-importer/tests"
)

func TestTests(t *testing.T) {
	RegisterFailHandler(tests.CDIFailHandler)
	reporters := make([]Reporter, 0)
	if ginkgo_reporters.Polarion.Run {
		reporters = append(reporters, &ginkgo_reporters.Polarion)
	}
	if ginkgo_reporters.JunitOutput != "" {
		reporters = append(reporters, ginkgo_reporters.NewJunitReporter())
	}
	RunSpecsWithDefaultAndCustomReporters(t, "Tests Suite", reporters)
}

var _ = BeforeSuite(func() {
	client, err := tests.GetKubeClient()
	if err == nil {
		tests.DestroyAllTestNamespaces(client)
	}
})

var _ = AfterSuite(func() {
	client, err := tests.GetKubeClient()
	if err == nil {
		tests.DestroyAllTestNamespaces(client)
	}
})
