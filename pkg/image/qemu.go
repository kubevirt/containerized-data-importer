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
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

const (
	networkTimeoutSecs = 3600    //max is 10000
	maxMemory          = 1 << 30 //value from OpenStack Nova
	maxCPUSecs         = 30      //value from OpenStack Nova
)

// ConvertQcow2ToRaw is a wrapper for qemu-img convert which takes a qcow2 file (specified by src) and converts
// it to a raw image (written to the provided dest file)
func ConvertQcow2ToRaw(src, dest string) error {
	err := validate(src, "qcow2")
	if err != nil {
		return errors.Wrap(err, "image validation failed")
	}

	_, err = execWithLimits("qemu-img", "convert", "-f", "qcow2", "-O", "raw", src, dest)
	if err != nil {
		return errors.Wrap(err, "could not convert local qcow2 image to raw")
	}

	return nil
}

// ConvertQcow2ToRawStream converts an http accessible qcow2 image to raw format without locally caching the qcow2 image
func ConvertQcow2ToRawStream(url *url.URL, dest string) error {
	err := validate(url.String(), "qcow2")
	if err != nil {
		return errors.Wrap(err, "image validation failed")
	}

	jsonArg := fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.url\": \"%s\", \"file.timeout\": %d}", url.Scheme, url, networkTimeoutSecs)
	_, err = execWithLimits("qemu-img", "convert", "-f", "qcow2", "-O", "raw", jsonArg, dest)
	if err != nil {
		return errors.Wrap(err, "could not stream/convert qcow2 image to raw")
	}

	return nil
}

func validate(image, format string) error {
	type imageInfo struct {
		Format      string `json:"format"`
		BackingFile string `json:"backing-filename"`
	}

	output, err := execWithLimits("qemu-img", "info", "--output=json", image)
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

func execWithLimits(command string, args ...string) ([]byte, error) {
	var buf bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Start()
	if err != nil {
		return nil, errors.Wrapf(err, "Couldn't start %s", command)
	}
	defer cmd.Process.Kill()

	err = setAddressSpaceLimit(cmd.Process.Pid, maxMemory)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't set address space limit")
	}

	err = setCPUTimeLimit(cmd.Process.Pid, maxCPUSecs)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't set CPU time limit")
	}

	err = cmd.Wait()
	output := buf.Bytes()
	if err != nil {
		glog.Errorf("%s %s failed output is:\n", command, args)
		glog.Errorf("%s\n", string(output))
		return nil, errors.Wrapf(err, "%s execution failed", command)
	}

	return output, nil
}
