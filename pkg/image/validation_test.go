package image_test

import (
	"fmt"

	. "github.com/kubevirt/containerized-data-importer/pkg/image"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validate file extensions", func() {
	type testT struct {
		imageFileName string
	}

	Context("IsValidImageFile: valid image name will succeed", func() {
		tests := []testT{
			{
				imageFileName: "test.qcow2",
			},
			{
				imageFileName: "test.tar",
			},
			{
				imageFileName: "test.gz",
			},
			{
				imageFileName: "test.xz",
			},
			{
				imageFileName: "test.img",
			},
			{
				imageFileName: "test.iso",
			},
		}
		for _, t := range tests {
			name := t.imageFileName
			It(fmt.Sprintf("checking filename %q", name), func() {
				Expect(IsSupporedFileType(name)).Should(BeTrue())
			})
		}
	})

	Context("IsValidImageFile: invalid image name will fail", func() {
		tests := []testT{
			{
				imageFileName: "xyz.abc",
			},
			{
				imageFileName: "xxx",
			},
			{
				imageFileName: "",
			},
		}
		for _, t := range tests {
			name := t.imageFileName
			It(fmt.Sprintf("checking filename %q", name), func() {
				Expect(IsSupporedFileType(name)).ShouldNot(BeTrue())
			})
		}
	})
})
