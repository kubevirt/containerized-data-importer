/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2018 Red Hat, Inc.
 *
 */

package uploadserver

import (
	"archive/tar"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang/snappy"
	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/importer"
	"kubevirt.io/containerized-data-importer/pkg/util"
	cryptowatch "kubevirt.io/containerized-data-importer/pkg/util/tls-crypto-watch"
)

const (
	healthzPath = "/healthz"
)

type Config struct {
	Insecure    bool
	BindAddress string
	BindPort    int

	Destination string

	ServerKeyFile, ServerCertFile string
	ClientCertFile, ClientName    string

	ImageSize          string
	FilesystemOverhead float64
	Preallocation      bool

	Deadline *time.Time

	CryptoConfig cryptowatch.CryptoConfig
}

// RunResult is the result of the upload server run
type RunResult struct {
	CloneTarget          bool
	PreallocationApplied bool
	DeadlinePassed       bool
}

// UploadServer is the interface to uploadServerApp
type UploadServer interface {
	Run() (*RunResult, error)
}

type uploadServerApp struct {
	config               *Config
	mux                  *http.ServeMux
	uploading            bool
	processing           bool
	done                 bool
	preallocationApplied bool
	cloneTarget          bool
	doneChan             chan struct{}
	errChan              chan error
	mutex                sync.Mutex
}

type imageReadCloser func(*http.Request) (io.ReadCloser, error)

// may be overridden in tests
var uploadProcessorFunc = newUploadStreamProcessor
var uploadProcessorFuncAsync = newAsyncUploadStreamProcessor

func bodyReadCloser(r *http.Request) (io.ReadCloser, error) {
	return r.Body, nil
}

func formReadCloser(r *http.Request) (io.ReadCloser, error) {
	multiReader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}

	var filePart *multipart.Part

	for {
		filePart, err = multiReader.NextPart()
		if err != nil || filePart.FormName() == "file" {
			break
		}
		klog.Infof("Ignoring part %s", filePart.FormName())
	}

	// multiReader.NextPart() returns io.EOF when read everything
	if err != nil {
		return nil, err
	}

	return filePart, nil
}

func isCloneTarget(contentType string) bool {
	return contentType == common.BlockdeviceClone || contentType == common.FilesystemCloneContentType
}

// NewUploadServer returns a new instance of uploadServerApp
func NewUploadServer(config *Config) UploadServer {
	server := &uploadServerApp{
		config:    config,
		mux:       http.NewServeMux(),
		uploading: false,
		done:      false,
		doneChan:  make(chan struct{}),
		errChan:   make(chan error),
	}

	server.mux.HandleFunc(healthzPath, server.healthzHandler)
	for _, path := range common.SyncUploadPaths {
		server.mux.HandleFunc(path, server.uploadHandler(bodyReadCloser))
	}
	for _, path := range common.AsyncUploadPaths {
		server.mux.HandleFunc(path, server.uploadHandlerAsync(bodyReadCloser))
	}
	for _, path := range common.ArchiveUploadPaths {
		server.mux.HandleFunc(path, server.uploadArchiveHandler(bodyReadCloser))
	}
	for _, path := range common.SyncUploadFormPaths {
		server.mux.HandleFunc(path, server.uploadHandler(formReadCloser))
	}
	for _, path := range common.AsyncUploadFormPaths {
		server.mux.HandleFunc(path, server.uploadHandlerAsync(formReadCloser))
	}

	return server
}

func (app *uploadServerApp) Run() (*RunResult, error) {
	uploadServer := http.Server{
		Handler:           app,
		ReadHeaderTimeout: 10 * time.Second,
	}

	uploadListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", app.config.BindAddress, app.config.BindPort))
	if err != nil {
		return nil, errors.Wrap(err, "Error creating upload listerner")
	}

	tlsConfig, err := app.getTLSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Error getting TLS config")
	}

	go func() {
		defer uploadListener.Close()

		// maybe bind port was 0 (unit tests) assign port here
		app.config.BindPort = uploadListener.Addr().(*net.TCPAddr).Port

		if tlsConfig != nil {
			uploadServer.TLSConfig = tlsConfig
			app.errChan <- uploadServer.ServeTLS(uploadListener, "", "")
			return
		}

		// not sure we want to support this code path
		app.errChan <- uploadServer.Serve(uploadListener)
	}()

	var timeChan <-chan time.Time

	if app.config.Deadline != nil {
		timeChan = time.After(time.Until(*app.config.Deadline))
	} else {
		tc := make(chan time.Time)
		defer close(tc)
		timeChan = tc
	}

	select {
	case err = <-app.errChan:
		if err != nil {
			klog.Errorf("HTTP server returned error %s", err.Error())
			return nil, err
		}
	case <-app.doneChan:
		klog.Info("Shutting down http server after successful upload")
		if err := uploadServer.Shutdown(context.Background()); err != nil {
			klog.Errorf("failed to shutdown uploadServer; %v", err)
		}
	case <-timeChan:
		klog.Info("deadline exceeded, shutting down")
		app.mutex.Lock()
		defer app.mutex.Unlock()
		for {
			if app.uploading || app.processing {
				klog.Info("waiting for upload to finish")
				app.mutex.Unlock()
				time.Sleep(2 * time.Second)
				app.mutex.Lock()
			} else {
				break
			}
		}
		if !app.done {
			klog.Info("upload not done, process exiting")
			return &RunResult{DeadlinePassed: true}, nil
		}
	}

	result := &RunResult{
		CloneTarget:          app.cloneTarget,
		PreallocationApplied: app.preallocationApplied,
	}

	return result, nil
}

