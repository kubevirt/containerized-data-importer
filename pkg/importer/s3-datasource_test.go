package importer

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	minio "github.com/minio/minio-go"
	"github.com/pkg/errors"
)

var _ = Describe("S3 data source", func() {
	var (
		sd     *S3DataSource
		tmpDir string
		err    error
	)

	BeforeEach(func() {
		newClientFunc = createMockS3Client
		tmpDir, err = ioutil.TempDir("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		newClientFunc = getS3Client
		if sd != nil {
			sd.Close()
		}
		os.RemoveAll(tmpDir)
	})

	It("NewS3DataSource should Error, when passed in an invalid endpoint", func() {
		sd, err = NewS3DataSource("thisisinvalid#$%#ep", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should Error, when failing to create minio client", func() {
		newClientFunc = failMockS3Client
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should Error, when failing to get object", func() {
		newClientFunc = createErrMockS3Client
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("Info should return Error, when passed in an invalid image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Info should return Transfer, when passed in a valid image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	It("Info should return TransferDataFile, when passed in a valid raw image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
	})

	table.DescribeTable("calling transfer should", func(fileName, scratchPath string, want []byte, wantErr bool) {
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		sourceFile, err := os.Open(fileName)
		Expect(err).NotTo(HaveOccurred())

		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = sourceFile
		nextPhase, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(nextPhase))
		result, err := sd.Transfer(scratchPath)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(ProcessingPhaseProcess).To(Equal(result))
			file, err := os.Open(filepath.Join(scratchPath, tempFile))
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()
			fileStat, err := file.Stat()
			Expect(err).NotTo(HaveOccurred())
			Expect(int64(len(want))).To(Equal(fileStat.Size()))
			resultBuffer, err := ioutil.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())
			Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
			Expect(file.Name()).To(Equal(sd.GetURL().String()))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(result))
		}
	},
		table.Entry("return Error with missing scratch space", cirrosFilePath, "/imaninvalidpath", nil, true),
		table.Entry("return Process with scratch space and valid qcow file", cirrosFilePath, "", cirrosData, false),
	)

	It("Transfer should fail on reader error", func() {
		sourceFile, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())

		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = sourceFile
		nextPhase, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(nextPhase))
		err = sourceFile.Close()
		Expect(err).NotTo(HaveOccurred())
		result, err := sd.Transfer(tmpDir)
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile should succeed when writing to valid file", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile should fail on streaming error", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Process should return Convert", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://amazon.com", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Process()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseConvert).To(Equal(result))
	})

	It("GetS3Client should return a real client", func() {
		_, err := getS3Client("", "", false)
		Expect(err).NotTo(HaveOccurred())
	})
})

// MockMinioClient is a mock minio client
type MockMinioClient struct {
	accKey string
	secKey string
	secure bool
	doErr  bool
}

func failMockS3Client(accKey, secKey string, secure bool) (S3Client, error) {
	return nil, errors.New("Failed to create client")
}

func createMockS3Client(accKey, secKey string, secure bool) (S3Client, error) {
	return &MockMinioClient{
		accKey: accKey,
		secKey: secKey,
		secure: secure,
		doErr:  false,
	}, nil
}

func createErrMockS3Client(accKey, secKey string, secure bool) (S3Client, error) {
	return &MockMinioClient{
		doErr: true,
	}, nil
}

func (mc *MockMinioClient) GetObject(bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	if !mc.doErr {
		return &minio.Object{}, nil
	}
	return nil, errors.New("Failed to get object")
}
