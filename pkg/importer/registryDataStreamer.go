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
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

type registryData struct {
	dataDir  string
	filePath string
	file     *os.File
}

//Implementtaion of StreamContext interfaces
func (i registryData) cleanup() error {
	klog.V(1).Infof("registryData - deleting all the data")
	if i.file != nil {
		i.file.Close()
		i.file = nil
	}
	if len(i.dataDir) > 0 {
		err := os.RemoveAll(i.dataDir)
		if err != nil {
			klog.Errorf(fmt.Sprintf("Failed removing directory ", err.Error()))
		}
		i.dataDir = ""
	}
	return nil
}

func (i registryData) getDataFilePath() string {
	return i.filePath
}

type regisitryDataStreamer struct {
	secKey      string
	accessKey   string
	certDir     string
	insecureTLS bool
	url         *url.URL
	dataDir     string
	data        registryData
}

//RegistryDataStreamer - creates a streamer that imports data from registry
func RegistryDataStreamer(url *url.URL, secKey, accessKey string, certDir string, insecureTLS bool, dataDir string) Streamer {
	return regisitryDataStreamer{
		url:         url,
		secKey:      secKey,
		accessKey:   accessKey,
		certDir:     certDir,
		insecureTLS: insecureTLS,
		dataDir:     dataDir,
	}
}

const (
	//TempContainerDiskDir - Temporary location for pulled containerImage
	TempContainerDiskDir = "tmp"
)

//This import source downloads specified container image from registry location
//Then it extracts the image to a temporary location and expects an image file to be located under /disk directory
//If such exists it creates a Reader on it and returns it for further processing
func (i regisitryDataStreamer) stream() (io.ReadCloser, StreamContext, error) {

	tmpData := registryData{"", "", nil}
	tmpData.dataDir = filepath.Join(i.dataDir, TempContainerDiskDir)

	imageDir := filepath.Join(tmpData.dataDir, ContainerDiskImageDir)

	if util.GetAvailableSpace(i.dataDir) <= int64(0) {
		// No scratch space available, exit with code indicating we need scratch space.
		return nil, nil, ErrRequiresScratchSpace
	}

	//1. create temporary directory if does not exist to which all the data will be extracted
	if _, err := os.Stat(tmpData.dataDir); os.IsNotExist(err) {
		err := os.Mkdir(tmpData.dataDir, os.ModeDir|os.ModePerm)
		if err != nil {
			klog.Errorf(fmt.Sprintf("Failed to create temporary directory"))
			return nil, nil, errors.Wrapf(err, fmt.Sprintf("Failed to create tempdirectory %s", tmpData.dataDir))
		}
	}

	//cleanup in case of failure
	cleanup := true
	defer func() {
		if cleanup {
			tmpData.cleanup()
		}
	}()

	//2. copy image from registry to the temporary location
	klog.V(1).Infof("using skopeo to copy from registry")
	err := image.CopyDirFromRegistryImage(i.url.String(), tmpData.dataDir, ContainerDiskImageDir, i.accessKey, i.secKey, i.certDir, i.insecureTLS)
	if err != nil {
		klog.Errorf(fmt.Sprintf("Failed to read data from registry"))
		return nil, nil, errors.Wrapf(err, fmt.Sprintf("Failed ro read from registry"))
	}

	//3. Search for file in /disk directory - if not found - failure
	imageFile, err := getImageFileName(imageDir)
	if err != nil {
		klog.Errorf(fmt.Sprintf("Error getting Image file from imageDirectory"))
		return nil, nil, errors.Wrapf(err, fmt.Sprintf("Cannot locate image file"))
	}

	// 4. If found - Create a reader that will read this file and attach it to the dataStream
	tmpData.file, err = os.Open(filepath.Join(imageDir, imageFile))
	if err != nil {
		klog.Errorf(fmt.Sprintf("Failed to open image file"))
		return nil, nil, errors.Wrapf(err, fmt.Sprintf("Fail to create data stream from image file"))
	}

	//got this far - do not cleanup
	cleanup = false

	i.data = tmpData
	klog.V(1).Infof("Sucecssfully found file. VM disk image filename is %s", imageFile)

	return ioutil.NopCloser(bufio.NewReader(tmpData.file)), i.data, nil
}

func getImageFileName(dir string) (string, error) {

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		klog.Errorf(fmt.Sprintf("image directory does not exist"))
		return "", errors.Errorf(fmt.Sprintf("image directory does not exist "))
	}

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		klog.Errorf(fmt.Sprintf("Error reading directory"))
		return "", errors.Wrapf(err, fmt.Sprintf("image file does not exist in image directory"))
	}

	if len(entries) == 0 {
		klog.Errorf(fmt.Sprintf("image file does not exist in image directory - directory is empty "))
		return "", errors.Errorf(fmt.Sprintf("image file does not exist in image directory - directory is empty"))
	}

	fileinfo := entries[len(entries)-1]
	if fileinfo.IsDir() {
		klog.Errorf(fmt.Sprintf("image file does not exist in image directory contains another directory "))
		return "", errors.Errorf(fmt.Sprintf("image file does not exist in image directory"))
	}

	filename := fileinfo.Name()

	if len(strings.TrimSpace(filename)) == 0 {
		klog.Errorf(fmt.Sprintf("image file does not exist in image directory - file has no name "))
		return "", errors.Errorf(fmt.Sprintf("image file does not exist in image directory"))
	}

	klog.V(1).Infof("VM disk image filename is %s", filename)

	return filename, nil
}
