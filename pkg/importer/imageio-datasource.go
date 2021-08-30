/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package importer

import (
	"context"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ovirtsdk4 "github.com/ovirt/go-ovirt"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

// ImageioDataSource is the data provider for ovirt-imageio.
type ImageioDataSource struct {
	imageioReader io.ReadCloser
	ctx           context.Context
	cancel        context.CancelFunc
	cancelLock    sync.Mutex
	cleanupLock   sync.Mutex
	cleanupDone   bool
	// stack of readers
	readers *FormatReaders
	// url the url to report to the caller of getURL, could be the endpoint, or a file in scratch space.
	url *url.URL
	// the content length reported by ovirt-imageio.
	contentLength uint64
	// imageTransfer is the tranfer object handling the tranfer of oVirt disk
	imageTransfer *ovirtsdk4.ImageTransfer
	// connection is connection to the oVirt system
	connection ConnectionInterface
	// currentSnapshot is the UUID of the snapshot to copy, if requested
	currentSnapshot string
	// previousSnapshot is the UUID of the parent snapshot, if requested
	previousSnapshot string
}

// NewImageioDataSource creates a new instance of the ovirt-imageio data provider.
func NewImageioDataSource(endpoint string, accessKey string, secKey string, certDir string, diskID string, currentCheckpoint string, previousCheckpoint string) (*ImageioDataSource, error) {
	ctx, cancel := context.WithCancel(context.Background())
	imageioReader, contentLength, it, conn, err := createImageioReader(ctx, endpoint, accessKey, secKey, certDir, diskID, currentCheckpoint, previousCheckpoint)
	if err != nil {
		cleanupError := cleanupTransfer(conn, it)
		if cleanupError != nil {
			klog.Errorf("Failed to close image transfer after failure creating data source: %v", cleanupError)
		}
		cancel()
		return nil, err
	}
	imageioSource := &ImageioDataSource{
		ctx:              ctx,
		cancel:           cancel,
		cleanupLock:      sync.Mutex{},
		imageioReader:    imageioReader,
		contentLength:    contentLength,
		imageTransfer:    it,
		connection:       conn,
		currentSnapshot:  currentCheckpoint,
		previousSnapshot: previousCheckpoint,
	}
	// We know this is a counting reader, so no need to check.
	countingReader := imageioReader.(*util.CountingReader)
	go imageioSource.pollProgress(countingReader, 10*time.Minute, time.Second)

	terminationChannel := newTerminationChannel()
	go func() {
		<-terminationChannel
		klog.Infof("Caught termination signal, closing ImageIO.")
		err = imageioSource.Close()
		if err != nil {
			klog.Errorf("Error closing source: %v", err)
		}
	}()

	return imageioSource, nil
}

// Info is called to get initial information about the data.
func (is *ImageioDataSource) Info() (ProcessingPhase, error) {
	var err error
	is.readers, err = NewFormatReaders(is.imageioReader, is.contentLength)
	if err != nil {
		klog.Errorf("Error creating readers: %v", err)
		return ProcessingPhaseError, err
	}

	if !is.readers.Convert {
		return ProcessingPhaseTransferDataFile, nil
	}
	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source to a scratch location.
func (is *ImageioDataSource) Transfer(path string) (ProcessingPhase, error) {
	defer is.cleanupTransfer()
	// we know that there won't be archives
	size, _ := util.GetAvailableSpace(path)
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}
	is.readers.StartProgressUpdate()
	file := filepath.Join(path, tempFile)
	err := util.StreamDataToFile(is.readers.TopReader(), file)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If we successfully wrote to the file, then the parse will succeed.
	is.url, _ = url.Parse(file)

	// Make sure the snapshot's backing file actually matches the expected parent snapshot.
	// Otherwise, it is not safe to rebase the snapshot onto the previously-downloaded image.
	// Need to check this after the transfer because it is not provided by the oVirt API.
	if is.IsDeltaCopy() {
		imageInfo, err := qemuOperations.Info(is.url)
		if err != nil {
			return ProcessingPhaseError, err
		}

		backingFile := filepath.Base(imageInfo.BackingFile)
		if backingFile != is.previousSnapshot {
			return ProcessingPhaseError, errors.Errorf("snapshot backing file '%s' does not match expected checkpoint '%s', unable to safely rebase snapshot", backingFile, is.previousSnapshot)
		}

		klog.Info("Successfully copied snapshot data, moving to merge phase.")
		return ProcessingPhaseMergeDelta, nil
	}

	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (is *ImageioDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	defer is.cleanupTransfer()
	is.readers.StartProgressUpdate()
	err := util.StreamDataToFile(is.readers.TopReader(), fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseResize, nil
}

// GetURL returns the URI that the data processor can use when converting the data.
func (is *ImageioDataSource) GetURL() *url.URL {
	return is.url
}

// Close all readers.
func (is *ImageioDataSource) Close() error {
	var err error
	if is.readers != nil {
		err = is.readers.Close()
	}
	is.cleanupTransfer()
	if is.connection != nil {
		err = is.connection.Close()
	}
	is.cancelLock.Lock()
	if is.cancel != nil {
		is.cancel()
		is.cancel = nil
	}
	is.cancelLock.Unlock()
	return err
}

func (is *ImageioDataSource) pollProgress(reader *util.CountingReader, idleTime, pollInterval time.Duration) {
	count := reader.Current
	lastUpdate := time.Now()
	for {
		if count < reader.Current {
			// Some progress was made, reset now.
			lastUpdate = time.Now()
			count = reader.Current
		}

		if time.Until(lastUpdate.Add(idleTime)).Nanoseconds() < 0 {
			is.cancelLock.Lock()
			if is.cancel != nil {
				// No progress for the idle time, cancel http client.
				is.cancel() // This will trigger dp.ctx.Done()
			}
			is.cancelLock.Unlock()
		}
		select {
		case <-time.After(pollInterval):
			continue
		case <-is.ctx.Done():
			return // Don't leak, once the transfer is cancelled or completed this is called.
		}
	}
}

// IsDeltaCopy is called to determine if this is a full copy or one delta copy stage
// in a multi-stage migration.
func (is *ImageioDataSource) IsDeltaCopy() bool {
	result := is.previousSnapshot != "" && is.currentSnapshot != ""
	return result
}

func createImageioReader(ctx context.Context, ep string, accessKey string, secKey string, certDir string, diskID string, currentCheckpoint string, previousCheckpoint string) (io.ReadCloser, uint64, *ovirtsdk4.ImageTransfer, ConnectionInterface, error) {
	conn, err := newOvirtClientFunc(ep, accessKey, secKey)
	if err != nil {
		return nil, uint64(0), nil, conn, errors.Wrap(err, "Error creating connection")
	}

	// Get disk
	disk, err := getDisk(conn, diskID)
	if err != nil {
		return nil, uint64(0), nil, conn, err
	}

	// Get snapshot if a checkpoint was specified
	var snapshot *ovirtsdk4.DiskSnapshot
	if currentCheckpoint != "" {
		var snapshotErr error
		snapshot, snapshotErr = getSnapshot(conn, disk, currentCheckpoint)
		if snapshot == nil { // Snapshot not found, check for a disk with a matching image ID
			imageID, available := disk.ImageId()
			if !available {
				return nil, uint64(0), nil, conn, errors.Wrap(snapshotErr, "Could not get disk's image ID!")
			}
			if imageID != currentCheckpoint {
				return nil, uint64(0), nil, conn, errors.Wrapf(snapshotErr, "Snapshot %s not found!", currentCheckpoint)
			}
			// Matching ID: use disk as checkpoint
			klog.Infof("Snapshot ID %s found on disk %s, transferring active disk as checkpoint.", currentCheckpoint, diskID)
		}
	}

	// For regular imports and the first stage of a multi-stage import, download as raw.
	// For actual snapshots and active disks whose image ID has been specified as the
	// snapshot to import, download as QCOW.
	formatType := ovirtsdk4.DISKFORMAT_RAW
	if currentCheckpoint != "" && previousCheckpoint != "" {
		klog.Info("Downloading snapshot as qcow")
		formatType = ovirtsdk4.DISKFORMAT_COW
	}

	// Get transfer ticket for disk or snapshot
	it, total, err := getTransfer(conn, disk, snapshot, formatType)
	if err != nil {
		return nil, uint64(0), it, conn, err
	}
	if it == nil {
		return nil, uint64(0), nil, conn, errors.New("returned ImageTransfer was nil")
	}

	// Use the create client from http source.
	client, err := createHTTPClient(certDir)
	if err != nil {
		return nil, uint64(0), it, conn, err
	}
	transferURL, available := it.TransferUrl()
	if !available {
		return nil, uint64(0), it, conn, errors.New("Error transfer url not available")
	}

	req, err := http.NewRequest("GET", transferURL, nil)
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return nil, uint64(0), it, conn, errors.Wrap(err, "Sending request failed")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, uint64(0), it, conn, errors.Errorf("bad status: %s", resp.Status)
	}

	if total == 0 {
		// The total seems bogus. Let's try the GET Content-Length header
		total = parseHTTPHeader(resp)
	}
	countingReader := &util.CountingReader{
		Reader:  resp.Body,
		Current: 0,
	}
	return countingReader, total, it, conn, nil
}

// Wrapper around cleanupTransfer that also clears is.imageTransfer.
// Avoids unnecessary cleanup work when the transfer is interrupted by SIGTERM.
func (is *ImageioDataSource) cleanupTransfer() {
	is.cleanupLock.Lock()
	defer is.cleanupLock.Unlock()
	if is.cleanupDone {
		return
	}
	err := cleanupTransfer(is.connection, is.imageTransfer)
	if err != nil {
		klog.Errorf("Failed to clean up image transfer: %v", err)
	} else {
		is.cleanupDone = true
	}
}

// cleanupTransfer makes sure the disk is unlocked before shutting down importer
func cleanupTransfer(conn ConnectionInterface, it *ovirtsdk4.ImageTransfer) error {
	var err error

	if conn == nil || it == nil {
		klog.Info("No transfer to clean up.")
		return nil
	}

	transferID, success := it.Id()
	if !success {
		return errors.New("unable to retrieve image transfer ID")
	}

	klog.Infof("Closing image transfer %s.", transferID)
	delay := 2 * time.Second
	transfersService := conn.SystemService().ImageTransfersService()
	transferService := transfersService.ImageTransferService(transferID)

	for retries := 10; retries > 0; retries-- {
		cancelTransfer := func() error {
			klog.Info("Cancelling image transfer.")
			if _, cancelError := transferService.Cancel().Send(); err != nil {
				klog.Errorf("Unable to cancel transfer request: %v", err)
				return cancelError
			}
			return nil
		}

		finalizeTransfer := func() error {
			klog.Info("Finalizing image transfer.")
			if _, finalizeError := transferService.Finalize().Send(); err != nil {
				klog.Errorf("Unable to finalize transfer request: %v", err)
				return finalizeError
			}
			return nil
		}

		imageTransferResponse, err := transferService.Get().Send()
		if err != nil {
			if strings.Contains(err.Error(), "404 Not Found") {
				klog.Info("Transfer ticket cleaned up.")
				return nil
			}
			// If transfer request can't be sent, blindly cancel and try again
			klog.Warningf("Unable to read image transfer response, %d retries remaining.", retries)
			cancelTransfer()
			time.Sleep(delay)
			continue
		}

		imageTransfer, available := imageTransferResponse.ImageTransfer()
		if !available {
			klog.Warningf("Unable to refresh image transfer, %d retries remaining.", retries)
			cancelTransfer()
			time.Sleep(delay)
			continue
		}

		transferPhase, available := imageTransfer.Phase()
		if !available {
			klog.Warningf("Unable to get transfer phase, %d retries remaining.", retries)
			cancelTransfer()
			time.Sleep(delay)
			continue
		}

		klog.Infof("Current image transfer phase is: %+v", transferPhase)
		phaseActions := map[ovirtsdk4.ImageTransferPhase]func() error{
			ovirtsdk4.IMAGETRANSFERPHASE_CANCELLED:          nil,
			ovirtsdk4.IMAGETRANSFERPHASE_FINALIZING_FAILURE: nil,
			ovirtsdk4.IMAGETRANSFERPHASE_FINALIZING_SUCCESS: nil,
			ovirtsdk4.IMAGETRANSFERPHASE_FINISHED_FAILURE:   nil,
			ovirtsdk4.IMAGETRANSFERPHASE_FINISHED_SUCCESS:   nil,
			ovirtsdk4.IMAGETRANSFERPHASE_INITIALIZING:       cancelTransfer,
			ovirtsdk4.IMAGETRANSFERPHASE_PAUSED_SYSTEM:      cancelTransfer,
			ovirtsdk4.IMAGETRANSFERPHASE_PAUSED_USER:        cancelTransfer,
			ovirtsdk4.IMAGETRANSFERPHASE_RESUMING:           cancelTransfer,
			ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING:       finalizeTransfer,
			ovirtsdk4.IMAGETRANSFERPHASE_UNKNOWN:            cancelTransfer,
			// Observed from RHV, but not yet listed in Go API:
			"cancelled_system":   nil,
			"cancelled_user":     nil,
			"finalizing_cleanup": nil,
		}
		action, available := phaseActions[transferPhase]
		if !available {
			klog.Warningf("Unknown transfer phase '%s', %d retries remaining.", transferPhase, retries)
			cancelTransfer()
			time.Sleep(delay)
			continue
		}
		if action != nil {
			err = action()
			if err != nil {
				klog.Warningf("Failed to run transfer cleanup command, %d retries remaining.", retries)
				time.Sleep(delay)
				continue
			}
		} else {
			klog.Info("No cleanup action required for this image transfer phase, done.")
			return nil
		}
	}

	return errors.Wrapf(err, "retry limit exceeded for transfer ticket cleanup, disk may remain locked until inactivity timeout")
}

// getDisk finds the disk with the given ID
func getDisk(conn ConnectionInterface, diskID string) (*ovirtsdk4.Disk, error) {
	disksService := conn.SystemService().DisksService()
	diskService := disksService.DiskService(diskID)
	diskRequest := diskService.Get()
	diskResponse, err := diskRequest.Send()
	if err != nil {
		return nil, errors.Wrapf(err, "error fetching disk %s", diskID)
	}
	disk, success := diskResponse.Disk()
	if !success {
		return nil, errors.Errorf("disk %s not found", diskID)
	}
	id, success := disk.Id()
	if !success {
		klog.Warningf("Unable to get ID from disk object, setting it from given %s.", diskID)
		disk.SetId(diskID)
	} else if id != diskID {
		klog.Warningf("Retrieved disk ID %s does not match expected disk ID %s!", id, diskID)
	}
	return disk, nil
}

// getSnapshot finds the snapshot with the given ID on the given disk.
func getSnapshot(conn ConnectionInterface, disk *ovirtsdk4.Disk, snapshotID string) (*ovirtsdk4.DiskSnapshot, error) {
	diskID, _ := disk.Id()
	storageDomains, success := disk.StorageDomains()
	if !success {
		return nil, errors.Errorf("no storage domains listed for disk %s", diskID)
	}

	for index, storageDomain := range storageDomains.Slice() {
		storageDomainName, success := storageDomain.Id()
		if !success {
			klog.Warningf("Skipping storage domain #%d listed on disk %s", index, diskID)
			continue
		}

		storageDomainService := conn.SystemService().StorageDomainsService().StorageDomainService(storageDomainName)
		if storageDomainService == nil {
			return nil, errors.Errorf("no service available for storage domain %s", storageDomainName)
		}
		snapshotsListResponse, err := storageDomainService.DiskSnapshotsService().List().Send()
		if err != nil {
			return nil, errors.Wrapf(err, "error listing snapshots in storage domain %s", storageDomainName)
		}

		snapshotsList, success := snapshotsListResponse.Snapshots()
		if !success || len(snapshotsList.Slice()) == 0 {
			return nil, errors.Errorf("no snapshots listed in storage domain %s", storageDomainName)
		}

		for snapshotIndex, snapshot := range snapshotsList.Slice() {
			id, success := snapshot.Id()
			if !success {
				klog.Warningf("Skipping snapshot #%d in storage domain %s", snapshotIndex, storageDomainName)
				continue
			}
			if snapshotID == id {
				klog.Infof("Successfully located snapshot %s on disk %s, in storage domain %s", snapshotID, diskID, storageDomainName)
				return snapshot, nil
			}
		}
	}

	return nil, errors.Errorf("could not find snapshot %s on disk %s", snapshotID, diskID)
}

func getTransfer(conn ConnectionInterface, disk *ovirtsdk4.Disk, snapshot *ovirtsdk4.DiskSnapshot, formatType ovirtsdk4.DiskFormat) (*ovirtsdk4.ImageTransfer, uint64, error) {
	totalSize, available := disk.TotalSize()
	if !available {
		return nil, uint64(0), errors.New("Error total disk size not available")
	}

	id, available := disk.Id()
	if !available {
		return nil, uint64(0), errors.New("Error disk id not available")
	}

	var imageTransferBuilder *ovirtsdk4.ImageTransferBuilder
	if snapshot != nil {
		imageTransferBuilder = ovirtsdk4.NewImageTransferBuilder().Snapshot(snapshot)
	} else {
		image, err := ovirtsdk4.NewImageBuilder().Id(id).Build()
		if err != nil {
			return nil, uint64(0), errors.Wrap(err, "Error building image object")
		}
		imageTransferBuilder = ovirtsdk4.NewImageTransferBuilder().Image(image)
	}

	imageTransfer, err := imageTransferBuilder.Direction(
		ovirtsdk4.IMAGETRANSFERDIRECTION_DOWNLOAD,
	).Format(
		formatType,
	).InactivityTimeout(
		60,
	).Build()
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "Error preparing transfer object")
	}

	transfer := conn.SystemService().ImageTransfersService().Add()
	transfer.ImageTransfer(imageTransfer)
	var it = &ovirtsdk4.ImageTransfer{}
	for {
		response, err := transfer.Send()
		if err != nil {
			if strings.Contains(err.Error(), "Disk is locked") || strings.Contains(err.Error(), "disks are locked") {
				klog.Infoln("waiting for disk to unlock")
				time.Sleep(15 * time.Second)
				continue
			}
			return nil, uint64(0), errors.Wrap(err, "Error sending transfer image request")
		}
		it, available = response.ImageTransfer()
		if !available {
			return nil, uint64(0), errors.New("Error image transfer not available")
		}
		phase, available := it.Phase()
		if !available {
			return it, uint64(0), errors.New("Error phase not available")
		}
		if phase == ovirtsdk4.IMAGETRANSFERPHASE_INITIALIZING {
			time.Sleep(1 * time.Second)
		} else if phase == ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING {
			break
		} else {
			return it, uint64(0), errors.Errorf("Error transfer phase: %s", phase)
		}
	}

	// For QCOW downloads, try to use actual size instead of total disk size for progress logging
	if snapshot != nil && formatType == ovirtsdk4.DISKFORMAT_COW {
		snapshotSize, available := snapshot.ActualSize()
		if !available {
			klog.Warning("Actual snapshot size not available, using total disk size. Progress percentage may be inaccurate.")
		} else {
			klog.Infof("Snapshot size is %d, adjusting size from previous total of %d", snapshotSize, totalSize)
			totalSize = snapshotSize
		}
	}

	return it, uint64(totalSize), nil
}

