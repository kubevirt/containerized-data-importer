package system

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPrLimit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prlimit Suite")
}
