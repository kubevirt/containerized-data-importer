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

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/system"
	"kubevirt.io/containerized-data-importer/pkg/util"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	networkTimeoutSecs = 3600    //max is 10000
	maxMemory          = 1 << 30 //value from OpenStack Nova
	maxCPUSecs         = 30      //value from OpenStack Nova
	matcherString      = "\\((\\d?\\d\\.\\d\\d)\\/100%\\)"
)

// QEMUOperations defines the interface for executing qemu subprocesses
type QEMUOperations interface {
	ConvertQcow2ToRaw(string, string) error
	ConvertQcow2ToRawStream(*url.URL, string) error
	Validate(string, string) error
}

type qemuOperations struct{}

var (
	qemuExecFunction = system.ExecWithLimits
	qemuLimits       = &system.ProcessLimitValues{AddressSpaceLimit: maxMemory, CPUTimeLimit: maxCPUSecs}
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
	prometheus.MustRegister(progress)
	ownerUID, _ = util.ParseEnvVar(common.OwnerUID, false)
}

// NewQEMUOperations returns the default implementation of QEMUOperations
func NewQEMUOperations() QEMUOperations {
	return &qemuOperations{}
}

func (o *qemuOperations) ConvertQcow2ToRaw(src, dest string) error {
	_, err := qemuExecFunction(qemuLimits, nil, "qemu-img", "convert", "-p", "-f", "qcow2", "-O", "raw", src, dest)
	if err != nil {
		os.Remove(dest)
		return errors.Wrap(err, "could not convert local qcow2 image to raw")
	}

	return nil
}

func (o *qemuOperations) ConvertQcow2ToRawStream(url *url.URL, dest string) error {
	jsonArg := fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.url\": \"%s\", \"file.timeout\": %d}", url.Scheme, url, networkTimeoutSecs)

	_, err := qemuExecFunction(qemuLimits, reportProgress, "qemu-img", "convert", "-p", "-f", "qcow2", "-O", "raw", jsonArg, dest)
	if err != nil {
		os.Remove(dest)
		return errors.Wrap(err, "could not stream/convert qcow2 image to raw")
	}

	return nil
}

func (o *qemuOperations) Validate(image, format string) error {
	type imageInfo struct {
		Format      string `json:"format"`
		BackingFile string `json:"backing-filename"`
	}

	output, err := qemuExecFunction(qemuLimits, reportProgress, "qemu-img", "info", "--output=json", image)
	if err != nil {
		return errors.Wrapf(err, "Error getting info on image %s", image)
	}

	var info imageInfo
	err = json.Unmarshal(output, &info)
	if err != nil {
		glog.Errorf("Invalid JSON:\n%s\n", string(output))
		return errors.Wrapf(err, "Invalid json for image %s", image)
	}

	if info.Format != format {
		return errors.Errorf("Invalid format %s for image %s", info.Format, image)
	}

	if len(info.BackingFile) > 0 {
		return errors.Errorf("Image %s is invalid because it has backing file %s", image, info.BackingFile)
	}

	return nil
}

// ConvertQcow2ToRaw is a wrapper for qemu-img convert which takes a qcow2 file (specified by src) and converts
// it to a raw image (written to the provided dest file)
func ConvertQcow2ToRaw(src, dest string) error {
	return qemuIterface.ConvertQcow2ToRaw(src, dest)
}

// ConvertQcow2ToRawStream converts an http accessible qcow2 image to raw format without locally caching the qcow2 image
func ConvertQcow2ToRawStream(url *url.URL, dest string) error {
	return qemuIterface.ConvertQcow2ToRawStream(url, dest)
}

// Validate does basic validation of a qemu image
func Validate(image, format string) error {
	return qemuIterface.Validate(image, format)
}

func reportProgress(line string) {
	// (45.34/100%)
	matches := re.FindStringSubmatch(line)
	if len(matches) == 2 && ownerUID != "" {
		glog.V(1).Info(matches[1])
		// Don't need to check for an error, the regex made sure its a number we can parse.
		v, _ := strconv.ParseFloat(matches[1], 64)
		progress.WithLabelValues(ownerUID).Set(v)
	}
}
