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
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/system"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	networkTimeoutSecs = 3600    //max is 10000
	maxMemory          = 1 << 30 //value from OpenStack Nova
	maxCPUSecs         = 30      //value from OpenStack Nova
	matcherString      = "\\((\\d?\\d\\.\\d\\d)\\/100%\\)"
)

// ImgInfo contains the virtual image information.
type ImgInfo struct {
	// Format contains the format of the image
	Format string `json:"format"`
	// BackingFile is the file name of the backing file
	BackingFile string `json:"backing-filename"`
	// VirtualSize is the disk size of the image which will be read by vm
	VirtualSize int64 `json:"virtual-size"`
	// ActualSize is the size of the qcow2 image
	ActualSize int64 `json:"actual-size"`
}

// QEMUOperations defines the interface for executing qemu subprocesses
type QEMUOperations interface {
	ConvertToRawStream(*url.URL, string) error
	Resize(string, resource.Quantity) error
	Info(url *url.URL) (*ImgInfo, error)
	Validate(*url.URL, int64) error
	CreateBlankImage(string, resource.Quantity) error
}

type qemuOperations struct{}

var (
	qemuExecFunction = system.ExecWithLimits
	qemuInfoLimits   = &system.ProcessLimitValues{AddressSpaceLimit: maxMemory, CPUTimeLimit: maxCPUSecs}
	qemuIterface     = NewQEMUOperations()
	re               = regexp.MustCompile(matcherString)

	progress = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "import_progress",
			Help: "The import progress in percentage",
		},
		[]string{"ownerUID"},
	)
	ownerUID string
)

func init() {
	if err := prometheus.Register(progress); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// A counter for that metric has been registered before.
			// Use the old counter from now on.
			progress = are.ExistingCollector.(*prometheus.CounterVec)
		} else {
			klog.Errorf("Unable to create prometheus progress counter")
		}
	}
	ownerUID, _ = util.ParseEnvVar(common.OwnerUID, false)
}

// NewQEMUOperations returns the default implementation of QEMUOperations
func NewQEMUOperations() QEMUOperations {
	return &qemuOperations{}
}

func convertToRaw(src, dest string) error {
	_, err := qemuExecFunction(nil, nil, "qemu-img", "convert", "-t", "none", "-p", "-O", "raw", src, dest)
	if err != nil {
		os.Remove(dest)
		return errors.Wrap(err, "could not convert image to raw")
	}

	return nil
}

func (o *qemuOperations) ConvertToRawStream(url *url.URL, dest string) error {
	if len(url.Scheme) == 0 {
		// File, instead of URL
		return convertToRaw(url.String(), dest)
	}

	var jsonArg string
	if url.Scheme == "nbd" && url.Path != "" {
		// Convert from local Unix socket
		jsonArg = fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.path\": \"%s\"}", url.Scheme, url.Path)
	} else {
		jsonArg = fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.url\": \"%s\", \"file.timeout\": %d}", url.Scheme, url, networkTimeoutSecs)
	}

	_, err := qemuExecFunction(nil, reportProgress, "qemu-img", "convert", "-t", "none", "-p", "-O", "raw", jsonArg, dest)
	if err != nil {
		// TODO: Determine what to do here, the conversion failed, and we need to clean up the mess, but we could be writing to a block device
		os.Remove(dest)
		return errors.Wrap(err, "could not stream/convert image to raw")
	}

	return nil
}

// convertQuantityToQemuSize translates a quantity string into a Qemu compatible string.
func convertQuantityToQemuSize(size resource.Quantity) string {
	int64Size, asInt := size.AsInt64()
	if !asInt {
		size.AsDec().SetScale(0)
		return size.AsDec().String()
	}
	return strconv.FormatInt(int64Size, 10)
}

func (o *qemuOperations) Resize(image string, size resource.Quantity) error {
	_, err := qemuExecFunction(nil, nil, "qemu-img", "resize", "-f", "raw", image, convertQuantityToQemuSize(size))
	if err != nil {
		return errors.Wrapf(err, "Error resizing image %s", image)
	}
	return nil
}