func loadCA(certDir string) (*x509.CertPool, error) {
	if certDir == "" {
		return nil, errors.New("Error CA not provided")
	}
	files, err := ioutil.ReadDir(certDir)
	if err != nil {
		return nil, errors.Wrapf(err, "Error listing files in %s", certDir)
	}

	caCertPool := x509.NewCertPool()
	for _, file := range files {
		if file.IsDir() || file.Name()[0] == '.' {
			continue
		}

		fp := path.Join(certDir, file.Name())

		klog.Infof("Attempting to get certs from %s", fp)

		certs, err := ioutil.ReadFile(fp)
		if err != nil {
			return nil, errors.Wrapf(err, "Error reading file %s", fp)
		}

		if ok := caCertPool.AppendCertsFromPEM(certs); !ok {
			klog.Warningf("No certs in %s", fp)
		}
	}
	return caCertPool, nil
}

// may be overridden in tests
var newOvirtClientFunc = getOvirtClient

// Not well defined abstractions in the SDK so we need to define below interfaces to mock the calls

// ConnectionInterface defines connection methods
type ConnectionInterface interface {
	SystemService() SystemServiceInteface
	Close() error
}

// DisksServiceInterface defines service methods
type DisksServiceInterface interface {
	DiskService(string) DiskServiceInterface
}

