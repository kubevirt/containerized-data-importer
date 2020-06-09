package importer

import (
	"bytes"
	"context"
	"encoding/pem"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ovirtsdk4 "github.com/ovirt/go-ovirt"
	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

var it = &ovirtsdk4.ImageTransfer{}
var disk = &ovirtsdk4.Disk{}
var diskAvailable = true
var diskCreateError error

var _ = Describe("Imageio reader", func() {
	var (
		ts      *httptest.Server
		tempDir string
	)

	BeforeEach(func() {
		newOvirtClientFunc = createMockOvirtClient
		tempDir = createCert()
		ts = createTestServer(imageDir)
		disk.SetTotalSize(1024)
		disk.SetId("123")
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING)
		it.SetTransferUrl(ts.URL + "/" + cirrosFileName)
		diskCreateError = nil
		diskAvailable = true
	})

	AfterEach(func() {
		newOvirtClientFunc = getOvirtClient
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
		ts.Close()
	})

	It("should fail creating client", func() {
		newOvirtClientFunc = failMockOvirtClient
		_, total, _, _, err := createImageioReader(context.Background(), "invalid/", "", "", "", "")
		Expect(err).To(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
	})

	It("should create reader", func() {
		reader, total, _, _, err := createImageioReader(context.Background(), "", "", "", tempDir, "")
		Expect(err).ToNot(HaveOccurred())
		Expect(uint64(1024)).To(Equal(total))
		err = reader.Close()
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("Imageio data source", func() {
	var (
		ts      *httptest.Server
		tempDir string
		err     error
	)

	BeforeEach(func() {
		newOvirtClientFunc = createMockOvirtClient
		tempDir = createCert()
		ts = createTestServer(imageDir)
		disk.SetTotalSize(1024)
		disk.SetId("123")
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING)
		it.SetTransferUrl(ts.URL)
		diskAvailable = true
		diskCreateError = nil
	})

	AfterEach(func() {
		newOvirtClientFunc = getOvirtClient
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
		ts.Close()
	})

	It("NewImageioDataSource should fail when called with an invalid endpoint", func() {
		newOvirtClientFunc = getOvirtClient
		_, err = NewImageioDataSource("httpd://!@#$%^&*()dgsdd&3r53/invalid", "", "", "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource info should not fail when called with valid endpoint", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Info()
		Expect(err).ToNot(HaveOccurred())
	})

	It("NewImageioDataSource proccess should not fail with valid endpoint ", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Process()
		Expect(err).ToNot(HaveOccurred())
	})

	It("NewImageioDataSource tranfer should fail if invalid path", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Transfer("")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource tranferfile should fail when invalid path", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Info()
		Expect(err).NotTo(HaveOccurred())
		phase, err := dp.TransferFile("")
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
	})

	It("NewImageioDataSource url should be nil if not set", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).ToNot(HaveOccurred())
		url := dp.GetURL()
		Expect(url).To(BeNil())
	})

	It("NewImageioDataSource close should succeed if valid url", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).ToNot(HaveOccurred())
		err = dp.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("NewImageioDataSource should fail if transfer in unknown state", func() {
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_UNKNOWN)
		_, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource should fail if disk creation fails", func() {
		diskCreateError = errors.New("this is error message")
		_, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource should fail if disk does not exists", func() {
		diskAvailable = false
		_, err := NewImageioDataSource(ts.URL, "", "", tempDir, "")
		Expect(err).To(HaveOccurred())
	})

})

