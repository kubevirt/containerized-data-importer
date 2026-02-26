package main

import (
	"errors"
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

var _ = Describe("Prometheus Endpoint", func() {
	It("Should start prometheus endpoint", func() {
		By("Creating cert directory, we can store self signed CAs")
		certsDirectory, err := os.MkdirTemp("", "certsdir")
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

var _ = Describe("SEEK_HOLE Detection", func() {
	It("Should detect SEEK_HOLE support on tmpfs", func() {
		tmpDir, err := os.MkdirTemp("", "seekhole_test")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		// This should return true on most modern linux systems with tmpfs
		Expect(filesystemSupportsSeekHole(tmpDir)).To(BeTrue())
	})

	It("Should handle inaccessible directory gracefully", func() {
		nonExistentDir := "/nonexistent/seekhole/test"

		// Should return false when it can't create test file
		result := filesystemSupportsSeekHole(nonExistentDir)
		Expect(result).To(BeFalse())
	})
})

func isDirEmpty(dirName string) (bool, error) {
	f, err := os.Open(dirName)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}