// DiskServiceInterface defines service methods
type DiskServiceInterface interface {
	Get() DiskServiceGetInterface
}

// DiskServiceGetInterface defines service methods
type DiskServiceGetInterface interface {
	Send() (DiskServiceResponseInterface, error)
}

// DiskServiceGetResponseInterface defines service methods
type DiskServiceGetResponseInterface interface {
	Disk() (*ovirtsdk4.Disk, bool)
}

// SystemServiceInteface defines service methods
type SystemServiceInteface interface {
	DisksService() DisksServiceInterface
	ImageTransfersService() ImageTransfersServiceInterface
	StorageDomainsService() StorageDomainsServiceInterface
}

// DiskSnapshotsServiceInterface defines disk snapshot service methods
type DiskSnapshotsServiceInterface interface {
	List() DiskSnapshotsServiceListRequestInterface
}

// DiskSnapshotsServiceListRequestInterface defines disk snapshot service list methods
type DiskSnapshotsServiceListRequestInterface interface {
	Send() (DiskSnapshotsServiceListResponseInterface, error)
}

// DiskSnapshotsServiceListResponseInterface defines disk snapshot service list response methods
type DiskSnapshotsServiceListResponseInterface interface {
	Snapshots() (DiskSnapshotSliceInterface, bool)
}

