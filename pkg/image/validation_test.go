package image_test

import (
	. "github.com/kubevirt/containerized-data-importer/pkg/image"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validate Test IsValidImageFile", func() {
	Context("valid image name will success", func() {
		type testT struct {
			imageFileName string
		}
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
				imageFileName: "test.img",
			},
			{
				imageFileName: "test.iso",
			},
		}
		It("returns false when the suffix are invalid type or empty", func() {
			for _, t := range tests {
				Expect(IsSupporedFileType(t.imageFileName)).Should(BeTrue())
			}
		})
	})

	Context("invalid image name will be fail", func() {
		type testT struct {
			imageFileName string
		}
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
		It("returns true when the suffix are invalid type or empty", func() {
			for _, t := range tests {
				Expect(IsSupporedFileType(t.imageFileName)).ShouldNot(BeTrue())
			}
		})
	})
})
