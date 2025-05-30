package importer

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

var (
	cirrosFileName          = "cirros-qcow2.img"
	diskimageTarFileName    = "cirros.tar"
	tinyCoreGz              = "tinyCore.iso.gz"
	tinyCoreXz              = "tinyCore.iso.xz"
	cirrosData, _           = readFile(cirrosFilePath)
	diskimageArchiveData, _ = readFile(diskimageTarFileName)
)

var _ = Describe("Http data source", func() {
	var (
		ts        *httptest.Server
		dp        *HTTPDataSource
		err       error
		flushRead []byte
		tmpDir    string
	)

	BeforeEach(func() {
		createNbdkitCurl = image.NewMockNbdkitCurl
		By("[BeforeEach] Creating test server")
		ts = createTestServer(imageDir, nil)
		dp = nil
		tmpDir, err = os.MkdirTemp("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		if dp != nil {
			resultBuffer := make([]byte, len(flushRead))
			if dp.readers != nil {

				By("flushing data from readers we can finish test")
				_, err = dp.readers.read(resultBuffer)
				Expect(err).Should(Or(Not(HaveOccurred()), MatchError("EOF")))
			}
			By("[AfterEach] closing data provider")
			Expect(dp.Close()).To(Succeed())
		}
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
		By("[AfterEach] closing test server")
		ts.Close()
	})

	It("NewHTTPDataSource should fail when called with an invalid endpoint", func() {
		_, err = NewHTTPDataSource("httpd://!@#$%^&*()dgsdd&3r53/invalid", "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "unable to parse endpoint")).To(BeTrue())
	})

	It("NewHTTPDataSource should fail when called with an invalid certdir", func() {
		image := ts.URL + "/" + cirrosFileName
		_, err = NewHTTPDataSource(image, "", "", "/invaliddir", cdiv1.DataVolumeKubeVirt)
		Expect(err).To(HaveOccurred())
	})

	DescribeTable("calling info should", func(image string, contentType cdiv1.DataVolumeContentType, expectedPhase ProcessingPhase, want []byte, wantErr bool, brokenForQemuImg bool) {
		flushRead = want
		if image != "" {
			image = ts.URL + "/" + image
		}
		dp, err = NewHTTPDataSource(image, "", "", "", contentType)
		dp.brokenForQemuImg = brokenForQemuImg
		Expect(err).NotTo(HaveOccurred())
		newPhase, err := dp.Info()
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedPhase).To(Equal(newPhase))
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		Entry("return ValidatePreScratch phase when image size can be validated", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseValidatePreScratch, cirrosData, false, false),
		Entry("return TransferScratch phase when target server is broken for nbdkit+qemu-img", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseTransferScratch, cirrosData, false, true),
		Entry("return TransferTarget with archive content type but not archive endpoint ", cirrosFileName, cdiv1.DataVolumeArchive, ProcessingPhaseTransferDataDir, cirrosData, false, false),
		Entry("return TransferTarget with archive content type and archive endpoint ", diskimageTarFileName, cdiv1.DataVolumeArchive, ProcessingPhaseTransferDataDir, diskimageArchiveData, false, false),
	)

	DescribeTable("calling transfer should", func(image string, contentType cdiv1.DataVolumeContentType, expectedPhase ProcessingPhase, scratchPath string, want []byte, wantErr bool, validationErr error) {
		flushRead = want
		if scratchPath == "" {
			scratchPath = tmpDir
		}
		if image != "" {
			image = ts.URL + "/" + image
		}
		dp, err = NewHTTPDataSource(image, "", "", "", contentType)
		Expect(err).NotTo(HaveOccurred())
		_, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
		qemuOperations := NewFakeQEMUOperations(nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, validationErr, nil, nil)
		replaceQEMUOperations(qemuOperations, func() {
			newPhase, err := dp.Transfer(scratchPath, false)
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
				Expect(expectedPhase).To(Equal(newPhase))
				if newPhase == ProcessingPhaseConvert {
					file, err := os.Open(filepath.Join(scratchPath, tempFile))
					Expect(err).NotTo(HaveOccurred())
					defer file.Close()
					fileStat, err := file.Stat()
					Expect(err).NotTo(HaveOccurred())
					Expect(int64(len(want))).To(Equal(fileStat.Size()))
					resultBuffer, err := io.ReadAll(file)
					Expect(err).NotTo(HaveOccurred())
					Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
				}
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	},
		Entry("return Error with missing scratch space", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseError, "/imaninvalidpath", cirrosData, true, nil),
		Entry("return Error with invalid content type ", cirrosFileName, cdiv1.DataVolumeContentType("invalid"), ProcessingPhaseError, "", cirrosData, true, nil),
		Entry("return Complete with archive content type and archive endpoint ", diskimageTarFileName, cdiv1.DataVolumeArchive, ProcessingPhaseComplete, "", diskimageArchiveData, false, nil),
		Entry("return Error with invalid target path and archive", diskimageTarFileName, cdiv1.DataVolumeArchive, ProcessingPhaseError, "/imaninvalidpath", cirrosData, true, nil),
		Entry("return Convert with scratch space and valid qcow file", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseConvert, "", cirrosData, false, nil),
		Entry("return Error with insufficient scratch space capacity", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseError, "", cirrosData, true, image.ErrLargerPVCRequired),
	)

	DescribeTable("should succeed when writing to a valid file with phase", func(expectedPhase ProcessingPhase, brokenForQemuImg bool, imageType string) {
		dp, err = NewHTTPDataSource(ts.URL+"/"+imageType, "", "", "", cdiv1.DataVolumeKubeVirt)
		dp.brokenForQemuImg = brokenForQemuImg
		Expect(err).NotTo(HaveOccurred())
		result, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(expectedPhase))
	},
		Entry("ValidatePreScratch when reading raw gz", ProcessingPhaseTransferScratch, true, tinyCoreGz),
		Entry("TransferScratch when reading raw gz and target server is broken for nbdkit+qemu-img", ProcessingPhaseTransferScratch, true, tinyCoreGz),
		Entry("ValidatePreScratch when reading raw xz", ProcessingPhaseTransferScratch, true, tinyCoreXz),
		Entry("TransferScratch when reading raw xz target server is broken for nbdkit+qemu-img", ProcessingPhaseTransferScratch, true, tinyCoreXz),
	)

	It("should get extra headers on creation of new HTTP data source", func() {
		os.Setenv(common.ImporterExtraHeader+"0", "Extra-Header: 321")
		os.Setenv(common.ImporterExtraHeader+"1", "Second-Extra-Header: 321")
		ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			_, firstExists := r.Header["Extra-Header"]
			_, secondExists := r.Header["Second-Extra-Header"]
			if firstExists && secondExists {
				response, err := ts.Client().Get(ts.URL + "/" + r.RequestURI)
				Expect(err).NotTo(HaveOccurred())
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				_, _ = w.Write(body)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		dp, err = NewHTTPDataSource(ts2.URL+"/"+tinyCoreGz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())
		_, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
	})

	It("GetTerminationMessage should return nil when pullMethod is not node", func() {
		Expect(os.Setenv(common.ImporterPullMethod, string(cdiv1.RegistryPullPod))).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(common.ImporterPullMethod)).To(Succeed())
		})

		dp, err = NewHTTPDataSource(ts.URL+"/"+tinyCoreGz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())

		termMsg := dp.GetTerminationMessage()
		Expect(termMsg).To(BeNil())
	})

	DescribeTable("GetTerminationMessage should handle empty Env returned by http server", func(emptyEnv []string) {
		Expect(os.Setenv(common.ImporterPullMethod, string(cdiv1.RegistryPullNode))).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(common.ImporterPullMethod)).To(Succeed())
		})

		ts2 := createTestServer(imageDir, emptyEnv)

		dp, err = NewHTTPDataSource(ts2.URL+"/"+tinyCoreGz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())

		termMsg := dp.GetTerminationMessage()
		Expect(termMsg.Labels).To(BeEmpty())
		Expect(termMsg.String()).To(Equal("{}"))
	},
		Entry("empty slice", []string{}),
		Entry("nil slice", nil),
	)

	It("GetTerminationMessage should contain labels collected from the containerimage-server when pullMethod is node", func() {
		Expect(os.Setenv(common.ImporterPullMethod, string(cdiv1.RegistryPullNode))).To(Succeed())
		DeferCleanup(func() {
			Expect(os.Unsetenv(common.ImporterPullMethod)).To(Succeed())
		})

		ts2 := createTestServer(imageDir, []string{
			"INSTANCETYPE_KUBEVIRT_IO_DEFAULT_INSTANCETYPE=u1.small",
			"INSTANCETYPE_KUBEVIRT_IO_DEFAULT_PREFERENCE=fedora",
		})

		dp, err = NewHTTPDataSource(ts2.URL+"/"+tinyCoreGz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())

		termMsg := dp.GetTerminationMessage()
		Expect(termMsg).ToNot(BeNil())
		Expect(termMsg.Labels).To(HaveLen(2))
		Expect(termMsg.Labels).To(HaveKeyWithValue("instancetype.kubevirt.io/default-instancetype", "u1.small"))
		Expect(termMsg.Labels).To(HaveKeyWithValue("instancetype.kubevirt.io/default-preference", "fedora"))
	})
})

