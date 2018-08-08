package polarion_xml_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestPolarionXml(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PolarionXml Suite")
}
