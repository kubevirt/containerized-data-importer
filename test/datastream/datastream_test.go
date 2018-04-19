// +build functional_test

package datastream

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kubevirt/containerized-data-importer/pkg/image"
	"github.com/kubevirt/containerized-data-importer/pkg/importer"
	f "github.com/kubevirt/containerized-data-importer/test/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
				Expect(sampleFilename).To(Equal(of), "Test data filename doesn't match expected file name.")

				By("Creating a local dataStream w/o auth credentials")
				fUrl := "file:/" + sampleFilename
				ep, err := importer.ParseEndpoint(fUrl)
				Expect(err).NotTo(HaveOccurred())
				ds := importer.NewDataStream(ep, "", "")

				dest := filepath.Join(os.TempDir(), of)
				By(fmt.Sprintf("Copying the sample file to %q", dest))
				err = ds.Copy(dest)
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("Checking the output file %q", dest))
				if useVSize {
					By("Checking output image virtual size")
					Expect(getImageVirtualSize(dest)).To(Equal(size))
				} else {
					By("Stating output file")
					finfo, err := os.Stat(dest)
					Expect(err).NotTo(HaveOccurred())
					Expect(finfo.Size()).To(Equal(size))
				}
				By(fmt.Sprintf("Removing the output file %q", dest))
				err = os.Remove(dest)
				Expect(err).NotTo(HaveOccurred())
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
		return nil, fmt.Errorf("error occurred during rand.Read()")
	}

	// Write the byte slice to a file at /
	// Trigger the defer Close before calling FormatTestData
	By("Writing test data to file")
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create test file %q", filename)
	}
	defer file.Close()
	if _, err = file.Write(sampleData); err != nil {
		return nil, fmt.Errorf("failed to write sample data to file")
	}
	return sampleData, nil
}
