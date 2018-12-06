package importer

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const TestImagesDir = "../../tests/images"

const (
	SmallActualSize  = 1024
	SmallVirtualSize = 1024
	BigActualSize    = 50000000
	BigVirtualSize   = 50000000
)

var fakeBigImageInfo = image.ImgInfo{Format: "", BackingFile: "", VirtualSize: BigVirtualSize, ActualSize: BigActualSize}
var fakeSmallImageInfo = image.ImgInfo{Format: "", BackingFile: "", VirtualSize: SmallVirtualSize, ActualSize: SmallActualSize}
var fakeZeroImageInfo = image.ImgInfo{Format: "", BackingFile: "", VirtualSize: 0, ActualSize: 0}
var fakeInfoRet = fakeInfoOpRetVal{imgInfo: &fakeSmallImageInfo, e: nil}
var imageDir, _ = filepath.Abs(TestImagesDir)
var cirrosFileName = "cirros-qcow2.img"
var tinyCoreFileName = "tinyCore.iso"
var archiveFileName = "archive.tar"
var archiveFileNameWithoutExt = strings.TrimSuffix(archiveFileName, filepath.Ext(archiveFileName))
var cirrosFilePath = filepath.Join(imageDir, cirrosFileName)
var tinyCoreFilePath = filepath.Join(imageDir, tinyCoreFileName)
var tinyCoreXzFilePath, _ = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtXz)
var tinyCoreGzFilePath, _ = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtGz)
var tinyCoreTarFilePath, _ = utils.FormatTestData(tinyCoreFilePath, os.TempDir(), image.ExtTar)
var archiveFilePath, _ = utils.ArchiveFiles(archiveFileNameWithoutExt, os.TempDir(), tinyCoreFilePath, cirrosFilePath)
var testfiles = []string{tinyCoreXzFilePath, tinyCoreGzFilePath, tinyCoreTarFilePath, archiveFilePath}
var cirrosData, _ = readFile(cirrosFilePath)
var tinyCoreData, _ = readFile(tinyCoreFilePath)
var archiveData, _ = readFile(archiveFilePath)
var stringRdr = strings.NewReader("test data for reader 1")
var qcow2Rdr, _ = os.Open(cirrosFilePath)
var testFileRdrs = bufio.NewReader(qcow2Rdr)

type fakeInfoOpRetVal struct {
	imgInfo *image.ImgInfo
	e       error
}

func (r *fakeInfoOpRetVal) err(e error) fakeInfoOpRetVal {
	r.e = e
	return *r
}

type fakeQEMUOperations struct {
	e1   error
	e2   error
	e3   error
	ret4 fakeInfoOpRetVal
	e5   error
	e6   error
}

// EndlessReader doesn't return any value read, te r
type EndlessReader struct {
	Reader io.ReadCloser
}

