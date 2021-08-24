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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	ovirtsdk4 "github.com/ovirt/go-ovirt"
	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

var it = &ovirtsdk4.ImageTransfer{}
var disk = &ovirtsdk4.Disk{}
var diskID = "disk-123"
var diskAvailable = true
var diskCreateError error
var diskSnapshots = &ovirtsdk4.DiskSnapshotSlice{}
var storageDomain = &ovirtsdk4.StorageDomain{}
var storageDomains = &ovirtsdk4.StorageDomainSlice{}

var _ = Describe("Imageio reader", func() {
	var (
		ts      *httptest.Server
		tempDir string
	)

	BeforeEach(func() {
		newOvirtClientFunc = createMockOvirtClient
		newTerminationChannel = createMockTerminationChannel
		tempDir = createCert()
		ts = createTestServer(imageDir)
		disk.SetTotalSize(1024)
		disk.SetId(diskID)
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING)
		it.SetTransferUrl(ts.URL + "/" + cirrosFileName)
		it.SetId(diskID)
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
		_, total, _, _, err := createImageioReader(context.Background(), "invalid/", "", "", "", diskID, "", "")
		Expect(err).To(HaveOccurred())
		Expect(uint64(0)).To(Equal(total))
	})

	It("should create reader", func() {
		reader, total, _, _, err := createImageioReader(context.Background(), "", "", "", tempDir, diskID, "", "")
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
		newTerminationChannel = createMockTerminationChannel
		tempDir = createCert()
		ts = createTestServer(imageDir)
		disk.SetTotalSize(1024)
		disk.SetId(diskID)
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING)
		it.SetTransferUrl(ts.URL)
		it.SetId(diskID)
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
		_, err = NewImageioDataSource("httpd://!@#$%^&*()dgsdd&3r53/invalid", "", "", "", diskID, "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource info should not fail when called with valid endpoint", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Info()
		Expect(err).ToNot(HaveOccurred())
	})

	It("NewImageioDataSource tranfer should fail if invalid path", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Transfer("")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource tranferfile should fail when invalid path", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).ToNot(HaveOccurred())
		_, err = dp.Info()
		Expect(err).NotTo(HaveOccurred())
		phase, err := dp.TransferFile("")
		Expect(err).To(HaveOccurred())
		Expect(phase).To(Equal(ProcessingPhaseError))
	})

	It("NewImageioDataSource url should be nil if not set", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).ToNot(HaveOccurred())
		url := dp.GetURL()
		Expect(url).To(BeNil())
	})

	It("NewImageioDataSource close should succeed if valid url", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).ToNot(HaveOccurred())
		err = dp.Close()
		Expect(err).ToNot(HaveOccurred())
	})

	It("NewImageioDataSource should fail if transfer in unknown state", func() {
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_UNKNOWN)
		_, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource should fail if disk creation fails", func() {
		diskCreateError = errors.New("this is error message")
		_, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).To(HaveOccurred())
	})

	It("NewImageioDataSource should fail if disk does not exists", func() {
		diskAvailable = false
		_, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
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

var _ = Describe("Imageio cancel", func() {
	var (
		ts      *httptest.Server
		tempDir string
	)

	BeforeEach(func() {
		newOvirtClientFunc = createMockOvirtClient
		newTerminationChannel = createMockTerminationChannel
		tempDir = createCert()
		ts = createTestServer(imageDir)
		disk.SetTotalSize(1024)
		disk.SetId(diskID)
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING)
		it.SetTransferUrl(ts.URL)
		it.SetId(diskID)
		diskAvailable = true
		diskCreateError = nil
	})

	AfterEach(func() {
		mockCancelHook = nil
		mockFinalizeHook = nil
		newOvirtClientFunc = getOvirtClient
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
		ts.Close()
	})

	It("should clean up transfer on SIGTERM", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		Expect(err).ToNot(HaveOccurred())
		timesFinalized := 0
		resultChannel := make(chan struct {
			*ImageioDataSource
			int
		}, 1)
		mockFinalizeHook = func() error {
			dp.imageTransfer.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_FINALIZING_SUCCESS)
			timesFinalized++
			resultChannel <- struct {
				*ImageioDataSource
				int
			}{dp, timesFinalized}
			return nil
		}
		mockTerminationChannel <- os.Interrupt
		timeout := time.After(10 * time.Second)
		select {
		case <-timeout:
			Fail("Timed out waiting for cancel result")
		case result := <-resultChannel:
			timesFinalized = result.int
			dp = result.ImageioDataSource
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(timesFinalized).To(Equal(1))
		Expect(dp.imageTransfer.MustPhase()).To(Equal(ovirtsdk4.IMAGETRANSFERPHASE_FINALIZING_SUCCESS))
	})

	DescribeTable("should finalize successful transfer on close", func(initialPhase, expectedPhase ovirtsdk4.ImageTransferPhase) {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		dp.imageTransfer.SetPhase(initialPhase)
		Expect(err).ToNot(HaveOccurred())
		timesFinalized := 0
		mockFinalizeHook = func() error {
			dp.imageTransfer.SetPhase(expectedPhase)
			timesFinalized++
			return nil
		}
		err = dp.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.imageTransfer.MustPhase()).To(Equal(expectedPhase))
		Expect(timesFinalized).To(Equal(1))
	},
		Entry("from transferring", ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING, ovirtsdk4.IMAGETRANSFERPHASE_FINALIZING_SUCCESS),
	)

	DescribeTable("should cancel failed transfer on close", func(initialPhase, expectedPhase ovirtsdk4.ImageTransferPhase) {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		dp.imageTransfer.SetPhase(initialPhase)
		Expect(err).ToNot(HaveOccurred())
		timesCancelled := 0
		mockCancelHook = func() error {
			dp.imageTransfer.SetPhase(expectedPhase)
			timesCancelled++
			return nil
		}
		err = dp.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.imageTransfer.MustPhase()).To(Equal(expectedPhase))
		Expect(timesCancelled).To(Equal(1))
	},
		Entry("from initializing", ovirtsdk4.IMAGETRANSFERPHASE_INITIALIZING, ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED),
		Entry("from paused_system", ovirtsdk4.IMAGETRANSFERPHASE_PAUSED_SYSTEM, ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED),
		Entry("from paused_user", ovirtsdk4.IMAGETRANSFERPHASE_PAUSED_USER, ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED),
		Entry("from resuming", ovirtsdk4.IMAGETRANSFERPHASE_RESUMING, ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED),
		Entry("from unknown", ovirtsdk4.IMAGETRANSFERPHASE_UNKNOWN, ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED),
	)

	DescribeTable("should take no action on final transfer states", func(initialPhase ovirtsdk4.ImageTransferPhase) {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, "", "")
		dp.imageTransfer.SetPhase(initialPhase)
		Expect(err).ToNot(HaveOccurred())
		timesFinalized := 0
		mockFinalizeHook = func() error {
			dp.imageTransfer.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_UNKNOWN)
			timesFinalized++
			return nil
		}
		timesCancelled := 0
		mockCancelHook = func() error {
			dp.imageTransfer.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_UNKNOWN)
			timesCancelled++
			return nil
		}
		err = dp.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.imageTransfer.MustPhase()).To(Equal(initialPhase))
		Expect(timesCancelled).To(Equal(0))
		Expect(timesFinalized).To(Equal(0))
	},
		Entry("from cancelled", ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED),
		Entry("from finalizing_failure", ovirtsdk4.IMAGETRANSFERPHASE_FINALIZING_FAILURE),
		Entry("from finalizing_success", ovirtsdk4.IMAGETRANSFERPHASE_FINALIZING_SUCCESS),
		Entry("from finished_failure", ovirtsdk4.IMAGETRANSFERPHASE_FINISHED_FAILURE),
		Entry("from finished_success", ovirtsdk4.IMAGETRANSFERPHASE_FINISHED_SUCCESS),
	)
})