// DiskSnapshotSliceInterface defines disk snapshot slice methods
type DiskSnapshotSliceInterface interface {
	Slice() []*ovirtsdk4.DiskSnapshot
}

// StorageDomainsServiceInterface defines storage domains service methods
type StorageDomainsServiceInterface interface {
	StorageDomainService(string) StorageDomainServiceInterface
}

// StorageDomainServiceInterface defines storage domain service methods
type StorageDomainServiceInterface interface {
	DiskSnapshotsService() DiskSnapshotsServiceInterface
}

// ImageTransfersServiceInterface defines service methods
type ImageTransfersServiceInterface interface {
	Add() ImageTransferServiceAddInterface
	ImageTransferService(string) ImageTransferServiceInterface
}

// ImageTransferServiceInterface defines service methods
type ImageTransferServiceInterface interface {
	Cancel() ImageTransferServiceCancelRequestInterface
	Finalize() ImageTransferServiceFinalizeRequestInterface
	Get() ImageTransferServiceGetRequestInterface
}

// ImageTransferServiceCancelRequestInterface defines service methods
type ImageTransferServiceCancelRequestInterface interface {
	Send() (ImageTransferServiceCancelResponseInterface, error)
}

// ImageTransferServiceCancelResponseInterface defines service methods
type ImageTransferServiceCancelResponseInterface interface {
}

