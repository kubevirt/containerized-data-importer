package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/common"
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

var _ = Describe("getInputStream content type handling", func() {
	var testDir string

	BeforeEach(func() {
		var err error
		testDir, err = os.MkdirTemp("", "clone-test")
		Expect(err).NotTo(HaveOccurred())
		mountPoint = testDir
	})

	AfterEach(func() {
		os.RemoveAll(testDir)
	})

	Context("filesystem-clone mode", func() {
		It("Should create tar reader for filesystem-clone", func() {
			contentType = "filesystem-clone"
			diskImgPath := filepath.Join(testDir, common.DiskImageName)
			err := os.WriteFile(diskImgPath, []byte("test data"), 0644)
			Expect(err).NotTo(HaveOccurred())

			stream := getInputStream(false)
			Expect(stream).NotTo(BeNil())
			stream.Close()
		})
	})

	Context("disk-image-clone mode", func() {
		It("Should open disk.img directly for disk-image-clone", func() {
			contentType = "disk-image-clone"
			diskImgPath := filepath.Join(testDir, common.DiskImageName)
			err := os.WriteFile(diskImgPath, []byte("test data"), 0644)
			Expect(err).NotTo(HaveOccurred())

			stream := getInputStream(false)
			Expect(stream).NotTo(BeNil())
			stream.Close()
		})
	})

	Context("blockdevice-clone mode", func() {
		It("Should open block device for blockdevice-clone", func() {
			contentType = "blockdevice-clone"
			blockDevPath := filepath.Join(testDir, "block-device")
			err := os.WriteFile(blockDevPath, []byte("block data"), 0644)
			Expect(err).NotTo(HaveOccurred())
			mountPoint = blockDevPath

			stream := getInputStream(false)
			Expect(stream).NotTo(BeNil())
			stream.Close()
		})
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
