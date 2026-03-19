package main

import (
	"errors"
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/sys/unix"

	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

// Mock implementations for testing

type mockSeekHoleFile struct {
	name       string
	writeError error
	seekError  error
	closed     bool
}

func (m *mockSeekHoleFile) Write(p []byte) (n int, err error) {
	if m.writeError != nil {
		return 0, m.writeError
	}
	return len(p), nil
}

func (m *mockSeekHoleFile) Seek(offset int64, whence int) (int64, error) {
	if m.seekError != nil {
		return 0, m.seekError
	}
	return offset, nil
}

func (m *mockSeekHoleFile) Name() string {
	return m.name
}

func (m *mockSeekHoleFile) Close() error {
	m.closed = true
	return nil
}

type mockSeekHoleChecker struct {
	createError error
	removeError error
	file        *mockSeekHoleFile
}

func (m *mockSeekHoleChecker) createTempFile(dir, pattern string) (seekHoleFile, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	return m.file, nil
}

func (m *mockSeekHoleChecker) removeFile(name string) error {
	return m.removeError
}

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
	It("Should detect SEEK_HOLE support when filesystem supports it", func() {
		mockFile := &mockSeekHoleFile{
			name:       "/tmp/test",
			writeError: nil,
			seekError:  nil,
		}
		mockChecker := &mockSeekHoleChecker{
			createError: nil,
			file:        mockFile,
		}

		result := filesystemSupportsSeekHole("/test/path", mockChecker)
		Expect(result).To(BeTrue())
		Expect(mockFile.closed).To(BeTrue())
	})

	It("Should return false when SEEK_HOLE is not supported (Seek returns error)", func() {
		mockFile := &mockSeekHoleFile{
			name:       "/tmp/test",
			writeError: nil,
			seekError:  unix.EINVAL, // Filesystem doesn't support SEEK_HOLE
		}
		mockChecker := &mockSeekHoleChecker{
			createError: nil,
			file:        mockFile,
		}

		result := filesystemSupportsSeekHole("/test/path", mockChecker)
		Expect(result).To(BeFalse())
		Expect(mockFile.closed).To(BeTrue())
	})

	It("Should return false when temp file creation fails", func() {
		mockChecker := &mockSeekHoleChecker{
			createError: errors.New("permission denied"),
		}

		result := filesystemSupportsSeekHole("/test/path", mockChecker)
		Expect(result).To(BeFalse())
	})

	It("Should return false when write to test file fails", func() {
		mockFile := &mockSeekHoleFile{
			name:       "/tmp/test",
			writeError: errors.New("write failed"),
		}
		mockChecker := &mockSeekHoleChecker{
			createError: nil,
			file:        mockFile,
		}

		result := filesystemSupportsSeekHole("/test/path", mockChecker)
		Expect(result).To(BeFalse())
		Expect(mockFile.closed).To(BeTrue())
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
