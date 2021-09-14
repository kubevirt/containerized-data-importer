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
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
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
	var tmpDir, destPath string

	BeforeEach(func() {
		tmpDir, err := ioutil.TempDir(os.TempDir(), "qemutestdest")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
		destPath = filepath.Join(tmpDir, "dest")
		_, err = os.Create(destPath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("should return no error if exec function returns no error", func() {
		replaceExecFunction(mockExecFunction("", "", nil, "convert", "-p", "-O", "raw", "source", destPath), func() {
			err := convertToRaw("source", destPath, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should return conversion error if exec function returns error", func() {
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "convert", "-p", "-O", "raw", "source", destPath), func() {
			err := convertToRaw("source", destPath, false)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not convert image to raw")).To(BeTrue())
		})
	})

	It("should stream file to destination", func() {
		replaceExecFunction(mockExecFunction("", "", nil, "convert", "-p", "-O", "raw", "/somefile/somewhere", destPath), func() {
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ConvertToRawStream(ep, destPath, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should add preallocation if requested", func() {
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "convert", "-o", "preallocation=falloc", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", destPath), func() {
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ConvertToRawStream(ep, destPath, true)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should not add preallocation if not requested", func() {
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "convert", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", destPath), func() {
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ConvertToRawStream(ep, destPath, false)
			Expect(err).NotTo(HaveOccurred())
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
			err = o.Resize("image", quantity, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should fail if qemu-img resize fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "resize", "-f", "raw", "image", size), func() {
			o := NewQEMUOperations()
			err = o.Resize("image", quantity, false)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "Error resizing image image")).To(BeTrue())
		})
	})
})

var _ = Describe("Validate", func() {
	imageName, _ := url.Parse("myimage.qcow2")

	table.DescribeTable("Validate should", func(execfunc execFunctionType, errString string, image *url.URL, overhead float64) {
		replaceExecFunction(execfunc, func() {
			err := Validate(image, 42949672960, overhead)

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
		table.Entry("should return success", mockExecFunction(goodValidateJSON, "", expectedLimits, "info", "--output=json", imageName.String()), "", imageName, 0.0),
		table.Entry("should return error", mockExecFunction("explosion", "exit 1", expectedLimits), "explosion, exit 1", imageName, 0.0),
		table.Entry("should return error on bad json", mockExecFunction(badValidateJSON, "", expectedLimits), "unexpected end of JSON input", imageName, 0.0),
		table.Entry("should return error on bad format", mockExecFunction(badFormatValidateJSON, "", expectedLimits), fmt.Sprintf("Invalid format raw2 for image %s", imageName), imageName, 0.0),
		table.Entry("should return error on invalid backing file", mockExecFunction(backingFileValidateJSON, "", expectedLimits), fmt.Sprintf("Image %s is invalid because it has invalid backing file backing-file.qcow2", imageName), imageName, 0.0),
		table.Entry("should return error when PVC is too small", mockExecFunction(hugeValidateJSON, "", expectedLimits), fmt.Sprintf("Virtual image size %d is larger than available size %d (PVC size %d, reserved overhead %f%%). A larger PVC is required.", 52949672960, 42949672960, 52949672960, 0.0), imageName, 0.0),
		table.Entry("should return error when PVC is too small with overhead", mockExecFunction(hugeValidateJSON, "", expectedLimits), fmt.Sprintf("Virtual image size %d is larger than available size %d (PVC size %d, reserved overhead %f%%). A larger PVC is required.", 52949672960, 34359738368, 52949672960, 0.2), imageName, 0.2),
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
			err = CreateBlankImage("image", quantity, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should fail if qemu-img resize fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "create", "-f", "raw", "image", size), func() {
			err = CreateBlankImage("image", quantity, false)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not create raw image with size ")).To(BeTrue())
		})
	})

	It("should add preallocation if requested", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "create", "-f", "raw", "image", size, "-o", "preallocation=falloc"), func() {
			err = CreateBlankImage("image", quantity, true)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should not add preallocation if not requested", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "create", "-f", "raw", "image", size), func() {
			err = CreateBlankImage("image", quantity, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("Try different preallocation modes", func() {
	It("Should try falloc first", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) ([]byte, error) {
			Expect(args).To(Equal([]string{"command", "-o", "preallocation=falloc"}))
			calledCount++
			return []byte{}, nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calledCount).To(Equal(1))
	})

	It("Should try full if falloc fails", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) ([]byte, error) {
			if args[2] == "preallocation=falloc" {
				calledCount++
				return []byte("Unsupported preallocation mode"), fmt.Errorf("No, no, no")
			}
			Expect(args).To(Equal([]string{"command", "-o", "preallocation=full"}))
			calledCount++
			return []byte{}, nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calledCount).To(Equal(2))
	})

	It("Should try -S0 if full fails", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) ([]byte, error) {
			if calledCount < 2 {
				calledCount++
				return []byte("Unsupported preallocation mode"), fmt.Errorf("No, no, no")
			}
			Expect(args).To(Equal([]string{"command", "-S", "0"}))
			calledCount++
			return []byte{}, nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calledCount).To(Equal(3))
	})

	It("Should fail if output is different than 'Unsupported preallocation'", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) ([]byte, error) {
			calledCount++
			return []byte("General Protection Fault"), fmt.Errorf("No, no, no")
		})

		Expect(err).To(HaveOccurred())
		Expect(calledCount).To(Equal(1))
	})
})

var _ = Describe("Rebase and commit", func() {
	It("Should successfully rebase image", func() {
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "rebase", "-p", "-u", "-F", "raw", "-b", "backing-file", "delta"), func() {
			o := NewQEMUOperations()
			err := o.Rebase("backing-file", "delta")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should successfully commit image to base", func() {
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "commit", "-p", "delta"), func() {
			o := NewQEMUOperations()
			err := o.Commit("delta")
			Expect(err).NotTo(HaveOccurred())
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

func mockExecFunctionStrict(output, errString string, expectedLimits *system.ProcessLimitValues, checkArgs ...string) execFunctionType {
	return func(limits *system.ProcessLimitValues, f func(string), cmd string, args ...string) (bytes []byte, err error) {
		Expect(reflect.DeepEqual(expectedLimits, limits)).To(BeTrue())

		Expect(checkArgs).To(Equal(args))

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
