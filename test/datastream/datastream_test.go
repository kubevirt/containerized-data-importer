package datastream

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"

	"github.com/kubevirt/containerized-data-importer/pkg/image"
	f "github.com/kubevirt/containerized-data-importer/test/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type testCase struct {
	testDesc      string
	srcData       io.Reader
	inFileName    string
	expectFormats []string
}

type Tests []testCase

var _ = Describe("Streaming Data Conversion", func() {

	Context("when data is in a supported file format", func() {

		const (
			infilePath = "tinyCore.iso"
		)

		// Test Table
		tests := Tests{
			{
				testDesc:      "should decompress gzip",
				inFileName:    infilePath,
				expectFormats: []string{image.ExtGz},
			},
			{
				testDesc:      "should decompress xz",
				inFileName:    infilePath,
				expectFormats: []string{image.ExtXz},
			},
			{
				testDesc:      "should unarchive tar",
				inFileName:    infilePath,
				expectFormats: []string{image.ExtTar},
			},
			{
				testDesc:      "should unpack .tar.gz",
				inFileName:    infilePath,
				expectFormats: []string{image.ExtTar, image.ExtGz},
			},
			{
				testDesc:      "should unpack .tar.xz",
				inFileName:    infilePath,
				expectFormats: []string{image.ExtTar, image.ExtXz},
			},
			{
				testDesc:      "should convert .qcow2",
				inFileName:    infilePath,
				expectFormats: []string{image.ExtQcow2},
			},
			{
				testDesc:      "should pass through unformatted data",
				inFileName:    infilePath,
				expectFormats: []string{""},
			},
		}

		for _, t := range tests {

			desc := t.testDesc
			ff := t.expectFormats
			fn := t.inFileName

			It(desc, func() {

				By("Stating the source image file")
				finfo, err := os.Stat(fn)
				Expect(err).NotTo(HaveOccurred())
				size := finfo.Size()

				By(fmt.Sprintf("Converting sample file to format: %v", ff))
				// Generate the expected data format from the random bytes
				sampleFilename, err := f.FormatTestData(fn, ff...)
				Expect(err).NotTo(HaveOccurred(), "Error formatting test data.")

				By(fmt.Sprintf("Confirming sample file name %q matches expected file name %q", sampleFilename, fn+strings.Join(ff, "")))
				Expect(sampleFilename).To(Equal(fn+strings.Join(ff, "")), "Test data filename doesn't match expected file name.")

				// BEGIN TEST
				By("Opening sample file for test.")
				// Finally, open the file for reading
				sampleFile, err := os.Open(sampleFilename)
				Expect(err).NotTo(HaveOccurred(), "Failed to open sample file %s", sampleFilename)
				defer sampleFile.Close()

				By("Passing file reader to the data stream")
				r, err := image.UnpackData(sampleFilename, sampleFile)
				Expect(err).NotTo(HaveOccurred())
				defer r.Close()

				var output bytes.Buffer
				io.Copy(&output, r)

				By("Checking the output of the data stream")
				Expect(err).NotTo(HaveOccurred(), "ioutil.ReadAll erred")
				Expect(int64(output.Len())).To(Equal(size))
				//Expect(output.Bytes()).To(Equal(sampleData)) // TODO replace with checksum?
				By("Closing sample test file.")
			})
		}
	})
})

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
