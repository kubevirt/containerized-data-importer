//
// This file implements the image upload-related functions of the oVirt client.
//

package ovirtclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) UploadImage(
	alias string,
	storageDomainID string,
	sparse bool,
	size uint64,
	reader readSeekCloser,
	retries ...RetryStrategy,
) (UploadImageResult, error) {
	retries = defaultRetries(retries, defaultLongTimeouts())
	progress, err := o.StartImageUpload(alias, storageDomainID, sparse, size, reader, retries...)
	if err != nil {
		return nil, err
	}
	<-progress.Done()
	if err := progress.Err(); err != nil {
		return nil, err
	}
	return progress, nil
}

func (o *oVirtClient) StartImageUpload(
	alias string,
	storageDomainID string,
	sparse bool,
	size uint64,
	reader readSeekCloser,
	retries ...RetryStrategy,
) (UploadImageProgress, error) {
	retries = defaultRetries(retries, defaultLongTimeouts())

	o.logger.Infof("Starting disk image upload...")

	format, qcowSize, err := extractQCOWParameters(size, reader)
	if err != nil {
		return nil, err
	}

	newCtx, cancel := context.WithCancel(context.Background())

	disk, err := o.createDiskForUpload(storageDomainID, alias, format, qcowSize, sparse, cancel)
	if err != nil {
		return nil, wrap(
			err,
			EUnidentified,
			"failed to create disk for image upload",
		)
	}

	return o.createProgress(
		newCtx,
		cancel,
		alias,
		qcowSize,
		size,
		reader,
		storageDomainID,
		sparse,
		disk,
		retries,
		format,
	)
}

func (o *oVirtClient) createDiskForUpload(
	storageDomainID string,
	alias string,
	format ImageFormat,
	qcowSize uint64,
	sparse bool,
	cancel context.CancelFunc,
) (*ovirtsdk4.Disk, error) {
	storageDomain, err := ovirtsdk4.NewStorageDomainBuilder().Id(storageDomainID).Build()
	if err != nil {
		return nil, wrap(
			err,
			EBug,
			"failed to build storage domain object from storage domain ID: %s",
			storageDomainID,
		)
	}
	diskBuilder := ovirtsdk4.NewDiskBuilder().
		Alias(alias).
		Format(ovirtsdk4.DiskFormat(format)).
		ProvisionedSize(int64(qcowSize)).
		InitialSize(int64(qcowSize)).
		StorageDomainsOfAny(storageDomain)
	diskBuilder.Sparse(sparse)
	disk, err := diskBuilder.Build()
	if err != nil {
		cancel()
		return nil, wrap(
			err,
			EBug,
			"failed to build disk with alias %s, format %s, provisioned and initial size %d",
			alias,
			format,
			qcowSize,
		)
	}
	return disk, nil
}

func (o *oVirtClient) createProgress(
	newCtx context.Context,
	cancel context.CancelFunc,
	alias string,
	qcowSize uint64,
	size uint64,
	reader readSeekCloser,
	storageDomainID string,
	sparse bool,
	disk *ovirtsdk4.Disk,
	retries []RetryStrategy,
	format ImageFormat,
) (UploadImageProgress, error) {
	progress := &uploadImageProgress{
		cli:             o,
		correlationID:   fmt.Sprintf("image_transfer_%s", alias),
		uploadedBytes:   0,
		cowSize:         qcowSize,
		size:            size,
		reader:          reader,
		storageDomainID: storageDomainID,
		sparse:          sparse,
		alias:           alias,
		ctx:             newCtx,
		done:            make(chan struct{}),
		lock:            &sync.Mutex{},
		cancel:          cancel,
		err:             nil,
		conn:            o.conn,
		httpClient:      o.httpClient,
		sdkDisk:         disk,
		client:          o,
		logger:          o.logger,
		retries:         retries,
		format:          format,
	}
	go progress.upload()
	return progress, nil
}

type uploadImageProgress struct {
	cli             *oVirtClient
	uploadedBytes   uint64
	cowSize         uint64
	size            uint64
	reader          readSeekCloser
	storageDomainID string
	sparse          bool
	alias           string

	// logger contains the facility to write log messages
	logger Logger
	// ctx is the context that indicates that the upload should terminate as soon as possible. The actual upload may run
	// longer in order to facilitate proper cleanup.
	ctx context.Context
	// done is the channel that is closed when the upload is completely done, either with an error, or successfully.
	done chan struct{}
	// lock is a lock that prevents race conditions during the upload process.
	lock *sync.Mutex
	// cancel is the cancel function for the context. HasCode is called to ensure that the context is properly canceled.
	cancel context.CancelFunc
	// err holds the error that happened during the upload. It can be queried using the Err() method.
	err error
	// conn is the underlying oVirt connection.
	conn *ovirtsdk4.Connection
	// httpClient is the raw HTTP client for connecting the oVirt Engine.
	httpClient http.Client
	// disk is the oVirt disk that was provisioned during the upload. May be nil if the disk doesn't exist yet.
	disk Disk
	// client is the Client instance that created this image upload.
	client *oVirtClient
	// correlationID is an identifier for the upload process.
	correlationID string
	// retries contains the retry configuration
	retries []RetryStrategy
	// format is the format of the image requested, or the image being uploaded.
	format ImageFormat
	// sdkDisk is the internal disk object stored for creating the disk
	sdkDisk *ovirtsdk4.Disk
}

func (u *uploadImageProgress) CorrelationID() string {
	return u.correlationID
}

func (u *uploadImageProgress) Disk() Disk {
	return u.disk
}

