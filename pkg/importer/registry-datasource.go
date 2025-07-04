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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// containerDiskImageDir - Expected disk image location in container image as described in
	// https://github.com/kubevirt/kubevirt/blob/main/docs/container-register-disks.md
	containerDiskImageDir = "disk"
)

// RegistryDataSource is the struct containing the information needed to import from a registry data source.
// Sequence of phases:
// 1. Info -> Transfer
// 2. Transfer -> Convert
type RegistryDataSource struct {
	endpoint          string
	accessKey         string
	secKey            string
	imageArchitecture string
	certDir           string
	insecureTLS       bool
	imageDir          string
	//The discovered image file in scratch space.
	url *url.URL
	//The discovered image info from the registry.
	info *types.ImageInspectInfo
}

// NewRegistryDataSource creates a new instance of the Registry Data Source.
func NewRegistryDataSource(endpoint, accessKey, secKey, imageArchitecture, certDir string, insecureTLS bool) *RegistryDataSource {
	allCertDir, err := CreateCertificateDir(certDir)
	if err != nil {
		klog.Infof("Error creating allCertDir %v", err)
		if allCertDir != "/" {
			err = os.RemoveAll(allCertDir)
			if err != nil {
				klog.Errorf("Unable to clean up all cert dir %v", err)
			}
		}
		allCertDir = certDir
	}
	return &RegistryDataSource{
		endpoint:          endpoint,
		accessKey:         accessKey,
		secKey:            secKey,
		imageArchitecture: imageArchitecture,
		certDir:           allCertDir,
		insecureTLS:       insecureTLS,
	}
}

// Info is called to get initial information about the data. No information available for registry currently.
func (rd *RegistryDataSource) Info() (ProcessingPhase, error) {
	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source registry to a temporary location.
func (rd *RegistryDataSource) Transfer(path string, preallocation bool) (ProcessingPhase, error) {
	rd.imageDir = filepath.Join(path, containerDiskImageDir)
	if err := CleanAll(rd.imageDir); err != nil {
		return ProcessingPhaseError, err
	}

	size, err := GetAvailableSpace(path)
	if err != nil {
		return ProcessingPhaseError, err
	}
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}

	klog.V(1).Infof("Copying registry image to scratch space.")
	rd.info, err = CopyRegistryImage(rd.endpoint, path, containerDiskImageDir, rd.accessKey, rd.secKey, rd.imageArchitecture, rd.certDir, rd.insecureTLS, preallocation)
	if err != nil {
		return ProcessingPhaseError, errors.Wrapf(err, "Failed to read registry image")
	}

	imageFile, err := getImageFileName(rd.imageDir)
	if err != nil {
		return ProcessingPhaseError, errors.Wrapf(err, "Cannot locate image file")
	}

	// imageFile and rd.imageDir are both valid, thus the Join will be valid, and the parse will work, no need to check for parse errors
	rd.url, _ = url.Parse(filepath.Join(rd.imageDir, imageFile))
	klog.V(3).Infof("Successfully found file. VM disk image filename is %s", rd.url.String())
	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (rd *RegistryDataSource) TransferFile(fileName string, preallocation bool) (ProcessingPhase, error) {
	return ProcessingPhaseError, errors.New("Transferfile should not be called")
}

// GetURL returns the url that the data processor can use when converting the data.
func (rd *RegistryDataSource) GetURL() *url.URL {
	return rd.url
}

// GetTerminationMessage returns data to be serialized and used as the termination message of the importer.
func (rd *RegistryDataSource) GetTerminationMessage() *common.TerminationMessage {
	if rd.info == nil {
		return nil
	}
	return &common.TerminationMessage{
		Labels: envsToLabels(rd.info.Env),
	}
}

// Close closes any readers or other open resources.
func (rd *RegistryDataSource) Close() error {
	// No-op, no open readers
	return nil
}

func getImageFileName(dir string) (string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		klog.Errorf("image directory does not exist")
		return "", errors.Errorf("image directory does not exist")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		klog.Errorf("Error reading directory")
		return "", errors.Wrapf(err, "image file does not exist in image directory")
	}

	if len(entries) == 0 {
		klog.Errorf("image file does not exist in image directory - directory is empty ")
		return "", errors.New("image file does not exist in image directory - directory is empty")
	}

	if len(entries) > 1 {
		klog.Errorf("image directory contains more than one file")
		return "", errors.New("image directory contains more than one file")
	}

	fileinfo := entries[0]
	if fileinfo.IsDir() {
		klog.Errorf("image file does not exist in image directory contains another directory ")
		return "", errors.New("image directory contains another directory")
	}

	filename := fileinfo.Name()

	if len(strings.TrimSpace(filename)) == 0 {
		klog.Errorf("image file does not exist in image directory - file has no name ")
		return "", errors.New("image file does has no name")
	}

	klog.V(1).Infof("VM disk image filename is %s", filename)

	return filename, nil
}

// CreateCertificateDir creates a common certificate dir
func CreateCertificateDir(registryCertDir string) (string, error) {
	allCerts := "/tmp/all_certs"
	if err := os.MkdirAll(allCerts, 0700); err != nil {
		return allCerts, err
	}

	klog.Info("Copying proxy certs")
	if err := collectCerts(common.ImporterProxyCertDir, allCerts, "proxy-"); err != nil {
		return allCerts, err
	}

	if registryCertDir == "" {
		klog.Info("Registry certs directory not configured")
		return allCerts, nil
	}

	klog.Info("Copying registry certs")
	if err := collectCerts(registryCertDir, allCerts, ""); err != nil {
		return allCerts, err
	}
	return allCerts, nil
}

func collectCerts(certDir, targetDir, targetPrefix string) error {
	directory, err := os.Open(certDir)
	if err != nil {
		return err
	}
	objects, err := directory.Readdir(-1)
	if err != nil {
		return err
	}
	for _, obj := range objects {
		if !strings.HasSuffix(obj.Name(), ".crt") {
			continue
		}
		if err := LinkFile(filepath.Join(certDir, obj.Name()), filepath.Join(targetDir, targetPrefix+obj.Name())); err != nil {
			return err
		}
	}
	return nil
}
