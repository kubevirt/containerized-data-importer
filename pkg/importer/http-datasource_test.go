package importer

import (
	"context"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

var (
	cirrosFileName          = "cirros-qcow2.img"
	diskimageTarFileName    = "cirros.tar"
	cirrosQCow2TarFileName  = "cirros.qcow2.tar"
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
		By("[BeforeEach] Creating test server")
		ts = createTestServer(imageDir)
		dp = nil
		tmpDir, err = ioutil.TempDir("", "scratch")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
	})

	AfterEach(func() {
		if dp != nil {
			resultBuffer := make([]byte, len(flushRead))
			if dp.readers != nil {
				By("flushing data from readers we can finish test")
				dp.readers.read(resultBuffer)
			}
			By("[AfterEach] closing data provider")
			err = dp.Close()
			Expect(err).NotTo(HaveOccurred())
		}
		os.RemoveAll(tmpDir)
		By("[AfterEach] closing test server")
		ts.Close()
	})

	It("NewHTTPDataSource should fail when called with an invalid endpoint", func() {
		_, err = NewHTTPDataSource("httpd://!@#$%^&*()dgsdd&3r53/invalid", "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "unable to parse endpoint")).To(BeTrue())
	})

	It("endpoint User object should be set when accessKey and secKey are not blank", func() {
		image := ts.URL + "/" + cirrosFileName
		dp, err = NewHTTPDataSource(image, "user", "password", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())
		user := dp.endpoint.User
		Expect("user").To(Equal(user.Username()))
		pw, set := user.Password()
		Expect("password").To(Equal(pw))
		Expect(set).To(BeTrue())
	})

	It("NewHTTPDataSource should fail when called with an invalid certdir", func() {
		image := ts.URL + "/" + cirrosFileName
		_, err = NewHTTPDataSource(image, "", "", "/invaliddir", cdiv1.DataVolumeKubeVirt)
		Expect(err).To(HaveOccurred())
	})

	table.DescribeTable("calling info should", func(image string, contentType cdiv1.DataVolumeContentType, expectedPhase ProcessingPhase, want []byte, wantErr bool) {
		flushRead = want
		if image != "" {
			image = ts.URL + "/" + image
		}
		dp, err = NewHTTPDataSource(image, "", "", "", contentType)
		Expect(err).NotTo(HaveOccurred())
		newPhase, err := dp.Info()
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedPhase).To(Equal(newPhase))
			if newPhase == ProcessingPhaseConvert {
				expectURL, err := url.Parse(image)
				Expect(err).NotTo(HaveOccurred())
				Expect(expectURL).To(Equal(dp.GetURL()))
			}
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("return Convert phase ", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseConvert, cirrosData, false),
		table.Entry("return TransferTarget with archive content type but not archive endpoint ", cirrosFileName, cdiv1.DataVolumeArchive, ProcessingPhaseTransferDataDir, cirrosData, false),
		table.Entry("return TransferTarget with archive content type and archive endpoint ", diskimageTarFileName, cdiv1.DataVolumeArchive, ProcessingPhaseTransferDataDir, diskimageArchiveData, false),
	)

	It("calling info with raw image should return TransferDataFile", func() {
		dp, err = NewHTTPDataSource(ts.URL+"/"+tinyCoreGz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())
		newPhase, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(newPhase))
	})

	table.DescribeTable("calling transfer should", func(image string, contentType cdiv1.DataVolumeContentType, expectedPhase ProcessingPhase, scratchPath string, want []byte, wantErr bool) {
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
		newPhase, err := dp.Transfer(scratchPath)
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			Expect(expectedPhase).To(Equal(newPhase))
			if newPhase == ProcessingPhaseProcess {
				file, err := os.Open(filepath.Join(scratchPath, tempFile))
				Expect(err).NotTo(HaveOccurred())
				defer file.Close()
				fileStat, err := file.Stat()
				Expect(err).NotTo(HaveOccurred())
				Expect(int64(len(want))).To(Equal(fileStat.Size()))
				resultBuffer, err := ioutil.ReadAll(file)
				Expect(err).NotTo(HaveOccurred())
				Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
			}
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("return Error with missing scratch space", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseError, "/imaninvalidpath", cirrosData, true),
		table.Entry("return Error with invalid content type ", cirrosFileName, cdiv1.DataVolumeContentType("invalid"), ProcessingPhaseError, "", cirrosData, true),
		table.Entry("return Complete with archive content type and archive endpoint ", diskimageTarFileName, cdiv1.DataVolumeArchive, ProcessingPhaseComplete, "", diskimageArchiveData, false),
		table.Entry("return Error with invalid target path and archive", diskimageTarFileName, cdiv1.DataVolumeArchive, ProcessingPhaseError, "/imaninvalidpath", cirrosData, true),
		table.Entry("return Process with scratch space and valid qcow file", cirrosFileName, cdiv1.DataVolumeKubeVirt, ProcessingPhaseProcess, "", cirrosData, false),
	)

	It("TransferFile should succeed when writing to valid file, and reading raw gz", func() {
		dp, err = NewHTTPDataSource(ts.URL+"/"+tinyCoreGz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())
		result, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = dp.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile should succeed when writing to valid file and reading raw xz", func() {
		dp, err = NewHTTPDataSource(ts.URL+"/"+tinyCoreXz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())
		result, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = dp.TransferFile(filepath.Join(tmpDir, "file"))
		Expect(err).ToNot(HaveOccurred())
		Expect(ProcessingPhaseResize).To(Equal(result))
	})

	It("TransferFile should fail on streaming error", func() {
		dp, err = NewHTTPDataSource(ts.URL+"/"+tinyCoreGz, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())
		result, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseTransferDataFile).To(Equal(result))
		result, err = dp.TransferFile("/invalidpath/invalidfile")
		Expect(err).To(HaveOccurred())
		Expect(ProcessingPhaseError).To(Equal(result))
	})

	It("calling Process should return Convert", func() {
		flushRead = cirrosData
		dp, err = NewHTTPDataSource(ts.URL+"/"+cirrosFileName, "", "", "", cdiv1.DataVolumeKubeVirt)
		Expect(err).NotTo(HaveOccurred())
		_, err := dp.Info()
		Expect(err).NotTo(HaveOccurred())
		newPhase, err := dp.Process()
		Expect(err).NotTo(HaveOccurred())
		Expect(ProcessingPhaseConvert).To(Equal(newPhase))
	})
})

