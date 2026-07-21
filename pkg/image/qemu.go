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
	"io/fs"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-importer"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const matcherString = "\\((\\d?\\d\\.\\d\\d)\\/100%\\)"

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
	ConvertToRawStream(*url.URL, string, bool, string) error
	Resize(string, resource.Quantity, bool) error
	Info(url *url.URL) (*ImgInfo, error)
	Validate(*url.URL, int64) error
	CreateBlankImage(string, resource.Quantity, bool) error
	PreallocateBlankBlock(string, resource.Quantity) error
	Rebase(backingFile string, delta string) error
	Commit(image string) error
}

type qemuOperations struct {
	cmd *qemuCmd
}

var (
	ErrLargerPVCRequired = errors.New("A larger PVC is required")

	qemuInterface = NewQEMUOperations()
	re            = regexp.MustCompile(matcherString)

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
	odirectChecker = NewDirectIOChecker(RealOS{})
)

func init() {
	if err := metrics.SetupMetrics(); err != nil {
		klog.Errorf("Unable to create prometheus progress counter: %v", err)
	}
	ownerUID, _ = util.ParseEnvVar(common.OwnerUID, false)
}

// NewQEMUOperations returns the default implementation of QEMUOperations
func NewQEMUOperations() QEMUOperations {
	return &qemuOperations{cmd: newQemuCmd()}
}

func (o *qemuOperations) convertToRaw(src, dest string, preallocate bool, cacheMode string) error {
	cacheMode, err := getCacheMode(dest, cacheMode)
	if err != nil {
		return err
	}
	args := []string{"convert", "-t", cacheMode, "-p", "-O", "raw", src, dest}

	if preallocate {
		err = addPreallocation(args, convertPreallocationMethods, func(args []string) error {
			return o.cmd.ExecWithProgress(args...)
		})
	} else {
		klog.V(1).Infof("Running qemu-img with args: %v", args)
		err = o.cmd.ExecWithProgress(args...)
	}
	if err != nil {
		os.Remove(dest)
		errorMsg := "could not convert image to raw"
		if nbdkitLog, err := os.ReadFile(common.NbdkitLogPath); err == nil {
			errorMsg += " " + string(nbdkitLog)
		}
		return errors.Wrap(err, errorMsg)
	}

	return nil
}

func getCacheMode(path string, cacheMode string) (string, error) {
	if cacheMode != common.CacheModeTryNone {
		return "writeback", nil
	}

	var supportDirectIO bool
	var stat unix.Stat_t
	var err error

	if err = unix.Stat(path, &stat); err != nil && !errors.Is(err, fs.ErrNotExist) {
		// volumeDevices specified on pod level definitely exist, must be filesystem
		return "", fmt.Errorf("cannot stat for establishing O_DIRECT support: %w", err)
	}

	if err == nil && ((stat.Mode & unix.S_IFMT) == unix.S_IFBLK) {
		supportDirectIO, err = odirectChecker.CheckBlockDevice(path)
	} else {
		supportDirectIO, err = odirectChecker.CheckFile(path)
	}

	if err != nil {
		return "", err
	}

	if supportDirectIO {
		return "none", nil
	}

	return "writeback", nil
}

func (o *qemuOperations) ConvertToRawStream(url *url.URL, dest string, preallocate bool, cacheMode string) error {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" {
		return fmt.Errorf("not valid schema %s", url.Scheme)
	}
	return o.convertToRaw(url.String(), dest, preallocate, cacheMode)
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

