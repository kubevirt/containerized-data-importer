package main

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"kubevirt.io/containerized-data-importer/tests/reporters"
)

func TestClonerTarget(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Cloner Test Suite", reporters.NewReporters())
}
