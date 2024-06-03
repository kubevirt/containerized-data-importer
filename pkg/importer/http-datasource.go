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
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	tempFile          = "tmpimage"
	nbdkitPid         = "/tmp/nbdkit.pid"
	nbdkitSocket      = "/tmp/nbdkit.sock"
	defaultUserAgent  = "cdi-golang-importer"
	httpContentType   = "Content-Type"
	httpContentLength = "Content-Length"
)

// HTTPDataSource is the data provider for http(s) endpoints.
// Sequence of phases:
// 1a. Info -> Convert (In Info phase the format readers are configured), if the source Reader image is not archived, and no custom CA is used, and can be converted by QEMU-IMG (RAW/QCOW2)
// 1b. Info -> TransferArchive if the content type is archive
// 1c. Info -> Transfer in all other cases.
// 2a. Transfer -> Convert if content type is kube virt
// 2b. Transfer -> Complete if content type is archive (Transfer is called with the target instead of the scratch space). Non block PVCs only.
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
	// path to the custom CA. Empty if not used
	customCA string
	// true if we know `qemu-img` will fail to download this
	brokenForQemuImg bool
	// the content length reported by the http server.
	contentLength uint64

	n image.NbdkitOperation
}

var createNbdkitCurl = image.NewNbdkitCurl

// NewHTTPDataSource creates a new instance of the http data provider.
func NewHTTPDataSource(endpoint, accessKey, secKey, certDir string, contentType cdiv1.DataVolumeContentType) (*HTTPDataSource, error) {
	ep, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", endpoint))
	}
	ctx, cancel := context.WithCancel(context.Background())

	extraHeaders, secretExtraHeaders, err := getExtraHeaders()
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "Error getting extra headers for HTTP client")
	}

	httpReader, contentLength, brokenForQemuImg, err := createHTTPReader(ctx, ep, accessKey, secKey, certDir, extraHeaders, secretExtraHeaders, contentType)
	if err != nil {
		cancel()
		return nil, err
	}

	httpSource := &HTTPDataSource{
		ctx:              ctx,
		cancel:           cancel,
		httpReader:       httpReader,
		contentType:      contentType,
		endpoint:         ep,
		customCA:         certDir,
		brokenForQemuImg: brokenForQemuImg,
		contentLength:    contentLength,
	}
	httpSource.n = createNbdkitCurl(nbdkitPid, accessKey, secKey, certDir, nbdkitSocket, extraHeaders, secretExtraHeaders)
	// We know this is a counting reader, so no need to check.
	countingReader := httpReader.(*util.CountingReader)
	go httpSource.pollProgress(countingReader, 10*time.Minute, time.Second)
	return httpSource, nil
}