func (app *uploadServerApp) getTLSConfig() (*tls.Config, error) {
	if app.config.ServerCertFile == "" || app.config.ServerKeyFile == "" {
		if !app.config.Insecure {
			return nil, errors.New("invalid TLS config")
		}
		return nil, nil
	}

	//nolint:gosec // False positive: Min version is not known statically
	config := &tls.Config{
		CipherSuites: app.config.CryptoConfig.CipherSuites,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		MinVersion:   app.config.CryptoConfig.MinVersion,
	}

	if app.config.ClientCertFile != "" {
		bs, err := os.ReadFile(app.config.ClientCertFile)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(bs); !ok {
			return nil, err
		}

		config.ClientCAs = caCertPool
	}

	cert, err := tls.LoadX509KeyPair(app.config.ServerCertFile, app.config.ServerKeyFile)
	if err != nil {
		return nil, err
	}

	config.Certificates = []tls.Certificate{cert}

	return config, nil
}

func (app *uploadServerApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.mux.ServeHTTP(w, r)
}

func (app *uploadServerApp) healthzHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := io.WriteString(w, "OK"); err != nil {
		klog.Errorf("healthzHandler: failed to send response; %v", err)
	}
}

func (app *uploadServerApp) validateShouldHandleRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return false
	}

	if r.TLS != nil {
		if len(r.TLS.VerifiedChains) == 0 {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		found := false
		for _, cert := range r.TLS.PeerCertificates {
			if cert.Subject.CommonName == app.config.ClientName {
				found = true
				break
			}
		}

		if !found {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
	} else {
		if !app.config.Insecure {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		klog.V(3).Infof("Handling HTTP connection")
	}

	app.mutex.Lock()
	defer app.mutex.Unlock()

	if app.uploading || app.processing {
		klog.Warning("Got concurrent upload request")
		w.WriteHeader(http.StatusServiceUnavailable)
		return false
	}

	if app.done {
		klog.Warning("Got upload request after already done")
		w.WriteHeader(http.StatusConflict)
		return false
	}

	app.uploading = true

	return true
}

func (app *uploadServerApp) uploadHandlerAsync(irc imageReadCloser) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}

		if !app.validateShouldHandleRequest(w, r) {
			return
		}

		cdiContentType := r.Header.Get(common.UploadContentTypeHeader)

		klog.Infof("Content type header is %q\n", cdiContentType)

		readCloser, err := irc(r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}

		processor, err := uploadProcessorFuncAsync(readCloser, app.config.Destination, app.config.ImageSize, app.config.FilesystemOverhead, app.config.Preallocation, cdiContentType)

		app.mutex.Lock()
		defer app.mutex.Unlock()
		app.uploading = false

		if err != nil {
			handleStreamError(w, err)
			return
		}

		app.processing = true

		// Start processing.
		go func() {
			err := processor.ProcessDataResume()
			app.mutex.Lock()
			defer app.mutex.Unlock()
			app.processing = false
			if err != nil {
				klog.Errorf("Error during resumed processing: %v", err)
				app.errChan <- err
				return
			}
			defer close(app.doneChan)
			app.done = true
			app.preallocationApplied = processor.PreallocationApplied()
			app.cloneTarget = isCloneTarget(cdiContentType)
			klog.Infof("Wrote data to %s", app.config.Destination)
		}()

		klog.Info("Returning success to caller, continue processing in background")
	}
}

func (app *uploadServerApp) processUpload(irc imageReadCloser, w http.ResponseWriter, r *http.Request, dvContentType cdiv1.DataVolumeContentType) {
	if !app.validateShouldHandleRequest(w, r) {
		return
	}

	cdiContentType := r.Header.Get(common.UploadContentTypeHeader)

	klog.Infof("Content type header is %q\n", cdiContentType)

	readCloser, err := irc(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}

	preallocationApplied, err := uploadProcessorFunc(readCloser, app.config.Destination, app.config.ImageSize, app.config.FilesystemOverhead, app.config.Preallocation, cdiContentType, dvContentType)

	app.mutex.Lock()
	defer app.mutex.Unlock()
	app.uploading = false

	if err != nil {
		handleStreamError(w, err)
		return
	}

	app.done = true
	app.preallocationApplied = preallocationApplied
	app.cloneTarget = isCloneTarget(cdiContentType)
	close(app.doneChan)

	if dvContentType == cdiv1.DataVolumeArchive {
		klog.Infof("Wrote archive data")
	} else {
		klog.Infof("Wrote data to %s", app.config.Destination)
	}
}

