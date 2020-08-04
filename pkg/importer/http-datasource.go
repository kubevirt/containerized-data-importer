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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	tempFile = "tmpimage"
)

// HTTPDataSource is the data provider for http(s) endpoints.
// Sequence of phases:
// 1a. Info -> Convert (In Info phase the format readers are configured), if the source Reader image is not archived, and no custom CA is used, and can be converted by QEMU-IMG (RAW/QCOW2)
// 1b. Info -> TransferArchive if the content type is archive
// 1c. Info -> Transfer in all other cases.
// 2a. Transfer -> Process if content type is kube virt
// 2b. Transfer -> Complete if content type is archive (Transfer is called with the target instead of the scratch space). Non block PVCs only.
// 3. Process -> Convert
type HTTPDataSource struct {
	httpReader io.ReadCloser
	ctx        context.Context
	cancel     context.CancelFunc
	cancelLock sync.Mutex
	// content type expected by the to live on the endpoint.
	contentType cdiv1.DataVolumeContentType
	// stack of readers
	readers *FormatReaders
	// endpoint the http endpoint to retrieve the data from.
	endpoint *url.URL
	// url the url to report to the caller of getURL, could be the endpoint, or a file in scratch space.
	url *url.URL
	// true if we are using a custom CA (and thus have to use scratch storage)
	customCA bool
	// true if we know `qemu-img` will fail to download this
	brokenForQemuImg bool
	// the content length reported by the http server.
	contentLength uint64
}

// NewHTTPDataSource creates a new instance of the http data provider.
func NewHTTPDataSource(endpoint, accessKey, secKey, certDir string, contentType cdiv1.DataVolumeContentType) (*HTTPDataSource, error) {
	ep, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", endpoint))
	}
	ctx, cancel := context.WithCancel(context.Background())
	httpReader, contentLength, brokenForQemuImg, err := createHTTPReader(ctx, ep, accessKey, secKey, certDir)
	if err != nil {
		cancel()
		return nil, err
	}

	if accessKey != "" && secKey != "" {
		ep.User = url.UserPassword(accessKey, secKey)
	}
	httpSource := &HTTPDataSource{
		ctx:              ctx,
		cancel:           cancel,
		httpReader:       httpReader,
		contentType:      contentType,
		endpoint:         ep,
		customCA:         certDir != "",
		brokenForQemuImg: brokenForQemuImg,
		contentLength:    contentLength,
	}
	// We know this is a counting reader, so no need to check.
	countingReader := httpReader.(*util.CountingReader)
	go httpSource.pollProgress(countingReader, 10*time.Minute, time.Second)
	return httpSource, nil
}

// Info is called to get initial information about the data.
func (hs *HTTPDataSource) Info() (ProcessingPhase, error) {
	var err error
	hs.readers, err = NewFormatReaders(hs.httpReader, hs.contentLength)
	if hs.contentType == cdiv1.DataVolumeArchive {
		return ProcessingPhaseTransferDataDir, nil
	}
	if err != nil {
		klog.Errorf("Error creating readers: %v", err)
		return ProcessingPhaseError, err
	}
	// The readers now contain all the information needed to determine if we can stream directly or if we need scratch space to download
	// the file to, before converting.
	if !hs.readers.Archived && !hs.customCA && !hs.brokenForQemuImg && hs.readers.Convert {
		// We can pass straight to conversion from the endpoint. No scratch required.
		hs.url = hs.endpoint
		return ProcessingPhaseConvert, nil
	}
	if !hs.readers.Convert {
		return ProcessingPhaseTransferDataFile, nil
	}
	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source to a scratch location.
func (hs *HTTPDataSource) Transfer(path string) (ProcessingPhase, error) {
	if hs.contentType == cdiv1.DataVolumeKubeVirt {
		size, err := util.GetAvailableSpace(path)
		if size <= int64(0) {
			//Path provided is invalid.
			return ProcessingPhaseError, ErrInvalidPath
		}
		file := filepath.Join(path, tempFile)
		err = util.StreamDataToFile(hs.readers.TopReader(), file)
		if err != nil {
			return ProcessingPhaseError, err
		}
		// If we successfully wrote to the file, then the parse will succeed.
		hs.url, _ = url.Parse(file)
		return ProcessingPhaseProcess, nil
	} else if hs.contentType == cdiv1.DataVolumeArchive {
		if err := util.UnArchiveTar(hs.readers.TopReader(), path); err != nil {
			return ProcessingPhaseError, errors.Wrap(err, "unable to untar files from endpoint")
		}
		hs.url = nil
		return ProcessingPhaseComplete, nil
	}
	return ProcessingPhaseError, errors.Errorf("Unknown content type: %s", hs.contentType)
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (hs *HTTPDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	hs.readers.StartProgressUpdate()
	err := util.StreamDataToFile(hs.readers.TopReader(), fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseResize, nil
}

// Process is called to do any special processing before giving the URI to the data back to the processor
func (hs *HTTPDataSource) Process() (ProcessingPhase, error) {
	return ProcessingPhaseConvert, nil
}

// GetURL returns the URI that the data processor can use when converting the data.
func (hs *HTTPDataSource) GetURL() *url.URL {
	return hs.url
}

// Close all readers.
func (hs *HTTPDataSource) Close() error {
	var err error
	if hs.readers != nil {
		err = hs.readers.Close()
	}
	hs.cancelLock.Lock()
	if hs.cancel != nil {
		hs.cancel()
		hs.cancel = nil
	}
	hs.cancelLock.Unlock()
	return err
}

func createHTTPClient(certDir string) (*http.Client, error) {
	client := &http.Client{
		// Don't set timeout here, since that will be an absolute timeout, we need a relative to last progress timeout.
	}

	if certDir == "" {
		return client, nil
	}

	// let's get system certs as well
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.Wrap(err, "Error getting system certs")
	}

	files, err := ioutil.ReadDir(certDir)
	if err != nil {
		return nil, errors.Wrapf(err, "Error listing files in %s", certDir)
	}

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

		if ok := certPool.AppendCertsFromPEM(certs); !ok {
			klog.Warningf("No certs in %s", fp)
		}
	}

	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
	}

	return client, nil
}

