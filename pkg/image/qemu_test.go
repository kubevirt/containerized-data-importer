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
	"fmt"
	"net/url"
	"testing"

	"github.com/pkg/errors"

	"kubevirt.io/containerized-data-importer/pkg/system"
)

const goodValidateJSON = `
{
    "virtual-size": 4294967296,
    "filename": "myimage.qcow2",
    "cluster-size": 65536,
    "format": "qcow2",
    "actual-size": 262152192,
    "format-specific": {
        "type": "qcow2",
        "data": {
            "compat": "0.10",
            "refcount-bits": 16
        }
    },
    "dirty-flag": false
}
`

const badValidateJSON = `
{
    "virtual-size": 4294967296,
    "filename": "myimage.qcow2",
    "cluster-size": 65536,
    "format": "qcow2",
    "actual-size": 262152192,
    "format-specific": {
        "type": "qcow2",
        "data": {
            "compat": "0.10",
            "refcount-bits": 16
        }
    },
    "dirty-flag": false
`

const badFormatValidateJSON = `
{
    "virtual-size": 4294967296,
    "filename": "myimage.qcow2",
    "cluster-size": 65536,
    "format": "raw",
    "actual-size": 262152192,
    "dirty-flag": false
}
`

const backingFileValidateJSON = `
{
    "virtual-size": 4294967296,
    "filename": "myimage.qcow2",
    "cluster-size": 65536,
    "format": "qcow2",
    "actual-size": 262152192,
    "format-specific": {
        "type": "qcow2",
        "data": {
            "compat": "0.10",
            "refcount-bits": 16
        }
	},
	"backing-filename": "backing-file.qcow2",
    "dirty-flag": false
}
`

type execFunctionType func(*system.ProcessLimitValues, string, ...string) ([]byte, error)

type convertTest struct {
	name      string
	execFunc  execFunctionType
	errString string
}

func TestConvertQcow2ToRaw(t *testing.T) {
	const (
		source = "/upload/myimage.qcow2"
		dest   = "/data/disk.img"
	)

	runConvertTests(t, "non-streaming", func() error { return ConvertQcow2ToRaw(source, dest) })
}

func TestConvertQcow2ToRawStream(t *testing.T) {
	source, _ := url.Parse("http://localhost:8080/myimage.qcow2")
	dest := "/tmp/myimage.qcow2"

	runConvertTests(t, "streaming", func() error { return ConvertQcow2ToRawStream(source, dest) })
}

func runConvertTests(t *testing.T, prefix string, f func() error) {
	const (
		source = "/upload/myimage.qcow2"
		dest   = "/data/disk.img"
	)

	tests := []convertTest{
		{
			name:      prefix + " convert success",
			execFunc:  mockExecFunction("", ""),
			errString: "",
		},
		{
			name:      prefix + " convert qemu-img failure",
			execFunc:  mockExecFunction("", "exit status 1"),
			errString: "exit status 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replaceExecFunction(tt.execFunc, func() {
				err := f()

				if err != nil {
					if tt.errString == "" {
						t.Errorf("'%s' got unexpected failure: %s", tt.name, err)
					} else {
						rootErr := errors.Cause(err)
						if rootErr.Error() != tt.errString {
							t.Errorf("'%s' got wrong failure: %s, expected %s", tt.name, rootErr, tt.errString)
						}
					}

				} else if tt.errString != "" {
					t.Errorf("'%s' got unexpected success, expected: %s", tt.name, tt.errString)
				}
			})
		})
	}
}

func TestValidate(t *testing.T) {
	const imageName = "myimage.qcow2"

	tests := []struct {
		name      string
		execFunc  execFunctionType
		errString string
	}{
		{
			name:      "validate success",
			execFunc:  mockExecFunction(goodValidateJSON, ""),
			errString: "",
		},
		{
			name:      "validate error",
			execFunc:  mockExecFunction("", "exit 1"),
			errString: "exit 1",
		},
		{
			name:      "validate bad json",
			execFunc:  mockExecFunction(badValidateJSON, ""),
			errString: "unexpected end of JSON input",
		},
		{
			name:      "validate bad format",
			execFunc:  mockExecFunction(badFormatValidateJSON, ""),
			errString: fmt.Sprintf("Invalid format raw for image %s", imageName),
		},
		{
			name:      "validate has backing file",
			execFunc:  mockExecFunction(backingFileValidateJSON, ""),
			errString: fmt.Sprintf("Image %s is invalid because it has backing file backing-file.qcow2", imageName),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replaceExecFunction(tt.execFunc, func() {
				err := Validate(imageName, "qcow2")

				if err != nil {
					if tt.errString == "" {
						t.Errorf("'%s' got unexpected failure: %s", tt.name, err)
					} else {
						rootErr := errors.Cause(err)
						if rootErr.Error() != tt.errString {
							t.Errorf("'%s' got wrong failure: %s, expected %s", tt.name, rootErr, tt.errString)
						}
					}

				} else if tt.errString != "" {
					t.Errorf("'%s' got unexpected success, expected: %s", tt.name, tt.errString)
				}
			})
		})
	}
}

func mockExecFunction(output, errString string) execFunctionType {
	return func(*system.ProcessLimitValues, string, ...string) (bytes []byte, err error) {
		if output != "" {
			bytes = []byte(output)
		}
		if errString != "" {
			err = errors.New(errString)
		}
		return
	}
}

func replaceExecFunction(replacement execFunctionType, f func()) {
	orig := qemuExecFunction
	if replacement != nil {
		qemuExecFunction = replacement
		defer func() { qemuExecFunction = orig }()
	}
	f()
}
