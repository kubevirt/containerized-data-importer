package importer_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"kubevirt.io/containerized-data-importer/pkg/image"
	imagesize "kubevirt.io/containerized-data-importer/pkg/lib/size"
	"kubevirt.io/containerized-data-importer/tests/utils"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"syscall"
	"kubevirt.io/containerized-data-importer/pkg/importer"
)

var _ = Describe("Streaming Data Conversion", func() {

	Context("when data is in a supported file format", func() {

		tinyCoreIso := "tinyCore.iso"
		baseTestImage, err := filepath.Abs(filepath.Join(importer.TestImagesDir, tinyCoreIso))
		if err != nil {
			Fail(fmt.Sprintf("Error getting abs path: %v\n", err))
		}

		var tmpTestDir string
		BeforeEach(func() {
			tmpTestDir = testDir(os.TempDir())
			if err != nil {
				Fail(fmt.Sprintf("[BeforeEach] Failed created test dir: %v\n", err))
			}
			By(fmt.Sprintf("[BeforeEach] Creating temporary dir %s", tmpTestDir))
			syscall.Umask(0000)
			err = os.Mkdir(tmpTestDir, 0777)
			if err != nil {
				Fail(fmt.Sprintf("Could not create tmp file: %v\n ", err))
			}
		})

		AfterEach(func() {
			By(fmt.Sprintf("[AfterEach] Cleaning up temporary dir %s", tmpTestDir))
			os.RemoveAll(tmpTestDir)
		})

		// Test Table
		tests := []struct {
			testDesc      string
			originalFile  string
			useVirtSize   bool
			expectFormats []string
		}{
			{
				testDesc:      "should decompress gzip",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtGz},
			},
			{
				testDesc:      "should decompress xz",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtXz},
			},
			{
				testDesc:      "should unarchive tar",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar},
			},
			{
				testDesc:      "should unpack .tar.gz",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar, image.ExtGz},
			},
			// Disabled until issue 335 is resolved
			// https://github.com/kubevirt/containerized-data-importer/issues/335
			//{
			//	testDesc:      "should unpack .tar.xz",
			//	originalFile:  baseTestImage,
			//	useVirtSize:   false,
			//	expectFormats: []string{image.ExtTar, image.ExtXz},
			//},
			{
				testDesc:      "should convert .qcow2",
				originalFile:  baseTestImage,
				useVirtSize:   true,
				expectFormats: []string{image.ExtQcow2},
			},
			{
				testDesc:      "should convert and unpack .qcow2.gz",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtGz},
			},
			{
				testDesc:      "should convert and unpack .qcow2.xz",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtXz},
			},
			{
				testDesc:      "should convert and untar .qcow2.tar",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar},
			},
			{
				testDesc:      "should convert and untar and unpack .qcow2.tar.gz",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar, image.ExtGz},
			},
			{
				testDesc:      "should convert and untar and unpack .qcow2.tar.xz",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar, image.ExtXz},
			},
			{
				testDesc:      "should pass through unformatted data",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{""},
			},
		}

		for _, t := range tests {
			desc := t.testDesc
			ff := t.expectFormats
			origFile := t.originalFile
			useVSize := t.useVirtSize

			It(desc, func() {
				By(fmt.Sprintf("Getting size of source file %q", origFile))
				finfo, err := os.Stat(origFile)
				Expect(err).NotTo(HaveOccurred())
				sourceSize := finfo.Size()
				fmt.Fprintf(GinkgoWriter, "INFO: size = %d\n", sourceSize)

				By(fmt.Sprintf("Converting source file to format: %s", ff))
				// Generate the expected data format from the random bytes
				testSample, err := utils.FormatTestData(origFile, tmpTestDir, ff...)
				Expect(err).NotTo(HaveOccurred(), "Error formatting test data.")
				fmt.Fprintf(GinkgoWriter, "INFO: converted source file name is %q\n", testSample)

				testEp := "file://" + testSample
				testTarget := filepath.Join(tmpTestDir, common.IMPORTER_WRITE_FILE)
				By(fmt.Sprintf("Importing %q to %q", testEp, testTarget))
				err = importer.CopyImage(testTarget, testEp, "", "")
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Checking size of the output file %q", testTarget))
				var targetSize int64
				if useVSize {
					By("... using output image's virtual size")
					targetSize = getImageVirtualSize(testTarget)
					Expect(targetSize).To(Equal(sourceSize))
				} else {
					By("... using stat()")
					finfo, err = os.Stat(testTarget)
					Expect(err).NotTo(HaveOccurred())
					targetSize = finfo.Size()
					Expect(targetSize).To(Equal(sourceSize))
				}
				fmt.Fprintf(GinkgoWriter, "INFO: byte size = %d\n", targetSize)

				By(fmt.Sprintf("Calling `size.Size()` on same endpoint %q", testEp))
				// extract the file extension(s) and check if file should be skipped
				testBase := filepath.Base(testSample)
				i := strings.Index(testBase, ".")
				if i > 0 {
					targetExt := testBase[i:]
					if _, ok := sizeExceptions[targetExt]; ok {
						Skip(fmt.Sprintf("*** skipping endpoint extension %q as exception", targetExt))
					}
				}
				targetSize, err = imagesize.Size(testEp, "", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(targetSize).To(Equal(sourceSize))

				fmt.Fprintf(GinkgoWriter, "End test on test file %q\n", testSample)
			})
		}
		fmt.Fprintf(GinkgoWriter, "\nDEPRECATION NOTICE:\n   Support for local (file://) endpoints will be removed from CDI in the next release.\n   There is no replacement and no work-around.\n   All import endpoints must reference http(s) or s3 endpoints\n")
	})
})

func getImageVirtualSize(outFile string) int64 {
	//call qemu-img info
	virtSizeParseLen := 8

	//create command
	cmd := fmt.Sprintf("qemu-img info %s | grep 'virtual size:'", outFile)
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return 0
	}
	sOut := string(out)

	index1 := strings.Index(sOut, "(")
	sSize := sOut[index1+1 : len(sOut)-virtSizeParseLen]

	vSize, err := strconv.ParseInt(sSize, 10, 64)
	if err != nil {
		return 0
	}
	return vSize
}

func testDir(parent string) string {
	suf := util.RandAlphaNum(12)
	return filepath.Join(parent, fmt.Sprintf(".tmp-%s", string(suf)))
}