var _ = Describe("imageio snapshots", func() {
	var (
		ts               *httptest.Server
		tempDir          string
		snapshotID       string
		parentSnapshotID string
		snapshotSize     int64
		diskSize         int64
	)

	BeforeEach(func() {
		snapshotID = "snapshot-12345"
		parentSnapshotID = "snapshot-12344"
		snapshotSize = 256
		diskSize = 1024

		newOvirtClientFunc = createMockOvirtClient
		newTerminationChannel = createMockTerminationChannel
		tempDir = createCert()
		ts = createTestServer(imageDir)
		disk.SetTotalSize(diskSize)
		disk.SetId(diskID)
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING)
		it.SetTransferUrl(ts.URL)
		it.SetId(snapshotID)
		diskAvailable = true
		diskCreateError = nil

		disks := &ovirtsdk4.DiskSlice{}
		disks.SetSlice([]*ovirtsdk4.Disk{disk})

		snapshot := ovirtsdk4.NewSnapshotBuilder().Id(snapshotID).MustBuild()
		snapshot.SetDisks(disks)

		parentSnapshot := ovirtsdk4.NewSnapshotBuilder().Id(parentSnapshotID).MustBuild()
		parentSnapshot.SetDisks(disks)

		snapshots := new(ovirtsdk4.SnapshotSlice)
		snapshots.SetSlice([]*ovirtsdk4.Snapshot{snapshot, parentSnapshot})

		diskSnapshot := ovirtsdk4.NewDiskSnapshotBuilder().Id(snapshotID).MustBuild()
		diskSnapshot.SetSnapshot(snapshot)
		diskSnapshot.SetActualSize(snapshotSize)

		parentDiskSnapshot := ovirtsdk4.NewDiskSnapshotBuilder().Id(parentSnapshotID).MustBuild()
		parentDiskSnapshot.SetSnapshot(parentSnapshot)
		parentDiskSnapshot.SetActualSize(snapshotSize)

		diskSnapshots.SetSlice([]*ovirtsdk4.DiskSnapshot{diskSnapshot, parentDiskSnapshot})

		storageDomain = ovirtsdk4.NewStorageDomainBuilder().Name("The Storage Domain").Id("sd-12345").MustBuild()
		storageDomain.SetDiskSnapshots(diskSnapshots)

		storageDomains.SetSlice([]*ovirtsdk4.StorageDomain{storageDomain})

		disk.SetStorageDomains(storageDomains)
		disk.SetStorageDomain(storageDomain)
	})

	AfterEach(func() {
		newOvirtClientFunc = getOvirtClient
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
		ts.Close()
	})

	It("should correctly get initial snapshot transfer", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, snapshotID, "")
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.currentSnapshot).To(Equal(snapshotID))
		Expect(dp.previousSnapshot).To(Equal(""))
		Expect(dp.contentLength).To(Equal(uint64(diskSize)))
		Expect(dp.IsDeltaCopy()).To(Equal(false))
	})

	It("should correctly get child snapshot transfer", func() {
		dp, err := NewImageioDataSource(ts.URL, "", "", tempDir, diskID, snapshotID, parentSnapshotID)
		Expect(err).ToNot(HaveOccurred())
		Expect(dp.currentSnapshot).To(Equal(snapshotID))
		Expect(dp.previousSnapshot).To(Equal(parentSnapshotID))
		Expect(dp.contentLength).To(Equal(uint64(snapshotSize)))
		Expect(dp.IsDeltaCopy()).To(Equal(true))
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

type MockCancelService struct {
	client *MockOvirtClient
}

type MockFinalizeService struct {
	client *MockOvirtClient
}

type MockGetService struct {
	client *MockOvirtClient
}

type MockStorageDomainsService struct {
	client *MockOvirtClient
}

type MockImageTransfersService struct {
	client *MockOvirtClient
}

type MockImageTransferService struct{}

type MockStorageDomainService struct{}

type MockDiskSnapshotsService struct{}

type MockDiskSnapshotsServiceListRequest struct{}

type MockDiskSnapshotsServiceListResponse struct{}

type MockDiskSnapshotSlice struct{}

type MockImageTransfersServiceAddResponse struct {
	srv *ovirtsdk4.ImageTransfersServiceAddResponse
}

type MockImageTransfersServiceAddRequest struct{}

type MockImageTransferServiceCancelResponse struct {
	srv *ovirtsdk4.ImageTransferServiceCancelResponse
}

type MockImageTransferServiceFinalizeResponse struct {
	srv *ovirtsdk4.ImageTransferServiceFinalizeResponse
}

type MockImageTransferServiceGetResponse struct {
	srv *ovirtsdk4.ImageTransferServiceGetResponse
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
	return &MockImageTransferService{}
}

func (service *MockImageTransferService) Cancel() ImageTransferServiceCancelRequestInterface {
	return &MockCancelService{}
}

func (service *MockImageTransferService) Finalize() ImageTransferServiceFinalizeRequestInterface {
	return &MockFinalizeService{}
}

func (service *MockImageTransferService) Get() ImageTransferServiceGetRequestInterface {
	return &MockGetService{}
}

func (conn *MockOvirtClient) StorageDomainsService() StorageDomainsServiceInterface {
	return &MockStorageDomainsService{
		client: conn,
	}
}

func (service *MockStorageDomainsService) StorageDomainService(id string) StorageDomainServiceInterface {
	return &MockStorageDomainService{}
}

func (service *MockStorageDomainService) DiskSnapshotsService() DiskSnapshotsServiceInterface {
	return &MockDiskSnapshotsService{}
}

func (service *MockDiskSnapshotsService) List() DiskSnapshotsServiceListRequestInterface {
	return &MockDiskSnapshotsServiceListRequest{}
}

func (service *MockDiskSnapshotsServiceListRequest) Send() (DiskSnapshotsServiceListResponseInterface, error) {
	return &MockDiskSnapshotsServiceListResponse{}, nil
}

func (service *MockDiskSnapshotsServiceListResponse) Snapshots() (DiskSnapshotSliceInterface, bool) {
	return diskSnapshots, true
}

func (service *MockDiskSnapshotSlice) Slice() []*ovirtsdk4.DiskSnapshot {
	return diskSnapshots.Slice()
}

func (conn *MockOvirtClient) Cancel() ImageTransferServiceCancelRequestInterface {
	if mockCancelHook != nil {
		mockCancelHook()
	} else {
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED)
	}
	return &MockCancelService{
		client: conn,
	}
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

func (conn *MockCancelService) Send() (ImageTransferServiceCancelResponseInterface, error) {
	var err error
	if mockCancelHook != nil {
		err = mockCancelHook()
	} else {
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED) // default to cancelled
	}
	return &MockImageTransferServiceCancelResponse{srv: nil}, err
}

func (conn *MockFinalizeService) Send() (ImageTransferServiceFinalizeResponseInterface, error) {
	var err error
	if mockFinalizeHook != nil {
		err = mockFinalizeHook()
	} else {
		it.SetPhase(ovirtsdk4.IMAGETRANSFERPHASE_FINISHED_SUCCESS) // default to success
	}
	return &MockImageTransferServiceFinalizeResponse{srv: nil}, err
}

func (conn *MockGetService) Send() (ImageTransferServiceGetResponseInterface, error) {
	return &MockImageTransferServiceGetResponse{srv: nil}, nil
}

func (conn *MockImageTransferServiceGetResponse) ImageTransfer() (*ovirtsdk4.ImageTransfer, bool) {
	return it, true
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

var mockCancelHook func() error
var mockFinalizeHook func() error

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
