package importer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
		tmpDir, err = os.MkdirTemp("", "scratch")
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
		sd, err = NewS3DataSource("thisisinvalid#$%#ep", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should Error, when failing to create S3 client", func() {
		newClientFunc = failMockS3Client
		sd, err = NewS3DataSource("http://amazon.com", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should Error, when failing to get object", func() {
		newClientFunc = createErrMockS3Client
		sd, err = NewS3DataSource("http://amazon.com", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewS3DataSource should fail when called with an invalid certdir", func() {
		newClientFunc = getS3Client
		sd, err = NewS3DataSource("http://amazon.com", "", "", "/invaliddir")
		Expect(err).To(HaveOccurred())
	})

	It("Info should return Error, when passed in an invalid image", func() {
		// Don't need to defer close, since ud.Close will close the reader
		file, err := os.Open(filepath.Join(imageDir, "content.tar"))
		Expect(err).NotTo(HaveOccurred())
		err = file.Close()
		Expect(err).NotTo(HaveOccurred())
		sd, err = NewS3DataSource("http://region.amazon.com/bucket-1/object-1", "", "", "")
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
		sd, err = NewS3DataSource("http://region.amazon.com/bucket-1/object-1", "", "", "")
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
		sd, err = NewS3DataSource("http://region.amazon.com/bucket-1/object-1", "", "", "")
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

		sd, err = NewS3DataSource("http://region.amazon.com/bucket-1/object-1", "", "", "")
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

		sd, err = NewS3DataSource("http://region.amazon.com/bucket-1/object-1", "", "", "")
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
		sd, err = NewS3DataSource("http://region.amazon.com/bucket-1/object-1", "", "", "")
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
		sd, err = NewS3DataSource("http://region.amazon.com/bucket-1/object-1", "", "", "")
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

	It("GetS3Client should return a real client", func() {
		_, err := getS3Client("", "", "", "", "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("getS3Client should address the bucket in the path and honor the endpoint scheme", func() {
		var gotPath, gotHost, gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotHost, gotAuth = r.URL.Path, r.Host, r.Header.Get("Authorization")
			_, _ = w.Write([]byte("hello"))
		}))
		defer srv.Close()

		// Address the server by hostname, not by IP: the SDK cannot use virtual-host
		// addressing against a bare IP and silently falls back to path style, which
		// would leave UsePathStyle unverified.
		host := "localhost:" + srv.URL[strings.LastIndex(srv.URL, ":")+1:]
		svc, err := getS3Client(host, "accessKey", "secKey", "", httpScheme)
		Expect(err).NotTo(HaveOccurred())

		out, err := svc.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: aws.String("bucket-1"),
			Key:    aws.String("object-1"),
		})
		Expect(err).NotTo(HaveOccurred())
		defer out.Body.Close()

		Expect(gotHost).To(Equal(host))
		Expect(gotPath).To(Equal("/bucket-1/object-1"))
		Expect(gotAuth).To(HavePrefix("AWS4-HMAC-SHA256 Credential=accessKey/"))
	})

	It("getS3Client should fall back to the SDK default credential chain when no static credentials are given", func() {
		// A DataVolume without a secretRef reaches getS3Client with empty keys.
		// The pod's ambient identity (IRSA and EKS Pod Identity inject env vars
		// or a token file) must then be picked up through the SDK default chain;
		// env credentials stand in for that injection here.
		GinkgoT().Setenv("AWS_ACCESS_KEY_ID", "chainAccessKey")
		GinkgoT().Setenv("AWS_SECRET_ACCESS_KEY", "chainSecretKey")

		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte("hello"))
		}))
		defer srv.Close()

		host := "localhost:" + srv.URL[strings.LastIndex(srv.URL, ":")+1:]
		svc, err := getS3Client(host, "", "", "", httpScheme)
		Expect(err).NotTo(HaveOccurred())

		out, err := svc.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: aws.String("bucket-1"),
			Key:    aws.String("object-1"),
		})
		Expect(err).NotTo(HaveOccurred())
		defer out.Body.Close()

		Expect(gotAuth).To(HavePrefix("AWS4-HMAC-SHA256 Credential=chainAccessKey/"))
	})

	DescribeTable("extractRegion should derive a valid region", func(endpoint, want string) {
		Expect(extractRegion(endpoint)).To(Equal(want))
	},
		Entry("dotless host with port", "minio:9000", "minio"),
		Entry("dotted host with port", "minio-service.default:9000", "minio-service"),
		Entry("AWS regional endpoint", "s3.us-east-1.amazonaws.com", "us-east-1"),
		Entry("IP host with port", "127.0.0.1:9000", "127"),
		Entry("localhost with port", "localhost:9000", "localhost"),
		Entry("host without port", "minio", "minio"),
	)

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

func (mc *MockS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if !mc.doErr {
		return &s3.GetObjectOutput{}, nil
	}
	return nil, errors.New("Failed to get object")
}