// ImageTransferServiceFinalizeRequestInterface defines service methods
type ImageTransferServiceFinalizeRequestInterface interface {
	Send() (ImageTransferServiceFinalizeResponseInterface, error)
}

// ImageTransferServiceFinalizeResponseInterface defines service methods
type ImageTransferServiceFinalizeResponseInterface interface {
}

// ImageTransferServiceGetRequestInterface defines service methods
type ImageTransferServiceGetRequestInterface interface {
	Send() (ImageTransferServiceGetResponseInterface, error)
}

// ImageTransferServiceGetResponseInterface defines service methods
type ImageTransferServiceGetResponseInterface interface {
	ImageTransfer() (*ovirtsdk4.ImageTransfer, bool)
}

// ImageTransferServiceAddInterface defines service methods
type ImageTransferServiceAddInterface interface {
	ImageTransfer(imageTransfer *ovirtsdk4.ImageTransfer) *ovirtsdk4.ImageTransfersServiceAddRequest
	Send() (ImageTransfersServiceAddResponseInterface, error)
}

// ImageTransfersServiceAddResponseInterface defines service methods
type ImageTransfersServiceAddResponseInterface interface {
	ImageTransfer() (*ovirtsdk4.ImageTransfer, bool)
}

// DiskServiceResponseInterface defines service methods
type DiskServiceResponseInterface interface {
	Disk() (*ovirtsdk4.Disk, bool)
}

