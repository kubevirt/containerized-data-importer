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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/importer"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
	cryptowatch "kubevirt.io/containerized-data-importer/pkg/util/tls-crypto-watch"
)

func newServer() *uploadServerApp {
	config := &Config{
		Insecure:           true,
		BindAddress:        "127.0.0.1",
		BindPort:           0,
		Destination:        "disk.img",
		FilesystemOverhead: 0.055,
		CryptoConfig:       *cryptowatch.DefaultCryptoConfig(),
	}
	server := NewUploadServer(config)
	return server.(*uploadServerApp)
}

func newTLSServer(clientCertName, expectedName string) (*uploadServerApp, *triple.KeyPair, *x509.Certificate, func()) {
	dir, err := os.MkdirTemp("", "tls")
	Expect(err).ToNot(HaveOccurred())

	cleanup := func() {
		os.RemoveAll(dir)
	}

	serverCA, err := triple.NewCA("server")
	Expect(err).ToNot(HaveOccurred())

	clientCA, err := triple.NewCA("client")
	Expect(err).ToNot(HaveOccurred())

	serverKeyPair, err := triple.NewServerKeyPair(serverCA, "localhost", "localhost", "default", "local", []string{"127.0.0.1"}, []string{"localhost"})
	Expect(err).ToNot(HaveOccurred())

	_ = os.WriteFile(filepath.Join(dir, "tls.key"), cert.EncodePrivateKeyPEM(serverKeyPair.Key), 0600)
	_ = os.WriteFile(filepath.Join(dir, "tls.crt"), cert.EncodeCertPEM(serverKeyPair.Cert), 0600)
	_ = os.WriteFile(filepath.Join(dir, "client.crt"), cert.EncodeCertPEM(clientCA.Cert), 0600)

	config := &Config{
		BindAddress:        "127.0.0.1",
		BindPort:           0,
		Destination:        "disk.img",
		ServerKeyFile:      filepath.Join(dir, "tls.key"),
		ServerCertFile:     filepath.Join(dir, "tls.crt"),
		ClientCertFile:     filepath.Join(dir, "client.crt"),
		ClientName:         expectedName,
		FilesystemOverhead: 0.055,
		CryptoConfig:       *cryptowatch.DefaultCryptoConfig(),
	}

	server := NewUploadServer(config).(*uploadServerApp)

	clientKeyPair, err := triple.NewClientKeyPair(clientCA, clientCertName, []string{})
	Expect(err).ToNot(HaveOccurred())

	return server, clientKeyPair, serverCA.Cert, cleanup
}

func newHTTPClient(clientKeyPair *triple.KeyPair, serverCACert *x509.Certificate) *http.Client {
	clientCert, err := tls.X509KeyPair(cert.EncodeCertPEM(clientKeyPair.Cert), cert.EncodePrivateKeyPEM(clientKeyPair.Key))
	Expect(err).ToNot(HaveOccurred())

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(cert.EncodeCertPEM(serverCACert))

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	return client
}

func saveProcessorSuccess(stream io.ReadCloser, dest, imageSize string, filesystemOverhead float64, preallocation bool, contentType string, dvContentType cdiv1.DataVolumeContentType) (bool, error) {
	return false, nil
}

func saveProcessorFailure(stream io.ReadCloser, dest, imageSize string, filesystemOverhead float64, preallocation bool, contentType string, dvContentType cdiv1.DataVolumeContentType) (bool, error) {
	return false, fmt.Errorf("Error using datastream")
}

func withProcessorSuccess(f func()) {
	replaceProcessorFunc(saveProcessorSuccess, f)
}

func withProcessorFailure(f func()) {
	replaceProcessorFunc(saveProcessorFailure, f)
}

func replaceProcessorFunc(replacement func(io.ReadCloser, string, string, float64, bool, string, cdiv1.DataVolumeContentType) (bool, error), f func()) {
	origProcessorFunc := uploadProcessorFunc
	uploadProcessorFunc = replacement
	defer func() {
		uploadProcessorFunc = origProcessorFunc
	}()
	f()
}

type AsyncMockDataSource struct {
}

// Info is called to get initial information about the data.
func (amd *AsyncMockDataSource) Info() (importer.ProcessingPhase, error) {
	return importer.ProcessingPhaseTransferDataFile, nil
}

