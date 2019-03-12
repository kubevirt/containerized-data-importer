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
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"time"

	"net/url"

	"github.com/pkg/errors"
	"k8s.io/klog"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

//HttpDataStreamer - represents DataStreamer that streams from http target
type HttpDataStreamer struct {
	secKey    string
	accessKey string
	certDir   string
	url       *url.URL
	ctx       context.Context
	cancel    context.CancelFunc
}

func (i HttpDataStreamer) cleanup() error {
	if i.cancel != nil {
		i.cancel()
	}
	return nil
}

func (i HttpDataStreamer) isEncryptedChannel() bool {
	return (i.url.Scheme == "https" &&
		i.accessKey != "" &&
		i.secKey != "")
}

func (i HttpDataStreamer) isRemoteStreaming() bool {
	return true
}

func (i HttpDataStreamer) getDataFilePath() string {
	return ""
}

//HTTPDataStreamer  -creates a streamer that fetches file froem http source
func HTTPDataStreamer(url *url.URL, secKey, accessKey string, certDir string, insecureTLS bool) HttpDataStreamer {
	ctx, cancel := context.WithCancel(context.Background())
	return HttpDataStreamer{
		url:       url,
		secKey:    secKey,
		accessKey: accessKey,
		certDir:   certDir,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (i HttpDataStreamer) createHTTPClient() (*http.Client, error) {
	client := &http.Client{
		// Don't set timeout here, since that will be an absolute timeout, we need a relative to last progress timeout.
	}

	if i.certDir == "" {
		return client, nil
	}

	// let's get system certs as well
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.Wrap(err, "Error getting system certs")
	}

	files, err := ioutil.ReadDir(i.certDir)
	if err != nil {
		return nil, errors.Wrapf(err, "Error listing files in %s", i.certDir)
	}

	for _, file := range files {
		if file.IsDir() || file.Name()[0] == '.' {
			continue
		}

		fp := path.Join(i.certDir, file.Name())

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

func (i HttpDataStreamer) stream() (io.ReadCloser, StreamContext, error) {
	client, err := i.createHTTPClient()
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error creating http client")
	}

	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(i.accessKey) > 0 && len(i.secKey) > 0 {
			r.SetBasicAuth(i.accessKey, i.secKey) // Redirects will lose basic auth, so reset them manually
		}
		return nil
	}

	req, err := http.NewRequest("GET", i.url.String(), nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not create HTTP request")
	}
	req = req.WithContext(i.ctx)
	if len(i.accessKey) > 0 && len(i.secKey) > 0 {
		req.SetBasicAuth(i.accessKey, i.secKey)
	}
	klog.V(2).Infof("Attempting to get object %q via http client\n", i.url)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "HTTP request errored")
	}
	if resp.StatusCode != 200 {
		klog.Errorf("http: expected status code 200, got %d", resp.StatusCode)
		return nil, nil, errors.Errorf("expected status code 200, got %d. Status: %s", resp.StatusCode, resp.Status)
	}
	countingReader := &util.CountingReader{
		Reader:  resp.Body,
		Current: 0,
	}
	go i.pollProgress(countingReader, 10*time.Minute, time.Second)
	return countingReader, i, nil
}

func (i HttpDataStreamer) pollProgress(reader *util.CountingReader, idleTime, pollInterval time.Duration) {
	count := reader.Current
	lastUpdate := time.Now()
	for {
		if count < reader.Current {
			// Some progress was made, reset now.
			lastUpdate = time.Now()
			count = reader.Current
		}
		if lastUpdate.Add(idleTime).Sub(time.Now()).Nanoseconds() < 0 {
			// No progress for the idle time, cancel http client.
			i.cancel() // This will trigger i.ctx.Done()
		}
		select {
		case <-time.After(pollInterval):
			continue
		case <-i.ctx.Done():
			return // Don't leak, once the transfer is cancelled or completed this is called.
		}
	}
}