// ConnectionWrapper wraps ovirt connection
type ConnectionWrapper struct {
	conn *ovirtsdk4.Connection
}

// SystemService wraps ovirt system service
type SystemService struct {
	srv *ovirtsdk4.SystemService
}

// DisksService wraps ovirt disks service
type DisksService struct {
	srv *ovirtsdk4.DisksService
}

// DiskService wraps ovirt disk service
type DiskService struct {
	srv *ovirtsdk4.DiskService
}

// DiskServiceGet wraps ovirt disk get service
type DiskServiceGet struct {
	srv *ovirtsdk4.DiskServiceGetRequest
}

// DiskServiceResponse wraps ovirt response get service
type DiskServiceResponse struct {
	srv *ovirtsdk4.DiskServiceGetResponse
}

// ImageTransfersService wraps ovirt transfer service
type ImageTransfersService struct {
	srv *ovirtsdk4.ImageTransfersService
}

// ImageTransferService wraps ovirt transfer service
type ImageTransferService struct {
	srv *ovirtsdk4.ImageTransferService
}

// ImageTransfersServiceAdd wraps ovirt add transfer service
type ImageTransfersServiceAdd struct {
	srv *ovirtsdk4.ImageTransfersServiceAddRequest
}

// ImageTransfersServiceResponse wraps ovirt add transfer service
type ImageTransfersServiceResponse struct {
	srv *ovirtsdk4.ImageTransfersServiceAddRequest
}

// ImageTransfersServiceAddResponse wraps ovirt add transfer service
type ImageTransfersServiceAddResponse struct {
	srv *ovirtsdk4.ImageTransfersServiceAddResponse
}

