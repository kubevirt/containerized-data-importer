package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	ovirtclient "github.com/ovirt/go-ovirt-client"
)

type ovirtTestReader struct {
	data     []byte
	position int64
}

func (o *ovirtTestReader) Read(p []byte) (n int, err error) {
	n = copy(p, o.data[o.position:])
	o.position += int64(n)
	return n, nil
}

func (o *ovirtTestReader) Seek(offset int64, whence int) (int64, error) {
	newPosition := o.position
	switch whence {
	case io.SeekStart:
		newPosition = offset
	case io.SeekEnd:
		newPosition = int64(len(o.data)) + offset
	case io.SeekCurrent:
		newPosition = o.position + offset
	default:
		return 0, fmt.Errorf("invalid whence value for seek: %d", whence)
	}

	if o.position > int64(len(o.data)) {
		return 0, fmt.Errorf("seek beyond file end")
	}
	if o.position < 0 {
		return 0, fmt.Errorf("seek before file start")
	}
	o.position = newPosition
	return o.position, nil
}

func (o *ovirtTestReader) Close() error {
	return nil
}

var _ = Describe("Imageio data source", func() {
	var (
		diskID  string
		tempDir string
		err     error
	)

	BeforeEach(func() {
		tempDir, err = os.MkdirTemp(os.TempDir(), "imageio-test-*")
		if err != nil {
			panic(fmt.Errorf("failed to create temporary directory (%w)", err))
		}
		newOVirtClient = mockOvirtClientFactory

		mockOVirtClient = ovirtclient.NewMock()
		// The mock has a built-in storage domain that can be used for testing purposes.
		storageDomains, _ := mockOVirtClient.ListStorageDomains()

		// Upload an empty image for testing.
		uploadResult, _ := mockOVirtClient.UploadImage(
			"test",
			storageDomains[0].ID(),
			false,
			512,
			&ovirtTestReader{make([]byte, 512), 0},
		)
		diskID = uploadResult.Disk().ID()
	})

	AfterEach(func() {
		newOVirtClient = defaultOVirtClientFactory
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	It("NewImageioDataSource should fail when called with an invalid endpoint", func() {
		newOVirtClient = defaultOVirtClientFactory
		_, err = NewImageioDataSource("httpd://!@#$%^&*()dgsdd&3r53/invalid", "", "", "", diskID)
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource info should not fail when called with valid endpoint", func() {
		dp, err := NewImageioDataSource("", "", "", tempDir, diskID)
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Info()
		Expect(err).ToNot(HaveOccurred())
	})

	It("NewImageioDataSource TransferFile should fail when invalid path", func() {
		dp, err := NewImageioDataSource("", "", "", tempDir, diskID)
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Info()
		Expect(err).NotTo(HaveOccurred())
		phase, err := dp.TransferFile("")
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
	})

	It("NewImageioDataSource TransferFile should succeed with a valid path", func() {
		path := filepath.Join(tempDir, "image.img")
		dp, err := NewImageioDataSource("", "", "", tempDir, diskID)
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Info()
		Expect(err).NotTo(HaveOccurred())
		phase, err := dp.TransferFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseResize))
		Expect(path).To(BeAnExistingFile())
		Expect(path).To(haveFileSize(512))
	})
})

// haveFileSize succeeds if the given file has the specified size.
func haveFileSize(size int64) types.GomegaMatcher {
	return &haveFileSizeMatcher{size}
}

type haveFileSizeMatcher struct {
	size int64
}

func (matcher *haveFileSizeMatcher) Match(actual interface{}) (success bool, err error) {
	actualFilename, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("BeAnExistingFileMatcher matcher expects a file path")
	}

	stat, err := os.Stat(actualFilename)
	if err != nil {
		switch {
		case os.IsNotExist(err):
			return false, nil
		default:
			return false, err
		}
	}

	return stat.Size() == matcher.size, nil
}

func (matcher *haveFileSizeMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("to have a size of %d", matcher.size))
}

func (matcher *haveFileSizeMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, fmt.Sprintf("to not have a size of %d", matcher.size))
}
