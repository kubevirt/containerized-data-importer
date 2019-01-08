package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	dto "github.com/prometheus/client_model/go"

	"kubevirt.io/containerized-data-importer/pkg/util"

	"github.com/prometheus/client_golang/prometheus"
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
		util.StartPrometheusEndpoint(certsDirectory)
		time.Sleep(time.Second)
		empty, err = isDirEmpty(certsDirectory)
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeFalse())
		defer os.RemoveAll(certsDirectory)
	})
})

var _ = Describe("Update Progress", func() {
	BeforeEach(func() {
		progress = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "import_progress",
				Help: "The import progress in percentage",
			},
			[]string{"ownerUID"},
		)
	})

	It("Parse valid progress update", func() {
		By("Verifying the initial value is 0")
		progress.WithLabelValues(ownerUID).Add(0)
		metric := &dto.Metric{}
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
		By("Calling updateProgress with value")
		promReader := &prometheusProgressReader{
			CountingReader: util.CountingReader{
				Current: int64(45),
			},
			total: int64(100),
		}
		promReader.updateProgress()
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(45)))
	})

	It("0 total should return 0", func() {
		metric := &dto.Metric{}
		By("Calling updateProgress with value")
		promReader := &prometheusProgressReader{
			CountingReader: util.CountingReader{
				Current: int64(45),
			},
			total: int64(0),
		}
		promReader.updateProgress()
		progress.WithLabelValues(ownerUID).Write(metric)
		Expect(*metric.Counter.Value).To(Equal(float64(0)))
	})

})

var _ = Describe("Read total", func() {
	It("should read total from valid Reader", func() {
		reader := strings.NewReader("1234\n")
		result := readTotal(reader)
		Expect(result).To(Equal(int64(1234)))
		result = readTotal(reader)
		Expect(result).To(Equal(int64(-1)))
	})

	It("should read total from valid Reader", func() {
		reader := strings.NewReader("abc\n")
		result := readTotal(reader)
		Expect(result).To(Equal(int64(-1)))
	})

	It("should read total size from existing file", func() {
		file := filepath.Join(testImagesDir, "totalsize.txt")
		namedPipe = &file
		result, err := collectTotalSize()
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(int64(1234567890)))
	})

	It("should not read total size from non-existing file", func() {
		invalidFile := "idontexist"
		namedPipe = &invalidFile
		result, err := collectTotalSize()
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(int64(-1)))
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
		promReader := &prometheusProgressReader{
			CountingReader: util.CountingReader{
				Reader:  tarFileReader,
				Current: 0,
			},
			total: int64(10240), //10240 is the size of the tar containing the file.
		}
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
