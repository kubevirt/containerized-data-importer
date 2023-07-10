package image

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestQEMU(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QEMU Suite")
}
