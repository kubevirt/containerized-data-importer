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
	"kubevirt.io/containerized-data-importer/pkg/importer"
	imagesize "kubevirt.io/containerized-data-importer/pkg/lib/size"
	f "kubevirt.io/containerized-data-importer/test/framework"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"syscall"
)

var _ = Describe("Streaming Data Conversion", func() {

	Context("when data is in a supported file format", func() {
		const (
			baseImageRelPath = "../../images"
			baseImage        = "tinyCore"
			baseImageExt     = ".iso"
			baseImageIso     = baseImage + baseImageExt
		)

		baseTestImage, err := filepath.Abs(filepath.Join(baseImageRelPath, baseImageIso))
		if err != nil {
			Fail(fmt.Sprintf("Error getting abs path: %v\n", err))
		}

		var tmpTestDir string
		BeforeEach(func() {
			tmpDir := os.TempDir()
			tmpTestDir = testDir(tmpDir)
			if err != nil {
				Fail(fmt.Sprintf("Failed created test dir: %v\n", err))
			}
			syscall.Umask(0000)
			err = os.Mkdir(tmpTestDir, 0777)
			if err != nil {
				Fail(fmt.Sprintf("Could not create tmp file: %v\n ", err))
			}
			By(fmt.Sprintf("Created temporary dir %s", tmpTestDir))
		})

		AfterEach(func() {
			By(fmt.Sprintf("Cleaning up temporary dir %s", tmpTestDir))
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
			{
				testDesc:      "should unpack .tar.xz",
				originalFile:  baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar, image.ExtXz},
			},
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
				By(fmt.Sprintf("Getting size of source file (%s)\n", origFile))
				finfo, err := os.Stat(origFile)
				Expect(err).NotTo(HaveOccurred())
				sourceSize := finfo.Size()

				By(fmt.Sprintf("Converting sample file to format: %v", ff))
				// Generate the expected data format from the random bytes
				testSample, err := f.FormatTestData(origFile, tmpTestDir, ff...)
				Expect(err).NotTo(HaveOccurred(), "Error formatting test data.")

				testSample = "file://" + testSample
				testTarget := filepath.Join(tmpTestDir, common.IMPORTER_WRITE_FILE)
				By(fmt.Sprintf("Processing sample file %q to %q", testSample, testTarget))
				err = importer.CopyImage(testTarget, testSample, "", "")
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Checking size of the output file %q", testTarget))
				if useVSize {
					By("Checking output image virtual size")
					targetSize := getImageVirtualSize(testTarget)
					Expect(targetSize).To(Equal(sourceSize))
					By("Calling `Size` function to check size")
					targetSize, err = imagesize.Size(testSample, "", "")
					Expect(err).NotTo(HaveOccurred())
					Expect(targetSize).To(Equal(sourceSize))
				} else {
					By("stat() output file")
					finfo, err = os.Stat(testTarget)
					Expect(err).NotTo(HaveOccurred())
					targetSize := finfo.Size()
					Expect(targetSize).To(Equal(sourceSize))
				}
			})
		}
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
