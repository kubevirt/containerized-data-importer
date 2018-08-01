package ginkgo_reporters_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGinkgoReporters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GinkgoReporters Suite")
}
