// +build functional_test

package importer

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/importer"
	imagesize "kubevirt.io/containerized-data-importer/pkg/lib/size"
	f "kubevirt.io/containerized-data-importer/test/framework"
	"time"
)

var _ = Describe("Streaming Data Conversion", func() {

	Context("when data is in a supported file format", func() {
		const (
			baseImageRelPath = "../../images"
			baseImage = "tinyCore"
			baseImageExt = ".iso"
			baseImageIso = baseImage + baseImageExt
		)

		baseTestImage, err := filepath.Abs(filepath.Join(baseImageRelPath, baseImageIso))
		if err != nil {
			Fail(fmt.Sprintf("Error getting abs path: %v\n", err))
		}

		var tempTestDir string

		BeforeEach(func() {
			tmpDir := os.TempDir()
			testDir := tempDir(tmpDir)
			if err != nil {
				Fail(fmt.Sprintf("Failed created test dir: %v\n", err))
			}

			err = os.Mkdir(tempTestDir, 0666)
			if err != nil {
				Fail(fmt.Sprintf("Could not create tmp file: %v\n ", err))
			}
			By("Mkdir " + tempTestDir)
		})

		//AfterEach(func() {
		//	os.Remove(testDir)
		//	By("Rmdir "+ testDir)
		//})

		// Test Table
		tests := []struct {
			testDesc      string
			originalFile  string
			testFile      string
			useVirtSize   bool
			expectFormats []string
		}{
			{
				testDesc:      "should decompress gzip",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".iso.gz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtGz},
			},
			{
				testDesc:      "should decompress xz",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".iso.xz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtXz},
			},
			{
				testDesc:      "should unarchive tar",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".iso.tar"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar},
			},
			{
				testDesc:      "should unpack .tar.gz",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".iso.tar.gz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar, image.ExtGz},
			},
			{
				testDesc:      "should unpack .tar.xz",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".iso.tar.xz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar, image.ExtXz},
			},
			{
				testDesc:      "should convert .qcow2",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".qcow2"),
				useVirtSize:   true,
				expectFormats: []string{image.ExtQcow2},
			},
			{
				testDesc:      "should convert and unpack .qcow2.gz",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".qcow2.gz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtGz},
			},
			{
				testDesc:      "should convert and unpack .qcow2.xz",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".qcow2.xz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtXz},
			},
			{
				testDesc:      "should convert and untar .qcow2.tar",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".qcow2.tar"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar},
			},
			{
				testDesc:      "should convert and untar and unpack .qcow2.tar.gz",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".qcow2.tar.gz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar, image.ExtGz},
			},
			{
				testDesc:      "should convert and untar and unpack .qcow2.tar.xz",
				originalFile:  baseTestImage,
				testFile:      filepath.Join(tempTestDir, testFile + ".qcow2.tar.xz"),
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar, image.ExtXz},
			},
			{
				testDesc:      "should pass through unformatted data",
				originalFile:  baseTestImage,
				testFile:      baseTestImage,
				useVirtSize:   false,
				expectFormats: []string{""},
			},
		}

		var i int
		for _, t := range tests {
			i++
			desc := fmt.Sprintf("[%d] %s", i, t.testDesc)
			ff := t.expectFormats
			origFile := t.originalFile
			testFile := t.testFile
			useVSize := t.useVirtSize

			It(desc, func() {

				os.Open(testFile)

				By(fmt.Sprintf("Getting size of source file (%s)\n", origFile))
				finfo, err := os.Stat(origFile)
				Expect(err).NotTo(HaveOccurred())
				size := finfo.Size()

				By(fmt.Sprintf("Converting sample file to format: %v", ff))
				// Generate the expected data format from the random bytes
				sampleFilename, err := f.FormatTestData(origFile, testFile, ff...)
				Expect(err).NotTo(HaveOccurred(), "Error formatting test data.")

				dest := fmt.Sprintf("%s.%d", filepath.Join(os.TempDir(), filepath.Base(testFile)), i)
				fUrl := "file:/" + sampleFilename
				By(fmt.Sprintf("Copying sample file to %q using `local` dataStream w/o auth", dest))
				err = importer.CopyImage(dest, fUrl, "", "")
				Expect(err).NotTo(HaveOccurred())
				defer func() {
					By(fmt.Sprintf("Removing the output file %q", dest))
					err := os.Remove(dest)
					Expect(err).NotTo(HaveOccurred())
				}()

				By(fmt.Sprintf("Checking size of the output file %q", dest))
				if useVSize {
					By("Checking output image virtual size")
					newSize := getImageVirtualSize(dest)
					Expect(newSize).To(Equal(size))
					By("Calling `Size` function to check size")
					newSize, err = imagesize.Size(fUrl, "", "")
					Expect(err).NotTo(HaveOccurred())
					Expect(newSize).To(Equal(size))
				} else {
					By("stat() output file")
					finfo, err = os.Stat(dest)
					Expect(err).NotTo(HaveOccurred())
					newSize := finfo.Size()
					Expect(newSize).To(Equal(size))
				}
				By(fmt.Sprintf("[%d] End test on image file %q", i, origFile))
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

func tempDir(parent string) string {
	const suffixSize = 8
	sufSource := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	rand.Seed(time.Now().UnixNano())
	suf := make([]rune, suffixSize)
	for i := range suf {
		suf[i] = sufSource[rand.Intn(len(sufSource))]
	}
	return filepath.Join(parent, fmt.Sprintf(".tmp-%s", string(suf)))
}

func generateTestFile(size int, filename string) ([]byte, error) {
	// Create a some random data to compress and/or archive.
	By("Generating test data")
	sampleData := make([]byte, size)
	if _, err := rand.Read(sampleData); err != nil {
		return nil, errors.Wrap(err, "unable to generate random number")
	}

	// Write the byte slice to a file at /
	// Trigger the defer Close before calling FormatTestData
	By("Writing test data to file")
	file, err := os.Create(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create test file %q", filename)
	}
	defer file.Close()
	if _, err = file.Write(sampleData); err != nil {
		return nil, errors.Wrap(err, "failed to write sample data to file")
	}
	return sampleData, nil
}
