// +build functional_test

package importer

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/importer"
	imagesize "kubevirt.io/containerized-data-importer/pkg/lib/size"
	f "kubevirt.io/containerized-data-importer/test/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

type testCase struct {
	testDesc      string
	srcData       io.Reader
	inFileName    string
	outFileName   string
	useVirtSize   bool
	expectFormats []string
}

type Tests []testCase

var _ = Describe("Streaming Data Conversion", func() {

	Context("when data is in a supported file format", func() {

		const (
			infilePath  = "tinyCore.iso"
			outfileBase = "tinyCore"
		)

		// Test Table
		tests := Tests{
			{
				testDesc:      "should decompress gzip",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".iso.gz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtGz},
			},
			{
				testDesc:      "should decompress xz",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".iso.xz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtXz},
			},
			{
				testDesc:      "should unarchive tar",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".iso.tar",
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar},
			},
			{
				testDesc:      "should unpack .tar.gz",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".iso.tar.gz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar, image.ExtGz},
			},
			{
				testDesc:      "should unpack .tar.xz",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".iso.tar.xz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtTar, image.ExtXz},
			},
			{
				testDesc:      "should convert .qcow2",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".qcow2",
				useVirtSize:   true,
				expectFormats: []string{image.ExtQcow2},
			},
			{
				testDesc:      "should convert and unpack .qcow2.gz",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".qcow2.gz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtGz},
			},
			{
				testDesc:      "should convert and unpack .qcow2.xz",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".qcow2.xz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtXz},
			},
			{
				testDesc:      "should convert and untar .qcow2.tar",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".qcow2.tar",
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar},
			},
			{
				testDesc:      "should convert and untar and unpack .qcow2.tar.gz",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".qcow2.tar.gz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar, image.ExtGz},
			},
			{
				testDesc:      "should convert and untar and unpack .qcow2.tar.xz",
				inFileName:    infilePath,
				outFileName:   outfileBase + ".qcow2.tar.xz",
				useVirtSize:   false,
				expectFormats: []string{image.ExtQcow2, image.ExtTar, image.ExtXz},
			},
			{
				testDesc:      "should pass through unformatted data",
				inFileName:    infilePath,
				outFileName:   infilePath,
				useVirtSize:   false,
				expectFormats: []string{""},
			},
		}

		for _, t := range tests {
			desc := t.testDesc
			ff := t.expectFormats
			fn := t.inFileName
			of := t.outFileName
			useVSize := t.useVirtSize

			It(desc, func() {
				By(fmt.Sprintf("Stating the source image file %q", fn))
				finfo, err := os.Stat(fn)
				Expect(err).NotTo(HaveOccurred())
				size := finfo.Size()

				By(fmt.Sprintf("Converting sample file to format: %v", ff))
				// Generate the expected data format from the random bytes
				sampleFilename, err := f.FormatTestData(fn, ff...)
				Expect(err).NotTo(HaveOccurred(), "Error formatting test data.")
				defer func() {
					if sampleFilename != fn { // don't rm source file
						os.Remove(sampleFilename) // ignore err
					}
				}()
				Expect(sampleFilename).To(Equal(of), "Test data filename doesn't match expected file name.")

				dest := filepath.Join(os.TempDir(), of)
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
					By("Calling `ImageSize` function to check size")
					newSize, err = imagesize.ImageSize(fUrl, "", "")
					Expect(err).NotTo(HaveOccurred())
					Expect(newSize).To(Equal(size))
				} else {
					By("Stating output file")
					finfo, err := os.Stat(dest)
					Expect(err).NotTo(HaveOccurred())
					newSize := finfo.Size()
					Expect(newSize).To(Equal(size))
				}

				//Expect(output.Bytes()).To(Equal(size)) // TODO replace with checksum?
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
