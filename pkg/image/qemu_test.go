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

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"

	dto "github.com/prometheus/client_model/go"

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

type progressFunctionType func(string)

type execFunctionType func(*system.ProcessLimitValues, func(string), string, ...string) ([]byte, error)

func init() {
	ownerUID = "1111-1111-111"
}

var _ = Describe("Importer", func() {
	source := "/upload/myimage.qcow2"
	dest := "/data/disk.img"
	sourceStream, _ := url.Parse("http://localhost:8080/myimage.qcow2")
	destStream := "/tmp/myimage.qcow2"
	imageName := "myimage.qcow2"

	table.DescribeTable("with import source should", func(execfunc execFunctionType, errString string, errFunc func() error) {
		replaceExecFunction(execfunc, func() {
			err := errFunc()

			if errString == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				rootErr := errors.Cause(err)
				if rootErr.Error() != errString {
					Fail(fmt.Sprintf("Got wrong failure: %s, expected %s", rootErr, errString))
				}
			}
		})
	},
		table.Entry("non-streaming convert success", mockExecFunction("", ""), "", func() error { return ConvertQcow2ToRaw(source, dest) }),
		table.Entry("non-streaming  convert qemu-img failure", mockExecFunction("", "exit status 1"), "exit status 1", func() error { return ConvertQcow2ToRaw(source, dest) }),
		table.Entry("streaming convert success", mockExecFunction("", ""), "", func() error { return ConvertQcow2ToRawStream(sourceStream, destStream) }),
		table.Entry("streaming  convert qemu-img failure", mockExecFunction("", "exit status 1"), "exit status 1", func() error { return ConvertQcow2ToRawStream(sourceStream, destStream) }),
	)

	table.DescribeTable("Validate should", func(execfunc execFunctionType, errString string) {
		replaceExecFunction(execfunc, func() {
			err := Validate(imageName, "qcow2")

			if errString == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				rootErr := errors.Cause(err)
				if rootErr.Error() != errString {
					Fail(fmt.Sprintf("got wrong failure: %s, expected %s", rootErr, errString))
				}
			}
		})
	},
		table.Entry("validate success", mockExecFunction(goodValidateJSON, ""), ""),
		table.Entry("validate error", mockExecFunction("", "exit 1"), "exit 1"),
		table.Entry("validate bad json", mockExecFunction(badValidateJSON, ""), "unexpected end of JSON input"),
		table.Entry("validate bad format", mockExecFunction(badFormatValidateJSON, ""), fmt.Sprintf("Invalid format raw for image %s", imageName)),
		table.Entry("validate has backing file", mockExecFunction(backingFileValidateJSON, ""), fmt.Sprintf("Image %s is invalid because it has backing file backing-file.qcow2", imageName)),
	)

})

var _ = Describe("Report Progress", func() {
	It("Parse valid progress line", func() {
		By("Verifying the initial value is 0")
		progress.WithLabelValues(ownerUID).Set(0)
		metric := &dto.Metric{}
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling reportProgress with value")
		reportProgress("(45.34/100%)")
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(45.34))
	})

	It("Parse invalid progress line", func() {
		By("Verifying the initial value is 0")
		progress.WithLabelValues(ownerUID).Set(0)
		metric := &dto.Metric{}
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling reportProgress with invalid value")
		reportProgress("45.34")
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
	})
})

var _ = Describe("quantity to qemu", func() {
	It("Should properly parse quantity to qemu", func() {
		result := convertQuantityToQemuSize(resource.MustParse("1Gi"))
		Expect(result).To(Equal("1G"))
		result = convertQuantityToQemuSize(resource.MustParse("10Ki"))
		Expect(result).To(Equal("10k"))
	})
})

func mockExecFunction(output, errString string) execFunctionType {
	return func(*system.ProcessLimitValues, func(string), string, ...string) (bytes []byte, err error) {
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
