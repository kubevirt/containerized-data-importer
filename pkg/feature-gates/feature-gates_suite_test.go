package featuregates

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFeatureGates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FeatureGates Suite")
}
