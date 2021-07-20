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
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

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
	ConvertToRawStream(*url.URL, string, bool) error
	Resize(string, resource.Quantity, bool) error
	Info(url *url.URL) (*ImgInfo, error)
	Validate(*url.URL, int64, float64) error
	CreateBlankImage(string, resource.Quantity, bool) error
	Rebase(backingFile string, delta string) error
	Commit(image string) error
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
	ownerUID                    string
	convertPreallocationMethods = [][]string{
		{"-o", "preallocation=falloc"},
		{"-o", "preallocation=full"},
		{"-S", "0"},
	}
	resizePreallocationMethods = [][]string{
		{"--preallocation=falloc"},
		{"--preallocation=full"},
	}
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

func convertToRaw(src, dest string, preallocate bool) error {
	args := []string{"convert", "-t", "none", "-p", "-O", "raw", src, dest}
	var err error
	if preallocate {
		err = addPreallocation(args, convertPreallocationMethods, func(args []string) ([]byte, error) {
			return qemuExecFunction(nil, reportProgress, "qemu-img", args...)
		})
	} else {
		_, err = qemuExecFunction(nil, reportProgress, "qemu-img", args...)
	}
	if err != nil {
		os.Remove(dest)
		errorMsg := "could not convert image to raw"
		if nbdkitLog, err := ioutil.ReadFile(common.NbdkitLogPath); err == nil {
			errorMsg += " " + string(nbdkitLog)
		}
		return errors.Wrap(err, errorMsg)
	}

	return nil
}

func (o *qemuOperations) ConvertToRawStream(url *url.URL, dest string, preallocate bool) error {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" {
		return fmt.Errorf("Not valid schema %s", url.Scheme)
	}
	return convertToRaw(url.String(), dest, preallocate)
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

// Resize resizes the given image to size
func Resize(image string, size resource.Quantity, preallocate bool) error {
	return qemuIterface.Resize(image, size, preallocate)
}

func (o *qemuOperations) Resize(image string, size resource.Quantity, preallocate bool) error {
	var err error
	args := []string{"resize", "-f", "raw", image, convertQuantityToQemuSize(size)}
	if preallocate {
		err = addPreallocation(args, resizePreallocationMethods, func(args []string) ([]byte, error) {
			return qemuExecFunction(nil, nil, "qemu-img", args...)
		})
	} else {
		_, err = qemuExecFunction(nil, nil, "qemu-img", args...)
	}
	if err != nil {
		return errors.Wrapf(err, "Error resizing image %s", image)
	}
	return nil
}

func checkOutputQemuImgInfo(output []byte, image string) (*ImgInfo, error) {
	var info ImgInfo
	err := json.Unmarshal(output, &info)
	if err != nil {
		klog.Errorf("Invalid JSON:\n%s\n", string(output))
		return nil, errors.Wrapf(err, "Invalid json for image %s", image)
	}
	return &info, nil

}

// Info returns information about the image from the url
func Info(url *url.URL) (*ImgInfo, error) {
	return qemuIterface.Info(url)
}

func (o *qemuOperations) Info(url *url.URL) (*ImgInfo, error) {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" {
		return nil, fmt.Errorf("Not valid schema %s", url.Scheme)
	}
	output, err := qemuExecFunction(qemuInfoLimits, nil, "qemu-img", "info", "--output=json", url.String())
	if err != nil {
		errorMsg := fmt.Sprintf("%s, %s", output, err.Error())
		if nbdkitLog, err := ioutil.ReadFile(common.NbdkitLogPath); err == nil {
			errorMsg += " " + string(nbdkitLog)
		}
		return nil, errors.Errorf(errorMsg)
	}
	return checkOutputQemuImgInfo(output, url.String())
}

func isSupportedFormat(value string) bool {
	switch value {
	case "raw", "qcow2", "vmdk", "vdi", "vpc", "vhdx":
		return true
	default:
		return false
	}
}