func createHTTPReader(ctx context.Context, ep *url.URL, accessKey, secKey, certDir string) (io.ReadCloser, uint64, bool, error) {
	var brokenForQemuImg bool
	client, err := createHTTPClient(certDir)
	if err != nil {
		return nil, uint64(0), false, errors.Wrap(err, "Error creating http client")
	}

	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(accessKey) > 0 && len(secKey) > 0 {
			r.SetBasicAuth(accessKey, secKey) // Redirects will lose basic auth, so reset them manually
		}
		return nil
	}

	total, err := getContentLength(client, ep, accessKey, secKey)
	if err != nil {
		brokenForQemuImg = true
	}
	// http.NewRequest can only return error on invalid METHOD, or invalid url. Here the METHOD is always GET, and the url is always valid, thus error cannot happen.
	req, _ := http.NewRequest("GET", ep.String(), nil)

	req = req.WithContext(ctx)
	if len(accessKey) > 0 && len(secKey) > 0 {
		req.SetBasicAuth(accessKey, secKey)
	}
	klog.V(2).Infof("Attempting to get object %q via http client\n", ep.String())
	resp, err := client.Do(req)
	if err != nil {
		return nil, uint64(0), true, errors.Wrap(err, "HTTP request errored")
	}
	if resp.StatusCode != 200 {
		klog.Errorf("http: expected status code 200, got %d", resp.StatusCode)
		return nil, uint64(0), true, errors.Errorf("expected status code 200, got %d. Status: %s", resp.StatusCode, resp.Status)
	}

	acceptRanges, ok := resp.Header["Accept-Ranges"]
	if !ok || acceptRanges[0] == "none" {
		klog.V(2).Infof("Accept-Ranges isn't bytes, avoiding qemu-img")
		brokenForQemuImg = true
	}

	if total == 0 {
		// The total seems bogus. Let's try the GET Content-Length header
		total = parseHTTPHeader(resp)
	}
	countingReader := &util.CountingReader{
		Reader:  resp.Body,
		Current: 0,
	}
	return countingReader, total, brokenForQemuImg, nil
}

func (hs *HTTPDataSource) pollProgress(reader *util.CountingReader, idleTime, pollInterval time.Duration) {
	count := reader.Current
	lastUpdate := time.Now()
	for {
		if count < reader.Current {
			// Some progress was made, reset now.
			lastUpdate = time.Now()
			count = reader.Current
		}

		if time.Until(lastUpdate.Add(idleTime)).Nanoseconds() < 0 {
			hs.cancelLock.Lock()
			if hs.cancel != nil {
				// No progress for the idle time, cancel http client.
				hs.cancel() // This will trigger dp.ctx.Done()
			}
			hs.cancelLock.Unlock()
		}
		select {
		case <-time.After(pollInterval):
			continue
		case <-hs.ctx.Done():
			return // Don't leak, once the transfer is cancelled or completed this is called.
		}
	}
}

func getContentLength(client *http.Client, ep *url.URL, accessKey, secKey string) (uint64, error) {
	req, err := http.NewRequest("HEAD", ep.String(), nil)
	if err != nil {
		return uint64(0), errors.Wrap(err, "could not create HTTP request")
	}
	if len(accessKey) > 0 && len(secKey) > 0 {
		req.SetBasicAuth(accessKey, secKey)
	}

	klog.V(2).Infof("Attempting to HEAD %q via http client\n", ep.String())
	resp, err := client.Do(req)
	if err != nil {
		return uint64(0), errors.Wrap(err, "HTTP request errored")
	}

	if resp.StatusCode != 200 {
		klog.Errorf("http: expected status code 200, got %d", resp.StatusCode)
		return uint64(0), errors.Errorf("expected status code 200, got %d. Status: %s", resp.StatusCode, resp.Status)
	}

	for k, v := range resp.Header {
		klog.V(3).Infof("GO CLIENT: key: %s, value: %s\n", k, v)
	}

	total := parseHTTPHeader(resp)

	err = resp.Body.Close()
	if err != nil {
		return uint64(0), errors.Wrap(err, "could not close head read")
	}
	return total, nil
}

func parseHTTPHeader(resp *http.Response) uint64 {
	var err error
	total := uint64(0)
	if val, ok := resp.Header["Content-Length"]; ok {
		total, err = strconv.ParseUint(val[0], 10, 64)
		if err != nil {
			klog.Errorf("could not convert content length, got %v", err)
		}
		klog.V(3).Infof("Content length: %d\n", total)
	}

	return total
}
