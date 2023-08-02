package pvkrbdrxbounce

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRbdVolumesCollector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RbdVolumesCollector Suite")
}
