package importer

import (
	"crypto/sha256"
	"encoding/hex"
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

	DescribeTable("can construct readers", func(filename string, numRdrs int, wantErr, archived, convert bool) {
		f, err := os.Open(filename)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()

		fr, err = NewFormatReaders(f, uint64(0), nil)
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
		Entry("successfully construct a xz reader", tinyCoreXzFilePath, 4, false, true, false),              // [stream, multi-r, xz, multi-r] convert = false
		Entry("successfully construct a gz reader", tinyCoreGzFilePath, 4, false, true, false),              // [stream, multi-r, gz, multi-r] convert = false
		Entry("successfully return the base reader when archived", archiveFilePath, 3, false, false, false), // [stream, multi-r, multi-r] convert = false
		Entry("successfully construct qcow2 reader", cirrosFilePath, 2, false, false, true),                 // [stream, multi-r] convert = true
		Entry("successfully construct .iso reader", tinyCoreFilePath, 2, false, false, false),               // [stream, multi-r] convert = false
	)

	DescribeTable("can append readers", func(rType int, r interface{}, numRdrs int, isCloser bool) {
		f, err := os.Open(cirrosFilePath)
		Expect(err).ToNot(HaveOccurred())
		defer f.Close()
		fr, err = NewFormatReaders(f, uint64(0), nil)
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
		testReader, err := NewFormatReaders(stringReader, uint64(0), nil)
		// Not passing a real string, so the header checking will fail.
		Expect(err).To(HaveOccurred())
		Expect(testReader.progressReader).To(BeNil())
		// This should not crash
		testReader.StartProgressUpdate()
	})

	Describe("with checksum validator", func() {
		var (
			testData       []byte
			testDataSHA256 string
		)

		BeforeEach(func() {
			// Read test file and calculate its SHA256
			var err error
			testData, err = os.ReadFile(tinyCoreFilePath)
			Expect(err).NotTo(HaveOccurred())
			hash := sha256.Sum256(testData)
			testDataSHA256 = hex.EncodeToString(hash[:])
			By(fmt.Sprintf("Test file SHA256: %s, length: %d bytes", testDataSHA256, len(testData)))
		})

		DescribeTable("should validate checksum correctly", func(filename string, useValidChecksum bool, wantValidationErr bool) {
			// Calculate actual checksum for the file
			fileData, err := os.ReadFile(filename)
			Expect(err).NotTo(HaveOccurred())
			hash := sha256.Sum256(fileData)
			actualSHA256 := hex.EncodeToString(hash[:])

			var checksumStr string
			if useValidChecksum {
				checksumStr = fmt.Sprintf("sha256:%s", actualSHA256)
			} else {
				// Use wrong checksum
				checksumStr = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			}

			validator, err := NewChecksumValidator(checksumStr)
			Expect(err).NotTo(HaveOccurred())

			// Open file for reading
			f, err := os.Open(filename)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			fr, err = NewFormatReaders(f, uint64(0), validator)
			Expect(err).ToNot(HaveOccurred())

			// Read all data through the reader stack
			_, err = io.Copy(io.Discard, fr.TopReader())
			Expect(err).NotTo(HaveOccurred())

			// Validate checksum
			err = fr.ValidateChecksum()
			if wantValidationErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
			Entry("should succeed with valid SHA256 for ISO file", tinyCoreFilePath, true, false),
			Entry("should fail with invalid checksum", tinyCoreFilePath, false, true),
		)

		It("should calculate and validate checksum for ISO file", func() {
			f, err := os.Open(tinyCoreFilePath)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			checksumStr := fmt.Sprintf("sha256:%s", testDataSHA256)
			validator, err := NewChecksumValidator(checksumStr)
			Expect(err).NotTo(HaveOccurred())

			fr, err = NewFormatReaders(f, uint64(0), validator)
			Expect(err).ToNot(HaveOccurred())

			// Verify checksum validator is in the reader stack
			Expect(fr.checksumValidator).NotTo(BeNil())
			Expect(fr.checksumValidator.Algorithm()).To(Equal("sha256"))

			// Read all data
			_, err = io.Copy(io.Discard, fr.TopReader())
			Expect(err).NotTo(HaveOccurred())

			// Validate checksum should succeed
			err = fr.ValidateChecksum()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail validation with wrong checksum", func() {
			f, err := os.Open(tinyCoreFilePath)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			// Use wrong checksum
			wrongChecksum := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			validator, err := NewChecksumValidator(wrongChecksum)
			Expect(err).NotTo(HaveOccurred())

			fr, err = NewFormatReaders(f, uint64(0), validator)
			Expect(err).ToNot(HaveOccurred())

			// Read all data
			_, err = io.Copy(io.Discard, fr.TopReader())
			Expect(err).NotTo(HaveOccurred())

			// Validate checksum should fail
			err = fr.ValidateChecksum()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("checksum mismatch"))
		})

		It("should work with compressed files and checksum", func() {
			// Calculate checksum for the compressed file
			compressedData, err := os.ReadFile(tinyCoreGzFilePath)
			Expect(err).NotTo(HaveOccurred())
			hash := sha256.Sum256(compressedData)
			compressedSHA256 := hex.EncodeToString(hash[:])

			f, err := os.Open(tinyCoreGzFilePath)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			checksumStr := fmt.Sprintf("sha256:%s", compressedSHA256)
			validator, err := NewChecksumValidator(checksumStr)
			Expect(err).NotTo(HaveOccurred())

			fr, err = NewFormatReaders(f, uint64(0), validator)
			Expect(err).ToNot(HaveOccurred())

			// Read all data (this will decompress)
			_, err = io.Copy(io.Discard, fr.TopReader())
			Expect(err).NotTo(HaveOccurred())

			// Validate checksum should succeed (validates the compressed data)
			err = fr.ValidateChecksum()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should work with qcow2 files and checksum", func() {
			// Calculate checksum for qcow2 file
			qcow2Data, err := os.ReadFile(cirrosFilePath)
			Expect(err).NotTo(HaveOccurred())
			hash := sha256.Sum256(qcow2Data)
			qcow2SHA256 := hex.EncodeToString(hash[:])

			f, err := os.Open(cirrosFilePath)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			checksumStr := fmt.Sprintf("sha256:%s", qcow2SHA256)
			validator, err := NewChecksumValidator(checksumStr)
			Expect(err).NotTo(HaveOccurred())

			fr, err = NewFormatReaders(f, uint64(0), validator)
			Expect(err).ToNot(HaveOccurred())

			// Read all data
			_, err = io.Copy(io.Discard, fr.TopReader())
			Expect(err).NotTo(HaveOccurred())

			// Validate checksum should succeed
			err = fr.ValidateChecksum()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle nil validator gracefully", func() {
			f, err := os.Open(tinyCoreFilePath)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			fr, err = NewFormatReaders(f, uint64(0), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(fr.checksumValidator).To(BeNil())

			// Read all data
			_, err = io.Copy(io.Discard, fr.TopReader())
			Expect(err).NotTo(HaveOccurred())

			// Validate checksum should not error when validator is nil
			err = fr.ValidateChecksum()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
