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
package image

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConvertQcow2ToRaw(t *testing.T) {
	type args struct {
		src  string
		dest string
	}
	// for local tests use repo path
	// for dockerized tests use depFile name
	// also having an issue with the makefile unit tests
	// if running in docker it all works fine
	imageDir, _ := filepath.Abs("../../tests/images")
	goodImage := filepath.Join(imageDir, "cirros-qcow2.img")
	badImage := filepath.Join(imageDir, "tinyCore.iso")
	if _, err := os.Stat(goodImage); os.IsNotExist(err) {
		goodImage = "cirros-qcow2.img"
	}
	if _, err := os.Stat(badImage); os.IsNotExist(err) {
		badImage = "tinyCore.iso"
	}

	defer os.Remove("/tmp/cirros-test-good")
	defer os.Remove("/tmp/cirros-test-bad")

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "convert qcow2 image to Raw",
			args:    args{goodImage, filepath.Join(os.TempDir(), "cirros-test-good")},
			wantErr: false,
		},
		{
			name:    "failed to convert non qcow2 image to Raw",
			args:    args{badImage, filepath.Join(os.TempDir(), "cirros-test-bad")},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertQcow2ToRaw(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("ConvertQcow2ToRaw() error = %v, wantErr %v %v", err, tt.wantErr, imageDir)
			}
		})
	}
}

func TestConvertQcow2ToRawStream(t *testing.T) {
	type args struct {
		src  *url.URL
		dest string
	}
	httpPort := 8080
	imageDir, _ := filepath.Abs("../../test/images")
	// adapted from above test not totally sure necessary
	if _, err := os.Stat(imageDir); os.IsNotExist(err) {
		imageDir = "./"
	}

	server := startHTTPServer(httpPort, imageDir)

	defer server.Shutdown(nil)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "convert qcow2 image to Raw",
			args:    args{toURL(httpPort, "cirros-qcow2.img"), tempFile("cirros-test-good")},
			wantErr: false,
		},
		{
			name:    "failed to convert non qcow2 image to Raw",
			args:    args{toURL(httpPort, "tinyCore.iso"), tempFile("cirros-test-bad")},
			wantErr: true,
		},
		{
			name:    "failed to convert invalid qcow2 image to Raw",
			args:    args{toURL(httpPort, "cirros-snapshot-qcow2.img"), tempFile("cirros-snapshot-test-bad")},
			wantErr: true,
		},
		{
			name:    "failed to convert non-existing qcow2 image to Raw",
			args:    args{toURL(httpPort, "foobar.img"), tempFile("foobar-test-bad")},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertQcow2ToRawStream(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("ConvertQcow2ToRawStream() error = %v, wantErr %v %v", err, tt.wantErr, imageDir)
			}
		})
		os.Remove(tt.args.dest)
	}
}

func toURL(port int, fileName string) (result *url.URL) {
	result, _ = url.Parse(fmt.Sprintf("http://localhost:%d/%s", port, fileName))
	return
}

func tempFile(fileName string) string {
	return filepath.Join(os.TempDir(), fileName)
}

func startHTTPServer(port int, dir string) *http.Server {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.FileServer(http.Dir(dir)),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			// cannot panic, because this probably is an intentional close
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	for i := 0; i < 10; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		resp.Body.Close()
		break
	}

	return server
}