var _ = Describe("Data Stream", func() {
	var ts *httptest.Server

	BeforeEach(func() {
		By("[BeforeEach] Creating test server")
		ts = createTestServer(imageDir)
	})

	AfterEach(func() {
		By("[AfterEach] closing test server")
		ts.Close()
	})

	table.DescribeTable("with import source should", func(image, accessKeyID, secretKey, contentType string, qemu bool, want []byte, wantErr bool) {
		if image != "" {
			image = ts.URL + "/" + image
		}
		dest := common.ImporterWritePath
		if contentType == controller.ContentTypeArchive {
			dest = common.ImporterVolumePath
		}
		By(fmt.Sprintf("Creating new datastream for %s", image))
		ds, err := NewDataStream(&DataStreamOptions{
			dest,
			image,
			accessKeyID,
			secretKey,
			controller.SourceHTTP,
			contentType,
			""})
		if ds != nil && len(ds.Readers) > 0 {
			defer ds.Close()
		}
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			resultBuffer := make([]byte, len(want))
			By("Reading data from buffer")
			ds.Read(resultBuffer)
			Expect(ds.AccessKey).To(Equal(accessKeyID))
			Expect(ds.SecKey).To(Equal(secretKey))
			Expect(ds.ContentType).To(Equal(contentType))

			Expect(ds.qemu).To(Equal(qemu))
			Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("expect NewDataStream to succeed with valid image", cirrosFileName, "", "", controller.ContentTypeKubevirt, true, cirrosData, false),
		table.Entry("expect NewDataStream to fail with non existing image", "badimage.iso", "", "", controller.ContentTypeKubevirt, false, nil, true),
		table.Entry("expect NewDataStream to fail with invalid or missing image", "", "", "", controller.ContentTypeKubevirt, false, nil, true),
		table.Entry("expect NewDataStream to succeed with valid iso image", tinyCoreFileName, "accessKey", "secretKey", controller.ContentTypeKubevirt, false, tinyCoreData, false),
		table.Entry("expect NewDataStream to fail with a valid image and an incorrect content", cirrosFileName, "", "", controller.ContentTypeArchive, true, cirrosData, true),
	)

	It("can close all readers", func() {
		By(fmt.Sprintf("Creating new datastream for %s", ts.URL+"/"+tinyCoreFileName))
		ds, err := NewDataStream(&DataStreamOptions{
			common.ImporterWritePath,
			ts.URL + "/" + tinyCoreFileName,
			"",
			"",
			controller.SourceHTTP,
			controller.ContentTypeKubevirt,
			"1G"})
		Expect(err).NotTo(HaveOccurred())
		By("Closing data stream")
		err = ds.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	table.DescribeTable("can build selector", func(image, ep string, wantErr bool) {
		if ep == "" {
			ep = ts.URL
		}
		By(fmt.Sprintf("Creating new datastream for %s", ep+"/"+image))
		ds, err := NewDataStream(&DataStreamOptions{
			common.ImporterWritePath,
			ep + "/" + image,
			"",
			"",
			controller.SourceHTTP,
			controller.ContentTypeKubevirt,
			"20M"})
		if ds != nil && len(ds.Readers) > 0 {
			defer ds.Close()
		}
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			err = ds.dataStreamSelector()
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("success building selector for valid endpoint and file", tinyCoreFileName, "", false),
		table.Entry("fail trying to build selector for invalid http endpoint", "", "http://www.google.com", true),
		table.Entry("fail trying to build invalid selector", tinyCoreFileName, "fake://somefakefile", true),
	)

	table.DescribeTable("can construct readers", func(filename, contentType string, numRdrs int, wantErr bool) {
		By(fmt.Sprintf("Creating new fileserver for %s and file %s", filepath.Dir(filename), filepath.Base(filename)))
		tempTestServer := createTestServer(filepath.Dir(filename))
		dest := common.ImporterWritePath
		if contentType == controller.ContentTypeArchive {
			dest = common.ImporterVolumePath
		}
		By(fmt.Sprintf("Creating new datastream to %s", tempTestServer.URL+"/"+filepath.Base(filename)))
		ds, err := NewDataStream(&DataStreamOptions{
			dest,
			tempTestServer.URL + "/" + filepath.Base(filename),
			"",
			"",
			controller.SourceHTTP,
			contentType,
			"20M"})
		defer func() {
			tempTestServer.Close()
		}()
		if ds != nil && len(ds.Readers) > 0 {
			defer ds.Close()
		}

		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			for _, r := range ds.Readers {
				fmt.Fprintf(GinkgoWriter, "INFO: Reader type: %d\n", r.rdrType)
			}
			Expect(len(ds.Readers)).To(Equal(numRdrs))
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("successfully construct a xz reader", tinyCoreXzFilePath, controller.ContentTypeKubevirt, 5, false),     // [http, multi-r, xz, multi-r]
		table.Entry("successfully construct a gz reader", tinyCoreGzFilePath, controller.ContentTypeKubevirt, 5, false),     // [http, multi-r, gz, multi-r]
		table.Entry("successfully construct a tar reader", tinyCoreTarFilePath, controller.ContentTypeKubevirt, 4, false),   // [http, multi-r, tar, multi-r]
		table.Entry("successfully constructed an archive reader", archiveFilePath, controller.ContentTypeArchive, 4, false), // [http, multi-r, mul-tar, multi-r]
		table.Entry("successfully construct qcow2 reader", cirrosFilePath, controller.ContentTypeKubevirt, 2, false),        // [http, multi-r]
		table.Entry("successfully construct .iso reader", tinyCoreFilePath, controller.ContentTypeKubevirt, 3, false),       // [http, multi-r]
		table.Entry("fail constructing reader for invalid file path", filepath.Join(imageDir, "tinyCorebad.iso"), controller.ContentTypeKubevirt, 0, true),
		table.Entry("fail constructing reader for a valid archive file and a wrong content", archiveFileName, controller.ContentTypeKubevirt, 0, true),
	)
})

var _ = Describe("SaveStream", func() {

	It("Should successfully save the stream", func() {
		defer os.Remove("testqcow2file")
		replaceQEMUOperations(NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil, fakeInfoRet, nil, nil), func() {
			rdr, err := os.Open(cirrosFilePath)
			Expect(err).NotTo(HaveOccurred())
			defer rdr.Close()
			_, err = SaveStream(rdr, "testqcow2file")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("Copy", func() {
	var ts *httptest.Server

	BeforeEach(func() {
		By("[BeforeEach] Creating test server")
		ts = createTestServer(imageDir)
	})

	AfterEach(func() {
		By("[AfterEach] closing test server")
		ts.Close()
	})

	table.DescribeTable("Image, with import source should", func(dest, endpt string, qemuOperations image.QEMUOperations, wantErr bool) {
		By("Configuring endpoint")

		endpt = ts.URL + "/" + endpt
		By(fmt.Sprintf("end point is %s", endpt))
		defer os.Remove(dest)
		By("Replacing QEMU Operations")
		replaceQEMUOperations(qemuOperations, func() {
			By("Copying image")
			err := CopyData(&DataStreamOptions{
				dest,
				endpt,
				"",
				"",
				controller.SourceHTTP,
				controller.ContentTypeKubevirt,
				""})
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	},
		table.Entry("successfully copy local image", "tinyCore.raw", tinyCoreFileName, NewQEMUAllErrors(), false),
		table.Entry("expect failure trying to copy non-existing local image", "cdi-testcopy", "tinyCoreBad.iso", NewQEMUAllErrors(), true),
		table.Entry("successfully copy streaming image", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(errors.New("should not be called"), nil, nil, fakeInfoRet, nil, nil), false),
		table.Entry("streaming image qemu validation fails", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(nil, nil, nil, fakeInfoRet, errors.New("invalid image"), nil), true),
		table.Entry("streaming image qemu convert fails", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(nil, errors.New("exit 1"), nil, fakeInfoRet, nil, nil), true),
		table.Entry("streaming image qemu convert succeeds since there is no space validation on streaming", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeBigImageInfo, nil}, nil, nil), false),
		table.Entry("streaming image qemu convert suscceeds since there is no space validation on streaming", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil), false),
	)

	table.DescribeTable("Archived image, with import source should", func(originalFile string, qemuOperations image.QEMUOperations, wantErr bool, expectFormats ...string) {
		baseTestImage := filepath.Join(imageDir, originalFile)
		//createt test temp directory
		tmpTestDir := testDir(os.TempDir())
		syscall.Umask(0000)
		err := os.Mkdir(tmpTestDir, 0777)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Converting source file to format: %s", expectFormats))
		// Generate the expected data format from the random bytes
		testSample, err := utils.FormatTestData(baseTestImage, tmpTestDir, expectFormats...)
		Expect(err).NotTo(HaveOccurred(), "Error formatting test data.")
		fmt.Fprintf(GinkgoWriter, "INFO: converted source file name is %q, in dir %q\n", testSample, tmpTestDir)

		tempTestServer := createTestServer(filepath.Dir(testSample))
		defer tempTestServer.Close()

		dest := common.DiskImageName
		testBase := filepath.Base(testSample)
		testTarget := filepath.Join(tmpTestDir, dest)
		replaceQEMUOperations(qemuOperations, func() {
			By(fmt.Sprintf("Importing %q to %q", tempTestServer.URL, testTarget))
			err = CopyData(&DataStreamOptions{
				testTarget,
				tempTestServer.URL + "/" + testBase,
				"",
				"",
				controller.SourceHTTP,
				controller.ContentTypeKubevirt,
				"1G"})
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		os.RemoveAll(tmpTestDir)
	},
		// Should fail
		table.Entry("should fail due to insuficient space when trying to convert  qcow2 to raw. Initial format .qcow2.gz", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeBigImageInfo, errors.New("qcow2 image conversion to raw failed due to insuficient space")}, nil, nil), true, image.ExtQcow2, image.ExtGz),
		table.Entry("should fail due to insuficient space when trying to convert  qcow2 to raw. Initial format .qcow2.xz)", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeBigImageInfo, errors.New("qcow2 image conversion to raw failed due to insuficient space")}, nil, nil), true, image.ExtQcow2, image.ExtXz),
		table.Entry("should fail due to insuficient space when trying to convert  qcow2 to raw. Initial format .qcow2.tar", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeBigImageInfo, errors.New("qcow2 image conversion to raw failed due to insuficient space")}, nil, nil), true, image.ExtQcow2, image.ExtTar),

		table.Entry("should fails due to invalid size info in qcow2 info struct. Initial format .qcow2.gz", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Local image validation failed - no image size info is provided")}, nil, nil), true, image.ExtQcow2, image.ExtGz),
		table.Entry("should fails due to invalid size info in qcow2 info struct. Initial format .qcow2.xz", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Local image validation failed - no image size info is provided")}, nil, nil), true, image.ExtQcow2, image.ExtXz),
		table.Entry("should fails due to invalid size info in qcow2 info struct. Initial format .qcow2.tar", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Local image validation failed - no image size info is provided")}, nil, nil), true, image.ExtQcow2, image.ExtTar),

		//Should succeed
		table.Entry("should succeed to convert  qcow2 to raw .qcow2.gz", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoRet, nil, nil), false, image.ExtQcow2, image.ExtGz),
		table.Entry("should succeed to convert  qcow2 to raw .qcow2.xz", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoRet, nil, nil), false, image.ExtQcow2, image.ExtXz),
		table.Entry("should succeed to convert  qcow2 to raw .qcow2.tar", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoRet, nil, nil), false, image.ExtQcow2, image.ExtTar),
	)

	table.DescribeTable("internal copy", func(r io.Reader, out string, qemu bool, qemuOperations image.QEMUOperations, wantErr bool) {
		defer os.Remove(out)
		By("Replacing QEMU Operations")
		replaceQEMUOperations(qemuOperations, func() {
			err := copy(r, out, qemu, "")
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	},
		table.Entry("successfully copy reader", stringRdr, "testoutfile", false, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil, fakeInfoRet, nil, nil), false),
		table.Entry("successfully copy qcow2 reader", testFileRdrs, "testqcow2file", true, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil, fakeInfoRet, nil, nil), false),
		table.Entry("expect error trying to copy invalid format", testFileRdrs, "testinvalidfile", true, NewFakeQEMUOperations(nil, nil, nil, fakeInfoRet, errors.New("invalid format"), nil), true),
		table.Entry("expect error trying to copy qemu process fails", testFileRdrs, "testinvalidfile", true, NewFakeQEMUOperations(errors.New("exit 1"), nil, nil, fakeInfoRet, nil, nil), true),
		table.Entry("copy fails due to insuficient space", testFileRdrs, "testqcow2file", true, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil, fakeInfoOpRetVal{&fakeBigImageInfo, errors.New("qcow2 image conversion to raw failed due to insuficient space")}, nil, nil), true),
		table.Entry("copy fails due to invalid info in qcow2 info struct", testFileRdrs, "testqcow2file", true, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Local image validation failed - no image size info is provided")}, nil, nil), true),
	)
})

var _ = Describe("http", func() {
	It("Should properly finish with valid reader", func() {
		By("Creating context for the transfer, we have the ability to cancel it")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ds := &DataStream{
			ctx:    ctx,
			cancel: cancel,
		}
		By("Creating string reader we can test just the poll progress part")
		stringReader := ioutil.NopCloser(strings.NewReader("This is a test string"))
		endlessReader := EndlessReader{
			Reader: stringReader,
		}
		countingReader := &util.CountingReader{
			Reader:  &endlessReader,
			Current: 0,
		}
		By("Creating pollProgress as go routine, we can use channels to monitor progress")
		go ds.pollProgress(countingReader, 5*time.Second, time.Second)
		By("Waiting for timeout or success")
		select {
		case <-time.After(10 * time.Second):
			Fail("Transfer not cancelled after 10 seconds")
		case <-ds.ctx.Done():
			By("Having context be done, we confirm finishing of transfer")
		}
	})
})

var _ = Describe("close readers", func() {
	type args struct {
		readers []reader
	}

	rdrs1 := ioutil.NopCloser(strings.NewReader("test data for reader 1"))
	rdrs2 := ioutil.NopCloser(strings.NewReader("test data for reader 2"))
	rdrA := reader{rdrGz, rdrs1}
	rdrB := reader{rdrXz, rdrs2}

	rdrsTest := []reader{rdrA, rdrB}

	It("Should successfully close readers", func() {
		err := closeReaders(rdrsTest)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("Random file name", func() {
	const numbyte = 8

	It("create expected random name", func() {
		randName := make([]byte, numbyte)
		rand.Read(randName)
		wantString := hex.EncodeToString(randName)

		got := randTmpName("testfile.img")
		base, fn := filepath.Split(got)

		Expect(len(fn)).To(Equal(len("testfile.img") + len(wantString)))
		Expect(filepath.Clean(base)).To(Equal(filepath.Dir("testfile.img")))
		Expect(filepath.Ext(fn)).To(Equal(".img"))
	})
})

var _ = Describe("Streaming Data Conversion", func() {
	var tmpTestDir string

	BeforeEach(func() {
		By(fmt.Sprintf("[BeforeEach] Creating temporary dir %s", tmpTestDir))
		tmpTestDir = testDir(os.TempDir())
		syscall.Umask(0000)
		err := os.Mkdir(tmpTestDir, 0777)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		By(fmt.Sprintf("[AfterEach] Cleaning up temporary dir %s", tmpTestDir))
		os.RemoveAll(tmpTestDir)
	})

	table.DescribeTable("when data is in a supported file format", func(originalFile string, useVirtSize bool, expectFormats ...string) {
		By(fmt.Sprintf("Getting size of source file %q", originalFile))
		finfo, err := os.Stat(originalFile)
		Expect(err).NotTo(HaveOccurred())
		sourceSize := finfo.Size()
		fmt.Fprintf(GinkgoWriter, "INFO: size = %d\n", sourceSize)

		By(fmt.Sprintf("Converting source file to format: %s", expectFormats))
		// Generate the expected data format from the random bytes
		testSample, err := utils.FormatTestData(originalFile, tmpTestDir, expectFormats...)
		Expect(err).NotTo(HaveOccurred(), "Error formatting test data.")
		fmt.Fprintf(GinkgoWriter, "INFO: converted source file name is %q, in dir %q\n", testSample, tmpTestDir)

		tempTestServer := createTestServer(filepath.Dir(testSample))
		defer tempTestServer.Close()

		testBase := filepath.Base(testSample)
		testTarget := filepath.Join(tmpTestDir, common.DiskImageName)
		By(fmt.Sprintf("Importing %q to %q", tempTestServer.URL, testTarget))
		err = CopyData(&DataStreamOptions{
			testTarget,
			tempTestServer.URL + "/" + testBase,
			"",
			"",
			controller.SourceHTTP,
			controller.ContentTypeKubevirt,
			""})
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Checking size of the output file %q", testTarget))
		var targetSize int64
		if useVirtSize {
			By("... using output image's virtual size")
			targetSize = getImageVirtualSize(testTarget)
			Expect(targetSize).To(Equal(int64(sourceSize)))
		} else {
			By("... using stat()")
			finfo, err = os.Stat(testTarget)
			Expect(err).NotTo(HaveOccurred())
			targetSize = finfo.Size()
			Expect(targetSize).To(Equal(int64(sourceSize)))
		}
		fmt.Fprintf(GinkgoWriter, "INFO: byte size = %d\n", targetSize)

		By(fmt.Sprintf("Calling `size.Size()` on same endpoint %q", tempTestServer.URL))
		// extract the file extension(s) and check if file should be skipped
		i := strings.Index(testBase, ".")
		if i > 0 {
			targetExt := testBase[i:]
			if _, ok := sizeExceptions[targetExt]; ok {
				Skip(fmt.Sprintf("*** skipping endpoint extension %q as exception", targetExt))
			}
		}
		ds, err := NewDataStream(&DataStreamOptions{
			common.ImporterWritePath,
			tempTestServer.URL + "/" + testBase,
			"",
			"",
			controller.SourceHTTP,
			controller.ContentTypeKubevirt,
			"1G"})

		Expect(err).NotTo(HaveOccurred())
		defer ds.Close()
		Expect(ds.Size).To(Equal(sourceSize))

		fmt.Fprintf(GinkgoWriter, "End test on test file %q\n", testSample)
	},
		table.Entry("should decompress gzip", tinyCoreFilePath, false, image.ExtGz),
		table.Entry("should decompress xz", tinyCoreFilePath, false, image.ExtXz),
		table.Entry("should unarchive tar", tinyCoreFilePath, false, image.ExtTar),
		table.Entry("should unpack .tar.gz", tinyCoreFilePath, false, image.ExtTar, image.ExtGz),
		// Disabled until issue 335 is resolved
		// https://github.com/kubevirt/containerized-data-importer/issues/335
		//table.Entry("should unpack .tar.xz", tinyCoreFilePath, false, image.ExtTar, image.ExtXz),
		table.Entry("should convert .qcow2", tinyCoreFilePath, true, image.ExtQcow2),
		table.Entry("should convert and unpack .qcow2.gz", tinyCoreFilePath, false, image.ExtQcow2, image.ExtGz),
		table.Entry("should convert and unpack .qcow2.xz", tinyCoreFilePath, false, image.ExtQcow2, image.ExtXz),
		table.Entry("should convert and untar .qcow2.tar", tinyCoreFilePath, false, image.ExtQcow2, image.ExtTar),
		table.Entry("should convert and untar and unpack .qcow2.tar.gz", tinyCoreFilePath, false, image.ExtQcow2, image.ExtTar, image.ExtGz),
		table.Entry("should convert and untar and unpack .qcow2.tar.xz", tinyCoreFilePath, false, image.ExtQcow2, image.ExtTar, image.ExtXz),
		table.Entry("should pass through unformatted data", tinyCoreFilePath, false, ""),
	)
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

// Read the contents of the file into a byte array, don't use this on really huge files.
func readFile(fileName string) ([]byte, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result, err := ioutil.ReadAll(f)
	return result, err
}

func createTestServer(imageDir string) *httptest.Server {
	return httptest.NewServer(http.FileServer(http.Dir(imageDir)))
}

func replaceQEMUOperations(replacement image.QEMUOperations, f func()) {
	orig := qemuOperations
	if replacement != nil {
		qemuOperations = replacement
		defer func() { qemuOperations = orig }()
	}
	f()
}

func NewQEMUAllErrors() image.QEMUOperations {
	err := errors.New("qemu should not be called from this test override with replaceQEMUOperations")
	return NewFakeQEMUOperations(err, err, err, fakeInfoOpRetVal{nil, err}, err, err)
}

func NewFakeQEMUOperations(e1, e2, e3 error, ret4 fakeInfoOpRetVal, e5 error, e6 error) image.QEMUOperations {
	return &fakeQEMUOperations{e1, e2, e3, ret4, e5, e6}
}

func (o *fakeQEMUOperations) ConvertQcow2ToRaw(string, string) error {
	return o.e1
}

func (o *fakeQEMUOperations) ConvertQcow2ToRawStream(*url.URL, string) error {
	return o.e2
}

func (o *fakeQEMUOperations) Validate(string, string) error {
	return o.e5
}

func (o *fakeQEMUOperations) Resize(string, resource.Quantity) error {
	return o.e3
}

func (o *fakeQEMUOperations) Info(string) (*image.ImgInfo, error) {
	return o.ret4.imgInfo, o.ret4.e
}

func (o *fakeQEMUOperations) CreateBlankImage(dest string, size resource.Quantity) error {
	return o.e6
}

// Read doesn't return any values
func (r *EndlessReader) Read(p []byte) (n int, err error) {
	r.Reader.Read(p)
	return 0, nil
}

// Close closes the stream
func (r *EndlessReader) Close() error {
	return r.Reader.Close()
}
