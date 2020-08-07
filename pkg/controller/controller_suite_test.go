package controller_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

var _ = BeforeSuite(func() {
	logf.SetLogger(logf.ZapLoggerTo(GinkgoWriter, true))
})

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Controller Suite", reporters.NewReporters())
}
