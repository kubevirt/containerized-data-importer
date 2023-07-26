package system

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPrLimit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prlimit Suite")
}
