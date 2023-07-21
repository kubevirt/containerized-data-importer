package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var (
	imageDir, _            = filepath.Abs(TestImagesDir)
	noDiskTarFileName      = "archive.tar"
	noDiskTarFilePath      = filepath.Join(imageDir, noDiskTarFileName)
	tinyCoreFileName       = "tinyCore.iso"
	tinyCoreFilePath       = filepath.Join(imageDir, tinyCoreFileName)
	tinyCoreXzFilePath, _  = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtXz)
	tinyCoreGzFilePath, _  = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtGz)
	tinyCoreTarFilePath, _ = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtTar)
	archiveFilePath, _     = utils.ArchiveFiles("archive.tar", os.TempDir(), tinyCoreFilePath, cirrosFilePath)
	archiveGzFilePath, _   = utils.ToGz(archiveFilePath, os.TempDir(), "")
	archiveXzFilePath, _   = utils.ToXz(archiveFilePath, os.TempDir(), "")
	stringRdr              = strings.NewReader("test data for reader 1")
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

	DescribeTable("can construct readers", func(filename string, numRdrs int, wantErr bool, wantAttr formatAttr) {
		f, err := os.Open(filename)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		fr, err = NewFormatReaders(f, uint64(0))
		if wantErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
			for _, r := range fr.readers {
				fmt.Fprintf(GinkgoWriter, "INFO: Reader type: %d\n", r.rdrType)
			}
			Expect(numRdrs).To(Equal(len(fr.readers)))
			Expect(wantAttr).To(Equal(fr.formatAttr))
		}
	},
		Entry("successfully construct a xz reader", tinyCoreXzFilePath, 4, false, formatAttr{
			Convert:    false,
			Compressed: true,
		}), // [stream, multi-r, xz, multi-r] convert = false
		Entry("successfully construct a gz reader", tinyCoreGzFilePath, 4, false, formatAttr{
			Convert:    false,
			Compressed: true,
		}), // [stream, multi-r, gz, multi-r] convert = false
		Entry("successfully construct a tar reader", archiveFilePath, 3, false, formatAttr{
			Convert:    false,
			ArchiveTar: true,
		}), // [stream, multi-r, multi-r] convert = false
		Entry("return the base reader when archive has no disk file", noDiskTarFilePath, 3, false, formatAttr{
			Convert:    false,
			ArchiveTar: true,
		}), // [stream, multi-r, multi-r] convert = false
		Entry("successfully construct a qcow2 reader", cirrosFilePath, 2, false, formatAttr{
			Convert: true,
		}), // [stream, multi-r] convert = true
		Entry("successfully construct a iso reader", tinyCoreFilePath, 2, false, formatAttr{
			Convert: false,
		}), // [stream, multi-r] convert = false
		Entry("successfully construct a tar.gz reader", archiveGzFilePath, 5, false, formatAttr{
			Convert:    false,
			ArchiveTar: true,
			Compressed: true,
		}), // [stream, multi-r, gz, multi-r, multi-r] convert = false
		Entry("successfully construct a tar.xz reader", archiveXzFilePath, 5, false, formatAttr{
			Convert:    false,
			ArchiveTar: true,
			Compressed: true,
		}), // [stream, multi-r, xz, multi-r, multi-r] convert = false
	)

	DescribeTable("can append readers", func(rType int, r interface{}, numRdrs int, isCloser bool) {
		f, err := os.Open(cirrosFilePath)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		fr, err = NewFormatReaders(f, uint64(0))
		Expect(err).ToNot(HaveOccurred())
		By("Verifying there are currently 2 readers")
		Expect(fr.readers).To(HaveLen(2))
		fr.appendReader(rType, r)
		By("Verifying there expected number of readers are there")
		Expect(numRdrs).To(Equal(len(fr.readers)))
		if isCloser {
			By("Verifying the type of the new reader is io.Closer")
			if _, ok := fr.TopReader().(io.Closer); !ok {
				Expect(ok).To(BeTrue())
			}
		}
	},
		Entry("should not append nil reader", rdrGz, nil, 2, false),
		Entry("should not append non reader", rdrGz, nil, 2, false),
		Entry("should append io.reader", rdrGz, stringRdr, 3, false),
		Entry("should append io.Multireader", rdrMulti, stringRdr, 3, false),
	)

	It("should not crash on no progress reader", func() {
		stringReader := io.NopCloser(strings.NewReader("This is a test string"))
		testReader, err := NewFormatReaders(stringReader, uint64(0))
		// Not passing a real string, so the header checking will fail.
		Expect(err).To(HaveOccurred())
		Expect(testReader.progressReader).To(BeNil())
		// This should not crash
		testReader.StartProgressUpdate()
	})
})
