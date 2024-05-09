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
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	dto "github.com/prometheus/client_model/go"

	"k8s.io/apimachinery/pkg/api/resource"

	"kubevirt.io/containerized-data-importer/pkg/common"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
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
		var err error
		// dest is usually not tmpfs, stay honest in unit tests as well
		tmpDir, err = os.MkdirTemp("/var/tmp", "qemutestdest")
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
			err := convertToRaw("source", destPath, false, "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should return conversion error if exec function returns error", func() {
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "convert", "-p", "-O", "raw", "source", destPath), func() {
			err := convertToRaw("source", destPath, false, "")
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not convert image to raw")).To(BeTrue())
		})
	})

	It("should stream file to destination", func() {
		replaceExecFunction(mockExecFunction("", "", nil, "convert", "-p", "-O", "raw", "/somefile/somewhere", destPath), func() {
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ConvertToRawStream(ep, destPath, false, "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should add preallocation if requested", func() {
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "convert", "-o", "preallocation=falloc", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", destPath), func() {
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ConvertToRawStream(ep, destPath, true, "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("should not add preallocation if not requested", func() {
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "convert", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", destPath), func() {
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ConvertToRawStream(ep, destPath, false, "")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("cache mode adjusted according to O_DIRECT support", func() {
		var tmpFsDir string

		BeforeEach(func() {
			var err error

			tmpFsDir, err = os.MkdirTemp("/mnt/cditmpfs", "qemutestdestontmpfs")
			Expect(err).NotTo(HaveOccurred())
			By("tmpFsDir: " + tmpFsDir)
		})

		AfterEach(func() {
			os.RemoveAll(tmpFsDir)
		})

		It("should use cache=none when destination supports O_DIRECT", func() {
			replaceExecFunction(mockExecFunctionStrict("", "", nil, "convert", "-t", "none", "-p", "-O", "raw", "/somefile/somewhere", destPath), func() {
				ep, err := url.Parse("/somefile/somewhere")
				Expect(err).NotTo(HaveOccurred())
				err = ConvertToRawStream(ep, destPath, false, common.CacheModeTryNone)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		It("should use cache=writeback when destination does not support O_DIRECT", func() {
			// ensure tmpfs destination
			out, err := exec.Command("/usr/bin/findmnt", "-T", tmpFsDir, "-o", "FSTYPE").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(string(out)).To(ContainSubstring("tmpfs"))

			tmpFsDestPath := filepath.Join(tmpFsDir, "dest")
			_, err = os.Create(tmpFsDestPath)
			Expect(err).NotTo(HaveOccurred())

			replaceExecFunction(mockExecFunctionStrict("", "", nil, "convert", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", tmpFsDestPath), func() {
				ep, err := url.Parse("/somefile/somewhere")
				Expect(err).NotTo(HaveOccurred())
				err = ConvertToRawStream(ep, tmpFsDestPath, false, common.CacheModeTryNone)
				Expect(err).NotTo(HaveOccurred())
			})
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
			Expect(strings.Contains(err.Error(), "Error resizing image")).To(BeTrue())
		})
	})
})

var _ = Describe("Validate", func() {
	imageName, _ := url.Parse("myimage.qcow2")

	DescribeTable("Validate should", func(execfunc execFunctionType, errString string, image *url.URL) {
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
		Entry("should return success", mockExecFunction(goodValidateJSON, "", expectedLimits, "info", "--output=json", imageName.String()), "", imageName),
		Entry("should return error", mockExecFunction("explosion", "exit 1", expectedLimits), "explosion, exit 1", imageName),
		Entry("should return error on bad json", mockExecFunction(badValidateJSON, "", expectedLimits), "unexpected end of JSON input", imageName),
		Entry("should return error on bad format", mockExecFunction(badFormatValidateJSON, "", expectedLimits), fmt.Sprintf("Invalid format raw2 for image %s", imageName), imageName),
		Entry("should return error on invalid backing file", mockExecFunction(backingFileValidateJSON, "", expectedLimits), fmt.Sprintf("Image %s is invalid because it has invalid backing file backing-file.qcow2", imageName), imageName),
		Entry("should return error when PVC is too small", mockExecFunction(hugeValidateJSON, "", expectedLimits), fmt.Sprintf("Virtual image size %d is larger than the reported available storage %d. A larger PVC is required.", 52949672960, 42949672960), imageName),
	)

})

var _ = Describe("Report Progress", func() {
	BeforeEach(func() {
		metrics.InitCloneProgressCounterVec()
	})

	It("Parse valid progress line", func() {
		By("Verifying the initial value is 0")
		metrics.AddCloneProgress(ownerUID, 0)
		metric := &dto.Metric{}
		err := metrics.WriteCloneProgress(ownerUID, metric)
		Expect(err).NotTo(HaveOccurred())
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling reportProgress with value")
		reportProgress("(45.34/100%)")
		err = metrics.WriteCloneProgress(ownerUID, metric)
		Expect(err).NotTo(HaveOccurred())
		Expect(*metric.Counter.Value).To(Equal(45.34))
	})

	It("Parse invalid progress line", func() {
		By("Verifying the initial value is 0")
		metrics.AddCloneProgress(ownerUID, 0)
		metric := &dto.Metric{}
		err := metrics.WriteCloneProgress(ownerUID, metric)
		Expect(err).NotTo(HaveOccurred())
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling reportProgress with invalid value")
		reportProgress("45.34")
		err = metrics.WriteCloneProgress(ownerUID, metric)
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
	var tmpDir, destPath string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp(os.TempDir(), "qemutestdest")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
		destPath = filepath.Join(tmpDir, "dest")
		_, err = os.Create(destPath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("Should complete successfully if qemu-img resize succeeds", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "", nil, "create", "-f", "raw", destPath, size), func() {
			err = CreateBlankImage(destPath, quantity, false)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	It("Should fail if qemu-img resize fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "create", "-f", "raw", destPath, size), func() {
			err = CreateBlankImage(destPath, quantity, false)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not create raw image with size ")).To(BeTrue())
		})
	})

	It("should add preallocation if requested", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "create", "-f", "raw", destPath, size, "-o", "preallocation=falloc"), func() {
			err = CreateBlankImage(destPath, quantity, true)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	It("should not add preallocation if not requested", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunctionStrict("", "", nil, "create", "-f", "raw", destPath, size), func() {
			err = CreateBlankImage(destPath, quantity, false)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("Create preallocated blank block", func() {
	var tmpDir, tmpFsDir, destPath string

	BeforeEach(func() {
		var err error
		// dest is usually not tmpfs, stay honest in unit tests as well
		tmpDir, err = os.MkdirTemp("/var/tmp", "qemutestdest")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
		destPath = filepath.Join(tmpDir, "dest")
		_, err = os.Create(destPath)
		Expect(err).NotTo(HaveOccurred())

		tmpFsDir, err = os.MkdirTemp("/mnt/cditmpfs", "qemutestdestontmpfs")
		Expect(err).NotTo(HaveOccurred())
		By("tmpFsDir: " + tmpFsDir)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
		os.RemoveAll(tmpFsDir)
	})

	It("Should complete successfully if preallocation succeeds", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		replaceExecFunction(mockExecFunction("", "", nil, "if=/dev/zero", "of="+destPath, "bs=1048576", "count=10240", "seek=0", "oflag=seek_bytes,direct"), func() {
			err = PreallocateBlankBlock(destPath, quantity)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should complete successfully with tmpfs dest without O_DIRECT if preallocation succeeds", func() {
		tmpFsDestPath := filepath.Join(tmpFsDir, "dest")
		_, err := os.Create(tmpFsDestPath)
		Expect(err).NotTo(HaveOccurred())
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		replaceExecFunction(mockExecFunction("", "", nil, "if=/dev/zero", "of="+tmpFsDestPath, "bs=1048576", "count=10240", "seek=0", "oflag=seek_bytes"), func() {
			err = PreallocateBlankBlock(tmpFsDestPath, quantity)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should complete successfully with value not aligned to 1MiB", func() {
		quantity, err := resource.ParseQuantity("5243392Ki")
		Expect(err).NotTo(HaveOccurred())
		firstCallArgs := []string{"if=/dev/zero", "of=" + destPath, "bs=1048576", "count=5120", "seek=0", "oflag=seek_bytes,direct"}
		secondCallArgs := []string{"if=/dev/zero", "of=" + destPath, "bs=524288", "count=1", "seek=5368709120", "oflag=seek_bytes,direct"}
		replaceExecFunction(mockExecFunctionTwoCalls("", "", nil, firstCallArgs, secondCallArgs), func() {
			err = PreallocateBlankBlock(destPath, quantity)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should fail if preallocation fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "if=/dev/zero", "of="+destPath, "bs=1048576", "count=10240", "seek=0", "oflag=seek_bytes,direct"), func() {
			err = PreallocateBlankBlock(destPath, quantity)
			Expect(strings.Contains(err.Error(), "Could not preallocate blank block volume at")).To(BeTrue())
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
			// if not found will fail and show the diff in the args
			if found != true {
				Expect(checkArgs).To(Equal(args))
			}
		}

		if output != "" {
			bytes = []byte(output)
		}
		if errString != "" {
			err = errors.New(errString)
		}

		return bytes, err
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

		return bytes, err
	}
}

func mockExecFunctionTwoCalls(output, errString string, expectedLimits *system.ProcessLimitValues, firstCallArgs []string, secondCallArgs []string) execFunctionType {
	firstCall := true
	return func(limits *system.ProcessLimitValues, f func(string), cmd string, args ...string) (bytes []byte, err error) {
		Expect(reflect.DeepEqual(expectedLimits, limits)).To(BeTrue())

		if firstCall {
			Expect(firstCallArgs).To(Equal(args))
			firstCall = false
		} else {
			Expect(secondCallArgs).To(Equal(args))
		}

		if output != "" {
			bytes = []byte(output)
		}
		if errString != "" {
			err = errors.New(errString)
		}

		return bytes, err
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
