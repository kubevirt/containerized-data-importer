package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"

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

var _ = Describe("seekHoleSupported", func() {
	var (
		tempDir      string
		originalSeek func(int, int64, int) (int64, error)
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "seek-test-")
		Expect(err).NotTo(HaveOccurred())
		originalSeek = seekFunc
	})

	AfterEach(func() {
		seekFunc = originalSeek
		os.RemoveAll(tempDir)
	})

	It("should return true when file does not exist", func() {
		result := seekHoleSupported(filepath.Join(tempDir, "does-not-exist.img"))
		Expect(result).To(BeTrue())
	})

	It("should return true for empty file", func() {
		emptyFile := filepath.Join(tempDir, "empty.img")
		Expect(os.WriteFile(emptyFile, []byte{}, 0644)).To(Succeed())

		result := seekHoleSupported(emptyFile)
		Expect(result).To(BeTrue())
	})

	It("should return true for entirely sparse file", func() {
		testFile := filepath.Join(tempDir, "sparse.img")
		Expect(os.WriteFile(testFile, []byte("test"), 0644)).To(Succeed())

		seekFunc = func(fd int, offset int64, whence int) (int64, error) {
			if whence == unix.SEEK_DATA {
				return 0, unix.ENXIO
			}
			return unix.Seek(fd, offset, whence)
		}

		result := seekHoleSupported(testFile)
		Expect(result).To(BeTrue())
	})

	It("should detect VFS fallback", func() {
		testFile := filepath.Join(tempDir, "vfs-fallback.img")
		fileContent := []byte("test data")
		Expect(os.WriteFile(testFile, fileContent, 0644)).To(Succeed())

		seekFunc = func(fd int, offset int64, whence int) (int64, error) {
			if whence == unix.SEEK_DATA {
				return 0, nil
			}
			if whence == unix.SEEK_HOLE {
				return int64(len(fileContent)), nil
			}
			return 0, errors.New("unexpected")
		}

		result := seekHoleSupported(testFile)
		Expect(result).To(BeFalse())
	})
})