func checkIfURLIsValid(info *ImgInfo, availableSize int64, filesystemOverhead float64, image string) error {
	if !isSupportedFormat(info.Format) {
		return errors.Errorf("Invalid format %s for image %s", info.Format, image)
	}

	if len(info.BackingFile) > 0 {
		if _, err := os.Stat(info.BackingFile); err != nil {
			return errors.Errorf("Image %s is invalid because it has invalid backing file %s", image, info.BackingFile)
		}
	}

	if int64(float64(availableSize)*(1-filesystemOverhead)) < info.VirtualSize {
		return errors.Errorf("Virtual image size %d is larger than available size %d (PVC size %d, reserved overhead %f%%). A larger PVC is required.", info.VirtualSize, int64((1-filesystemOverhead)*float64(availableSize)), info.VirtualSize, filesystemOverhead)
	}
	return nil
}

func (o *qemuOperations) Validate(url *url.URL, availableSize int64, filesystemOverhead float64) error {
	info, err := o.Info(url)
	if err != nil {
		return err
	}
	return checkIfURLIsValid(info, availableSize, filesystemOverhead, url.String())
}

// ConvertToRawStream converts an http accessible image to raw format without locally caching the image
func ConvertToRawStream(url *url.URL, dest string, preallocate bool) error {
	return qemuIterface.ConvertToRawStream(url, dest, preallocate)
}

// Validate does basic validation of a qemu image
func Validate(url *url.URL, availableSize int64, filesystemOverhead float64) error {
	return qemuIterface.Validate(url, availableSize, filesystemOverhead)
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
func CreateBlankImage(dest string, size resource.Quantity, preallocate bool) error {
	klog.V(1).Infof("creating raw image with size %s, preallocation %v", size.String(), preallocate)
	return qemuIterface.CreateBlankImage(dest, size, preallocate)
}

// CreateBlankImage creates a raw image with a given size
func (o *qemuOperations) CreateBlankImage(dest string, size resource.Quantity, preallocate bool) error {
	klog.V(3).Infof("image size is %s", size.String())
	args := []string{"create", "-f", "raw", dest, convertQuantityToQemuSize(size)}
	if preallocate {
		klog.V(1).Infof("Added preallocation")
		args = append(args, []string{"-o", "preallocation=falloc"}...)
	}
	_, err := qemuExecFunction(nil, nil, "qemu-img", args...)
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

// PreallocateBlankBlock writes requested amount of zeros to block device mounted at dest
func PreallocateBlankBlock(dest string, size resource.Quantity) error {
	klog.V(3).Infof("block volume size is %s", size.String())

	args := []string{"if=/dev/zero", "of=" + dest, "bs=" + convertQuantityToQemuSize(size), "count=1"}
	_, err := qemuExecFunction(nil, nil, "dd", args...)

	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Could not preallocate blank block volume at %s with size %s", dest, size.String()))
	}

	return nil
}

func addPreallocation(args []string, preallocationMethods [][]string, qemuFn func(args []string) ([]byte, error)) error {
	var err error
	for _, preallocationMethod := range preallocationMethods {
		var output []byte

		klog.V(1).Infof("Adding preallocation method: %v", preallocationMethod)
		// For some subcommands (e.g. resize), preallocation optinos must come before other options
		argsToTry := append([]string{args[0]}, preallocationMethod...)
		argsToTry = append(argsToTry, args[1:]...)

		output, err = qemuFn(argsToTry)
		if err != nil && strings.Contains(string(output), "Unsupported preallocation mode") {
			klog.V(1).Infof("Unsupported preallocation mode. Retrying")
		} else {
			break
		}
	}

	return err
}

// Rebase changes a QCOW's backing file to point to a previously-downloaded base image.
// Depends on original image having been downloaded as raw.
func (o *qemuOperations) Rebase(backingFile string, delta string) error {
	klog.V(1).Infof("Rebasing %s onto %s", delta, backingFile)
	args := []string{"rebase", "-p", "-u", "-F", "raw", "-b", backingFile, delta}
	_, err := qemuExecFunction(nil, reportProgress, "qemu-img", args...)
	return err
}

// Commit takes the changes written to a QCOW and applies them to its raw backing file.
func (o *qemuOperations) Commit(image string) error {
	klog.V(1).Infof("Committing %s to backing file...", image)
	args := []string{"commit", "-p", image}
	_, err := qemuExecFunction(nil, reportProgress, "qemu-img", args...)
	return err
}
