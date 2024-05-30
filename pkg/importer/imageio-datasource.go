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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	ovirtsdk4 "github.com/ovirt/go-ovirt"
	ovirtclient "github.com/ovirt/go-ovirt-client"
	ovirtclientlog "github.com/ovirt/go-ovirt-client-log-klog"
	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
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
	file := filepath.Join(path, tempFile)
	err := CleanAll(file)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// we know that there won't be archives
	size, _ := util.GetAvailableSpace(path)
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}
	is.readers.StartProgressUpdate()
	err = streamDataToFile(is.readers.TopReader(), file)
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
	if !is.IsDeltaCopy() {
		if err := CleanAll(fileName); err != nil {
			return ProcessingPhaseError, err
		}
	}

	defer is.cleanupTransfer()
	is.readers.StartProgressUpdate()

	if extentsReader, err := is.getExtentsReader(); err == nil {
		err := is.StreamExtents(extentsReader, fileName)
		if err != nil {
			return ProcessingPhaseError, err
		}
	} else {
		err := streamDataToFile(is.readers.TopReader(), fileName)
		if err != nil {
			return ProcessingPhaseError, err
		}
	}
	return ProcessingPhaseResize, nil
}

// GetURL returns the URI that the data processor can use when converting the data.
func (is *ImageioDataSource) GetURL() *url.URL {
	return is.url
}

// GetTerminationMessage returns data to be serialized and used as the termination message of the importer.
func (is *ImageioDataSource) GetTerminationMessage() *common.TerminationMessage {
	return nil
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

// getExtentsReader retrieves an extents reader from ImageioDataSource.imageioReader, if it has one.
func (is *ImageioDataSource) getExtentsReader() (*extentReader, error) {
	countingReader, ok := is.imageioReader.(*util.CountingReader)
	if !ok {
		return nil, errors.New("not a counting reader")
	}
	extentsReader, ok := countingReader.Reader.(*extentReader)
	if !ok {
		return nil, errors.New("not an extents reader")
	}
	return extentsReader, nil
}

// StreamExtents requests individual extents from the ImageIO API and copies them to the destination.
// It skips downloading ranges of all zero bytes.
func (is *ImageioDataSource) StreamExtents(extentsReader *extentReader, fileName string) error {
	outFile, err := util.OpenFileOrBlockDevice(fileName)
	if err != nil {
		return err
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			klog.Infof("Error closing destination file: %v", err)
		}
	}()

	// Monitor for progress in the background and make sure the transfer ticket does not expire
	transferID, success := is.imageTransfer.Id()
	if !success {
		return errors.New("unable to retrieve image transfer ID for ticket renewal")
	}
	doneChannel := make(chan struct{}, 1)
	defer func() { doneChannel <- struct{}{} }()
	go is.monitorExtentsProgress(transferID, extentsReader, 30*time.Second, doneChannel)

	// Gather some information about the destination to choose which zero method to use
	info, err := outFile.Stat()
	if err != nil {
		return err
	}
	isBlock := !info.Mode().IsRegular()
	preallocated := info.Size() >= int64(is.contentLength)

	// Choose seek for regular files, and hole punching for block devices and pre-allocated files
	zeroRange := util.AppendZeroWithTruncate
	if isBlock || preallocated {
		zeroRange = util.PunchHole
	}

	// Transfer all the non-zero extents, and try to quickly write out blocks of all zero bytes for extents that only contain zero
	for index, extent := range extentsReader.extents {
		if extent.Zero {
			err = zeroRange(outFile, extent.Start, extent.Length)
			if err != nil {
				klog.Infof("Initial zero method failed, trying AppendZeroWithWrite instead. Error was: %v", err)
				zeroRange = util.AppendZeroWithWrite // If the initial choice fails, fall back to regular file writing
				err = zeroRange(outFile, extent.Start, extent.Length)
				if err != nil {
					return errors.Wrap(err, "failed to zero range on destination")
				}
			}
			is.readers.progressReader.Current += uint64(extent.Length)
		} else {
			klog.Infof("Downloading %d-byte extent at offset %d", extent.Length, extent.Start)
			responseBody, err := extentsReader.GetRange(extent.Start, extent.Start+extent.Length-1)
			if err != nil { // Ignore special EOF case, extents should give the exact right size to read
				return errors.Wrap(err, "failed to get range")
			}
			final := (index == (len(extentsReader.extents) - 1))
			err = is.transferExtent(responseBody, outFile, extent, final)
			if err != nil {
				return errors.Wrap(err, "failed to transfer extent")
			}
		}
	}

	// A sync here seems to make it more likely that the next sync will work.
	err = outFile.Sync()
	if err != nil {
		klog.Infof("Error from first attempt syncing %s: %v", fileName, err)
	}

	return nil
}

