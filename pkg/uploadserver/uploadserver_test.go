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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

func newServer() *uploadServerApp {
	server := NewUploadServer("127.0.0.1", 0, "disk.img", "", "", "", "", "")
	return server.(*uploadServerApp)
}

func newTLSServer(t *testing.T, clientCertName, expectedName string) (*uploadServerApp, *triple.KeyPair, *x509.Certificate) {
	serverCA, err := triple.NewCA("server")
	if err != nil {
		t.Error("Error creating CA")
	}

	clientCA, err := triple.NewCA("client")
	if err != nil {
		t.Error("Error creating CA")
	}

	serverKeyPair, err := triple.NewServerKeyPair(serverCA, "localhost", "localhost", "default", "local", []string{"127.0.0.1"}, []string{"localhost"})
	if err != nil {
		t.Error("Error creating server cert")
	}

	tlsKey := string(cert.EncodePrivateKeyPEM(serverKeyPair.Key))
	tlsCert := string(cert.EncodeCertPEM(serverKeyPair.Cert))
	clientCert := string(cert.EncodeCertPEM(clientCA.Cert))

	server := NewUploadServer("127.0.0.1", 0, "disk.img", tlsKey, tlsCert, clientCert, expectedName, "").(*uploadServerApp)

	clientKeyPair, err := triple.NewClientKeyPair(clientCA, clientCertName, []string{})
	if err != nil {
		t.Error("Error creating client cert")
	}

	return server, clientKeyPair, serverCA.Cert
}

func newHTTPClient(t *testing.T, clientKeyPair *triple.KeyPair, serverCACert *x509.Certificate) *http.Client {
	clientCert, err := tls.X509KeyPair(cert.EncodeCertPEM(clientKeyPair.Cert), cert.EncodePrivateKeyPEM(clientKeyPair.Key))
	if err != nil {
		t.Error("Could not create tls.Certificate")
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(cert.EncodeCertPEM(serverCACert))

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	return client
}

func newRequest(t *testing.T) *http.Request {
	req, err := http.NewRequest("POST", common.UploadPath, strings.NewReader("data"))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func saveProcessorSuccess(stream io.ReadCloser, dest, imageSize, contentType string) error {
	return nil
}

func saveProcessorFailure(stream io.ReadCloser, dest, imageSize, contentType string) error {
	return fmt.Errorf("Error using datastream")
}

func withProcessorSuccess(f func()) {
	replaceProcessorFunc(saveProcessorSuccess, f)
}

func withProcessorFailure(f func()) {
	replaceProcessorFunc(saveProcessorFailure, f)
}

func replaceProcessorFunc(replacement func(io.ReadCloser, string, string, string) error, f func()) {
	origProcessorFunc := uploadProcessorFunc
	uploadProcessorFunc = replacement
	defer func() {
		uploadProcessorFunc = origProcessorFunc
	}()
	f()
}

func TestGetFails(t *testing.T) {
	withProcessorSuccess(func() {
		req, err := http.NewRequest("GET", common.UploadPath, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()

		server := newServer()
		server.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusNotFound {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusNotFound)
		}
	})
}

func TestHealthz(t *testing.T) {
	req, err := http.NewRequest("GET", healthzPath, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	app := uploadServerApp{}
	server, _ := app.createHealthzServer()
	server.Handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestInProcessUnavailable(t *testing.T) {
	withProcessorSuccess(func() {
		req := newRequest(t)

		rr := httptest.NewRecorder()

		server := newServer()
		server.uploading = true
		server.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusServiceUnavailable {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusServiceUnavailable)
		}
	})
}

func TestCompletedConflict(t *testing.T) {
	withProcessorSuccess(func() {
		req := newRequest(t)

		rr := httptest.NewRecorder()

		server := newServer()
		server.done = true
		server.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusConflict {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusConflict)
		}
	})
}

func TestSuccess(t *testing.T) {
	withProcessorSuccess(func() {
		req := newRequest(t)

		rr := httptest.NewRecorder()

		server := newServer()
		server.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	})
}

func TestStreamFail(t *testing.T) {
	withProcessorFailure(func() {
		req := newRequest(t)

		rr := httptest.NewRecorder()

		server := newServer()
		server.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusInternalServerError {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusInternalServerError)
		}
	})
}

func TestRealUploadWithClient(t *testing.T) {
	type testData struct {
		certName, expectedName string
		expectedResponse       int
	}
	for _, data := range []testData{
		{
			certName:         "client",
			expectedName:     "client",
			expectedResponse: 200,
		},
		{
			certName:         "foo",
			expectedName:     "bar",
			expectedResponse: 401,
		},
	} {
		withProcessorSuccess(func() {
			server, clientKeyPair, serverCACert := newTLSServer(t, data.certName, data.expectedName)

			client := newHTTPClient(t, clientKeyPair, serverCACert)

			ch := make(chan struct{})

			go func() {
				server.Run()
				close(ch)
			}()

			for i := 0; i < 10; i++ {
				if server.bindPort != 0 {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}

			if server.bindPort == 0 {
				t.Error("Couldn't start http server")
			}

			url := fmt.Sprintf("https://localhost:%d%s", server.bindPort, common.UploadPath)
			stringReader := strings.NewReader("nothing")

			resp, err := client.Post(url, "application/x-www-form-urlencoded", stringReader)
			if err != nil {
				t.Errorf("Request failed %+v", err)
			}

			if resp.StatusCode != data.expectedResponse {
				t.Errorf("Unexpected status code %d wanted %d", resp.StatusCode, data.expectedResponse)
			}

			if !server.done {
				close(server.doneChan)
			}

			<-ch
		})
	}
}
