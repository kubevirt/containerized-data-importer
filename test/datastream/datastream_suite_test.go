// +build functional_test

package datastream

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDatastream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datastream Suite")
}