// transferExtent copies one extent from the source to the destination, updates the progress
// counter, and closes the source. Each source reader is expected to contain one extent.
func (is *ImageioDataSource) transferExtent(source io.ReadCloser, dest io.Writer, extent imageioExtent, final bool) error {
	defer source.Close()
	is.readers.progressReader.SetNextReader(source, final)

	written, err := io.Copy(dest, is.readers.progressReader)
	if err != nil {
		return errors.Wrap(err, "failed to write to file")
	}
	if written != extent.Length {
		return errors.New("failed to copy total extent length")
	}

	return nil
}

// monitorExtentsProgress sends a ticket renewal if there has been no download progress during the
// polling time. This can happen if the destination storage does not have a fast way to punch holes.
func (is *ImageioDataSource) monitorExtentsProgress(transferID string, extentsReader *extentReader, pollTime time.Duration, doneChannel chan struct{}) {
	current := is.readers.progressReader.Current
	for {
		select {
		case <-time.After(pollTime):
			if is.readers.progressReader.Current <= current {
				klog.Infof("No progress in the last %s, attempting ticket renewal to avoid timeout", pollTime)
				if err := is.renewExtentsTicket(transferID, extentsReader); err != nil {
					klog.Infof("Error renewing ticket: %v", err)
				}
			} else {
				current = is.readers.progressReader.Current
			}
		case <-doneChannel:
			klog.Info("Closing ticket expiration monitor")
			return
		}
	}
}

// renewExtentsTicket ensures the ImageIO transfer ticket stays active by issuing a renewal and
// by doing a small amount of I/O to prevent the oVirt engine from canceling the download.
func (is *ImageioDataSource) renewExtentsTicket(transferID string, extentsReader *extentReader) error {
	transfersService := is.connection.SystemService().ImageTransfersService()
	transferService := transfersService.ImageTransferService(transferID)
	_, err := transferService.Extend().Send()
	if err != nil {
		return errors.Wrap(err, "failed to renew transfer ticket")
	}

	readSize := 512
	buf := make([]byte, readSize)
	responseBody, err := extentsReader.GetRange(0, int64(readSize-1))
	if err != nil {
		return errors.Wrap(err, "failed sending small read to prevent download timeout")
	}
	defer responseBody.Close()

	_, err = io.ReadFull(responseBody, buf)
	return err
}

// imageioExtent holds information about a particular sequence of bytes, decodable from the ImageIO API.
type imageioExtent struct {
	Start  int64 `json:"start"`
	Length int64 `json:"length"`
	Zero   bool  `json:"zero"`
	Hole   bool  `json:"hole"`
}

// extentReader wraps the ImageIO extents API with the ReadCloser interface so that it can be used
// as the imageioReader in the ImageioDataSource.
type extentReader struct {
	client      *http.Client
	extents     []imageioExtent
	transferURL string
	offset      int64
	size        int64
}

// Read downloads a range of bytes from the ImageIO source. Having this attached
// to extentReader provides compatibility with the FormatReader header parsing.
func (reader *extentReader) Read(p []byte) (int, error) {
	start := reader.offset
	last := reader.size - 1
	end := start + int64(len(p)) - 1
	if end > last {
		end = last
	}
	length := end - start + 1

	responseBody, err := reader.GetRange(start, end)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, errors.Wrap(err, "failed to read from range")
	}
	if responseBody != nil {
		defer responseBody.Close()
	}

	var written int
	if !errors.Is(err, io.EOF) {
		written, err = io.ReadFull(responseBody, p[:length])
	}

	reader.offset += int64(written)
	return written, err
}

// Close closes the extentReader, which currently does not require any work.
func (reader *extentReader) Close() error {
	return nil
}

