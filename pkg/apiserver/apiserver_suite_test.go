package apiserver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestApiserver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Server Suite")
}
