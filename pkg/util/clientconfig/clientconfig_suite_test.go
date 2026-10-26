package clientconfig

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestClientConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Config Test Suite")
}
