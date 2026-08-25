package prometheus

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/util/cert"

	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	ownerUID = "1111-1111-111"
)

var _ = Describe("Timed update", func() {

	It("Should start and stop when finished", func() {
		r := io.NopCloser(bytes.NewReader([]byte("hello world")))
		progressReader := NewProgressReader(r, metrics.Progress(ownerUID), uint64(11))
		progressReader.StartTimedUpdate()
		_, err := io.ReadAll(r)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("Update Progress", func() {
	var progressMetric ProgressMetric

	BeforeEach(func() {
		err := metrics.SetupMetrics()
		Expect(err).NotTo(HaveOccurred())
		progressMetric = metrics.Progress(ownerUID)
	})

	AfterEach(func() {
		progressMetric.Delete()
	})

	It("Parse valid progress update", func() {
		By("Verifying the initial value is 0")
		metrics.Progress(ownerUID).Add(0)
		progress, err := progressMetric.Get()
		Expect(err).ToNot(HaveOccurred())
		Expect(progress).To(Equal(float64(0)))
		By("Calling updateProgress with value")
		promReader := &ProgressReader{
			CountingReader: util.CountingReader{
				Current: uint64(45),
			},
			metric: progressMetric,
			total:  uint64(100),
			final:  true,
		}
		result := promReader.updateProgress()
		Expect(true).To(Equal(result))
		progress, err = progressMetric.Get()
		Expect(err).ToNot(HaveOccurred())
		Expect(progress).To(Equal(float64(45)))
	})

	It("0 total should return 0", func() {
		By("Calling updateProgress with value")
		promReader := &ProgressReader{
			CountingReader: util.CountingReader{
				Current: uint64(45),
			},
			metric: progressMetric,
			total:  uint64(0),
			final:  true,
		}
		result := promReader.updateProgress()
		Expect(false).To(Equal(result))
		progress, err := progressMetric.Get()
		Expect(err).ToNot(HaveOccurred())
		Expect(progress).To(Equal(float64(0)))
	})

	It("current and total equals should return false", func() {
		By("Calling updateProgress with value")
		promReader := &ProgressReader{
			CountingReader: util.CountingReader{
				Current: uint64(1000),
				Done:    true,
			},
			metric: metrics.Progress(ownerUID),
			total:  uint64(1000),
			final:  true,
		}
		result := promReader.updateProgress()
		Expect(false).To(Equal(result))
		progress, err := metrics.Progress(ownerUID).Get()
		Expect(err).ToNot(HaveOccurred())
		Expect(progress).To(Equal(float64(100)))
	})

	DescribeTable("update progress on non-final readers", func(readerDone, isFinal, expectedResult bool) {
		promReader := &ProgressReader{
			CountingReader: util.CountingReader{
				Current: uint64(1000),
				Done:    readerDone,
			},
			metric: progressMetric,
			total:  uint64(1000),
			final:  isFinal,
		}
		result := promReader.updateProgress()
		Expect(expectedResult).To(Equal(result))
	},
		Entry("should return true when reader is not done", false, false, true),
		Entry("should return true when reader is done", true, false, true),
		Entry("should return true when final reader is not done", false, true, true),
		Entry("should return false when final reader is done", true, true, false),
	)

	It("should continue to update progress after next reader is set", func() {
		firstReader := util.CountingReader{
			Reader: io.NopCloser(strings.NewReader("first")),
		}
		secondReader := util.CountingReader{
			Reader: io.NopCloser(strings.NewReader("second")),
		}
		thirdReader := util.CountingReader{
			Reader: io.NopCloser(strings.NewReader("third")),
		}
		promReader := &ProgressReader{
			CountingReader: firstReader,
			metric:         progressMetric,
			total:          uint64(16),
			final:          false,
		}

		data := make([]byte, 10)
		read, _ := promReader.Read(data)
		Expect(read).To(Equal(5))
		_, err := promReader.Read(data)
		Expect(err).To(Equal(io.EOF))
		result := promReader.updateProgress()
		Expect(true).To(Equal(result))
		Expect(promReader.CountingReader.Current).To(Equal(uint64(5)))

		promReader.SetNextReader(secondReader.Reader, false)
		read, _ = promReader.Read(data)
		Expect(read).To(Equal(6))
		_, err = promReader.Read(data)
		Expect(err).To(Equal(io.EOF))
		result = promReader.updateProgress()
		Expect(promReader.CountingReader.Reader).To(Equal(secondReader.Reader))
		Expect(promReader.CountingReader.Current).To(Equal(uint64(11)))
		Expect(true).To(Equal(result))

		promReader.SetNextReader(thirdReader.Reader, true)
		read, _ = promReader.Read(data)
		Expect(read).To(Equal(5))
		_, err = promReader.Read(data)
		Expect(err).To(Equal(io.EOF))
		result = promReader.updateProgress()
		Expect(promReader.CountingReader.Reader).To(Equal(thirdReader.Reader))
		Expect(promReader.CountingReader.Current).To(Equal(uint64(16)))
		Expect(false).To(Equal(result))
	})
})

var _ = Describe("StartPrometheusEndpointNoCertGeneration", func() {
	var certsDir string

	BeforeEach(func() {
		var err error
		certsDir, err = os.MkdirTemp("", "prometheus-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(certsDir)
	})

	It("should return nil when both cert and key files exist", func() {
		certBytes, keyBytes, err := cert.GenerateSelfSignedCertKey("localhost", nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(path.Join(certsDir, CertFileName), certBytes, 0600)).To(Succeed())
		Expect(os.WriteFile(path.Join(certsDir, KeyFileName), keyBytes, 0600)).To(Succeed())

		err = StartPrometheusEndpointNoCertGeneration(certsDir)
		Expect(err).NotTo(HaveOccurred())

		httpClient := &http.Client{
			Timeout: 2 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		}
		Eventually(func() error {
			_, err := httpClient.Get("https://localhost:8443/metrics")
			return err
		}, 5*time.Second, 200*time.Millisecond).Should(Succeed())
	})

	It("should return an error if the cert file does not exist", func() {
		err := StartPrometheusEndpointNoCertGeneration(certsDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cert file"))
		Expect(err.Error()).To(ContainSubstring("does not exist"))
	})

	It("should return an error if only the cert file exists (key missing)", func() {
		certBytes, _, err := cert.GenerateSelfSignedCertKey("localhost", nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(path.Join(certsDir, CertFileName), certBytes, 0600)).To(Succeed())

		err = StartPrometheusEndpointNoCertGeneration(certsDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("key file"))
		Expect(err.Error()).To(ContainSubstring("does not exist"))
	})

	It("should return an error if only the key file exists (cert missing)", func() {
		_, keyBytes, err := cert.GenerateSelfSignedCertKey("localhost", nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(path.Join(certsDir, KeyFileName), keyBytes, 0600)).To(Succeed())

		err = StartPrometheusEndpointNoCertGeneration(certsDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("cert file"))
		Expect(err.Error()).To(ContainSubstring("does not exist"))
	})
})

var _ = Describe("StartPrometheusEndpoint", func() {
	It("should generate cert and key files in the specified directory", func() {
		certsDir, err := os.MkdirTemp("", "prometheus-test-*")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(certsDir)

		StartPrometheusEndpoint(certsDir)

		Eventually(func() bool {
			_, certErr := os.Stat(path.Join(certsDir, "tls.crt"))
			_, keyErr := os.Stat(path.Join(certsDir, "tls.key"))
			return certErr == nil && keyErr == nil
		}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())
	})
})
