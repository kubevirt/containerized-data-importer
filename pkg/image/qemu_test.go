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
	"reflect"
	"strings"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"

	dto "github.com/prometheus/client_model/go"

	"kubevirt.io/containerized-data-importer/pkg/system"

	"github.com/prometheus/client_golang/prometheus"
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

const hugeValidateJSON = `
{
    "virtual-size": 52949672960,
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
    "format": "raw2",
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

type execFunctionType func(*system.ProcessLimitValues, func(string), string, ...string) ([]byte, error)

func init() {
	ownerUID = "1111-1111-111"
}

var expectedLimits = &system.ProcessLimitValues{AddressSpaceLimit: 1 << 30, CPUTimeLimit: 30}

var _ = Describe("Convert to Raw", func() {
	It("should return no error if exec function returns no error", func() {
		replaceExecFunction(mockExecFunction("", "", nil, "convert", "-p", "-O", "raw", "source", "dest"), func() {
			err := convertToRaw("source", "dest")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should return conversion error if exec function returns error", func() {
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "convert", "-p", "-O", "raw", "source", "dest"), func() {
			err := convertToRaw("source", "dest")
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not convert image to raw")).To(BeTrue())
		})
	})

	It("should stream file to destination", func() {
		replaceExecFunction(mockExecFunction("", "", nil, "convert", "-p", "-O", "raw", "/somefile/somewhere", "dest"), func() {
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ConvertToRawStream(ep, "dest")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should stream valid url to destination", func() {
		ep, err := url.Parse("http://someurl/somewhere")
		Expect(err).NotTo(HaveOccurred())
		jsonArg := fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.url\": \"%s\", \"file.timeout\": %d}", ep.Scheme, ep, networkTimeoutSecs)
		replaceExecFunction(mockExecFunction("", "", nil, "convert", "-p", "-O", "raw", jsonArg, "dest"), func() {
			err = ConvertToRawStream(ep, "dest")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should return conversion error if exec function returns error for url", func() {
		ep, err := url.Parse("http://someurl/somewhere")
		Expect(err).NotTo(HaveOccurred())
		jsonArg := fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.url\": \"%s\", \"file.timeout\": %d}", ep.Scheme, ep, networkTimeoutSecs)
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "convert", "-p", "-O", "raw", jsonArg, "dest"), func() {
			err := ConvertToRawStream(ep, "dest")
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not stream/convert image to raw")).To(BeTrue())
		})
	})

})

var _ = Describe("Resize", func() {
	It("Should complete successfully if qemu-img resize succeeds", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "", nil, "resize", "-f", "raw", "image", size), func() {
			o := NewQEMUOperations()
			err = o.Resize("image", quantity)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should fail if qemu-img resize fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "resize", "-f", "raw", "image", size), func() {
			o := NewQEMUOperations()
			err = o.Resize("image", quantity)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "Error resizing image image")).To(BeTrue())
		})
	})
})

var _ = Describe("Validate", func() {
	imageName, _ := url.Parse("myimage.qcow2")
	httpImage, _ := url.Parse("http://someurl/somewhere")
	jsonArg := fmt.Sprintf("json: {\"file.driver\": \"%s\", \"file.url\": \"%s\", \"file.timeout\": %d}", httpImage.Scheme, httpImage, networkTimeoutSecs)

	table.DescribeTable("Validate should", func(execfunc execFunctionType, errString string, image *url.URL) {
		replaceExecFunction(execfunc, func() {
			err := Validate(image, 42949672960)

			if errString == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				rootErr := errors.Cause(err)
				if rootErr.Error() != errString {
					Fail(fmt.Sprintf("got wrong failure: [%s], expected [%s]", rootErr, errString))
				}
			}
		})
	},
		table.Entry("should return success", mockExecFunction(goodValidateJSON, "", expectedLimits, "info", "--output=json", imageName.String()), "", imageName),
		table.Entry("should return success for http url", mockExecFunction(goodValidateJSON, "", expectedLimits, "info", "--output=json", jsonArg), "", httpImage),
		table.Entry("should return error", mockExecFunction("explosion", "exit 1", expectedLimits), "explosion, exit 1", imageName),
		table.Entry("should return error on bad json", mockExecFunction(badValidateJSON, "", expectedLimits), "unexpected end of JSON input", imageName),
		table.Entry("should return error on bad format", mockExecFunction(badFormatValidateJSON, "", expectedLimits), fmt.Sprintf("Invalid format raw2 for image %s", imageName), imageName),
		table.Entry("should return error on invalid backing file", mockExecFunction(backingFileValidateJSON, "", expectedLimits), fmt.Sprintf("Image %s is invalid because it has backing file backing-file.qcow2", imageName), imageName),
		table.Entry("should return error when PVC is too small", mockExecFunction(hugeValidateJSON, "", expectedLimits), fmt.Sprintf("Virtual image size %d is larger than available size %d. A larger PVC is required.", 52949672960, 42949672960), imageName),
	)

})

var _ = Describe("Report Progress", func() {
	BeforeEach(func() {
		progress = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "import_progress",
				Help: "The import progress in percentage",
			},
			[]string{"ownerUID"},
		)
	})

	It("Parse valid progress line", func() {
		By("Verifying the initial value is 0")
		progress.WithLabelValues(ownerUID).Add(0)
		metric := &dto.Metric{}
		err := progress.WithLabelValues(ownerUID).Write(metric)
		Expect(err).NotTo(HaveOccurred())
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling reportProgress with value")
		reportProgress("(45.34/100%)")
		err = progress.WithLabelValues(ownerUID).Write(metric)
		Expect(err).NotTo(HaveOccurred())
		Expect(*metric.Counter.Value).To(Equal(45.34))
	})

	It("Parse invalid progress line", func() {
		By("Verifying the initial value is 0")
		progress.WithLabelValues(ownerUID).Add(0)
		metric := &dto.Metric{}
		err := progress.WithLabelValues(ownerUID).Write(metric)
		Expect(err).NotTo(HaveOccurred())
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling reportProgress with invalid value")
		reportProgress("45.34")
		err = progress.WithLabelValues(ownerUID).Write(metric)
		Expect(err).NotTo(HaveOccurred())
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
	})
})

var _ = Describe("quantity to qemu", func() {
	It("Should properly parse quantity to qemu", func() {
		result := convertQuantityToQemuSize(resource.MustParse("1Gi"))
		Expect(result).To(Equal("1073741824"))
		result = convertQuantityToQemuSize(resource.MustParse("1G"))
		Expect(result).To(Equal("1000000000"))
		result = convertQuantityToQemuSize(resource.MustParse("10Ki"))
		Expect(result).To(Equal("10240"))
		result = convertQuantityToQemuSize(resource.MustParse("10k"))
		Expect(result).To(Equal("10000"))
	})
})

var _ = Describe("Create blank image", func() {
	It("Should complete successfully if qemu-img resize succeeds", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "", nil, "create", "-f", "raw", "image", size), func() {
			err = CreateBlankImage("image", quantity)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should fail if qemu-img resize fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "create", "-f", "raw", "image", size), func() {
			err = CreateBlankImage("image", quantity)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not create raw image with size ")).To(BeTrue())
		})
	})
})

func mockExecFunction(output, errString string, expectedLimits *system.ProcessLimitValues, checkArgs ...string) execFunctionType {
	return func(limits *system.ProcessLimitValues, f func(string), cmd string, args ...string) (bytes []byte, err error) {
		Expect(reflect.DeepEqual(expectedLimits, limits)).To(BeTrue())

		for _, ca := range checkArgs {
			found := false
			for _, a := range args {
				if ca == a {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		}

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
