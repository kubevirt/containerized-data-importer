package tests_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/tests"
	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func TestTests(t *testing.T) {
	defer GinkgoRecover()
	RegisterFailHandler(tests.CDIFailHandler)
	RunSpecsWithDefaultAndCustomReporters(t, "Tests Suite", reporters.NewReporters())
}