// Info is called to get initial information about the data.
func (hs *HTTPDataSource) Info() (ProcessingPhase, error) {
	var err error
	hs.readers, err = NewFormatReaders(hs.httpReader, hs.contentLength)
	if err != nil {
		klog.Errorf("Error creating readers: %v", err)
		return ProcessingPhaseError, err
	}
	if hs.contentType == cdiv1.DataVolumeArchive {
		return ProcessingPhaseTransferDataDir, nil
	}
	if pullMethod, _ := util.ParseEnvVar(common.ImporterPullMethod, false); pullMethod == string(cdiv1.RegistryPullNode) {
		hs.url, _ = url.Parse(fmt.Sprintf("nbd+unix:///?socket=%s", nbdkitSocket))
		if err = hs.n.StartNbdkit(hs.endpoint.String()); err != nil {
			return ProcessingPhaseError, err
		}
		return ProcessingPhaseConvert, nil
	}
	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source to a scratch location.
func (hs *HTTPDataSource) Transfer(path string) (ProcessingPhase, error) {
	if hs.contentType == cdiv1.DataVolumeKubeVirt {
		file := filepath.Join(path, tempFile)
		if err := CleanAll(file); err != nil {
			return ProcessingPhaseError, err
		}
		size, err := util.GetAvailableSpace(path)
		if err != nil || size <= 0 {
			return ProcessingPhaseError, ErrInvalidPath
		}
		hs.readers.StartProgressUpdate()
		err = streamDataToFile(hs.readers.TopReader(), file)
		if err != nil {
			return ProcessingPhaseError, err
		}
		// If we successfully wrote to the file, then the parse will succeed.
		hs.url, _ = url.Parse(file)
		return ProcessingPhaseConvert, nil
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
	if err := CleanAll(fileName); err != nil {
		return ProcessingPhaseError, err
	}
	hs.readers.StartProgressUpdate()
	err := streamDataToFile(hs.readers.TopReader(), fileName)
	if err != nil {
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseResize, nil
}

// GetURL returns the URI that the data processor can use when converting the data.
func (hs *HTTPDataSource) GetURL() *url.URL {
	return hs.url
}

// GetTerminationMessage returns data to be serialized and used as the termination message of the importer.
func (hs *HTTPDataSource) GetTerminationMessage() *common.TerminationMessage {
	if pullMethod, _ := util.ParseEnvVar(common.ImporterPullMethod, false); pullMethod != string(cdiv1.RegistryPullNode) {
		return nil
	}

	info, err := getServerInfo(hs.ctx, fmt.Sprintf("%s://%s/info", hs.endpoint.Scheme, hs.endpoint.Host))
	if err != nil {
		klog.Errorf("%+v", err)
		return nil
	}

	return &common.TerminationMessage{
		Labels: envsToLabels(info.Env),
	}
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

func createCertPool(certDir string) (*x509.CertPool, error) {
	// let's get system certs as well
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.Wrap(err, "Error getting system certs")
	}

	// append the user-provided trusted CA certificates bundle when making egress connections using proxy
	if files, err := os.ReadDir(common.ImporterProxyCertDir); err == nil {
		for _, file := range files {
			if file.IsDir() || file.Name()[0] == '.' {
				continue
			}
			fp := path.Join(common.ImporterProxyCertDir, file.Name())
			if certs, err := os.ReadFile(fp); err == nil {
				certPool.AppendCertsFromPEM(certs)
			}
		}
	}

	// append server CA certificates
	files, err := os.ReadDir(certDir)
	if err != nil {
		return nil, errors.Wrapf(err, "Error listing files in %s", certDir)
	}

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

		if ok := certPool.AppendCertsFromPEM(certs); !ok {
			klog.Warningf("No certs in %s", fp)
		}
	}

	return certPool, nil
}

func createHTTPClient(certDir string) (*http.Client, error) {
	client := &http.Client{
		// Don't set timeout here, since that will be an absolute timeout, we need a relative to last progress timeout.
	}

	if certDir == "" {
		return client, nil
	}

	certPool, err := createCertPool(certDir)
	if err != nil {
		return nil, err
	}

	// the default transport contains Proxy configurations to use environment variables and default timeouts
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}
	transport.GetProxyConnectHeader = func(ctx context.Context, proxyURL *url.URL, target string) (http.Header, error) {
		h := http.Header{}
		h.Add("User-Agent", defaultUserAgent)
		return h, nil
	}
	client.Transport = transport

	return client, nil
}

func addExtraheaders(req *http.Request, extraHeaders []string) {
	for _, header := range extraHeaders {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) > 1 {
			req.Header.Add(parts[0], parts[1])
		}
	}
	req.Header.Add("User-Agent", defaultUserAgent)
}