func (o *qemuOperations) Info(url *url.URL) (*ImgInfo, error) {
	var output []byte
	var err error

	if len(url.Scheme) > 0 {
		var jsonArg string
		if url.Scheme == "nbd" && url.Path != "" {
			// Get NBD info from local Unix socket
			jsonArg = fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.path\": \"%s\"}", url.Scheme, url.Path)
		} else {
			// Image is a URL, make sure the timeout is long enough.
			jsonArg = fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.url\": \"%s\", \"file.timeout\": %d}", url.Scheme, url, networkTimeoutSecs)
		}
		output, err = qemuExecFunction(qemuInfoLimits, nil, "qemu-img", "info", "--output=json", jsonArg)
	} else {
		output, err = qemuExecFunction(qemuInfoLimits, nil, "qemu-img", "info", "--output=json", url.String())
	}
	if err != nil {
		return nil, errors.Errorf("%s, %s", output, err.Error())
	}
	var info ImgInfo
	err = json.Unmarshal(output, &info)
	if err != nil {
		klog.Errorf("Invalid JSON:\n%s\n", string(output))
		return nil, errors.Wrapf(err, "Invalid json for image %s", url.String())
	}
	return &info, nil
}

func isSupportedFormat(value string) bool {
	switch value {
	case "raw", "qcow2":
		return true
	default:
		return false
	}
}

func (o *qemuOperations) Validate(url *url.URL, availableSize int64) error {
	info, err := o.Info(url)
	if err != nil {
		return err
	}

	if !isSupportedFormat(info.Format) {
		return errors.Errorf("Invalid format %s for image %s", info.Format, url.String())
	}

	if len(info.BackingFile) > 0 {
		return errors.Errorf("Image %s is invalid because it has backing file %s", url.String(), info.BackingFile)
	}

	if availableSize < info.VirtualSize {
		return errors.Errorf("Virtual image size %d is larger than available size %d. A larger PVC is required.", info.VirtualSize, availableSize)
	}
	return nil
}

// ConvertToRawStream converts an http accessible image to raw format without locally caching the image
func ConvertToRawStream(url *url.URL, dest string) error {
	return qemuIterface.ConvertToRawStream(url, dest)
}

// Validate does basic validation of a qemu image
func Validate(url *url.URL, availableSize int64) error {
	return qemuIterface.Validate(url, availableSize)
}

func reportProgress(line string) {
	// (45.34/100%)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 2 && ownerUID != "" {
		klog.V(1).Info(matches[1])
		// Don't need to check for an error, the regex made sure its a number we can parse.
		v, _ := strconv.ParseFloat(matches[1], 64)
		metric := &dto.Metric{}
		err := progress.WithLabelValues(ownerUID).Write(metric)
		if err == nil && v > 0 && v > *metric.Counter.Value {
			progress.WithLabelValues(ownerUID).Add(v - *metric.Counter.Value)
		}
	}
}

// CreateBlankImage creates empty raw image
func CreateBlankImage(dest string, size resource.Quantity) error {
	klog.V(1).Infof("creating raw image with size %s", size.String())
	return qemuIterface.CreateBlankImage(dest, size)
}

// CreateBlankImage creates a raw image with a given size
func (o *qemuOperations) CreateBlankImage(dest string, size resource.Quantity) error {
	klog.V(3).Infof("image size is %s", size.String())
	_, err := qemuExecFunction(nil, nil, "qemu-img", "create", "-f", "raw", dest, convertQuantityToQemuSize(size))
	if err != nil {
		os.Remove(dest)
		return errors.Wrap(err, fmt.Sprintf("could not create raw image with size %s in %s", size.String(), dest))
	}
	// Change permissions to 0660
	err = os.Chmod(dest, 0660)
	if err != nil {
		err = errors.Wrap(err, "Unable to change permissions of target file")
	}

	return nil
}