var _ = Describe("Http client", func() {
	var tempDir string

	BeforeEach(func() {
		var err error

		tempDir, err = ioutil.TempDir("/tmp", "cert-test")
		Expect(err).ToNot(HaveOccurred())

		keyPair, err := triple.NewCA("datastream.cdi.kubevirt.io")
		Expect(err).ToNot(HaveOccurred())

		certBytes := cert.EncodeCertPEM(keyPair.Cert)

		err = ioutil.WriteFile(path.Join(tempDir, "tls.crt"), certBytes, 0644)
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

		Expect(len(activeCAs.Subjects())).Should(Equal(len(systemCAs.Subjects()) + 1))
	})

})

var _ = Describe("Http reader", func() {
	It("should fail when passed an invalid cert directory", func() {
		_, total, _, err := createHTTPReader(context.Background(), nil, "", "", "/invalid")
		Expect(err).To(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
	})

	It("should pass auth info in request if set", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			defer w.WriteHeader(200)
			Expect(ok).To(BeTrue())
			Expect("user").To(Equal(user))
			Expect("password").To(Equal(pass))
			w.Header().Add("Content-Length", "25")
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		r, total, _, err := createHTTPReader(context.Background(), ep, "user", "password", "")
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
		r, total, _, err := createHTTPReader(context.Background(), ep, "user", "password", "")
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
		r, total, brokenForQemuImg, err := createHTTPReader(context.Background(), ep, "", "", "")
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
		r, total, _, err := createHTTPReader(context.Background(), ep, "", "", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should continue even if HEAD is rejected, but mark broken for qemu-img", func() {
		redirTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "HEAD" {
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
		r, total, brokenForQemuImg, err := createHTTPReader(context.Background(), ep, "", "", "")
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
		r, total, brokenForQemuImg, err := createHTTPReader(context.Background(), ep, "", "", "")
		Expect(brokenForQemuImg).To(BeTrue())
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(25)).To(Equal(total))
		err = r.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should fail if server returns error code", func() {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer ts.Close()
		ep, err := url.Parse(ts.URL)
		Expect(err).ToNot(HaveOccurred())
		_, total, _, err := createHTTPReader(context.Background(), ep, "", "", "")
		Expect(err).To(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
		Expect("expected status code 200, got 500. Status: 500 Internal Server Error").To(Equal(err.Error()))
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
		stringReader := ioutil.NopCloser(strings.NewReader("This is a test string"))
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

func createTestServer(imageDir string) *httptest.Server {
	return httptest.NewServer(http.FileServer(http.Dir(imageDir)))
}

// Read the contents of the file into a byte array, don't use this on really huge files.
func readFile(fileName string) ([]byte, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result, err := ioutil.ReadAll(f)
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