var _ = Describe("Http client", func() {
	var tempDir string

	BeforeEach(func() {
		var err error

		tempDir, err = os.MkdirTemp("/tmp", "cert-test")
		Expect(err).ToNot(HaveOccurred())

		keyPair, err := triple.NewCA("datastream.cdi.kubevirt.io")
		Expect(err).ToNot(HaveOccurred())

		certBytes := cert.EncodeCertPEM(keyPair.Cert)

		err = os.WriteFile(path.Join(tempDir, "tls.crt"), certBytes, 0600)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	It("should load the cert", func() {
		client, err := createHTTPClient(tempDir)
		Expect(err).ToNot(HaveOccurred())

		transport := client.Transport.(*http.Transport)
		Expect(transport).ToNot(BeNil())

		activeCAs := transport.TLSClientConfig.RootCAs
		Expect(transport).ToNot(BeNil())

		systemCAs, err := x509.SystemCertPool()
		Expect(err).ToNot(HaveOccurred())

		Expect(activeCAs.Subjects()).Should(HaveLen(len(systemCAs.Subjects()) + 1)) //nolint:staticcheck // todo: Subjects() is deprecated - check this
	})

})

var _ = Describe("Http reader", func() {
	It("should fail when passed an invalid cert directory", func() {
		_, total, _, err := createHTTPReader(context.Background(), nil, "", "", "/invalid", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(err).To(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
	})

	It("should pass auth info in request if set", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			defer w.WriteHeader(http.StatusOK)
			Expect(ok).To(BeTrue())
			Expect("user").To(Equal(user))
			Expect("password").To(Equal(pass))
			w.Header().Add("Content-Length", "25")
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, _, err := createHTTPReader(context.Background(), ep, "user", "password", "", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(25)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should pass auth info in request if set and redirected", func() {
		redirTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			defer w.WriteHeader(http.StatusOK)
			Expect(ok).To(BeTrue())
			Expect("user").To(Equal(user))
			Expect("password").To(Equal(pass))
			w.Header().Add("Content-Length", "25")
		}))
		defer redirTs.Close()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirTs.URL, http.StatusFound)
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, _, err := createHTTPReader(context.Background(), ep, "user", "password", "", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(25)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should redirect properly without auth if not set", func() {
		redirTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, ok := r.BasicAuth()
			defer w.WriteHeader(http.StatusOK)
			Expect(ok).To(BeFalse())
			w.Header().Add("Content-Length", "25")
			w.Header().Add("Accept-Ranges", "bytes")
		}))
		defer redirTs.Close()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirTs.URL, http.StatusFound)
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, brokenForQemuImg, err := createHTTPReader(context.Background(), ep, "", "", "", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(brokenForQemuImg).To(BeFalse())
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(25)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should continue even if Content-Length is bogus", func() {
		redirTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer w.WriteHeader(http.StatusOK)
			w.Header().Add("Content-Length", "intentional gibberish")
			w.Header().Add("Accept-Ranges", "bytes")
		}))
		defer redirTs.Close()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirTs.URL, http.StatusFound)
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, _, err := createHTTPReader(context.Background(), ep, "", "", "", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should continue even if HEAD is rejected, but mark broken for qemu-img", func() {
		redirTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusForbidden)
			} else {
				defer w.WriteHeader(http.StatusOK)
			}
			w.Header().Add("Content-Length", "25")
			w.Header().Add("Accept-Ranges", "bytes")
		}))
		defer redirTs.Close()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirTs.URL, http.StatusFound)
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, brokenForQemuImg, err := createHTTPReader(context.Background(), ep, "", "", "", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(brokenForQemuImg).To(BeTrue())
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(25)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should continue even if no Accept-Ranges header found, but mark broken for qemu-img", func() {
		redirTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Length", "25")
			w.WriteHeader(http.StatusOK)
		}))
		defer redirTs.Close()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, redirTs.URL, http.StatusFound)
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, brokenForQemuImg, err := createHTTPReader(context.Background(), ep, "", "", "", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(brokenForQemuImg).To(BeTrue())
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(25)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fail if server returns error code", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		_, total, _, err := createHTTPReader(context.Background(), ep, "", "", "", nil, nil, cdiv1.DataVolumeKubeVirt)
		Expect(err).To(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
		Expect("expected status code 200, got 500. Status: 500 Internal Server Error").To(Equal(err.Error()))
	})

	It("should pass through extra headers", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, exists := r.Header["Extra-Header"]; exists {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, _, err := createHTTPReader(context.Background(), ep, "", "", "", []string{"Extra-Header: 123"}, nil, cdiv1.DataVolumeKubeVirt)
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("http pollprogress", func() {
	It("Should properly finish with valid reader", func() {
		By("Creating context for the transfer, we have the ability to cancel it")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		dp := &HTTPDataSource{
			ctx:    ctx,
			cancel: cancel,
		}
		By("Creating string reader we can test just the poll progress part")
		stringReader := io.NopCloser(strings.NewReader("This is a test string"))
		endlessReader := EndlessReader{
			Reader: stringReader,
		}
		countingReader := &util.CountingReader{
			Reader:  &endlessReader,
			Current: 0,
		}
		By("Creating pollProgress as go routine, we can use channels to monitor progress")
		go dp.pollProgress(countingReader, 5*time.Second, time.Second)
		By("Waiting for timeout or success")
		select {
		case <-time.After(10 * time.Second):
			Fail("Transfer not cancelled after 10 seconds")
		case <-dp.ctx.Done():
			By("Having context be done, we confirm finishing of transfer")
		}
	})
})

func createTestServer(imageDir string, env []string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/info", func(w http.ResponseWriter, _ *http.Request) {
		defer GinkgoRecover()
		info := common.ServerInfo{
			Env: env,
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(info)
		Expect(err).NotTo(HaveOccurred())
	})
	mux.Handle("/", http.FileServer(http.Dir(imageDir)))
	return httptest.NewServer(mux)
}

// Read the contents of the file into a byte array, don't use this on really huge files.
func readFile(fileName string) ([]byte, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result, err := io.ReadAll(f)
	return result, err
}

// EndlessReader doesn't return any value read, te r
type EndlessReader struct {
	Reader io.ReadCloser
}

// Read doesn't return any values
func (r *EndlessReader) Read(p []byte) (int, error) {
	_, err := r.Reader.Read(p)
	Expect(err).ToNot(HaveOccurred())
	return 0, nil
}

// Close closes the stream
func (r *EndlessReader) Close() error {
	return r.Reader.Close()
}