// GetRange requests a range of bytes from the ImageIO server, and returns the
// response body so the caller can copy the data wherever it needs to go.
func (reader *extentReader) GetRange(start, end int64) (io.ReadCloser, error) {
	// Return EOF if there are no more bytes to read
	if start >= reader.size {
		return nil, io.EOF
	}

	// Don't try to read past end of available data
	last := reader.size - 1
	if end > last {
		end = last
		klog.Infof("Range request extends past end of image, trimming to %d", end)
	}

	request, err := http.NewRequest(http.MethodGet, reader.transferURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create range request")
	}
	request.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	response, err := reader.client.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do range request")
	}

	contentLength := response.Header.Get("Content-Length")
	length, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		response.Body.Close()
		return nil, errors.Wrap(err, fmt.Sprintf("bad content-length in range request: %s", contentLength))
	}

	// Validate the returned length, and return an error if it does not match the expected range size
	expectedLength := end - start + 1
	if length != expectedLength {
		response.Body.Close()
		return nil, errors.Errorf("wrong length returned: %d vs expected %d", length, expectedLength)
	}

	return response.Body, nil
}

func createImageioReader(ctx context.Context, ep string, accessKey string, secKey string, certDir string, diskID string, currentCheckpoint string, previousCheckpoint string) (io.ReadCloser, uint64, *ovirtsdk4.ImageTransfer, ConnectionInterface, error) {
	conn, err := newOvirtClientFunc(ep, accessKey, secKey, certDir)
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
				return nil, uint64(0), nil, conn, errors.Wrap(snapshotErr, "could not get disk's image ID")
			}
			if imageID != currentCheckpoint {
				return nil, uint64(0), nil, conn, errors.Wrapf(snapshotErr, "snapshot %s not found", currentCheckpoint)
			}
			// Matching ID: use disk as checkpoint
			klog.Infof("Snapshot ID %s found on disk %s, transferring active disk as checkpoint", currentCheckpoint, diskID)
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

	// For raw images, see if the transfer can be sped up with the extents API
	extentsFeature := false
	if formatType == ovirtsdk4.DISKFORMAT_RAW {
		extentsFeature, err = checkExtentsFeature(ctx, client, transferURL)
		if err != nil {
			klog.Infof("Unable to check extents feature on this endpoint: %v", err)
		}
	}

	var reader io.ReadCloser
	if extentsFeature {
		req, err := http.NewRequest(http.MethodGet, transferURL+"/extents", nil)
		if err != nil {
			return nil, uint64(0), it, conn, err
		}
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			return nil, uint64(0), it, conn, errors.Wrap(err, "failed to query extents")
		}
		defer resp.Body.Close()

		var extents []imageioExtent
		err = json.NewDecoder(resp.Body).Decode(&extents)
		if err != nil {
			return nil, uint64(0), it, conn, errors.Wrap(err, "unable to decode extents")
		}

		// Add up all extents to calculate true total data size
		total = 0
		nonzero := int64(0)
		for _, extent := range extents {
			total += uint64(extent.Length)
			if !extent.Zero {
				nonzero += extent.Length
			}
		}
		klog.Infof("Total size of non-zero extents: %d, total size of all extents: %d", nonzero, total)

		reader = &extentReader{
			client:      client,
			extents:     extents,
			transferURL: transferURL,
			size:        int64(total),
		}
	} else {
		req, err := http.NewRequest(http.MethodGet, transferURL, nil)
		if err != nil {
			return nil, uint64(0), it, conn, errors.Wrap(err, "Sending request failed")
		}
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

		reader = resp.Body
	}

	countingReader := &util.CountingReader{
		Reader:  reader,
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
			if strings.Contains(err.Error(), "404 Not Found") || strings.Contains(err.Error(), "404 page not found") {
				klog.Info("Transfer ticket cleaned up.")
				return nil
			}
			// If transfer request can't be sent, blindly cancel and try again
			klog.Warningf("Unable to read image transfer response, %d retries remaining.", retries)
			cancelTransferErr := cancelTransfer()
			if cancelTransferErr != nil {
				klog.Warningf("Unable to cancel image transfer; %v", cancelTransferErr)
			}
			time.Sleep(delay)
			continue
		}

		imageTransfer, available := imageTransferResponse.ImageTransfer()
		if !available {
			klog.Warningf("Unable to refresh image transfer, %d retries remaining.", retries)
			cancelTransferErr := cancelTransfer()
			if cancelTransferErr != nil {
				klog.Warningf("Unable to cancel image transfer; %v", cancelTransferErr)
			}
			time.Sleep(delay)
			continue
		}

		transferPhase, available := imageTransfer.Phase()
		if !available {
			klog.Warningf("Unable to get transfer phase, %d retries remaining.", retries)
			cancelTransferErr := cancelTransfer()
			if cancelTransferErr != nil {
				klog.Warningf("Unable to cancel image transfer; %v", cancelTransferErr)
			}
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
			cancelTransferErr := cancelTransfer()
			if cancelTransferErr != nil {
				klog.Warningf("Unable to cancel image transfer; %v", cancelTransferErr)
			}
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
	files, err := os.ReadDir(certDir)
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

		certs, err := os.ReadFile(fp)
		if err != nil {
			return nil, errors.Wrapf(err, "Error reading file %s", fp)
		}

		if ok := caCertPool.AppendCertsFromPEM(certs); !ok {
			klog.Warningf("No certs in %s", fp)
		}
	}
	return caCertPool, nil
}