// Transfer is called to transfer the data from the source to the passed in path.
func (amd *AsyncMockDataSource) Transfer(path string, preallocation bool) (importer.ProcessingPhase, error) {
	return importer.ProcessingPhasePause, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (amd *AsyncMockDataSource) TransferFile(fileName string, preallocation bool) (importer.ProcessingPhase, error) {
	return importer.ProcessingPhasePause, nil
}

// Process is called to do any special processing before giving the url to the data back to the processor
func (amd *AsyncMockDataSource) Process() (importer.ProcessingPhase, error) {
	return importer.ProcessingPhaseConvert, nil
}

// Close closes any readers or other open resources.
func (amd *AsyncMockDataSource) Close() error {
	return nil
}

// GetURL returns the url that the data processor can use when converting the data.
func (amd *AsyncMockDataSource) GetURL() *url.URL {
	return nil
}

// GetTerminationMessage returns data to be serialized and used as the termination message of the importer.
func (amd *AsyncMockDataSource) GetTerminationMessage() *common.TerminationMessage {
	return nil
}

// GetResumePhase returns the next phase to process when resuming
func (amd *AsyncMockDataSource) GetResumePhase() importer.ProcessingPhase {
	return importer.ProcessingPhaseComplete
}

func saveAsyncProcessorSuccess(stream io.ReadCloser, dest, imageSize string, filesystemOverhead float64, preallocation bool, contentType string) (*importer.DataProcessor, error) {
	return importer.NewDataProcessor(&AsyncMockDataSource{}, "", "", "", "", 0.055, false, ""), nil
}

func saveAsyncProcessorFailure(stream io.ReadCloser, dest, imageSize string, filesystemOverhead float64, preallocation bool, contentType string) (*importer.DataProcessor, error) {
	return importer.NewDataProcessor(&AsyncMockDataSource{}, "", "", "", "", 0.055, false, ""), fmt.Errorf("Error using datastream")
}

func withAsyncProcessorSuccess(f func()) {
	replaceAsyncProcessorFunc(saveAsyncProcessorSuccess, f)
}

func withAsyncProcessorFailure(f func()) {
	replaceAsyncProcessorFunc(saveAsyncProcessorFailure, f)
}

func replaceAsyncProcessorFunc(replacement func(io.ReadCloser, string, string, float64, bool, string) (*importer.DataProcessor, error), f func()) {
	origProcessorFuncAsync := uploadProcessorFuncAsync
	uploadProcessorFuncAsync = replacement
	defer func() {
		uploadProcessorFuncAsync = origProcessorFuncAsync
	}()
	f()
}

var _ = Describe("Upload server tests", func() {
	It("GET fails", func() {
		withProcessorSuccess(func() {
			req, err := http.NewRequest(http.MethodGet, common.UploadPathSync, nil)
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()

			server := newServer()
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusNotFound))
		})
	})

	It("healthz", func() {
		req, err := http.NewRequest(http.MethodGet, healthzPath, nil)
		Expect(err).ToNot(HaveOccurred())

		rr := httptest.NewRecorder()

		server := newServer()
		server.ServeHTTP(rr, req)

		status := rr.Code
		Expect(status).To(Equal(http.StatusOK))

	})

	DescribeTable("Process unavailable", func(uploadPath string) {
		withProcessorSuccess(func() {
			req, err := http.NewRequest(http.MethodPost, uploadPath, strings.NewReader("data"))
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()

			server := newServer()
			server.uploading = true
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusServiceUnavailable))
		})
	},
		Entry("async", common.UploadPathAsync),
		Entry("sync", common.UploadPathSync),
		Entry("archive", common.UploadArchivePath),
		Entry("form async", common.UploadFormAsync),
		Entry("form sync", common.UploadFormSync),
	)

	DescribeTable("Completion conflict", func(uploadPath string) {
		withAsyncProcessorSuccess(func() {
			req, err := http.NewRequest(http.MethodPost, uploadPath, strings.NewReader("data"))
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()

			server := newServer()
			server.done = true
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusConflict))
		})
	},
		Entry("async", common.UploadPathAsync),
		Entry("sync", common.UploadPathSync),
		Entry("archive", common.UploadArchivePath),
		Entry("form async", common.UploadFormAsync),
		Entry("form sync", common.UploadFormSync),
	)

	It("Success", func() {
		withProcessorSuccess(func() {
			req, err := http.NewRequest(http.MethodPost, common.UploadPathSync, strings.NewReader("data"))
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()

			server := newServer()
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusOK))
		})
	})

	DescribeTable("Success, async", func(method string) {
		withAsyncProcessorSuccess(func() {
			req, err := http.NewRequest(method, common.UploadPathAsync, strings.NewReader("data"))
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()

			server := newServer()
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusOK))
		})
	},
		Entry("POST", "POST"),
		Entry("HEAD", "HEAD"),
	)

	DescribeTable("Success, form", func(processorFunc func(func()), path string) {
		processorFunc(func() {
			req := newFormRequest(path)
			rr := httptest.NewRecorder()

			server := newServer()
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusOK))
		})
	},
		Entry("Sync", withProcessorSuccess, common.UploadFormSync),
		Entry("Async", withAsyncProcessorSuccess, common.UploadFormAsync),
	)

	DescribeTable("Stream fail", func(processorFunc func(func()), uploadPath string) {
		processorFunc(func() {
			req, err := http.NewRequest(http.MethodPost, uploadPath, strings.NewReader("data"))
			Expect(err).ToNot(HaveOccurred())

			rr := httptest.NewRecorder()

			server := newServer()
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusInternalServerError))
		})
	},
		Entry("async", withAsyncProcessorFailure, common.UploadPathAsync),
		Entry("sync", withProcessorFailure, common.UploadPathSync),
		Entry("archive", withProcessorFailure, common.UploadArchivePath),
	)

	DescribeTable("Stream fail form", func(processorFunc func(func()), uploadPath string) {
		processorFunc(func() {
			req := newFormRequest(uploadPath)
			rr := httptest.NewRecorder()

			server := newServer()
			server.ServeHTTP(rr, req)

			status := rr.Code
			Expect(status).To(Equal(http.StatusInternalServerError))
		})
	},
		Entry("async", withAsyncProcessorFailure, common.UploadFormAsync),
		Entry("sync", withProcessorFailure, common.UploadFormSync),
	)

	DescribeTable("Real upload with client", func(certName string, expectedName string, expectedResponse int) {
		withProcessorSuccess(func() {
			server, clientKeyPair, serverCACert, cleanup := newTLSServer(certName, expectedName)
			defer cleanup()

			client := newHTTPClient(clientKeyPair, serverCACert)

			ch := make(chan struct{})

			go func() {
				_, _ = server.Run()
				close(ch)
			}()

			for i := 0; i < 10; i++ {
				if server.config.BindPort != 0 {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}

			Expect(server.config.BindPort).ToNot(Equal(0))

			url := fmt.Sprintf("https://localhost:%d%s", server.config.BindPort, common.UploadPathSync)
			stringReader := strings.NewReader("nothing")

			resp, err := client.Post(url, "application/x-www-form-urlencoded", stringReader)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(expectedResponse))

			if !server.done {
				close(server.doneChan)
			}

			<-ch
		})
	},
		Entry("Valid data", "client", "client", 200),
		Entry("Invalid data", "foo", "bar", 401),
	)

	It("should handle deadline", func() {
		server, _, _, cleanup := newTLSServer("client", "client")
		defer cleanup()

		now := time.Now()
		server.config.Deadline = &now

		var runResult *RunResult
		var err error
		ch := make(chan struct{})

		go func() {
			runResult, err = server.Run()
			close(ch)
		}()

		select {
		case <-ch:
			Expect(err).ToNot(HaveOccurred())
			Expect(runResult.DeadlinePassed).To(BeTrue())
		case <-time.After(10 * time.Second):
			Fail("Timed out waiting for server to exit")
		}
	})
})

func newFormRequest(path string) *http.Request {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	data := strings.NewReader("data")

	fw, err := w.CreateFormFile("file", "myimage.img")
	Expect(err).ToNot(HaveOccurred())

	_, err = io.Copy(fw, data)
	Expect(err).ToNot(HaveOccurred())
	err = w.Close()
	Expect(err).ToNot(HaveOccurred())

	req, err := http.NewRequest(http.MethodPost, path, &b)
	Expect(err).ToNot(HaveOccurred())

	req.Header.Set("Content-Type", w.FormDataContentType())

	return req
}
