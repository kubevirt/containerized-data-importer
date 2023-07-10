package uploadproxy_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUploadproxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Uploadproxy Suite")
}
