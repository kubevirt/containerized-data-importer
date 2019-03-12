package importer

import (
	"bufio"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
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
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/cert/triple"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
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
var cirrosRaw = "cirros.raw"
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
		if contentType == string(cdiv1.DataVolumeArchive) {
			dest = common.ImporterVolumePath
		}
		By(fmt.Sprintf("Creating new datastream for %s", image))
		ds, err := NewDataStream(&DataStreamOptions{
			Dest:           dest,
			DataDir:        "",
			Endpoint:       image,
			AccessKey:      accessKeyID,
			SecKey:         secretKey,
			Source:         controller.SourceHTTP,
			ContentType:    contentType,
			ImageSize:      "",
			AvailableSpace: int64(1234567890),
			CertDir:        "",
			InsecureTLS:    false,
			ScratchDataDir: "",
		})
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

			Expect(reflect.TypeOf(ds.dataFormat).String()).To(Equal("QCOWDataFormat"))
			Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("expect NewDataStream to succeed with valid image", cirrosFileName, "", "", string(cdiv1.DataVolumeKubeVirt), true, cirrosData, false),
		table.Entry("expect NewDataStream to fail with non existing image", "badimage.iso", "", "", string(cdiv1.DataVolumeKubeVirt), false, nil, true),
		table.Entry("expect NewDataStream to fail with invalid or missing image", "", "", "", string(cdiv1.DataVolumeKubeVirt), false, nil, true),
		table.Entry("expect NewDataStream to succeed with valid iso image", tinyCoreFileName, "accessKey", "secretKey", string(cdiv1.DataVolumeKubeVirt), false, tinyCoreData, false),
		table.Entry("expect NewDataStream to fail with a valid image and an incorrect content", cirrosFileName, "", "", string(cdiv1.DataVolumeArchive), true, cirrosData, true),
	)

	It("can close all readers", func() {
		By(fmt.Sprintf("Creating new datastream for %s", ts.URL+"/"+tinyCoreFileName))
		ds, err := NewDataStream(&DataStreamOptions{
			Dest:           common.ImporterWritePath,
			DataDir:        "",
			Endpoint:       ts.URL + "/" + tinyCoreFileName,
			AccessKey:      "",
			SecKey:         "",
			Source:         controller.SourceHTTP,
			ContentType:    string(cdiv1.DataVolumeKubeVirt),
			ImageSize:      "1G",
			AvailableSpace: int64(1234567890),
			CertDir:        "",
			InsecureTLS:    false,
			ScratchDataDir: "",
		})
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
			Dest:           common.ImporterWritePath,
			DataDir:        "",
			Endpoint:       ep + "/" + image,
			AccessKey:      "",
			SecKey:         "",
			Source:         controller.SourceHTTP,
			ContentType:    string(cdiv1.DataVolumeKubeVirt),
			ImageSize:      "20M",
			AvailableSpace: int64(1234567890),
			CertDir:        "",
			InsecureTLS:    false,
			ScratchDataDir: "",
		})
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
		if contentType == string(cdiv1.DataVolumeArchive) {
			dest = common.ImporterVolumePath
		}
		By(fmt.Sprintf("Creating new datastream to %s", tempTestServer.URL+"/"+filepath.Base(filename)))
		ds, err := NewDataStream(&DataStreamOptions{
			Dest:           dest,
			DataDir:        "",
			Endpoint:       tempTestServer.URL + "/" + filepath.Base(filename),
			AccessKey:      "",
			SecKey:         "",
			Source:         controller.SourceHTTP,
			ContentType:    contentType,
			ImageSize:      "20M",
			AvailableSpace: int64(1234567890),
			CertDir:        "",
			InsecureTLS:    false,
			ScratchDataDir: "",
		})
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
		table.Entry("successfully construct a xz reader", tinyCoreXzFilePath, string(cdiv1.DataVolumeKubeVirt), 5, false),     // [http, multi-r, xz, multi-r]
		table.Entry("successfully construct a gz reader", tinyCoreGzFilePath, string(cdiv1.DataVolumeKubeVirt), 5, false),     // [http, multi-r, gz, multi-r]
		table.Entry("successfully construct a tar reader", tinyCoreTarFilePath, string(cdiv1.DataVolumeKubeVirt), 4, false),   // [http, multi-r, tar, multi-r]
		table.Entry("successfully constructed an archive reader", archiveFilePath, string(cdiv1.DataVolumeArchive), 4, false), // [http, multi-r, mul-tar, multi-r]
		table.Entry("successfully construct qcow2 reader", cirrosFilePath, string(cdiv1.DataVolumeKubeVirt), 2, false),        // [http, multi-r]
		table.Entry("successfully construct .iso reader", tinyCoreFilePath, string(cdiv1.DataVolumeKubeVirt), 3, false),       // [http, multi-r]
		table.Entry("fail constructing reader for invalid file path", filepath.Join(imageDir, "tinyCorebad.iso"), string(cdiv1.DataVolumeKubeVirt), 0, true),
		table.Entry("fail constructing reader for a valid archive file and a wrong content", archiveFileName, string(cdiv1.DataVolumeKubeVirt), 0, true),
	)
})

