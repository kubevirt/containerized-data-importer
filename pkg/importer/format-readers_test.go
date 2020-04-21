package importer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

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

	table.DescribeTable("can construct readers", func(filename string, numRdrs int, wantErr, archived, convert bool) {
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
			Expect(convert).To(Equal(fr.Convert))
			Expect(archived).To(Equal(fr.Archived))
		}
	},
		table.Entry("successfully construct a xz reader", tinyCoreXzFilePath, 4, false, true, false),              // [stream, multi-r, xz, multi-r] convert = false
		table.Entry("successfully construct a gz reader", tinyCoreGzFilePath, 4, false, true, false),              // [stream, multi-r, gz, multi-r] convert = false
		table.Entry("successfully return the base reader when archived", archiveFilePath, 3, false, false, false), // [stream, multi-r, multi-r] convert = false
		table.Entry("successfully construct qcow2 reader", cirrosFilePath, 2, false, false, true),                 // [stream, multi-r] convert = true
		table.Entry("successfully construct .iso reader", tinyCoreFilePath, 2, false, false, false),               // [stream, multi-r] convert = false
	)

	table.DescribeTable("can append readers", func(rType int, r interface{}, numRdrs int, isCloser bool) {
		f, err := os.Open(cirrosFilePath)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		fr, err = NewFormatReaders(f, uint64(0))
		Expect(err).ToNot(HaveOccurred())
		By("Verifying there are currently 2 readers")
		Expect(len(fr.readers)).To(Equal(2))
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
		table.Entry("should not append nil reader", rdrGz, nil, 2, false),
		table.Entry("should not append non reader", rdrGz, nil, 2, false),
		table.Entry("should append io.reader", rdrGz, stringRdr, 3, false),
		table.Entry("should append io.Multireader", rdrMulti, stringRdr, 3, false),
	)

	It("should not crash on no progress reader", func() {
		stringReader := ioutil.NopCloser(strings.NewReader("This is a test string"))
		testReader, err := NewFormatReaders(stringReader, uint64(0))
		// Not passing a real string, so the header checking will fail.
		Expect(err).To(HaveOccurred())
		Expect(testReader.progressReader).To(BeNil())
		// This should not crash
		testReader.StartProgressUpdate()
	})
})
