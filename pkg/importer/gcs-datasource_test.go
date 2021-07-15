package importer

import (
	"cloud.google.com/go/storage"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
)

var existentObject = "gs://bucket-bar/object-foo"
var validSAKey = "this is a valid service account key"

var _ = Describe("GCS data source", func() {
	var (
		gd     *GCSDataSource
		tmpDir string
		err    error
	)

	BeforeEach(func() {
		newGCSReader = createMockGCSReader
		tmpDir, err = ioutil.TempDir("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		newGCSReader = getGCSReader
		if gd != nil {
			gd.Close()
		}
		os.RemoveAll(tmpDir)
	})

	It("NewGCSDataSource should fail when accessing GCS with an invalid service account key", func() {
		gd, err = NewGCSDataSource("http://bucket-bar/object-utopia", "fake service account key")
		Expect(err).To(HaveOccurred())
	})

	It("NewGCSDataSource should fail when accessing a nonexistent object", func() {
		gd, err = NewGCSDataSource("http://bucket-bar/object-utopia", validSAKey)
		Expect(err).To(HaveOccurred())
	})

	It("Info should fail when reading an invalid image", func() {
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		gd, err = NewGCSDataSource("gs://bucket-bar/object-foo", validSAKey)
		Expect(err).NotTo(HaveOccurred())

		gd.gcsReader = file
		result, err := gd.Info()
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(ProcessingPhaseError))
	})

	It("Info should return Transfer when reading a valid image", func() {
		file, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())
		gd, err = NewGCSDataSource("gs://bucket-bar/object-foo", validSAKey)
		Expect(err).NotTo(HaveOccurred())

		gd.gcsReader = file
		result, err := gd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	It("Info should return TransferDataFile when reading a valid image", func() {
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		gd, err = NewGCSDataSource("gs://bucket-bar/object-foo", validSAKey)
		Expect(err).NotTo(HaveOccurred())

		gd.gcsReader = file
		result, err := gd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
	})

	table.DescribeTable("calling transfer should", func(fileName, scratchPath string, want []byte, wantErr bool) {
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		sourceFile, err := os.Open(fileName)
		Expect(err).NotTo(HaveOccurred())

		gd, err = NewGCSDataSource("gs://bucket-bar/object-foo", validSAKey)
		Expect(err).NotTo(HaveOccurred())

		gd.gcsReader = sourceFile
		nextPhase, err := gd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(nextPhase))
		result, err := gd.Transfer(scratchPath)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(ProcessingPhaseConvert).To(Equal(result))
			file, err := os.Open(filepath.Join(scratchPath, tempFile))
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()
			fileStat, err := file.Stat()
			Expect(err).NotTo(HaveOccurred())
			Expect(int64(len(want))).To(Equal(fileStat.Size()))
			resultBuffer, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())
			Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
			Expect(file.Name()).To(Equal(gd.GetURL().String()))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(result))
		}
	},
		table.Entry("return Error with missing scratch space", cirrosFilePath, "/imaninvalidpath", nil, true),
		table.Entry("return Convert with scratch space and valid qcow file", cirrosFilePath, "", cirrosData, false),
	)

	It("Transfer should fail on reader error", func() {
		sourceFile, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())

		gd, err = NewGCSDataSource("gs://bucket-bar/object-foo", validSAKey)
		Expect(err).NotTo(HaveOccurred())

		gd.gcsReader = sourceFile
		nextPhase, err := gd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(nextPhase))
		err = sourceFile.Close()
		Expect(err).NotTo(HaveOccurred())
		result, err := gd.Transfer(tmpDir)
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile should succeed when writing to valid file", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		gd, err = NewGCSDataSource("gs://bucket-bar/object-foo", validSAKey)
		Expect(err).NotTo(HaveOccurred())

		gd.gcsReader = file
		result, err := gd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = gd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile should fail on streaming error", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		gd, err = NewGCSDataSource("gs://bucket-bar/object-foo", validSAKey)
		Expect(err).NotTo(HaveOccurred())

		gd.gcsReader = file
		result, err := gd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = gd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})
})

func createMockGCSReader(endpoint *url.URL, saKey string) (*storage.Reader, error) {
	if saKey != validSAKey {
		return nil, errors.New("invalid service account key")
	}
	if endpoint.String() != existentObject {
		return nil, errors.New("object does not exist")
	}
	return nil, nil
}
