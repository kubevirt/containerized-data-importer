package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var (
	archiveFileName           = "archive.tar"
	imageDir, _               = filepath.Abs(TestImagesDir)
	tinyCoreFileName          = "tinyCore.iso"
	tinyCoreFilePath          = filepath.Join(imageDir, tinyCoreFileName)
	tinyCoreXzFilePath, _     = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtXz)
	tinyCoreGzFilePath, _     = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtGz)
	tinyCoreTarFilePath, _    = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtTar)
	archiveFilePath, _        = utils.ArchiveFiles(archiveFileNameWithoutExt, os.TempDir(), tinyCoreFilePath, cirrosFilePath)
	archiveFileNameWithoutExt = strings.TrimSuffix(archiveFileName, filepath.Ext(archiveFileName))
	cirrosFilePath            = filepath.Join(imageDir, cirrosFileName)
	stringRdr                 = strings.NewReader("test data for reader 1")
)

var _ = Describe("Format Readers", func() {
	var fr *FormatReaders
	BeforeEach(func() {
		fr = nil
	})

	AfterEach(func() {
		if fr != nil {
			fr.Close()
		}
	})

	table.DescribeTable("can construct readers", func(filename string, contentType cdiv1.DataVolumeContentType, numRdrs int, wantErr, archived, qemu bool) {
		f, err := os.Open(filename)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		fr, err = NewFormatReaders(f, contentType)
		if wantErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
			for _, r := range fr.readers {
				fmt.Fprintf(GinkgoWriter, "INFO: Reader type: %d\n", r.rdrType)
			}
			Expect(len(fr.readers)).To(Equal(numRdrs))
			Expect(qemu).To(Equal(fr.Qemu))
			Expect(archived).To(Equal(fr.Archived))
		}
	},
		table.Entry("successfully construct a xz reader", tinyCoreXzFilePath, cdiv1.DataVolumeKubeVirt, 5, false, true, true),      // [stream, multi-r, xz, multi-r, raw] qemu = true
		table.Entry("successfully construct a gz reader", tinyCoreGzFilePath, cdiv1.DataVolumeKubeVirt, 5, false, true, true),      // [stream, multi-r, gz, multi-r, raw] qemu = true
		table.Entry("successfully construct a tar reader", tinyCoreTarFilePath, cdiv1.DataVolumeKubeVirt, 4, false, true, false),   // [stream, multi-r, tar, multi-r] qemu = true
		table.Entry("successfully constructed an archive reader", archiveFilePath, cdiv1.DataVolumeArchive, 4, false, true, false), // [stream, multi-r, mul-tar, multi-r] qemu = false
		table.Entry("successfully construct qcow2 reader", cirrosFilePath, cdiv1.DataVolumeKubeVirt, 2, false, false, true),        // [stream, multi-r]
		table.Entry("successfully construct .iso reader", tinyCoreFilePath, cdiv1.DataVolumeKubeVirt, 3, false, false, true),       // [stream, multi-r, raw]
		table.Entry("Fail to construct a gz reader with archive contentType", tinyCoreGzFilePath, cdiv1.DataVolumeArchive, 0, true, false, false),
	)

	table.DescribeTable("can append readers", func(rType int, r interface{}, numRdrs int, isCloser bool) {
		f, err := os.Open(cirrosFilePath)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		fr, err = NewFormatReaders(f, cdiv1.DataVolumeKubeVirt)
		Expect(err).ToNot(HaveOccurred())
		By("Verifying there are currently 2 readers")
		Expect(len(fr.readers)).To(Equal(2))
		fr.appendReader(rType, r)
		By("Verifying there expected number of readers are there")
		Expect(len(fr.readers)).To(Equal(numRdrs))
		if isCloser {
			By("Verifying the type of the new reader is io.Closer")
			if _, ok := fr.TopReader().(io.Closer); !ok {
				Expect(ok).To(BeTrue())
			}
		}
	},
		table.Entry("should not append nil reader", rdrTar, nil, 2, false),
		table.Entry("should not append non reader", rdrTar, cdiv1.DataVolumeKubeVirt, 2, false),
		table.Entry("should append io.reader", rdrTar, stringRdr, 3, false),
		table.Entry("should append io.Multireader", rdrMulti, stringRdr, 3, false),
	)
})
