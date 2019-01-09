package importer

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

var (
	imageFile = filepath.Join(imageDir, "docker-image.tar")
	imageData = filepath.Join(imageDir, "data")
)

type fakeSkopeoOperations struct {
	e1 error
}

var _ = Describe("Copy from Registry", func() {
	table.DescribeTable("Image, with import source should", func(dest string, skopeoOperations image.SkopeoOperations, wantErr bool) {
		defer os.RemoveAll(dest)
		By("Replacing Skopeo Operations")
		replaceSkopeoOperations(skopeoOperations, func() {
			By("Copying image")
			err := CopyData(&DataStreamOptions{
				dest,
				"",
				"",
				"",
				controller.SourceRegistry,
				string(cdiv1.DataVolumeKubeVirt),
				"1G"})
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	},
		table.Entry("successfully copy registry image", imageData, NewFakeSkopeoOperations(nil), false),
		table.Entry("expect failure trying to copy non-existing image", "../fake", NewSkopeoAllErrors(), true),
	)
})

func replaceSkopeoOperations(replacement image.SkopeoOperations, f func()) {
	orig := image.SkopeoInterface
	if replacement != nil {
		image.SkopeoInterface = replacement
		defer func() { image.SkopeoInterface = orig }()
	}
	f()
}

func NewSkopeoAllErrors() image.SkopeoOperations {
	err := errors.New("skopeo should not be called from this test override with replaceSkopeoOperations")
	return NewFakeSkopeoOperations(err)
}

func NewFakeSkopeoOperations(e1 error) image.SkopeoOperations {
	return &fakeSkopeoOperations{e1}
}

func (o *fakeSkopeoOperations) CopyImage(string, string, string, string) error {
	if o.e1 == nil {
		if err := util.UnArchiveLocalTar(imageFile, imageDir); err != nil {
			return errors.New("could not extract layer tar")
		}
	}
	return o.e1
}