func (app *uploadServerApp) uploadHandler(irc imageReadCloser) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app.processUpload(irc, w, r, cdiv1.DataVolumeKubeVirt)
	}
}

func (app *uploadServerApp) uploadArchiveHandler(irc imageReadCloser) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app.processUpload(irc, w, r, cdiv1.DataVolumeArchive)
	}
}

func newAsyncUploadStreamProcessor(stream io.ReadCloser, dest, imageSize string, filesystemOverhead float64, preallocation bool, sourceContentType string) (*importer.DataProcessor, error) {
	if isCloneTarget(sourceContentType) {
		return nil, fmt.Errorf("async clone not supported")
	}

	uds := importer.NewAsyncUploadDataSource(newContentReader(stream, sourceContentType))
	processor := importer.NewDataProcessor(uds, dest, common.ImporterVolumePath, common.ScratchDataDir, imageSize, filesystemOverhead, preallocation, "")
	return processor, processor.ProcessDataWithPause()
}

func newUploadStreamProcessor(stream io.ReadCloser, dest, imageSize string, filesystemOverhead float64, preallocation bool, sourceContentType string, dvContentType cdiv1.DataVolumeContentType) (bool, error) {
	stream = newContentReader(stream, sourceContentType)
	if isCloneTarget(sourceContentType) {
		return cloneProcessor(stream, sourceContentType, dest, preallocation)
	}

	// Clone block device to block device or file system
	uds := importer.NewUploadDataSource(stream, dvContentType)
	processor := importer.NewDataProcessor(uds, dest, common.ImporterVolumePath, common.ScratchDataDir, imageSize, filesystemOverhead, preallocation, "")
	err := processor.ProcessData()
	return processor.PreallocationApplied(), err
}

func cloneProcessor(stream io.ReadCloser, contentType, dest string, preallocate bool) (bool, error) {
	if contentType == common.FilesystemCloneContentType {
		if dest != common.WriteBlockPath {
			return fileToFileCloneProcessor(stream)
		}

		tarImageReader, err := newTarDiskImageReader(stream)
		if err != nil {
			stream.Close()
			return false, err
		}
		stream = tarImageReader
	}

	defer stream.Close()

	_, _, err := importer.StreamDataToFile(stream, dest, preallocate)
	if err != nil {
		return false, err
	}

	return false, nil
}

func fileToFileCloneProcessor(stream io.ReadCloser) (bool, error) {
	defer stream.Close()
	if err := util.UnArchiveTar(stream, common.ImporterVolumePath); err != nil {
		return false, errors.Wrapf(err, "error unarchiving to %s", common.ImporterVolumePath)
	}
	return true, nil
}

type closeWrapper struct {
	io.Reader
	closers []io.Closer
}

func (c *closeWrapper) Close() error {
	var err error
	for _, closer := range c.closers {
		if e := closer.Close(); e != nil {
			err = e
		}
	}
	return err
}

type tarDiskImageReader struct {
	tr           *tar.Reader
	size, offset int64
}

func (r *tarDiskImageReader) Read(p []byte) (int, error) {
	if r.offset >= r.size {
		return 0, io.EOF
	}
	remaining := r.size - r.offset
	if int(remaining) < len(p) {
		p = p[:remaining]
	}
	n, err := r.tr.Read(p)
	r.offset += int64(n)
	klog.V(3).Infof("Read %d bytes, offset %d, size %d", n, r.offset, r.size)
	return n, err
}

func newTarDiskImageReader(stream io.ReadCloser) (io.ReadCloser, error) {
	tr := tar.NewReader(stream)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !strings.Contains(header.Name, common.DiskImageName) {
			continue
		}
		return &closeWrapper{
			Reader:  &tarDiskImageReader{tr: tr, size: header.Size},
			closers: []io.Closer{stream},
		}, nil
	}
	return nil, fmt.Errorf("no disk image found in tar")
}

func newContentReader(stream io.ReadCloser, contentType string) io.ReadCloser {
	if isCloneTarget(contentType) {
		return newSnappyReadCloser(stream)
	}
	return stream
}

func newSnappyReadCloser(stream io.ReadCloser) io.ReadCloser {
	return &closeWrapper{
		Reader:  snappy.NewReader(stream),
		closers: []io.Closer{stream},
	}
}

func handleStreamError(w http.ResponseWriter, err error) {
	if importer.IsNoCapacityError(err) {
		w.WriteHeader(http.StatusBadRequest)
		err = fmt.Errorf("effective image size is larger than the reported available storage: %w", err)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	klog.Errorf("Saving stream failed: %s", err)

	_, writeErr := fmt.Fprintf(w, "Saving stream failed: %s", err.Error())
	if writeErr != nil {
		klog.Errorf("failed to send response; %v", err)
	}
}
