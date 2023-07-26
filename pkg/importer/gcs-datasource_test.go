package importer

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Google Cloud Storage data source", func() {
	var (
		sd     *GCSDataSource
		tmpDir string
		err    error
	)

	BeforeEach(func() {
		newReaderFunc = mockGcsObjectReader
		tmpDir, err = os.MkdirTemp("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		if sd != nil {
			sd.Close()
		}
		os.RemoveAll(tmpDir)
	})

	It("Should Extract Bucket and Object form the GCS URL", func() {
		bucket, object := extractGcsBucketAndObject("gs://Bucket1/Object.tmp")
		Expect(bucket).Should(Equal("Bucket1"))
		Expect(object).Should(Equal("Object.tmp"))
		bucket, object = extractGcsBucketAndObject("gs://Bucket1/Folder1/Object.tmp")
		Expect(bucket).Should(Equal("Bucket1"))
		Expect(object).Should(Equal("Folder1/Object.tmp"))
	})

	It("Should Extract Bucket and Object form the HTTPS URL", func() {
		bucket, object, host := extractGcsBucketObjectAndHost("https://storage.cloud.google.com/Bucket1/Object.tmp")
		Expect(host).Should(Equal("https://storage.cloud.google.com/"))
		Expect(bucket).Should(Equal("Bucket1"))
		Expect(object).Should(Equal("Object.tmp"))
		bucket, object, host = extractGcsBucketObjectAndHost("https://storage.cloud.google.com/Bucket1/Folder1/Object.tmp")
		Expect(host).Should(Equal("https://storage.cloud.google.com/"))
		Expect(bucket).Should(Equal("Bucket1"))
		Expect(object).Should(Equal("Folder1/Object.tmp"))
	})

	It("NewGCSDataSource should Error, when passed in an invalid endpoint", func() {
		sd, err = NewGCSDataSource("thisisinvalid#$%#ep", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewGCSDataSource should Pass, when passed in an valid https endpoint without authentication", func() {
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/Object.tmp", "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("NewGCSDataSource should Pass, when passed in an valid gs endpoint without authentication", func() {
		sd, err = NewGCSDataSource("gs://Bucket1/Object.tmp", "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("NewGCSDataSource should Pass, when passed in an valid https endpoint with authentication", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/Object.tmp", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
	})

	It("NewGCSDataSource should Pass, when passed in an valid gs endpoint with authentication", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		sd, err = NewGCSDataSource("gs://Bucket1/Object.tmp", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Info should return Error, when passed in an invalid image using anonymous client and GCS endpoint", func() {
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/content.tar", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Info should return TransferDataFile, when passed in a valid RAW image using anonymous client and GCS endpoint", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros.raw", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
	})

	It("Info should return TransferScratch, when passed in a valid QCOW2 image using anonymous client and GCS endpoint", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros-qcow2.img", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	It("Info should return Error, when passed in an invalid image using authenticated client and GCS endpoint", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/content.tar", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Info should return TransferDataFile, when passed in a valid RAW image using authenticated client and GCS endpoint", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros.raw", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
	})

	It("Info should return TransferScratch, when passed in a valid QCOW2 image using authenticated client and GCS endpoint", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros-qcow2.img", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	It("Info should return Error, when passed in an invalid image using anonymous client and HTTP(s) endpoint", func() {
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/content.tar", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Info should return TransferDataFile, when passed in a valid RAW image using anonymous client and HTTP(s) endpoint", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros.raw", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
	})

	It("Info should return TransferScratch, when passed in a valid QCOW2 image using anonymous client and HTTP(s) endpoint", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros-qcow2.img", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	It("Info should return Error, when passed in an invalid image using authenticated client and HTTP(s) endpoint", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/content.tar", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Info should return TransferDataFile, when passed in a valid RAW image using authenticated client and HTTP(s) endpoint", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros.raw", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
	})

	It("Info should return TransferScratch, when passed in a valid QCOW2 image using authenticated client and HTTP(s) endpoint", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros-qcow2.img", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	It("TransferFile using anonymous client and GCS URL should succeed reading RAW image when writing to valid file", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros.raw", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using anonymous client and GCS URL should succeed reading QCOW2 image when writing to valid file", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros-qcow2.img", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using anonymous client and GCS should fail reading RAW image on streaming error", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros.raw", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile using anonymous client and GCS should fail reading QCOW2 image on streaming error", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros-qcow2.img", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile using authenticated client and GCS URL should succeed reading RAW image when writing to valid file", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros.raw", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using authenticated client and GCS URL should succeed reading QCOW2 image when writing to valid file", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros-qcow2.img", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using authenticated client and GCS should fail reading RAW image on streaming error", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros.raw", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile using authenticated client and GCS should fail reading QCOW2 image on streaming error", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("gs://Bucket1/cirros-qcow2.img", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile using anonymous client and HTTP(s) URL should succeed reading RAW image when writing to valid file", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros.raw", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using anonymous client and HTTP(s) URL should succeed reading QCOW2 image when writing to valid file", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros-qcow2.img", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using anonymous client and HTTP(s) should fail reading RAW image on streaming error", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros.raw", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile using anonymous client and HTTP(s) should fail reading QCOW2 image on streaming error", func() {
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros-qcow2.img", "")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile using authenticated client and HTTP(s) URL should succeed reading RAW image when writing to valid file", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros.raw", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using authenticated client and HTTP(s) URL should succeed reading QCOW2 image when writing to valid file", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros-qcow2.img", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile using authenticated client and HTTP(s) should fail reading RAW image on streaming error", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros.raw"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros.raw", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile using authenticated client and HTTP(s) should fail reading QCOW2 image on streaming error", func() {
		var sampleCredential = filepath.Join(imageDir, "gcs-secret.txt")
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", sampleCredential)
		file, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewGCSDataSource("https://storage.cloud.google.com/Bucket1/cirros-qcow2.img", "gcs-secret")
		Expect(err).NotTo(HaveOccurred())
		sd.gcsReader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})
})

// Create Cloud Storage Object Reader pointing to a sample image
func mockGcsObjectReader(ctx context.Context, client *storage.Client, bucket, object string) (io.ReadCloser, error) {
	var sampleImage = filepath.Join(imageDir, "cirros.raw")
	return os.Open(sampleImage)
}
