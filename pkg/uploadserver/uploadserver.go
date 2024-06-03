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
	healthzPort = 8080
	healthzPath = "/healthz"
)

type Config struct {
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

func isCloneTraget(contentType string) bool {
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

	healthzServer, err := app.createHealthzServer()
	if err != nil {
		return nil, errors.Wrap(err, "Error creating healthz http server")
	}

	uploadListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", app.config.BindAddress, app.config.BindPort))
	if err != nil {
		return nil, errors.Wrap(err, "Error creating upload listerner")
	}

	healthzListener, err := net.Listen("tcp", fmt.Sprintf(":%d", healthzPort))
	if err != nil {
		return nil, errors.Wrap(err, "Error creating healthz listerner")
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

	go func() {
		defer healthzServer.Close()

		app.errChan <- healthzServer.Serve(healthzListener)
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
		if err := healthzServer.Shutdown(context.Background()); err != nil {
			klog.Errorf("failed to shutdown healthzServer; %v", err)
		}
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
		return nil, nil
	}

	//nolint:gosec // False positive: Min version is not known statically
	config := &tls.Config{
		CipherSuites: app.config.CryptoConfig.CipherSuites,
		ClientAuth:   tls.RequireAndVerifyClientCert,
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

func (app *uploadServerApp) createHealthzServer() (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc(healthzPath, app.healthzHandler)
	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}, nil
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

		if err != nil {
			klog.Errorf("Saving stream failed: %s", err)
			if errors.As(err, &importer.ValidationSizeError{}) {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

			_, writeErr := fmt.Fprintf(w, "Saving stream failed: %s", err.Error())
			if writeErr != nil {
				klog.Errorf("failed to send response; %v", err)
			}

			app.uploading = false
			return
		}

		app.uploading = false
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
			app.cloneTarget = isCloneTraget(cdiContentType)
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
		klog.Errorf("Saving stream failed: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	app.done = true
	app.preallocationApplied = preallocationApplied
	app.cloneTarget = isCloneTraget(cdiContentType)
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
	if sourceContentType == common.FilesystemCloneContentType {
		return nil, fmt.Errorf("async filesystem clone not supported")
	}

	uds := importer.NewAsyncUploadDataSource(newContentReader(stream, sourceContentType))
	processor := importer.NewDataProcessor(uds, dest, common.ImporterVolumePath, common.ScratchDataDir, imageSize, filesystemOverhead, preallocation, "")
	return processor, processor.ProcessDataWithPause()
}

func newUploadStreamProcessor(stream io.ReadCloser, dest, imageSize string, filesystemOverhead float64, preallocation bool, sourceContentType string, dvContentType cdiv1.DataVolumeContentType) (bool, error) {
	if sourceContentType == common.FilesystemCloneContentType {
		return false, filesystemCloneProcessor(stream, dest)
	}

	// Clone block device to block device or file system
	uds := importer.NewUploadDataSource(newContentReader(stream, sourceContentType), dvContentType)
	processor := importer.NewDataProcessor(uds, dest, common.ImporterVolumePath, common.ScratchDataDir, imageSize, filesystemOverhead, preallocation, "")
	err := processor.ProcessData()
	return processor.PreallocationApplied(), err
}

// Clone file system to block device or file system
func filesystemCloneProcessor(stream io.ReadCloser, dest string) error {
	// Clone to block device
	if dest == common.WriteBlockPath {
		if err := untarToBlockdev(newSnappyReadCloser(stream), dest); err != nil {
			return errors.Wrapf(err, "error unarchiving to %s", dest)
		}
		return nil
	}

	// Clone to file system
	destDir := common.ImporterVolumePath
	if err := util.UnArchiveTar(newSnappyReadCloser(stream), destDir); err != nil {
		return errors.Wrapf(err, "error unarchiving to %s", destDir)
	}
	return nil
}

func untarToBlockdev(stream io.Reader, dest string) error {
	tr := tar.NewReader(stream)
	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}
		if !strings.Contains(header.Name, common.DiskImageName) {
			continue
		}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeGNUSparse:
			klog.Infof("Untaring %d bytes to %s", header.Size, dest)
			f, err := os.OpenFile(dest, os.O_APPEND|os.O_WRONLY, os.ModeDevice|os.ModePerm)
			if err != nil {
				return err
			}
			written, err := io.CopyN(f, tr, header.Size)
			if err != nil {
				return err
			}
			klog.Infof("Written %d", written)
			f.Close()
			return nil
		}
	}
}

func newContentReader(stream io.ReadCloser, contentType string) io.ReadCloser {
	if contentType == common.BlockdeviceClone {
		return newSnappyReadCloser(stream)
	}

	return stream
}

func newSnappyReadCloser(stream io.ReadCloser) io.ReadCloser {
	return io.NopCloser(snappy.NewReader(stream))
}
