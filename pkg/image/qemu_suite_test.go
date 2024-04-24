package image

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestQEMU(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QEMU Suite")
}