// ImageTransferServiceCancelRequest wraps cancel request
type ImageTransferServiceCancelRequest struct {
	srv *ovirtsdk4.ImageTransferServiceCancelRequest
}

// ImageTransferServiceCancelResponse wraps cancel response
type ImageTransferServiceCancelResponse struct {
	srv *ovirtsdk4.ImageTransferServiceCancelResponse
}

// ImageTransferServiceFinalizeRequest warps finalize request
type ImageTransferServiceFinalizeRequest struct {
	srv *ovirtsdk4.ImageTransferServiceFinalizeRequest
}

// ImageTransferServiceFinalizeResponse warps finalize response
type ImageTransferServiceFinalizeResponse struct {
	srv *ovirtsdk4.ImageTransferServiceFinalizeResponse
}

// ImageTransferServiceGetRequest wraps get request
type ImageTransferServiceGetRequest struct {
	srv *ovirtsdk4.ImageTransferServiceGetRequest
}

// ImageTransferServiceGetResponse wraps get response
type ImageTransferServiceGetResponse struct {
	srv *ovirtsdk4.ImageTransferServiceGetResponse
}

// ImageTransfer sets image transfer and returns add request
func (service *ImageTransfersServiceResponse) ImageTransfer(imageTransfer *ovirtsdk4.ImageTransfer) *ovirtsdk4.ImageTransfersServiceAddRequest {
	return service.srv.ImageTransfer(imageTransfer)
}

// Send return image transfer add response
func (service *ImageTransfersServiceAdd) Send() (*ovirtsdk4.ImageTransfersServiceAddResponse, error) {
	return service.srv.Send()
}

// Add returns image transfer add request
func (service *ImageTransfersService) Add() ImageTransferServiceAddInterface {
	return &ImageTransfersServiceResponse{
		service.srv.Add(),
	}
}

// Disk returns disk struct
func (service *DiskServiceResponse) Disk() (*ovirtsdk4.Disk, bool) {
	return service.srv.Disk()
}

// ImageTransfer returns disk struct
func (service *ImageTransfersServiceAddResponse) ImageTransfer() (*ovirtsdk4.ImageTransfer, bool) {
	return service.srv.ImageTransfer()
}

// Send returns disk get response
func (service *ImageTransfersServiceResponse) Send() (ImageTransfersServiceAddResponseInterface, error) {
	resp, err := service.srv.Send()
	return &ImageTransfersServiceAddResponse{
		srv: resp,
	}, err
}

// Send returns transfer cancel response
func (service *ImageTransferServiceCancelRequest) Send() (ImageTransferServiceCancelResponseInterface, error) {
	resp, err := service.srv.Send()
	return &ImageTransferServiceCancelResponse{
		srv: resp,
	}, err
}

// Send returns disk get response
func (service *ImageTransferServiceFinalizeRequest) Send() (ImageTransferServiceFinalizeResponseInterface, error) {
	resp, err := service.srv.Send()
	return &ImageTransferServiceFinalizeResponse{
		srv: resp,
	}, err
}

// Send returns image transfer get response
func (service *ImageTransferServiceGetRequest) Send() (ImageTransferServiceGetResponseInterface, error) {
	resp, err := service.srv.Send()
	return &ImageTransferServiceGetResponse{
		srv: resp,
	}, err
}

// ImageTransfer returns ImageTransfer struct
func (service *ImageTransferServiceGetResponse) ImageTransfer() (*ovirtsdk4.ImageTransfer, bool) {
	return service.srv.ImageTransfer()
}

// Send returns disk get response
func (service *DiskServiceGet) Send() (DiskServiceResponseInterface, error) {
	resp, err := service.srv.Send()
	return &DiskServiceResponse{
		srv: resp,
	}, err
}

// Get returns get service
func (service *DiskService) Get() DiskServiceGetInterface {
	return &DiskServiceGet{
		srv: service.srv.Get(),
	}
}

// DiskService returns disk service
func (service *DisksService) DiskService(id string) DiskServiceInterface {
	return &DiskService{
		srv: service.srv.DiskService(id),
	}
}

// DisksService returns disks service
func (service *SystemService) DisksService() DisksServiceInterface {
	return &DisksService{
		srv: service.srv.DisksService(),
	}
}

// ImageTransfersService returns image service
func (service *SystemService) ImageTransfersService() ImageTransfersServiceInterface {
	return &ImageTransfersService{
		srv: service.srv.ImageTransfersService(),
	}
}

