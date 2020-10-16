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
}

// NewImageioDataSource creates a new instance of the ovirt-imageio data provider.
func NewImageioDataSource(endpoint string, accessKey string, secKey string, certDir string, diskID string) (*ImageioDataSource, error) {
	ctx, cancel := context.WithCancel(context.Background())
	imageioReader, contentLength, it, conn, err := createImageioReader(ctx, endpoint, accessKey, secKey, certDir, diskID)
	if err != nil {
		cancel()
		return nil, err
	}
	imageioSource := &ImageioDataSource{
		ctx:           ctx,
		cancel:        cancel,
		imageioReader: imageioReader,
		contentLength: contentLength,
		imageTransfer: it,
		connection:    conn,
	}
	// We know this is a counting reader, so no need to check.
	countingReader := imageioReader.(*util.CountingReader)
	go imageioSource.pollProgress(countingReader, 10*time.Minute, time.Second)
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
	// we know that there won't be archives
	size, _ := util.GetAvailableSpace(path)
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}
	file := filepath.Join(path, tempFile)
	err := util.StreamDataToFile(is.readers.TopReader(), file)
	if err != nil {
		return ProcessingPhaseError, err
	}
	// If we successfully wrote to the file, then the parse will succeed.
	is.url, _ = url.Parse(file)
	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (is *ImageioDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
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
	if is.imageTransfer != nil {
		if itID, ok := is.imageTransfer.Id(); ok {
			transfersService := is.connection.SystemService().ImageTransfersService()
			_, err = transfersService.ImageTransferService(itID).Finalize().Send()
		}
	}
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

func createImageioReader(ctx context.Context, ep string, accessKey string, secKey string, certDir string, diskID string) (io.ReadCloser, uint64, *ovirtsdk4.ImageTransfer, ConnectionInterface, error) {
	conn, err := newOvirtClientFunc(ep, accessKey, secKey)
	if err != nil {
		return nil, uint64(0), nil, conn, errors.Wrap(err, "Error creating connection")
	}

	it, total, err := getTransfer(conn, diskID)
	if err != nil {
		return nil, uint64(0), it, conn, err
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

func getTransfer(conn ConnectionInterface, diskID string) (*ovirtsdk4.ImageTransfer, uint64, error) {
	disksService := conn.SystemService().DisksService()
	diskService := disksService.DiskService(diskID)
	diskRequest := diskService.Get()
	diskResponse, err := diskRequest.Send()
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "Error fetching disk")
	}
	disk, success := diskResponse.Disk()
	if !success {
		return nil, uint64(0), errors.New("Error disk not found")
	}

	totalSize, available := disk.TotalSize()
	if !available {
		return nil, uint64(0), errors.New("Error total disk size not available")
	}

	id, available := disk.Id()
	if !available {
		return nil, uint64(0), errors.New("Error disk id not available")
	}

	image, err := ovirtsdk4.NewImageBuilder().Id(id).Build()
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "Error building image object")
	}

	transfersService := conn.SystemService().ImageTransfersService()
	transfer := transfersService.Add()
	imageTransfer, err := ovirtsdk4.NewImageTransferBuilder().Image(
		image,
	).Direction(
		ovirtsdk4.IMAGETRANSFERDIRECTION_DOWNLOAD,
	).Format(
		ovirtsdk4.DISKFORMAT_RAW,
	).Build()
	if err != nil {
		return nil, uint64(0), errors.Wrap(err, "Error preparing transfer object")
	}

	transfer.ImageTransfer(imageTransfer)
	var it = &ovirtsdk4.ImageTransfer{}
	for {
		response, err := transfer.Send()
		if err != nil {
			if strings.Contains(err.Error(), "Disk is locked") {
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
			return nil, uint64(0), errors.New("Error phase not available")
		}
		if phase == ovirtsdk4.IMAGETRANSFERPHASE_INITIALIZING {
			time.Sleep(1 * time.Second)
		} else if phase == ovirtsdk4.IMAGETRANSFERPHASE_TRANSFERRING {
			break
		} else {
			return nil, uint64(0), errors.Errorf("Error transfer phase: %s", phase)
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
}

// ImageTransfersServiceInterface defines service methods
type ImageTransfersServiceInterface interface {
	Add() ImageTransferServiceAddInterface
	ImageTransferService(string) ImageTransferServiceInterface
}

// ImageTransferServiceInterface defines service methods
type ImageTransferServiceInterface interface {
	Finalize() ImageTransferServiceFinalizeRequestInterface
}

// ImageTransferServiceFinalizeRequestInterface defines service methods
type ImageTransferServiceFinalizeRequestInterface interface {
	Send() (ImageTransferServiceFinalizeResponseInterface, error)
}

// ImageTransferServiceFinalizeResponseInterface defines service methods
type ImageTransferServiceFinalizeResponseInterface interface {
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

// ImageTransferServiceFinalizeRequest warps finalize request
type ImageTransferServiceFinalizeRequest struct {
	srv *ovirtsdk4.ImageTransferServiceFinalizeRequest
}

// ImageTransferServiceFinalizeResponse warps finalize response
type ImageTransferServiceFinalizeResponse struct {
	srv *ovirtsdk4.ImageTransferServiceFinalizeResponse
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

// Send returns disk get response
func (service *ImageTransferServiceFinalizeRequest) Send() (ImageTransferServiceFinalizeResponseInterface, error) {
	resp, err := service.srv.Send()
	return &ImageTransferServiceFinalizeResponse{
		srv: resp,
	}, err
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

// Finalize returns image service
func (service *ImageTransferService) Finalize() ImageTransferServiceFinalizeRequestInterface {
	return &ImageTransferServiceFinalizeRequest{
		srv: service.srv.Finalize(),
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