var _ = Describe("SaveStream", func() {
	var dataDir, tmpDir string
	var err error

	BeforeEach(func() {
		dataDir, err = ioutil.TempDir("", "data-test")
		Expect(err).NotTo(HaveOccurred())
		tmpDir, err = ioutil.TempDir("", "scratch-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(dataDir)
		os.RemoveAll(tmpDir)
	})

	It("Should successfully save the stream", func() {
		defer os.Remove("testqcow2file")
		replaceQEMUOperations(NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil, fakeInfoRet, nil, nil), func() {
			rdr, err := os.Open(cirrosFilePath)
			Expect(err).NotTo(HaveOccurred())
			defer rdr.Close()
			_, err = SaveStream(rdr, "testqcow2file", filepath.Join(dataDir, "disk.img"), dataDir, tmpDir)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("Copy", func() {
	var tmpDir, dataDir string
	var err error
	var ts *httptest.Server

	BeforeEach(func() {
		By("[BeforeEach] Creating test server")
		ts = createTestServer(imageDir)
		dataDir, err = ioutil.TempDir("", "data-test")
		Expect(err).NotTo(HaveOccurred())
		tmpDir, err = ioutil.TempDir("", "copy-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		By("[AfterEach] closing test server")
		ts.Close()
		os.RemoveAll(dataDir)
		os.RemoveAll(tmpDir)
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
				Dest:           dest,
				DataDir:        dataDir,
				Endpoint:       endpt,
				AccessKey:      "",
				SecKey:         "",
				Source:         controller.SourceHTTP,
				ContentType:    string(cdiv1.DataVolumeKubeVirt),
				ImageSize:      "",
				AvailableSpace: int64(1234567890),
				CertDir:        "",
				InsecureTLS:    false,
				ScratchDataDir: tmpDir,
			})
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
		table.Entry("streaming image qemu convert succeeds since there is no space validation on streaming", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, nil}, nil, nil), false),
	)

	table.DescribeTable("Archived image, no scratch space, with import source should", func(originalFile string, qemuOperations image.QEMUOperations, wantErr bool, expectFormats ...string) {
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
				Dest:           testTarget,
				DataDir:        "",
				Endpoint:       tempTestServer.URL + "/" + testBase,
				AccessKey:      "",
				SecKey:         "",
				Source:         controller.SourceHTTP,
				ContentType:    string(cdiv1.DataVolumeKubeVirt),
				ImageSize:      "1G",
				AvailableSpace: int64(1234567890),
				CertDir:        "",
				InsecureTLS:    false,
				ScratchDataDir: "",
			})
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

		table.Entry("should succeed to convert  qcow2 to raw .qcow2.gz", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Scratch space required, and none found ")}, nil, nil), true, image.ExtQcow2, image.ExtGz),
		table.Entry("should succeed to convert  qcow2 to raw .qcow2.xz", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Scratch space required, and none found ")}, nil, nil), true, image.ExtQcow2, image.ExtXz),
		table.Entry("should succeed to convert  qcow2 to raw .qcow2.tar", "tinyCore.iso", NewFakeQEMUOperations(nil, nil, nil, fakeInfoOpRetVal{&fakeZeroImageInfo, errors.New("Scratch space required, and none found ")}, nil, nil), true, image.ExtQcow2, image.ExtTar),
	)
})

var _ = Describe("http", func() {
	It("Should properly finish with valid reader", func() {
		By("Creating context for the transfer, we have the ability to cancel it")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		httpStreamer := HttpDataStreamer{
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
		go httpStreamer.pollProgress(countingReader, 5*time.Second, time.Second)
		By("Waiting for timeout or success")
		select {
		case <-time.After(10 * time.Second):
			Fail("Transfer not cancelled after 10 seconds")
		case <-httpStreamer.ctx.Done():
			By("Having context be done, we confirm finishing of transfer")
		}
	})
})

var _ = Describe("ResizeImage", func() {
	var tmpDir string
	var err error

	BeforeEach(func() {
		tmpDir, err = ioutil.TempDir("", "imagedir")
		Expect(err).NotTo(HaveOccurred())
		input, err := ioutil.ReadFile(filepath.Join(imageDir, cirrosRaw))
		Expect(err).NotTo(HaveOccurred())
		err = ioutil.WriteFile(filepath.Join(tmpDir, cirrosRaw), input, 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.Remove(tmpDir)
	})

	It("Should successfully resize, to smaller available", func() {
		dest := filepath.Join(tmpDir, cirrosRaw)
		err := ResizeImage(dest, "20M", int64(18874368))
		Expect(err).NotTo(HaveOccurred())
		info, err := qemuOperations.Info(dest)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.VirtualSize).To(Equal(int64(18874368)))
	})

	It("Should fail with invalid file", func() {
		err := ResizeImage(filepath.Join(tmpDir, "invalid"), "20M", int64(18874368))
		Expect(err).To(HaveOccurred())
	})

	It("Should successfully resize even if more space available", func() {
		dest := filepath.Join(tmpDir, cirrosRaw)
		err := ResizeImage(dest, "20M", int64(200000000))
		Expect(err).NotTo(HaveOccurred())
		info, err := qemuOperations.Info(dest)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.VirtualSize).To(Equal(int64(20971520)))
	})

	It("Should successfully not resize if sizes are same.", func() {
		dest := filepath.Join(tmpDir, cirrosRaw)
		info, err := qemuOperations.Info(dest)
		originalSize := info.VirtualSize
		Expect(err).NotTo(HaveOccurred())
		err = ResizeImage(dest, strconv.FormatInt(originalSize, 10), int64(200000000))
		Expect(err).NotTo(HaveOccurred())
		info, err = qemuOperations.Info(dest)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.VirtualSize).To(Equal(originalSize))
	})

	It("Should fail with valid file, but empty imageSize", func() {
		dest := filepath.Join(tmpDir, cirrosRaw)
		err := ResizeImage(dest, "", int64(200000000))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("Image resize called with blank resize"))
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

var _ = Describe("test certs get loaded", func() {
	var tempDir string

	BeforeEach(func() {
		var err error

		tempDir, err = ioutil.TempDir("/tmp", "cert-test")
		Expect(err).ToNot(HaveOccurred())

		keyPair, err := triple.NewCA("datastream.cdi.kubevirt.io")
		Expect(err).ToNot(HaveOccurred())

		certBytes := cert.EncodeCertPEM(keyPair.Cert)

		err = ioutil.WriteFile(path.Join(tempDir, "tls.crt"), certBytes, 0644)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	It("should load the cert", func() {
		dso := &DataStreamOptions{CertDir: tempDir}
		ds := &DataStream{DataStreamOptions: dso}

		client, err := HTTPDataStreamer(ds.url, ds.SecKey, ds.AccessKey, ds.CertDir, ds.InsecureTLS).createHTTPClient()
		Expect(err).ToNot(HaveOccurred())

		transport := client.Transport.(*http.Transport)
		Expect(transport).ToNot(BeNil())

		activeCAs := transport.TLSClientConfig.RootCAs
		Expect(transport).ToNot(BeNil())

		systemCAs, err := x509.SystemCertPool()
		Expect(err).ToNot(HaveOccurred())

		Expect(len(activeCAs.Subjects())).Should(Equal(len(systemCAs.Subjects()) + 1))
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

func (o *fakeQEMUOperations) Validate(string, string, int64) error {
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
