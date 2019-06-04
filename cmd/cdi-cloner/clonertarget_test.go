package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/util"
	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

func init() {
	ownerUID = "1111-1111-111"
}

const testImagesDir = "../../tests/images"

var _ = Describe("Prometheus Endpoint", func() {
	It("Should start prometheus endpoint", func() {
		By("Creating cert directory, we can store self signed CAs")
		certsDirectory, err := ioutil.TempDir("", "certsdir")
		Expect(err).NotTo(HaveOccurred())
		empty, err := isDirEmpty(certsDirectory)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeTrue())
		prometheusutil.StartPrometheusEndpoint(certsDirectory)
		time.Sleep(time.Second)
		empty, err = isDirEmpty(certsDirectory)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeFalse())
		defer os.RemoveAll(certsDirectory)
	})
})

var _ = Describe("Read total", func() {
	It("should read total from valid Reader", func() {
		b := []byte(fmt.Sprintf("%016x", 1234))
		Expect(16).To(Equal(len(b)))
		reader := bytes.NewReader(b)
		result, err := readTotal(reader)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(uint64(1234)))
	})

	It("should read 0 total from valid Reader, with no error", func() {
		b := []byte(fmt.Sprintf("%016x", 0))
		Expect(16).To(Equal(len(b)))
		reader := bytes.NewReader(b)
		result, err := readTotal(reader)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(uint64(0)))
	})

	It("should read MAX uint64 total from valid Reader, with no error", func() {
		b := []byte(fmt.Sprintf("%016x", uint64(math.MaxUint64)))
		Expect(16).To(Equal(len(b)))
		reader := bytes.NewReader(b)
		result, err := readTotal(reader)
		Expect(err).NotTo(HaveOccurred())
		Expect(uint64(math.MaxUint64)).To(Equal(result))
	})

	It("should read total from valid Reader", func() {
		reader := strings.NewReader("abc\n")
		result, err := readTotal(reader)
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(uint64(0)))
	})

	It("should read total size from existing file", func() {
		file := filepath.Join(testImagesDir, "totalsize.txt")
		namedPipe = &file
		result, err := collectTotalSize()
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(uint64(math.MaxUint64)))
	})

	It("should not read total size from non-existing file", func() {
		invalidFile := "idontexist"
		namedPipe = &invalidFile
		result, err := collectTotalSize()
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(uint64(0)))
	})
})

var _ = Describe("Read total", func() {
	It("should extract files to the target directory", func() {
		targetDirectory, err := ioutil.TempDir("", "tardir")
		Expect(err).NotTo(HaveOccurred())
		empty, err := isDirEmpty(targetDirectory)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeTrue())

		defer os.RemoveAll(targetDirectory)
		tarFileName := filepath.Join(testImagesDir, "content.tar")
		tarFileReader, err := os.Open(tarFileName)
		Expect(err).NotTo(HaveOccurred())
		defer tarFileReader.Close()
		promReader := prometheusutil.NewProgressReader(tarFileReader, uint64(10240), progress, ownerUID)
		err = util.UnArchiveTar(promReader, targetDirectory)
		Expect(err).NotTo(HaveOccurred())
		empty, err = isDirEmpty(targetDirectory)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeFalse())
		outputFileName := filepath.Join(targetDirectory, "tar_content.txt")
		resultReader, err := os.Open(outputFileName)
		Expect(err).NotTo(HaveOccurred())
		resultScanner := bufio.NewScanner(resultReader)
		resultScanner.Scan()
		Expect(resultScanner.Text()).To(Equal("This is the actual content of the file"))
		resultScanner.Scan()
		Expect(resultScanner.Text()).To(Equal("Verify me"))

	})
})

func isDirEmpty(dirName string) (bool, error) {
	f, err := os.Open(dirName)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
