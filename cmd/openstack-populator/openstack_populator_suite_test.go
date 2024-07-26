package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOpenstackPopulator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenstackPopulator Suite")
}
