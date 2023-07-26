package uploadserver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUploadserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Upload Server Suite")
}
