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
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/api/resource"

	"kubevirt.io/containerized-data-importer/pkg/common"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-importer"
	"kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

// FakeODirectRefusingOS mocks out certain OS calls to avoid perturbing the filesystem
// If a member of the form `*Fn` is set, that function will be called in place
// of the real call.
type FakeODirectRefusingOS struct{}

// Stat is a fake that returns an error
func (FakeODirectRefusingOS) Stat(path string) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

// Remove is a fake call that returns nil.
func (FakeODirectRefusingOS) Remove(path string) error {
	return nil
}

// OpenFile is a fake call that return nil.
func (FakeODirectRefusingOS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if flag&syscall.O_DIRECT != 0 {
		return nil, &os.PathError{Op: "open", Path: name, Err: syscall.EINVAL}
	}

	return nil, nil
}

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

type runCmdFunc func(context.Context, string, ...string) ([]byte, error)
type runCmdWithStreamFunc func(context.Context, func(string), string, ...string) error

func init() {
	ownerUID = "1111-1111-111"
}

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
		ops := newTestOpsWithStream(mockRunCmdWithStreaming("", "convert", "-p", "-O", "raw", "source", destPath))
		err := ops.convertToRaw("source", destPath, false, "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return conversion error if exec function returns error", func() {
		ops := newTestOpsWithStream(mockRunCmdWithStreaming("exit 1", "convert", "-p", "-O", "raw", "source", destPath))
		err := ops.convertToRaw("source", destPath, false, "")
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("could not convert image to raw")))
	})

	It("should stream file to destination", func() {
		ops := newTestOpsWithStream(mockRunCmdWithStreaming("", "convert", "-p", "-O", "raw", "/somefile/somewhere", destPath))
		ep, err := url.Parse("/somefile/somewhere")
		Expect(err).NotTo(HaveOccurred())
		err = ops.ConvertToRawStream(ep, destPath, false, "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should add preallocation if requested", func() {
		ops := newTestOpsWithStream(mockRunCmdWithStreamingStrict("", "convert", "-o", "preallocation=falloc", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", destPath))
		ep, err := url.Parse("/somefile/somewhere")
		Expect(err).NotTo(HaveOccurred())
		err = ops.ConvertToRawStream(ep, destPath, true, "")
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not add preallocation if not requested", func() {
		ops := newTestOpsWithStream(mockRunCmdWithStreamingStrict("", "convert", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", destPath))
		ep, err := url.Parse("/somefile/somewhere")
		Expect(err).NotTo(HaveOccurred())
		err = ops.ConvertToRawStream(ep, destPath, false, "")
		Expect(err).NotTo(HaveOccurred())
	})

	Context("cache mode adjusted according to O_DIRECT support", func() {
		var tmpFsDir string
		var originalODirectChecker DirectIOChecker

		BeforeEach(func() {
			var err error

			tmpFsDir, err = os.MkdirTemp("/var/tmp", "qemutestdestontmpfs")
			Expect(err).ToNot(HaveOccurred())
			By("tmpFsDir: " + tmpFsDir)
			originalODirectChecker = odirectChecker
		})

		AfterEach(func() {
			os.RemoveAll(tmpFsDir)
			odirectChecker = originalODirectChecker
		})

		It("should use cache=none when destination supports O_DIRECT", func() {
			ops := newTestOpsWithStream(mockRunCmdWithStreamingStrict("", "convert", "-t", "none", "-p", "-O", "raw", "/somefile/somewhere", destPath))
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ops.ConvertToRawStream(ep, destPath, false, common.CacheModeTryNone)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should use cache=writeback when destination does not support O_DIRECT", func() {
			odirectChecker = NewDirectIOChecker(FakeODirectRefusingOS{})

			tmpFsDestPath := filepath.Join(tmpFsDir, "dest")
			_, err := os.Create(tmpFsDestPath)
			Expect(err).NotTo(HaveOccurred())

			ops := newTestOpsWithStream(mockRunCmdWithStreamingStrict("", "convert", "-t", "writeback", "-p", "-O", "raw", "/somefile/somewhere", tmpFsDestPath))
			ep, err := url.Parse("/somefile/somewhere")
			Expect(err).NotTo(HaveOccurred())
			err = ops.ConvertToRawStream(ep, tmpFsDestPath, false, common.CacheModeTryNone)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("Resize", func() {
	It("Should complete successfully if qemu-img resize succeeds", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		ops := newTestOpsWithRun(mockRunCmd("", "", "resize", "-f", "raw", "image", size))
		err = ops.Resize("image", quantity, false)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should fail if qemu-img resize fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		ops := newTestOpsWithRun(mockRunCmd("", "exit 1", "resize", "-f", "raw", "image", size))
		err = ops.Resize("image", quantity, false)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("Error resizing image")))
	})
})

var _ = Describe("Validate", func() {
	imageName, _ := url.Parse("myimage.qcow2")

	DescribeTable("Validate should", func(mockFn runCmdFunc, errString string, image *url.URL) {
		ops := newTestOpsWithRun(mockFn)
		err := ops.Validate(image, 42949672960)

		if errString == "" {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
			rootErr := errors.Cause(err)
			if rootErr.Error() != errString {
				Fail(fmt.Sprintf("got wrong failure: [%s], expected [%s]", rootErr, errString))
			}
		}
	},
		Entry("should return success", mockRunCmd(goodValidateJSON, "", "info", "--output=json", imageName.String()), "", imageName),
		Entry("should return error", mockRunCmd("explosion", "exit 1"), "exit 1", imageName),
		Entry("should return error on bad json", mockRunCmd(badValidateJSON, ""), "unexpected end of JSON input", imageName),
		Entry("should return error on bad format", mockRunCmd(badFormatValidateJSON, ""), fmt.Sprintf("Invalid format raw2 for image %s", imageName), imageName),
		Entry("should return error on invalid backing file", mockRunCmd(backingFileValidateJSON, ""), fmt.Sprintf("Image %s is invalid because it has invalid backing file backing-file.qcow2", imageName), imageName),
		Entry("should return error when PVC is too small", mockRunCmd(hugeValidateJSON, ""), fmt.Sprintf("virtual image size %d is larger than the reported available storage %d. A larger PVC is required", 52949672960, 42949672960), imageName),
	)

})

var _ = Describe("Report Progress", func() {
	var progressMetric prometheus.ProgressMetric

	BeforeEach(func() {
		err := metrics.SetupMetrics()
		Expect(err).NotTo(HaveOccurred())
		progressMetric = metrics.Progress(ownerUID)
	})

	AfterEach(func() {
		progressMetric.Delete()
	})

	It("Parse valid progress line", func() {
		By("Verifying the initial value is 0")
		progressMetric.Add(0)
		progress, err := progressMetric.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(progress).To(Equal(float64(0)))
		By("Calling reportProgress with value")
		reportProgress("(45.34/100%)")
		progress, err = progressMetric.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(progress).To(Equal(45.34))
	})

	It("Parse invalid progress line", func() {
		By("Verifying the initial value is 0")
		progressMetric.Add(0)
		progress, err := progressMetric.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(progress).To(Equal(float64(0)))
		By("Calling reportProgress with invalid value")
		reportProgress("45.34")
		progress, err = progressMetric.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(progress).To(Equal(float64(0)))
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
		ops := newTestOpsWithRun(mockRunCmd("", "", "create", "-f", "raw", destPath, size))
		err = ops.CreateBlankImage(destPath, quantity, false)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should fail if qemu-img resize fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		ops := newTestOpsWithRun(mockRunCmd("", "exit 1", "create", "-f", "raw", destPath, size))
		err = ops.CreateBlankImage(destPath, quantity, false)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(ContainSubstring("could not create raw image with size ")))
	})

	It("should add preallocation if requested", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		ops := newTestOpsWithRun(mockRunCmdStrict("", "", "create", "-f", "raw", destPath, size, "-o", "preallocation=falloc"))
		err = ops.CreateBlankImage(destPath, quantity, true)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should not add preallocation if not requested", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		ops := newTestOpsWithRun(mockRunCmdStrict("", "", "create", "-f", "raw", destPath, size))
		err = ops.CreateBlankImage(destPath, quantity, false)
		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("Create preallocated blank block", func() {
	var tmpDir, tmpFsDir, destPath string
	var originalODirectChecker DirectIOChecker

	BeforeEach(func() {
		var err error
		// dest is usually not tmpfs, stay honest in unit tests as well
		tmpDir, err = os.MkdirTemp("/var/tmp", "qemutestdest")
		Expect(err).NotTo(HaveOccurred())
		By("tmpDir: " + tmpDir)
		destPath = filepath.Join(tmpDir, "dest")
		_, err = os.Create(destPath)
		Expect(err).NotTo(HaveOccurred())

		tmpFsDir, err = os.MkdirTemp("/var/tmp", "qemutestdestontmpfs")
		Expect(err).NotTo(HaveOccurred())
		By("tmpFsDir: " + tmpFsDir)
		originalODirectChecker = odirectChecker
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
		os.RemoveAll(tmpFsDir)
		odirectChecker = originalODirectChecker
	})

	It("Should complete successfully if preallocation succeeds", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		ops := newTestOpsWithRun(mockRunCmd("", "", "if=/dev/zero", "of="+destPath, "bs=1048576", "count=10240", "seek=0", "oflag=seek_bytes,direct"))
		err = ops.PreallocateBlankBlock(destPath, quantity)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should complete successfully with tmpfs dest without O_DIRECT if preallocation succeeds", func() {
		odirectChecker = NewDirectIOChecker(FakeODirectRefusingOS{})
		tmpFsDestPath := filepath.Join(tmpFsDir, "dest")
		_, err := os.Create(tmpFsDestPath)
		Expect(err).NotTo(HaveOccurred())
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		ops := newTestOpsWithRun(mockRunCmd("", "", "if=/dev/zero", "of="+tmpFsDestPath, "bs=1048576", "count=10240", "seek=0", "oflag=seek_bytes"))
		err = ops.PreallocateBlankBlock(tmpFsDestPath, quantity)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should complete successfully with value not aligned to 1MiB", func() {
		quantity, err := resource.ParseQuantity("5243392Ki")
		Expect(err).NotTo(HaveOccurred())
		firstCallArgs := []string{"if=/dev/zero", "of=" + destPath, "bs=1048576", "count=5120", "seek=0", "oflag=seek_bytes,direct"}
		secondCallArgs := []string{"if=/dev/zero", "of=" + destPath, "bs=524288", "count=1", "seek=5368709120", "oflag=seek_bytes,direct"}
		ops := newTestOpsWithRun(mockRunCmdTwoCalls("", "", firstCallArgs, secondCallArgs))
		err = ops.PreallocateBlankBlock(destPath, quantity)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should fail if preallocation fails", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		ops := newTestOpsWithRun(mockRunCmd("", "exit 1", "if=/dev/zero", "of="+destPath, "bs=1048576", "count=10240", "seek=0", "oflag=seek_bytes,direct"))
		err = ops.PreallocateBlankBlock(destPath, quantity)
		Expect(err).To(MatchError(ContainSubstring("Could not preallocate blank block volume at")))
	})
})

var _ = Describe("Try different preallocation modes", func() {
	It("Should try falloc first", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) error {
			Expect(args).To(Equal([]string{"command", "-o", "preallocation=falloc"}))
			calledCount++
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calledCount).To(Equal(1))
	})

	It("Should try full if falloc fails", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) error {
			if args[2] == "preallocation=falloc" {
				calledCount++
				return &cmdExecError{name: "qemu-img", stderr: "Unsupported preallocation mode: falloc", err: fmt.Errorf("exit status 1")}
			}
			Expect(args).To(Equal([]string{"command", "-o", "preallocation=full"}))
			calledCount++
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calledCount).To(Equal(2))
	})

	It("Should try -S0 if full fails", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) error {
			if calledCount < 2 {
				calledCount++
				return &cmdExecError{name: "qemu-img", stderr: "Unsupported preallocation mode: full", err: fmt.Errorf("exit status 1")}
			}
			Expect(args).To(Equal([]string{"command", "-S", "0"}))
			calledCount++
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calledCount).To(Equal(3))
	})

	It("Should fail if output is different than 'Unsupported preallocation'", func() {
		calledCount := 0
		err := addPreallocation([]string{"command"}, convertPreallocationMethods, func(args []string) error {
			calledCount++
			return &cmdExecError{name: "qemu-img", stderr: "General Protection Fault", err: fmt.Errorf("exit status 1")}
		})

		Expect(err).To(HaveOccurred())
		Expect(calledCount).To(Equal(1))
	})
})

var _ = Describe("Rebase and commit", func() {
	It("Should successfully rebase image", func() {
		ops := newTestOpsWithStream(mockRunCmdWithStreamingStrict("", "rebase", "-p", "-u", "-F", "raw", "-b", "backing-file", "delta"))
		err := ops.Rebase("backing-file", "delta")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should successfully commit image to base", func() {
		ops := newTestOpsWithStream(mockRunCmdWithStreamingStrict("", "commit", "-p", "delta"))
		err := ops.Commit("delta")
		Expect(err).NotTo(HaveOccurred())
	})
})

func mockRunCmd(output, errString string, checkArgs ...string) runCmdFunc {
	return func(_ context.Context, name string, args ...string) (bytes []byte, err error) {
		for _, ca := range checkArgs {
			found := false
			for _, a := range args {
				if ca == a {
					found = true
					break
				}
			}
			if !found {
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

func mockRunCmdStrict(output, errString string, checkArgs ...string) runCmdFunc {
	return func(_ context.Context, name string, args ...string) (bytes []byte, err error) {
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

func mockRunCmdTwoCalls(output, errString string, firstCallArgs []string, secondCallArgs []string) runCmdFunc {
	firstCall := true
	return func(_ context.Context, name string, args ...string) (bytes []byte, err error) {
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

func mockRunCmdWithStreaming(errString string, checkArgs ...string) runCmdWithStreamFunc {
	return func(_ context.Context, _ func(string), name string, args ...string) error {
		for _, ca := range checkArgs {
			found := false
			for _, a := range args {
				if ca == a {
					found = true
					break
				}
			}
			if !found {
				Expect(checkArgs).To(Equal(args))
			}
		}

		if errString != "" {
			return errors.New(errString)
		}
		return nil
	}
}

func mockRunCmdWithStreamingStrict(errString string, checkArgs ...string) runCmdWithStreamFunc {
	return func(_ context.Context, _ func(string), name string, args ...string) error {
		Expect(checkArgs).To(Equal(args))

		if errString != "" {
			return errors.New(errString)
		}
		return nil
	}
}

func newTestOpsWithRun(run runCmdFunc) *qemuOperations {
	cmd := newQemuCmd()
	cmd.run = run
	return &qemuOperations{cmd: cmd}
}

func newTestOpsWithStream(stream runCmdWithStreamFunc) *qemuOperations {
	cmd := newQemuCmd()
	cmd.stream = stream
	return &qemuOperations{cmd: cmd}
}