var _ = Describe("Imageio client preparation", func() {
	var tempDir string

	BeforeEach(func() {
		tempDir = createCert()
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	It("should load the cert", func() {
		activeCAs, err := loadCA(tempDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(activeCAs.Subjects())).Should(Equal(1))
	})

	It("should return error if dir is empty", func() {
		_, err := loadCA("")
		Expect(err).To(HaveOccurred())
	})

	It("should return error if dir does not exists", func() {
		_, err := loadCA("/invalid-non-existent")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Imageio pollprogress", func() {
	It("Should properly finish with valid reader", func() {
		By("Creating context for the transfer, we have the ability to cancel it")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		dp := &ImageioDataSource{
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

// MockOvirtClient is a mock minio client
type MockOvirtClient struct {
	ep     string
	accKey string
	secKey string
	doErr  bool
}

type MockAddService struct {
	client *MockOvirtClient
}

type MockFinalizeService struct {
	client *MockOvirtClient
}

type MockImageTransfersServiceAddResponse struct {
	srv *ovirtsdk4.ImageTransfersServiceAddResponse
}

type MockImageTransferServiceFinalizeResponse struct {
	srv *ovirtsdk4.ImageTransferServiceFinalizeResponse
}

func (conn *MockOvirtClient) Disk() (*ovirtsdk4.Disk, bool) {
	return disk, diskAvailable
}

func (conn *MockOvirtClient) Send() (DiskServiceResponseInterface, error) {
	return conn, diskCreateError
}

func (conn *MockOvirtClient) Get() DiskServiceGetInterface {
	return conn
}

func (conn *MockOvirtClient) DiskService(string) DiskServiceInterface {
	return conn
}

func (conn *MockOvirtClient) DisksService() DisksServiceInterface {
	return conn
}

func (conn *MockOvirtClient) ImageTransfersService() ImageTransfersServiceInterface {
	return conn
}

func (conn *MockOvirtClient) ImageTransferService(string) ImageTransferServiceInterface {
	return conn
}

func (conn *MockOvirtClient) Finalize() ImageTransferServiceFinalizeRequestInterface {
	return &MockFinalizeService{
		client: conn,
	}
}

func (conn *MockOvirtClient) Add() ImageTransferServiceAddInterface {
	return &MockAddService{
		client: conn,
	}
}
func (conn *MockAddService) ImageTransfer(imageTransfer *ovirtsdk4.ImageTransfer) *ovirtsdk4.ImageTransfersServiceAddRequest {
	return &ovirtsdk4.ImageTransfersServiceAddRequest{}
}

func (conn *MockAddService) Send() (ImageTransfersServiceAddResponseInterface, error) {
	return &MockImageTransfersServiceAddResponse{srv: nil}, nil
}

func (conn *MockFinalizeService) Send() (ImageTransferServiceFinalizeResponseInterface, error) {
	return &MockImageTransferServiceFinalizeResponse{srv: nil}, nil
}

func (conn *MockImageTransfersServiceAddResponse) ImageTransfer() (*ovirtsdk4.ImageTransfer, bool) {
	return it, true
}

func (conn *MockOvirtClient) SystemService() SystemServiceInteface {
	return conn
}

func (conn *MockOvirtClient) Close() error {
	return nil
}

func failMockOvirtClient(ep string, accessKey string, secKey string) (ConnectionInterface, error) {
	return nil, errors.New("Failed to create client")
}

func createErrMockOvirtClient(ep string, accessKey string, secKey string) (ConnectionInterface, error) {
	return &MockOvirtClient{
		ep:     ep,
		accKey: accessKey,
		secKey: secKey,
		doErr:  true,
	}, nil
}

func createCert() string {
	var err error

	tempDir, err := ioutil.TempDir("/tmp", "cert-test")
	Expect(err).ToNot(HaveOccurred())

	keyPair, err := triple.NewCA("datastream.cdi.kubevirt.io")
	Expect(err).ToNot(HaveOccurred())

	certBytes := bytes.Buffer{}
	pem.Encode(&certBytes, &pem.Block{Type: cert.CertificateBlockType, Bytes: keyPair.Cert.Raw})

	err = ioutil.WriteFile(path.Join(tempDir, "tls.crt"), certBytes.Bytes(), 0644)
	Expect(err).ToNot(HaveOccurred())

	return tempDir
}

func createMockOvirtClient(ep string, accessKey string, secKey string) (ConnectionInterface, error) {
	return &MockOvirtClient{
		ep:     ep,
		accKey: accessKey,
		secKey: secKey,
		doErr:  false,
	}, nil
}