func (u *uploadImageProgress) updateDisk(disk Disk) {
	u.disk = disk
}

func (u *uploadImageProgress) UploadedBytes() uint64 {
	return u.uploadedBytes
}

func (u *uploadImageProgress) TotalBytes() uint64 {
	return u.size
}

func (u *uploadImageProgress) Err() error {
	u.lock.Lock()
	defer u.lock.Unlock()
	if u.err != nil {
		return u.err
	}
	return nil
}

func (u *uploadImageProgress) Done() <-chan struct{} {
	return u.done
}

func (u *uploadImageProgress) Read(p []byte) (n int, err error) {
	select {
	case <-u.ctx.Done():
		return 0, newError(ETimeout, "timeout while uploading image")
	default:
	}
	n, err = u.reader.Read(p)
	u.uploadedBytes += uint64(n)
	return
}

// upload uploads the image file in the background. It is intended to be called as a goroutine. The error status can
// be obtained from Err(), while the done status can be queried using Done().
func (u *uploadImageProgress) upload() {
	defer func() {
		// Cancel context to indicate done.
		u.lock.Lock()
		u.cancel()
		close(u.done)
		u.lock.Unlock()
	}()

	if err := u.processUpload(); err != nil {
		u.err = err
	}
}

// processUpload is the function that does the actual upload of the image.
func (u *uploadImageProgress) processUpload() error {
	if err := u.createDisk(); err != nil {
		u.removeDisk()
		return err
	}

	transfer := newImageTransfer(
		u.cli,
		u.logger,
		u.disk.ID(),
		u.correlationID,
		u.retries,
		ovirtsdk4.IMAGETRANSFERDIRECTION_UPLOAD,
		ovirtsdk4.DiskFormat(u.format),
		u.updateDisk,
	)
	transferURL := ""
	var err error
	if transferURL, err = transfer.initialize(); err != nil {
		err = transfer.finalize(err)
		u.removeDisk()
		return err
	}
	err = u.transferImage(transfer, transferURL)
	err = transfer.finalize(err)
	if err != nil {
		u.removeDisk()
	}
	return err
}

// transferImage does a HTTP request to transfer the image to the specified transfer URL.
func (u *uploadImageProgress) transferImage(transfer imageTransfer, transferURL string) error {
	return retry(
		fmt.Sprintf(
			"transferring image for disk %s via HTTP request to %s",
			u.disk.ID(),
			transferURL,
		),
		u.logger,
		u.retries,
		func() error {
			return u.putRequest(transferURL, transfer)
		},
	)
}

// putRequest performs a single HTTP put request to upload an image. This can be called multiple times to retry an
// upload.
func (u *uploadImageProgress) putRequest(transferURL string, transfer imageTransfer) error {
	// We ensure that the reader is at the first byte before attempting a PUT request, otherwise we may upload an
	// incomplete image.
	if _, err := u.reader.Seek(0, io.SeekStart); err != nil {
		return wrap(
			err,
			ELocalIO,
			"could not seek to the first byte of the disk image before upload",
		)
	}

	putRequest, err := http.NewRequest(http.MethodPut, transferURL, u)
	if err != nil {
		return wrap(err, EUnidentified, "failed to create HTTP request")
	}
	putRequest.Header.Add("content-type", "application/octet-stream")
	putRequest.ContentLength = int64(u.size)
	putRequest.Body = u.reader
	response, err := u.httpClient.Do(putRequest)
	if err != nil {
		return wrap(
			err,
			EUnidentified,
			"failed to upload image",
		)
	}
	if err := transfer.checkStatusCode(response.StatusCode); err != nil {
		_ = response.Body.Close()
		return err
	}
	if err := response.Body.Close(); err != nil {
		return wrap(
			err,
			EUnidentified,
			"failed to close response body while uploading image",
		)
	}
	return nil
}

// removeDisk removes a disk after a failed upload.
func (u *uploadImageProgress) removeDisk() {
	if u.disk != nil {
		if err := u.client.RemoveDisk(u.disk.ID(), u.retries...); err != nil {
			// We are already in a failure state so we can't do anything but log this error.
			u.logger.Warningf(
				"failed to remove disk %s after failed image upload, please remove manually (%v)",
				u.disk.ID(),
				err,
			)
		}
	}
}

// createDisk creates a disk on oVirt before the image upload. This function will retry until the request is successful
// or the retries are exhausted. It will also call the updateDisk hook.
func (u *uploadImageProgress) createDisk() (err error) {
	// This will never fail because we set up the disk in the previous step.
	diskAlias := u.sdkDisk.MustAlias()
	err = retry(
		fmt.Sprintf(
			"creating disk with alias %s for image upload",
			diskAlias,
		),
		u.logger,
		u.retries,
		u.attemptCreateDisk,
	)
	return
}

// attemptCreateDisk will launch a single attempt at creating a disk in the oVirt Engine.
func (u *uploadImageProgress) attemptCreateDisk() error {
	addDiskRequest := u.conn.SystemService().DisksService().Add().Disk(u.sdkDisk)
	addDiskRequest.Query("correlation_id", u.correlationID)
	addResp, err := addDiskRequest.Send()
	if err != nil {
		return err
	}
	sdkDisk, ok := addResp.Disk()
	if !ok {
		return newError(EFieldMissing, "add disk response did not contain a disk")
	}
	disk, err := convertSDKDisk(sdkDisk, u.cli)
	if err != nil {
		return wrap(err, EBug, "failed to convert disk object")
	}
	u.updateDisk(disk)
	return nil
}