func createHTTPReader(ctx context.Context, ep *url.URL, accessKey, secKey, certDir string, extraHeaders, secretExtraHeaders []string, contentType cdiv1.DataVolumeContentType) (io.ReadCloser, uint64, bool, error) {
	var brokenForQemuImg bool
	client, err := createHTTPClient(certDir)
	if err != nil {
		return nil, uint64(0), false, errors.Wrap(err, "Error creating http client")
	}

	allExtraHeaders := append(extraHeaders, secretExtraHeaders...)

	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(accessKey) > 0 && len(secKey) > 0 {
			r.SetBasicAuth(accessKey, secKey) // Redirects will lose basic auth, so reset them manually
		}
		addExtraheaders(r, allExtraHeaders)
		return nil
	}

	total, err := getContentLength(client, ep, accessKey, secKey, allExtraHeaders)
	if err != nil {
		brokenForQemuImg = true
	}
	// http.NewRequest can only return error on invalid METHOD, or invalid url. Here the METHOD is always GET, and the url is always valid, thus error cannot happen.
	req, _ := http.NewRequest(http.MethodGet, ep.String(), nil)

	addExtraheaders(req, allExtraHeaders)

	req = req.WithContext(ctx)
	if len(accessKey) > 0 && len(secKey) > 0 {
		req.SetBasicAuth(accessKey, secKey)
	}
	klog.V(2).Infof("Attempting to get object %q via http client\n", ep.String())
	resp, err := client.Do(req)
	if err != nil {
		return nil, uint64(0), true, errors.Wrap(err, "HTTP request errored")
	}
	if want := http.StatusOK; resp.StatusCode != want {
		klog.Errorf("http: expected status code %d, got %d", want, resp.StatusCode)
		return nil, uint64(0), true, errors.Errorf("expected status code %d, got %d. Status: %s", want, resp.StatusCode, resp.Status)
	}

	if contentType == cdiv1.DataVolumeKubeVirt {
		// Check the content-type if we are expecting a KubeVirt img.
		if val, ok := resp.Header[httpContentType]; ok {
			if strings.HasPrefix(val[0], "text/") {
				// We will continue with the import nonetheless, but content might be unexpected.
				klog.Warningf("Unexpected content type '%s'. Content might not be a KubeVirt image.", val[0])
			}
		}
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

func getContentLength(client *http.Client, ep *url.URL, accessKey, secKey string, extraHeaders []string) (uint64, error) {
	req, err := http.NewRequest(http.MethodHead, ep.String(), nil)
	if err != nil {
		return uint64(0), errors.Wrap(err, "could not create HTTP request")
	}
	if len(accessKey) > 0 && len(secKey) > 0 {
		req.SetBasicAuth(accessKey, secKey)
	}

	addExtraheaders(req, extraHeaders)

	klog.V(2).Infof("Attempting to HEAD %q via http client\n", ep.String())
	resp, err := client.Do(req)
	if err != nil {
		return uint64(0), errors.Wrap(err, "HTTP request errored")
	}

	if want := http.StatusOK; resp.StatusCode != want {
		klog.Errorf("http: expected status code %d, got %d", want, resp.StatusCode)
		return uint64(0), errors.Errorf("expected status code %d, got %d. Status: %s", want, resp.StatusCode, resp.Status)
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
	if val, ok := resp.Header[httpContentLength]; ok {
		total, err = strconv.ParseUint(val[0], 10, 64)
		if err != nil {
			klog.Errorf("could not convert content length, got %v", err)
		}
		klog.V(3).Infof("Content length: %d\n", total)
	}

	return total
}

// Check for any extra headers to pass along. Return secret headers separately so callers can suppress logging them.
func getExtraHeaders() ([]string, []string, error) {
	extraHeaders := getExtraHeadersFromEnvironment()
	secretExtraHeaders, err := getExtraHeadersFromSecrets()
	return extraHeaders, secretExtraHeaders, err
}

// Check for extra headers from environment variables.
func getExtraHeadersFromEnvironment() []string {
	var extraHeaders []string

	for _, value := range os.Environ() {
		if strings.HasPrefix(value, common.ImporterExtraHeader) {
			env := strings.SplitN(value, "=", 2)
			if len(env) > 1 {
				extraHeaders = append(extraHeaders, env[1])
			}
		}
	}

	return extraHeaders
}

// Check for extra headers from mounted secrets.
func getExtraHeadersFromSecrets() ([]string, error) {
	var secretExtraHeaders []string
	var err error

	secretDir := common.ImporterSecretExtraHeadersDir
	err = filepath.Walk(secretDir, func(filePath string, info fs.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return errors.Wrapf(err, "Error listing directories under %s", secretDir)
		}

		// Skip directories like ..data and ..2021_11_09_17_20_16.253260263
		if info.IsDir() && info.Name()[0] == '.' {
			return fs.SkipDir
		}

		// Don't try to read directories, or files that start with dots
		if info.IsDir() || info.Name()[0] == '.' {
			return nil
		}

		header, err := os.ReadFile(filePath)
		if err != nil {
			return errors.Wrapf(err, "Error reading headers from %s", filePath)
		}
		secretExtraHeaders = append(secretExtraHeaders, string(header))

		return err
	})

	return secretExtraHeaders, err
}

func getServerInfo(ctx context.Context, infoURL string) (*common.ServerInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct request for containerimage-server info")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed request containerimage-server info")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed request containerimage-server info: expected status code 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read body of containerimage-server info request")
	}

	info := &common.ServerInfo{}
	if err := json.Unmarshal(body, info); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal body of containerimage-server info request")
	}

	return info, nil
}