// ImageTransferService returns image service
func (service *ImageTransfersService) ImageTransferService(id string) ImageTransferServiceInterface {
	return &ImageTransferService{
		srv: service.srv.ImageTransferService(id),
	}
}

// Cancel returns image service
func (service *ImageTransferService) Cancel() ImageTransferServiceCancelRequestInterface {
	return &ImageTransferServiceCancelRequest{
		srv: service.srv.Cancel(),
	}
}

// Finalize returns image service
func (service *ImageTransferService) Finalize() ImageTransferServiceFinalizeRequestInterface {
	return &ImageTransferServiceFinalizeRequest{
		srv: service.srv.Finalize(),
	}
}

// Get returns image transfer object
func (service *ImageTransferService) Get() ImageTransferServiceGetRequestInterface {
	return &ImageTransferServiceGetRequest{
		srv: service.srv.Get(),
	}
}

// DiskSnapshotSlice wraps oVirt's DiskSnapshotSlice
type DiskSnapshotSlice struct {
	srv *ovirtsdk4.DiskSnapshotSlice
}

// Slice returns a list of disk snapshots
func (service *DiskSnapshotSlice) Slice() []*ovirtsdk4.DiskSnapshot {
	return service.srv.Slice()
}

// DiskSnapshotsServiceListResponse wraps oVirt's DiskSnapshotsServiceListResponse
type DiskSnapshotsServiceListResponse struct {
	srv *ovirtsdk4.DiskSnapshotsServiceListResponse
}

// Snapshots returns a DiskSnapshotSlice containing some disk snapshots
func (service *DiskSnapshotsServiceListResponse) Snapshots() (DiskSnapshotSliceInterface, bool) {
	slice, success := service.srv.Snapshots()
	return &DiskSnapshotSlice{
		srv: slice,
	}, success
}

// DiskSnapshotsServiceListRequest wraps oVirt's DiskSnapshotsServiceListRequest
type DiskSnapshotsServiceListRequest struct {
	srv *ovirtsdk4.DiskSnapshotsServiceListRequest
}

// Send returns a reponse from listing disk snapshots
func (service *DiskSnapshotsServiceListRequest) Send() (DiskSnapshotsServiceListResponseInterface, error) {
	resp, err := service.srv.Send()
	return &DiskSnapshotsServiceListResponse{
		srv: resp,
	}, err
}

// DiskSnapshotsService wraps oVirt's DiskSnapshotsService
type DiskSnapshotsService struct {
	srv *ovirtsdk4.DiskSnapshotsService
}

// List returns a request object to get disk snapshots
func (service *DiskSnapshotsService) List() DiskSnapshotsServiceListRequestInterface {
	return &DiskSnapshotsServiceListRequest{
		srv: service.srv.List(),
	}
}

// StorageDomainsService wraps oVirt's StorageDomainsService
type StorageDomainsService struct {
	srv *ovirtsdk4.StorageDomainsService
}

// StorageDomainService wraps oVirt's storage domain service
type StorageDomainService struct {
	srv *ovirtsdk4.StorageDomainService
}

// DiskSnapshotsService returns a disk snapshots service object
func (service *StorageDomainService) DiskSnapshotsService() DiskSnapshotsServiceInterface {
	return &DiskSnapshotsService{
		srv: service.srv.DiskSnapshotsService(),
	}
}

// StorageDomainService returns a storage domain service object
func (service *StorageDomainsService) StorageDomainService(id string) StorageDomainServiceInterface {
	return &StorageDomainService{
		srv: service.srv.StorageDomainService(id),
	}
}

// StorageDomainsService returns a storage domains service object
func (service *SystemService) StorageDomainsService() StorageDomainsServiceInterface {
	return &StorageDomainsService{
		srv: service.srv.StorageDomainsService(),
	}
}

// SystemService returns system service
func (wrapper *ConnectionWrapper) SystemService() SystemServiceInteface {
	return &SystemService{
		srv: wrapper.conn.SystemService(),
	}
}

// Close closes the connection to ovirt
func (wrapper *ConnectionWrapper) Close() error {
	return wrapper.conn.Close()
}

func getOvirtClient(ep string, accessKey string, secKey string) (ConnectionInterface, error) {
	conn, err := ovirtsdk4.NewConnectionBuilder().URL(ep).Username(accessKey).Password(secKey).Insecure(true).Compress(true).Build()
	return &ConnectionWrapper{
		conn: conn,
	}, err
}
