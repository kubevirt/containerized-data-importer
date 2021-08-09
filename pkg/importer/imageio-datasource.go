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
	"fmt"
	"io"
	"net/url"
	"os"

	ovirtclient "github.com/ovirt/go-ovirt-client"
	kloglogger "github.com/ovirt/go-ovirt-client-log-klog"
)

type ovirtClientFactory func(
	endpoint string,
	accessKey string,
	secKey string,
	certDir string,
) (
	ovirtclient.Client,
	error,
)

var defaultOVirtClientFactory ovirtClientFactory = func(endpoint string, accessKey string, secKey string, certDir string) (
	ovirtclient.Client,
	error,
) {
	return ovirtclient.New(
		endpoint,
		accessKey,
		secKey,
		ovirtclient.CACertsFromDir(certDir),
		nil,
		kloglogger.New(),
	)
}

var mockOVirtClient = ovirtclient.NewMock()

var mockOvirtClientFactory ovirtClientFactory = func(
	endpoint string,
	accessKey string,
	secKey string,
	certDir string,
) (ovirtclient.Client, error) {
	return mockOVirtClient, nil
}

var newOVirtClient = defaultOVirtClientFactory

// NewImageioDataSource creates a new instance of the ovirt-imageio data provider.
func NewImageioDataSource(endpoint string, accessKey string, secKey string, certDir string, diskID string) (DataSourceInterface, error) {
	cli, err := newOVirtClient(endpoint, accessKey, secKey, certDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create oVirt client (%w)", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	download, err := cli.StartImageDownload(diskID, ovirtclient.ImageFormatRaw, ovirtclient.ContextStrategy(ctx))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start image download for disk ID %s", diskID)
	}

	return &imageIODataSource{
		download: download,
		cancel:   cancel,
	}, nil
}

type imageIODataSource struct {
	download ovirtclient.ImageDownload
	cancel   context.CancelFunc
}

func (i *imageIODataSource) Info() (ProcessingPhase, error) {
	// Wait for transfer initialization to finish or error out.
	<-i.download.Initialized()
	if err := i.download.Err(); err != nil {
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseTransferDataFile, nil
}

func (i *imageIODataSource) Transfer(path string) (ProcessingPhase, error) {
	return ProcessingPhaseError, fmt.Errorf("transfer not implemented for imageIO")
}

func (i *imageIODataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	fh, err := os.Create(fileName)
	if err != nil {
		return ProcessingPhaseError, fmt.Errorf("failed to open destination path")
	}
	defer func() {
		_ = fh.Close()
	}()
	if _, err := io.Copy(fh, i.download); err != nil {
		return ProcessingPhaseError, err
	}
	// We go directly to resize since we downloaded in RAW format.
	return ProcessingPhaseResize, nil
}

func (i *imageIODataSource) GetURL() *url.URL {
	return nil
}

func (i *imageIODataSource) Close() error {
	i.cancel()
	return i.download.Close()
}
