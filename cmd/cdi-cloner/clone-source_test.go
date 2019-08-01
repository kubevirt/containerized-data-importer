package main

import (
	"io"
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

var _ = Describe("Prometheus Endpoint", func() {
	It("Should start prometheus endpoint", func() {
		By("Creating cert directory, we can store self signed CAs")
		certsDirectory, err := ioutil.TempDir("", "certsdir")
		Expect(err).NotTo(HaveOccurred())
		empty, err := isDirEmpty(certsDirectory)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeTrue())
		prometheusutil.StartPrometheusEndpoint(certsDirectory)
		time.Sleep(time.Second)
		empty, err = isDirEmpty(certsDirectory)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeFalse())
		defer os.RemoveAll(certsDirectory)
	})
})

func isDirEmpty(dirName string) (bool, error) {
	f, err := os.Open(dirName)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