func (o *qemuOperations) Resize(image string, size resource.Quantity, preallocate bool) error {
	var err error
	args := []string{"resize", "-f", "raw", image, convertQuantityToQemuSize(size)}
	if preallocate {
		err = addPreallocation(args, resizePreallocationMethods, func(args []string) error {
			return o.cmd.Exec(args...)
		})
	} else {
		err = o.cmd.Exec(args...)
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
	return qemuInterface.Info(url)
}

func (o *qemuOperations) Info(url *url.URL) (*ImgInfo, error) {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" && url.Scheme != "file" {
		return nil, fmt.Errorf("not valid schema %s", url.Scheme)
	}
	output, err := o.cmd.Info(qemuInfoTimeout, url)
	if err != nil {
		errorMsg := err.Error()
		if url.Scheme == "nbd+unix" {
			if nbdkitLog, err := os.ReadFile(common.NbdkitLogPath); err == nil {
				errorMsg += " " + string(nbdkitLog)
			}
		}
		return nil, errors.New(errorMsg)
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

func checkIfURLIsValid(info *ImgInfo, availableSize int64, image string) error {
	if !isSupportedFormat(info.Format) {
		return errors.Errorf("Invalid format %s for image %s", info.Format, image)
	}

	if len(info.BackingFile) > 0 {
		if _, err := os.Stat(info.BackingFile); err != nil {
			return errors.Errorf("Image %s is invalid because it has invalid backing file %s", image, info.BackingFile)
		}
	}

	if availableSize < info.VirtualSize {
		return fmt.Errorf("virtual image size %d is larger than the reported available storage %d. %w", info.VirtualSize, availableSize, ErrLargerPVCRequired)
	}
	return nil
}

func (o *qemuOperations) Validate(url *url.URL, availableSize int64) error {
	info, err := o.Info(url)
	if err != nil {
		return err
	}
	return checkIfURLIsValid(info, availableSize, url.String())
}

// ConvertToRawStream converts an http accessible image to raw format without locally caching the image
func ConvertToRawStream(url *url.URL, dest string, preallocate bool, cacheMode string) error {
	return qemuInterface.ConvertToRawStream(url, dest, preallocate, cacheMode)
}

// Validate does basic validation of a qemu image
func Validate(url *url.URL, availableSize int64) error {
	return qemuInterface.Validate(url, availableSize)
}

func reportProgress(line string) {
	// (45.34/100%)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 2 && ownerUID != "" {
		klog.V(1).Info(matches[1])
		// Don't need to check for an error, the regex made sure its a number we can parse.
		v, _ := strconv.ParseFloat(matches[1], 64)
		progress, err := metrics.Progress(ownerUID).Get()
		if err == nil && v > 0 && v > progress {
			metrics.Progress(ownerUID).Add(v - progress)
		}
	}
}

// CreateBlankImage creates empty raw image
func CreateBlankImage(dest string, size resource.Quantity, preallocate bool) error {
	klog.V(1).Infof("creating raw image with size %s, preallocation %v", size.String(), preallocate)
	return qemuInterface.CreateBlankImage(dest, size, preallocate)
}

// CreateBlankImage creates a raw image with a given size
func (o *qemuOperations) CreateBlankImage(dest string, size resource.Quantity, preallocate bool) error {
	klog.V(3).Infof("image size is %s", size.String())
	args := []string{"create", "-f", "raw", dest, convertQuantityToQemuSize(size)}
	if preallocate {
		klog.V(1).Infof("Added preallocation")
		args = append(args, []string{"-o", "preallocation=falloc"}...)
	}
	err := o.cmd.Exec(args...)
	if err != nil {
		os.Remove(dest)
		return errors.Wrap(err, fmt.Sprintf("could not create raw image with size %s in %s", size.String(), dest))
	}
	// Change permissions to 0660
	err = os.Chmod(dest, 0660)
	if err != nil {
		err = errors.Wrap(err, "Unable to change permissions of target file")
		return err
	}

	return nil
}

func (o *qemuOperations) execPreallocationBlock(dest string, bs, count, offset int64) error {
	oflag := "oflag=seek_bytes"
	supportDirectIO, err := odirectChecker.CheckBlockDevice(dest)
	if err != nil {
		return err
	}
	if supportDirectIO {
		oflag += ",direct"
	}
	args := []string{"if=/dev/zero", "of=" + dest, fmt.Sprintf("bs=%d", bs), fmt.Sprintf("count=%d", count), fmt.Sprintf("seek=%d", offset), oflag}
	err = o.cmd.ExecRaw("dd", args...)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Could not preallocate blank block volume at %s, running dd for size %d, offset %d", dest, bs*count, offset))
	}

	return nil
}

// PreallocateBlankBlock writes requested amount of zeros to block device mounted at dest
func PreallocateBlankBlock(dest string, size resource.Quantity) error {
	return qemuInterface.PreallocateBlankBlock(dest, size)
}

func (o *qemuOperations) PreallocateBlankBlock(dest string, size resource.Quantity) error {
	klog.V(3).Infof("block volume size is %s", size.String())

	qemuSize, err := strconv.ParseInt(convertQuantityToQemuSize(size), 10, 64)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Could not parse size for preallocating blank block volume at %s with size %s", dest, size.String()))
	}
	countBlocks, remainder := qemuSize/units.MiB, qemuSize%units.MiB
	err = o.execPreallocationBlock(dest, units.MiB, countBlocks, 0)
	if err != nil {
		return err
	}
	if remainder != 0 {
		return o.execPreallocationBlock(dest, remainder, 1, countBlocks*units.MiB)
	}
	return nil
}

func addPreallocation(args []string, preallocationMethods [][]string, qemuFn func(args []string) error) error {
	var err error
	for _, preallocationMethod := range preallocationMethods {
		klog.V(1).Infof("Adding preallocation method: %v", preallocationMethod)
		// For some subcommands (e.g. resize), preallocation options must come before other options
		argsToTry := append([]string{args[0]}, preallocationMethod...)
		argsToTry = append(argsToTry, args[1:]...)
		klog.V(1).Infof("Attempting preallocation method, qemu-img args: %v", argsToTry)

		err = qemuFn(argsToTry)
		if err != nil && isUnsupportedPreallocation(err) {
			klog.V(1).Infof("Unsupported preallocation mode. Retrying")
		} else {
			break
		}
	}

	return err
}

func isUnsupportedPreallocation(err error) bool {
	var execErr *cmdExecError
	if errors.As(err, &execErr) {
		return strings.Contains(execErr.stderr, "Unsupported preallocation mode")
	}
	return strings.Contains(err.Error(), "Unsupported preallocation mode")
}

// Rebase changes a QCOW's backing file to point to a previously-downloaded base image.
// Depends on original image having been downloaded as raw.
func (o *qemuOperations) Rebase(backingFile string, delta string) error {
	klog.V(1).Infof("Rebasing %s onto %s", delta, backingFile)
	return o.cmd.ExecWithProgress("rebase", "-p", "-u", "-F", "raw", "-b", backingFile, delta)
}

// Commit takes the changes written to a QCOW and applies them to its raw backing file.
func (o *qemuOperations) Commit(image string) error {
	klog.V(1).Infof("Committing %s to backing file...", image)
	return o.cmd.ExecWithProgress("commit", "-p", image)
}
