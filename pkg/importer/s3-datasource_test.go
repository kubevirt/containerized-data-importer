package importer

import (
	"io"
	"os"
	"path/filepath"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

var _ = Describe("S3 data source", func() {
	var (
		sd                        *S3DataSource
		tmpDir                    string
		err                       error
		originalAllowedSourceURLs string
		allowlistWasSet           bool
	)

	BeforeEach(func() {
		newClientFunc = createMockS3Client
		tmpDir, err = os.MkdirTemp("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)

		// Save and set allowlist for S3 test endpoints
		originalAllowedSourceURLs, allowlistWasSet = os.LookupEnv(common.ImporterAllowedSourceURLs)
		// Allow localhost for S3 tests (DNS rebinding protection requires resolvable hostnames)
		Expect(os.Setenv(common.ImporterAllowedSourceURLs, "localhost,127.0.0.0/8,::1/128")).To(Succeed())
	})

	AfterEach(func() {
		newClientFunc = getS3Client
		if sd != nil {
			sd.Close()
		}
		os.RemoveAll(tmpDir)

		// Restore original allowlist
		if allowlistWasSet {
			Expect(os.Setenv(common.ImporterAllowedSourceURLs, originalAllowedSourceURLs)).To(Succeed())
		} else {
			Expect(os.Unsetenv(common.ImporterAllowedSourceURLs)).To(Succeed())
		}
	})

	It("NewS3DataSource should Error, when passed in an invalid endpoint", func() {
		sd, err = NewS3DataSource("thisisinvalid#$%#ep", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should Error, when failing to create S3 client", func() {
		newClientFunc = failMockS3Client
		sd, err = NewS3DataSource("http://localhost", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should Error, when failing to get object", func() {
		newClientFunc = createErrMockS3Client
		sd, err = NewS3DataSource("http://localhost", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should fail when called with an invalid certdir", func() {
		newClientFunc = getS3Client
		sd, err = NewS3DataSource("http://localhost", "", "", "/invaliddir")
		Expect(err).To(HaveOccurred())
	})

	It("Info should return Error, when passed in an invalid image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://localhost/bucket-1/object-1", "", "", "")
		Expect(err).NotTo(HaveOccurred())
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("Info should return Transfer, when passed in a valid image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://localhost/bucket-1/object-1", "", "", "")
		Expect(err).NotTo(HaveOccurred())
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(result))
	})

	It("Info should return TransferDataFile, when passed in a valid raw image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://localhost/bucket-1/object-1", "", "", "")
		Expect(err).NotTo(HaveOccurred())
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
	})

	DescribeTable("calling transfer should", func(fileName, scratchPath string, want []byte, wantErr bool) {
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		sourceFile, err := os.Open(fileName)
		Expect(err).NotTo(HaveOccurred())

		sd, err = NewS3DataSource("http://localhost/bucket-1/object-1", "", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = sourceFile
		nextPhase, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(nextPhase))
		result, err := sd.Transfer(scratchPath, false)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(ProcessingPhaseConvert).To(Equal(result))
			file, err := os.Open(filepath.Join(scratchPath, tempFile))
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()
			fileStat, err := file.Stat()
			Expect(err).NotTo(HaveOccurred())
			Expect(int64(len(want))).To(Equal(fileStat.Size()))
			resultBuffer, err := io.ReadAll(file)
			Expect(err).NotTo(HaveOccurred())
			Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
			Expect(file.Name()).To(Equal(sd.GetURL().String()))
		} else {
			Expect(err).To(HaveOccurred())
			Expect(ProcessingPhaseError).To(Equal(result))
		}
	},
		Entry("return Error with missing scratch space", cirrosFilePath, "/imaninvalidpath", nil, true),
		Entry("return Convert with scratch space and valid qcow file", cirrosFilePath, "", cirrosData, false),
	)

	It("Transfer should fail on reader error", func() {
		sourceFile, err := os.Open(cirrosFilePath)
		Expect(err).NotTo(HaveOccurred())

		sd, err = NewS3DataSource("http://localhost/bucket-1/object-1", "", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = sourceFile
		nextPhase, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferScratch).To(Equal(nextPhase))
		err = sourceFile.Close()
		Expect(err).NotTo(HaveOccurred())
		result, err := sd.Transfer(tmpDir, false)
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("TransferFile should succeed when writing to valid file", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://localhost/bucket-1/object-1", "", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile(filepath.Join(tmpDir, "file"), false)
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile should fail on streaming error", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(tinyCoreFilePath)
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://localhost/bucket-1/object-1", "", "", "")
		Expect(err).NotTo(HaveOccurred())
		// Replace minio.Object with a reader we can use.
		sd.s3Reader = file
		result, err := sd.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = sd.TransferFile("/invalidpath/invalidfile", false)
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	Context("SSRF Protection", func() {
		var savedAllowlist string
		var savedAllowlistWasSet bool

		BeforeEach(func() {
			savedAllowlist, savedAllowlistWasSet = os.LookupEnv(common.ImporterAllowedSourceURLs)
		})

		AfterEach(func() {
			if savedAllowlistWasSet {
				Expect(os.Setenv(common.ImporterAllowedSourceURLs, savedAllowlist)).To(Succeed())
			} else {
				Expect(os.Unsetenv(common.ImporterAllowedSourceURLs)).To(Succeed())
			}
		})

		DescribeTable("should validate IPs according to blocklist and allowlist",
			func(endpoint string, allowlist string, expectSSRFError bool) {
				if allowlist != "" {
					Expect(os.Setenv(common.ImporterAllowedSourceURLs, allowlist)).To(Succeed())
				} else {
					Expect(os.Unsetenv(common.ImporterAllowedSourceURLs)).To(Succeed())
				}

				_, dsErr := NewS3DataSource(endpoint, "", "", "")
				if expectSSRFError {
					Expect(dsErr).To(HaveOccurred())
					Expect(dsErr.Error()).To(ContainSubstring("SSRF protection"))
				} else {
					// May fail for other reasons (connection, DNS, etc) but not SSRF
					if dsErr != nil {
						Expect(dsErr.Error()).NotTo(ContainSubstring("SSRF protection"))
					}
				}
			},
			// Blocked by default
			Entry("block AWS IMDS", "http://169.254.169.254/bucket/object", "", true),
			Entry("block Azure IMDS", "http://169.254.169.254/bucket/object", "", true),
			Entry("block private 10.x", "http://10.0.0.1/bucket/object", "", true),
			Entry("block private 192.168.x", "http://192.168.1.1/bucket/object", "", true),
			Entry("block CGNAT", "http://100.64.0.1/bucket/object", "", true),
			Entry("block loopback", "http://127.0.0.1:9000/bucket/object", "", true),

			// Allowed by allowlist (CIDR)
			Entry("allow 10.96.1.1 via CIDR allowlist", "http://10.96.1.1/bucket/object", "10.96.0.0/12", false),
			Entry("allow 192.168.1.100 via exact IP", "http://192.168.1.100/bucket/object", "192.168.1.100", false),

			// Allowed by allowlist (multiple entries)
			Entry("allow via multiple allowlist entries", "http://10.96.1.1/bucket/object", "172.16.0.0/12,10.96.0.0/12", false),

			// Localhost blocked without allowlist (loopback, not public)
			Entry("block localhost without allowlist", "http://localhost/bucket/object", "", true),
			Entry("allow localhost with allowlist", "http://localhost/bucket/object", "localhost,127.0.0.0/8,::1/128", false),
		)
	})

	It("GetS3Client should return a real client", func() {
		_, err := getS3Client("", "", "", "", "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should Extract Bucket and Object form the S3 URL", func() {
		bucket, object := extractBucketAndObject("Bucket1/Object.tmp")
		Expect(bucket).Should(Equal("Bucket1"))
		Expect(object).Should(Equal("Object.tmp"))

		bucket, object = extractBucketAndObject("Bucket1/Folder1/Object.tmp")
		Expect(bucket).Should(Equal("Bucket1"))
		Expect(object).Should(Equal("Folder1/Object.tmp"))
	})
})

// MockS3Client is a mock AWS S3 client
type MockS3Client struct {
	endpoint string //nolint:unused // TODO: check if need to remove this field
	accKey   string
	secKey   string
	certDir  string
	doErr    bool
}

func failMockS3Client(endpoint, accKey, secKey string, certDir string, urlScheme string) (S3Client, error) {
	return nil, errors.New("Failed to create client")
}

func createMockS3Client(endpoint, accKey, secKey string, certDir string, urlScheme string) (S3Client, error) {
	return &MockS3Client{
		accKey:  accKey,
		secKey:  secKey,
		certDir: certDir,
		doErr:   false,
	}, nil
}

func createErrMockS3Client(endpoint, accKey, secKey string, certDir string, urlScheme string) (S3Client, error) {
	return &MockS3Client{
		doErr: true,
	}, nil
}

func (mc *MockS3Client) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	if !mc.doErr {
		return &s3.GetObjectOutput{}, nil
	}
	return nil, errors.New("Failed to get object")
}
