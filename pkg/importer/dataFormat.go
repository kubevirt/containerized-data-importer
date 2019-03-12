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
	"io"
	"net/url"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

var qemuOperations = image.NewQEMUOperations()

// DataFormatInterface interface  to variuos data file formats
type DataFormatInterface interface {
	setArchived(archived bool)
	setDirectStreaming(encrypted bool, remoteStreaming bool, url *url.URL)
	setLocalPath(localPath string)
	store(reader io.ReadCloser, destPath string, scratchSpace string) error
	resizeImage(dest, imageSize string, totalTargetSpace int64) error
}

// GenericDataFormat - generic data format
type GenericDataFormat struct {
	qemu      bool
	archived  bool
	ImageSize string
	filePath  string
}

type format int

const (
	QCOW format = 0
	RAW  format = 1
)

func getDataFormat(qemu bool) format {
	if qemu {
		return QCOW
	} else {
		return RAW
	}
}

//NewDataFormat creates corresponding DataFormat with respect to provided Format
func NewDataFormat(dataFormat GenericDataFormat) (DataFormatInterface, error) {
	switch getDataFormat(dataFormat.qemu) {
	case QCOW:
		return &QCOWDataFormat{
			archived:        dataFormat.archived,
			filepath:        dataFormat.filePath,
			directStreaming: false,
			url:             nil,
			ImageSize:       dataFormat.ImageSize,
		}, nil
	case RAW:
		return &RawDataFormat{
			archived:  dataFormat.archived,
			ImageSize: dataFormat.ImageSize,
			filePath:  dataFormat.filePath,
		}, nil
	default:
		klog.Errorf("failed to construct DataFormat")
		return nil, errors.Errorf("failed to construct DataFormat")
	}
}

//RawDataFormat represents data in raw format
type RawDataFormat struct {
	archived  bool
	ImageSize string
	filePath  string
}

func (d *RawDataFormat) setLocalPath(localPath string) {
	d.filePath = localPath
}

func (d *RawDataFormat) setDirectStreaming(encrypted bool, remoteStreaming bool, url *url.URL) {}

func (d *RawDataFormat) setArchived(archived bool) {
	d.archived = archived
}

func (d *RawDataFormat) store(reader io.ReadCloser, destPath string, scratchSpace string) error {
	//file is not stored yet
	err := StreamDataToFile(reader, destPath)
	if err != nil {
		return err
	}
	return nil
}

func (d *RawDataFormat) resizeImage(dest, imageSize string, totalTargetSpace int64) error {
	return nil
}

//QCOWDataFormat represents data in qcow format
type QCOWDataFormat struct {
	archived        bool
	filepath        string
	directStreaming bool
	url             *url.URL
	ImageSize       string
}

func (d *QCOWDataFormat) isStoredLocally() bool {
	return len(d.filepath) > 0
}
func (d *QCOWDataFormat) setArchived(archived bool) {
	d.archived = archived
}

func (d *QCOWDataFormat) setDirectStreaming(encrypted bool, remoteStreaming bool, url *url.URL) {
	if !encrypted && !d.archived && remoteStreaming {
		d.directStreaming = true
		d.url = url
	}
}

func (d *QCOWDataFormat) setLocalPath(localPath string) {
	d.filepath = localPath
}

func (d *QCOWDataFormat) store(reader io.ReadCloser, destPath string, scratchSpace string) error {
	if d.directStreaming {
		err := d.convertQcow2ToRawStream(destPath)
		if err != nil {
			return err
		}
	} else {
		tmpDest := d.filepath
		if !d.isStoredLocally() {
			//need to store locally
			if util.GetAvailableSpace(scratchSpace) <= int64(0) {
				//Need scratch space but none provided.
				return ErrRequiresScratchSpace
			}

			tmpDest := filepath.Join(scratchSpace, filepath.Base(destPath))
			err := StreamDataToFile(reader, tmpDest)
			if err != nil {
				return err
			}
		}
		// The actual copy
		err := d.convertQcow2ToRaw(tmpDest, destPath)
		if err != nil {
			return err
		}
	} //else

	return nil
}

func (d *QCOWDataFormat) calculateTargetSize(dest string) int64 {
	targetQuantity := resource.NewScaledQuantity(util.GetAvailableSpace(filepath.Dir(dest)), 0)
	if d.ImageSize != "" {
		newImageSizeQuantity := resource.MustParse(d.ImageSize)
		minQuantity := util.MinQuantity(targetQuantity, &newImageSizeQuantity)
		targetQuantity = &minQuantity
	}
	targetSize, _ := targetQuantity.AsInt64()
	return targetSize
}

func (d *QCOWDataFormat) convertQcow2ToRaw(src, dest string) error {
	klog.V(3).Infoln("Validating qcow2 file")
	err := qemuOperations.Validate(src, "qcow2", d.calculateTargetSize(dest))
	if err != nil {
		return errors.Wrap(err, "Local image validation failed")
	}

	klog.V(2).Infoln("converting qcow2 image")
	err = qemuOperations.ConvertQcow2ToRaw(src, dest)
	if err != nil {
		return errors.Wrap(err, "Local qcow to raw conversion failed")
	}
	return nil
}

func (d *QCOWDataFormat) convertQcow2ToRawStream(dest string) error {
	klog.V(3).Infoln("Validating qcow2 file")

	err := qemuOperations.Validate(d.url.String(), "qcow2", d.calculateTargetSize(dest))
	if err != nil {
		return errors.Wrap(err, "Streaming image validation failed")
	}
	klog.V(3).Infoln("Doing streaming qcow2 to raw conversion")
	err = qemuOperations.ConvertQcow2ToRawStream(d.url, dest)
	if err != nil {
		return errors.Wrap(err, "Streaming qcow2 to raw conversion failed")
	}

	return nil
}

// ResizeImage resizes the images to match the requested size. Sometimes provisioners misbehave and the available space
// is not the same as the requested space. For those situations we compare the available space to the requested space and
// use the smallest of the two values.
func (d *QCOWDataFormat) resizeImage(dest, imageSize string, totalTargetSpace int64) error {
	info, err := qemuOperations.Info(dest)
	if err != nil {
		return err
	}
	if imageSize != "" {
		klog.V(3).Infoln("Resizing image")
		currentImageSizeQuantity := resource.NewScaledQuantity(info.VirtualSize, 0)
		newImageSizeQuantity := resource.MustParse(imageSize)
		minSizeQuantity := util.MinQuantity(resource.NewScaledQuantity(totalTargetSpace, 0), &newImageSizeQuantity)
		if minSizeQuantity.Cmp(newImageSizeQuantity) != 0 {
			// Available dest space is smaller than the size we want to resize to
			klog.Warningf("Available space less than requested size, resizing image to available space %s.\n", minSizeQuantity.String())
		}
		if currentImageSizeQuantity.Cmp(minSizeQuantity) == 0 {
			klog.V(1).Infof("No need to resize image. Requested size: %s, Image size: %d.\n", imageSize, info.VirtualSize)
			return nil
		}
		klog.V(1).Infof("Expanding image size to: %s\n", minSizeQuantity.String())
		return qemuOperations.Resize(dest, minSizeQuantity)
	}
	return errors.New("Image resize called with blank resize")
}
