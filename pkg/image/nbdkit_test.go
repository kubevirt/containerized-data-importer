package image

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"kubevirt.io/containerized-data-importer/pkg/system"
	"net/url"
	"reflect"
	"strings"
)

var (
	pidfile           = "nbdkit.pid"
	defaultNbdkitArgs = []string{"--foreground", "--readonly", "-U", "-", "--pidfile", pidfile}
	nbdkit            *Nbdkit
	n                 QEMUOperations
)

var _ = Describe("Start nbdkit with qemu-img", func() {
	It("should return no error if exec function returns no error", func() {
		qemuArgs := []string{"-h"}
		n := NewNbdkitCurl(pidfile, "")
		u := "http://someurl/somewhere/source.img"
		n.source, _ = url.Parse(u)
		args := append(defaultNbdkitArgs, "curl", fmt.Sprintf("url=%s", u), "--run", fmt.Sprintf("qemu-img convert $nbd %v", strings.Join(qemuArgs, " ")))
		replaceNbdkitExecFunction(mockExecFunction("", "", nil, args...), func() {
			_, err := n.startNbdkitWithQemuImg("convert", qemuArgs)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("Convert to Raw", func() {
	var (
		u = "http://someurl/somewhere/source.img"
	)
	BeforeEach(func() {
		nbdkit = NewNbdkitCurl(pidfile, "")
		n = NewNbdkitOperations(nbdkit)

	})

	It("should stream url to destination", func() {
		qemuArgs := []string{"-p", "-O", "raw", "dest", "-t", "none"}
		args := append(defaultNbdkitArgs, "curl", fmt.Sprintf("url=%s", u), "--run", fmt.Sprintf("qemu-img %s $nbd %v", "convert", strings.Join(qemuArgs, " ")))
		source, _ := url.Parse(u)
		replaceNbdkitExecFunction(mockExecFunction("", "", nil, args...), func() {
			err := n.ConvertToRawStream(source, "dest", false)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	It("should stream file to destination using qemu-img", func() {
		i := "myimage.qcow2"
		source, _ := url.Parse(i)
		qemuArgs := []string{"convert", "-p", "-O", "raw", "-t", "none", i, "dest"}
		replaceNbdkitExecFunction(mockExecFunction("", "", nil, qemuArgs...), func() {
			err := n.ConvertToRawStream(source, "dest", false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

})

var _ = Describe("Info", func() {
	var (
		u = "http://someurl/somewhere/source.img"
	)
	BeforeEach(func() {
		nbdkit = NewNbdkitCurl(pidfile, "")
		n = NewNbdkitOperations(nbdkit)

	})

	It("should read successfully from the url", func() {
		qemuArgs := []string{"--output=json"}
		args := append(defaultNbdkitArgs, "curl", fmt.Sprintf("url=%s", u), "--run", fmt.Sprintf("qemu-img %s $nbd %v", "info", strings.Join(qemuArgs, " ")))
		source, _ := url.Parse(u)
		replaceNbdkitExecFunction(validQemuImgInfo("", "", nil, args...), func() {
			_, err := n.Info(source)
			Expect(err).NotTo(HaveOccurred())
		})
	})

})

var _ = Describe("Resize", func() {
	BeforeEach(func() {
		n = NewNbdkitOperations(&Nbdkit{})

	})
	It("Should complete successfully and falling back in using qemu-img resize", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceNbdkitExecFunction(mockExecFunction("", "", nil, "resize", "-f", "raw", "image", size), func() {
			err = n.Resize("image", quantity)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	It("Should fail and falling back in using qemu-img resize", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceExecFunction(mockExecFunction("", "exit 1", nil, "resize", "-f", "raw", "image", size), func() {
			err = n.Resize("image", quantity)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "Error resizing image image")).To(BeTrue())
		})
	})

})

var _ = Describe("Create blank image", func() {
	BeforeEach(func() {
		n = NewNbdkitOperations(&Nbdkit{})

	})
	It("Should complete successfully falling back in using qemu-img", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceNbdkitExecFunction(mockExecFunction("", "", nil, "create", "-f", "raw", "image", size), func() {
			err = n.CreateBlankImage("image", quantity, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	It("Should fail if and falling back in using qemu-img", func() {
		quantity, err := resource.ParseQuantity("10Gi")
		Expect(err).NotTo(HaveOccurred())
		size := convertQuantityToQemuSize(quantity)
		replaceNbdkitExecFunction(mockExecFunction("", "exit 1", nil, "create", "-f", "raw", "image", size), func() {
			err = n.CreateBlankImage("image", quantity, false)
			Expect(err).To(HaveOccurred())
			Expect(strings.Contains(err.Error(), "could not create raw image with size ")).To(BeTrue())
		})
	})
})

func replaceNbdkitExecFunction(replacement execFunctionType, f func()) {
	origNbdkit := nbdkitExecFunction
	origQemu := qemuExecFunction
	if replacement != nil {
		nbdkitExecFunction = replacement
		qemuExecFunction = replacement
		defer func() {
			nbdkitExecFunction = origNbdkit
			qemuExecFunction = origQemu
		}()
	}
	f()
}

func validQemuImgInfo(output, errString string, expectedLimits *system.ProcessLimitValues, checkArgs ...string) execFunctionType {
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

		return []byte(goodValidateJSON), nil
	}
}
