package util

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	pattern       = "^[a-zA-Z0-9]+$"
	TestImagesDir = "../../tests/images"
)

var (
	fileDir, _ = filepath.Abs(TestImagesDir)
)

var _ = Describe("All tests", func() {
	var _ = Describe("Copy files", func() {
		var destTmp string
		var err error

		BeforeEach(func() {
			destTmp, err = os.MkdirTemp("", "dest")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err = os.RemoveAll(destTmp)
			Expect(err).NotTo(HaveOccurred())
			os.Remove("test.txt")
		})

		It("Should copy file from source to dest, with valid source and dest", func() {
			err = CopyFile(filepath.Join(TestImagesDir, "content.tar"), filepath.Join(destTmp, "target.tar"))
			Expect(err).ToNot(HaveOccurred())
			sourceMd5, err := Md5sum(filepath.Join(TestImagesDir, "content.tar"))
			Expect(err).ToNot(HaveOccurred())
			targetMd5, err := Md5sum(filepath.Join(destTmp, "target.tar"))
			Expect(err).ToNot(HaveOccurred())
			Expect(sourceMd5).Should(Equal(targetMd5))
		})

		It("Should not copy file from source to dest, with invalid source", func() {
			err = CopyFile(filepath.Join(TestImagesDir, "content.tar22"), filepath.Join(destTmp, "target.tar"))
			Expect(err).To(HaveOccurred())
		})

		It("Should not copy file from source to dest, with invalid target", func() {
			err = CopyFile(filepath.Join(TestImagesDir, "content.tar"), filepath.Join("/invalidpath", "target.tar"))
			Expect(err).To(HaveOccurred())
		})
	})

	var _ = Describe("Zero out ranges in files", func() {
		var testFile *os.File
		var testData []byte
		testData = append(testData, bytes.Repeat([]byte{0x55}, 1024)...)
		testData = append(testData, bytes.Repeat([]byte{0xAA}, 1024)...)
		testData = append(testData, bytes.Repeat([]byte{0xFF}, 1024)...)

		BeforeEach(func() {
			var err error

			testFile, err = os.CreateTemp("", "test")
			Expect(err).ToNot(HaveOccurred())
			written, err := testFile.Write(testData)
			Expect(err).ToNot(HaveOccurred())
			Expect(written).To(Equal(len(testData)))
		})

		AfterEach(func() {
			testFile.Close()
			os.Remove(testFile.Name())
		})

		It("Should successfully zero a range with fallocate", func() {
			start := 512
			length := 100
			end := start + length
			err := PunchHole(testFile, int64(start), int64(length))
			Expect(err).ToNot(HaveOccurred())
			err = testFile.Sync()
			Expect(err).ToNot(HaveOccurred())
			err = testFile.Close()
			Expect(err).ToNot(HaveOccurred())
			data, err := os.ReadFile(testFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(HaveLen(len(testData)))
			comparison := bytes.Compare(data[start:end], bytes.Repeat([]byte{0}, length))
			Expect(comparison).To(Equal(0))
			comparison = bytes.Compare(data[0:start], testData[0:start])
			Expect(comparison).To(Equal(0))
			comparison = bytes.Compare(data[end:], testData[end:])
			Expect(comparison).To(Equal(0))
		})

		DescribeTable("Should successfully append zeroes to a file", func(appendFunction func(f *os.File, start, length int64) error) {
			length := 1024
			err := appendFunction(testFile, int64(len(testData)), int64(length))
			Expect(err).ToNot(HaveOccurred())
			err = testFile.Sync()
			Expect(err).ToNot(HaveOccurred())
			err = testFile.Close()
			Expect(err).ToNot(HaveOccurred())
			data, err := os.ReadFile(testFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(HaveLen(len(testData) + length))
			comparison := bytes.Compare(data[:len(testData)], testData)
			Expect(comparison).To(Equal(0))
			comparison = bytes.Compare(data[len(testData):], bytes.Repeat([]byte{0}, length))
			Expect(comparison).To(Equal(0))
		},
			Entry("using truncate", AppendZeroWithTruncate),
			Entry("using write", AppendZeroWithWrite),
		)

		DescribeTable("Should fail to append zeroes to a file using seek if it already has data at the specified starting index", func(appendFunction func(f *os.File, start, length int64) error) {
			length := 1024
			err := appendFunction(testFile, 0, int64(length))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(MatchRegexp(".*cannot safely append.*"))
		},
			Entry("using truncate", AppendZeroWithTruncate),
			Entry("using write", AppendZeroWithWrite),
		)
	})

	Describe("StreamDataToFile tests", func() {
		Context("with tmp directory", func() {
			var destTmp string
			var err error
			var random *rand.Rand

			BeforeEach(func() {
				seed := time.Now().UnixNano()
				By(fmt.Sprintf("Random Seed: %d", seed))
				// #nosec G404 - seed is not used for cryptographic purposes
				random = rand.New(rand.NewSource(seed))
				destTmp, err = os.MkdirTemp("", "dest")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				err = os.RemoveAll(destTmp)
				Expect(err).NotTo(HaveOccurred())
			})

			randBool := func() bool {
				return random.Intn(2) == 1
			}

			DescribeTable("Should stream data to file", func(writeSize int, preallocate bool) {
				const (
					totalBytes     int64 = 1024 * 1024
					sparseBoundary int64 = 32 * 1024
				)

				// validate  writeSize
				Expect(totalBytes % int64(writeSize)).To(Equal(int64(0)))

				writeMap := make(map[int64]bool)
				destName := filepath.Join(destTmp, "disk.img")
				byteBuf := bytes.NewBuffer(make([]byte, 0, totalBytes))
				expectedBytesWritten := totalBytes

				for curOffset := int64(0); curOffset < totalBytes; curOffset += int64(writeSize) {
					b := byte(0)
					if randBool() {
						if !preallocate {
							boundaryEnd := curOffset + int64(writeSize)
							for boundaryStart := curOffset; boundaryStart < boundaryEnd; boundaryStart += sparseBoundary {
								sparseOffset := boundaryStart / sparseBoundary
								writeMap[sparseOffset] = true
							}
						}
						b = 1
					}
					byteBuf.Write(bytes.Repeat([]byte{b}, writeSize))
				}

				if !preallocate {
					expectedBytesWritten = int64(len(writeMap)) * sparseBoundary
				}

				bytesRead, bytesWritten, err := StreamDataToFile(bytes.NewReader(byteBuf.Bytes()), destName, preallocate)
				Expect(err).ToNot(HaveOccurred())
				Expect(bytesRead).To(Equal(totalBytes))
				Expect(bytesWritten).To(Equal(expectedBytesWritten))

				fileData, err := os.ReadFile(destName)
				Expect(err).ToNot(HaveOccurred())
				Expect(fileData).To(Equal(byteBuf.Bytes()))
			},
				Entry("without preallocation 4k block", 4*1024, false),
				Entry("without preallocation 16k block", 16*1024, false),
				Entry("without preallocation 32k block", 32*1024, false),
				Entry("without preallocation 64k block", 64*1024, false),
				Entry("without preallocation 128k block", 128*1024, false),
				Entry("with preallocation 16k block", 16*1024, true),
				Entry("with preallocation 32k block", 32*1024, true),
				Entry("with preallocation 64k block", 64*1024, true),
			)
		})
	})
})