// ImageioImageOptions is the ImageIO API's response to an OPTIONS request
type ImageioImageOptions struct {
	UnixSocket string   `json:"unix_socket"`
	Features   []string `json:"features"`
	MaxReaders int      `json:"max_readers"`
	MaxWriters int      `json:"max_writers"`
}

// checkExtentsFeature sends OPTIONS to check for ImageIO extents API feature
func checkExtentsFeature(ctx context.Context, httpClient *http.Client, transferURL string) (bool, error) {
	request, err := http.NewRequest(http.MethodOptions, transferURL, nil)
	if err != nil {
		return false, errors.Wrap(err, "failed to create options request")
	}
	request = request.WithContext(ctx)

	response, err := httpClient.Do(request)
	if err != nil {
		return false, errors.Wrap(err, "error sending options request")
	}
	defer response.Body.Close()

	options := &ImageioImageOptions{}
	err = json.NewDecoder(response.Body).Decode(options)
	if err != nil {
		return false, errors.Wrap(err, "unable to decode options response")
	}

	extentsEnabled := false
	for _, feature := range options.Features {
		if feature == "extents" {
			extentsEnabled = true
		}
	}

	return extentsEnabled, nil
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
	Extend() ImageTransferServiceExtendRequestInterface
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

// ImageTransferServiceExtendRequestInterface defines service methods
type ImageTransferServiceExtendRequestInterface interface {
	Send() (ImageTransferServiceExtendResponseInterface, error)
}

// ImageTransferServiceExtendResponseInterface defines service methods
type ImageTransferServiceExtendResponseInterface interface {
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

// ImageTransferServiceExtendRequest wraps cancel request
type ImageTransferServiceExtendRequest struct {
	srv *ovirtsdk4.ImageTransferServiceExtendRequest
}

// ImageTransferServiceExtendResponse wraps cancel response
type ImageTransferServiceExtendResponse struct {
	srv *ovirtsdk4.ImageTransferServiceExtendResponse
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

// Send returns transfer extend response
func (service *ImageTransferServiceExtendRequest) Send() (ImageTransferServiceExtendResponseInterface, error) {
	resp, err := service.srv.Send()
	return &ImageTransferServiceExtendResponse{
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

// Extend returns image service
func (service *ImageTransferService) Extend() ImageTransferServiceExtendRequestInterface {
	return &ImageTransferServiceExtendRequest{
		srv: service.srv.Extend(),
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

// Send returns a response from listing disk snapshots
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

type extraSettings struct {
	compression  bool
	extraHeaders map[string]string
}

func (e *extraSettings) Compression() bool {
	return e.compression
}
func (e *extraSettings) ExtraHeaders() map[string]string {
	return e.extraHeaders
}

func getOvirtClient(ep string, accessKey string, secKey string, certDir string) (ConnectionInterface, error) {
	var conn *ovirtsdk4.Connection

	certPool, err := createCertPool(certDir)
	if err != nil {
		return nil, err
	}
	tls := ovirtclient.TLS()
	tls.CACertsFromCertPool(certPool)

	logger := ovirtclientlog.New()
	extras := &extraSettings{
		compression:  true,
		extraHeaders: nil,
	}

	client, err := ovirtclient.New(ep, accessKey, secKey, tls, logger, extras)
	if client != nil {
		conn = client.GetSDKClient()
	}

	return &ConnectionWrapper{
		conn: conn,
	}, err
}
