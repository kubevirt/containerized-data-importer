package keys

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestKeyStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keystore Suite")
}
