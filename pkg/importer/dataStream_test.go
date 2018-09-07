package importer

import (
	"bufio"
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

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const TestImagesDir = "../../tests/images"

var imageDir, _ = filepath.Abs(TestImagesDir)
var cirrosData, _ = readFile(filepath.Join(imageDir, "cirros-qcow2.img"))
var tinyCoreData, _ = readFile(filepath.Join(imageDir, "tinyCore.iso"))
var testfiles = createTestData()
var stringRdr = strings.NewReader("test data for reader 1")
var qcow2Rdr, _ = os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
var testFileRdrs = bufio.NewReader(qcow2Rdr)

type fakeQEMUOperations struct {
	e1 error
	e2 error
	e3 error
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

	table.DescribeTable("with import source should", func(image, accessKeyID, secretKey string, qemu bool, want []byte, wantErr bool) {
		if image != "" {
			image = ts.URL + "/" + image
		}
		By(fmt.Sprintf("Creating new datastream for %s", image))
		ds, err := NewDataStream(image, accessKeyID, secretKey)
		if ds != nil && len(ds.Readers) > 0 {
			defer ds.Close()
		}
		if !wantErr {
			Expect(err).NotTo(HaveOccurred())
			resultBuffer := make([]byte, len(want))
			By("Reading data from buffer")
			ds.Read(resultBuffer)
			Expect(ds.accessKeyID).To(Equal(accessKeyID))
			Expect(ds.secretKey).To(Equal(secretKey))
			Expect(ds.qemu).To(Equal(qemu))
			Expect(reflect.DeepEqual(resultBuffer, want)).To(BeTrue())
		} else {
			Expect(err).To(HaveOccurred())
		}
	},
		table.Entry("expect NewDataStream to succeed with valid image", "cirros-qcow2.img", "", "", true, cirrosData, false),
		table.Entry("expect NewDataStream to fail with non existing image", "badimage.iso", "", "", false, nil, true),
		table.Entry("expect NewDataStream to fail with invalid or missing image", "", "", "", false, nil, true),
		table.Entry("expect NewDataStream to succeed with valid iso image", "tinyCore.iso", "accessKey", "secretKey", false, tinyCoreData, false),
	)

	It("can close all readers", func() {
		By(fmt.Sprintf("Creating new datastream for %s", ts.URL+"/tinyCore.iso"))
		ds, err := NewDataStream(ts.URL+"/tinyCore.iso", "", "")
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
		ds, err := NewDataStream(ep+"/"+image, "", "")
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
		table.Entry("success building selector for valid endpoint and file", "tinyCore.iso", "", false),
		table.Entry("fail trying to build selector for invalid http endpoint", "", "http://www.google.com", true),
		table.Entry("fail trying to build invalid selector", "tinyCore.iso", "fake://somefakefile", true),
	)

	table.DescribeTable("can construct readers", func(outfile, filename string, numRdrs int, wantErr bool) {
		By(fmt.Sprintf("Creating new fileserver for %s and file %s", filepath.Dir(filename), filepath.Base(filename)))
		tempTestServer := createTestServer(filepath.Dir(filename))
		By(fmt.Sprintf("Creating new datastream to %s", tempTestServer.URL+"/"+filepath.Base(filename)))
		ds, err := NewDataStream(tempTestServer.URL+"/"+filepath.Base(filename), "", "")
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
		table.Entry("successfully construct a xz reader", "tinyCore.iso.xz", testfiles[".xz"], 5, false),              // [http, multi-r, xz, multi-r]
		table.Entry("successfully construct a gz reader", "tinyCore.iso.gz", testfiles[".gz"], 5, false),              // [http, multi-r, gz, multi-r]
		table.Entry("successfully construct qcow2 reader", "", filepath.Join(imageDir, "cirros-qcow2.img"), 2, false), // [http, multi-r]
		table.Entry("successfully construct .iso reader", "", filepath.Join(imageDir, "tinyCore.iso"), 3, false),      // [http, multi-r]
		table.Entry("fail constructing reader for invalid file path", "", filepath.Join(imageDir, "tinyCorebad.iso"), 0, true),
	)
})

var _ = Describe("SaveStream", func() {

	It("Should successfully save the stream", func() {
		defer os.Remove("testqcow2file")
		replaceQEMUOperations(NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil), func() {
			rdr, err := os.Open(filepath.Join(imageDir, "cirros-qcow2.img"))
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
			err := CopyImage(dest, endpt, "", "")
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	},
		table.Entry("successfully copy local image", "tinyCore.raw", "tinyCore.iso", NewQEMUAllErrors(), false),
		table.Entry("expect failure trying to copy non-existing local image", "cdi-testcopy", "tinyCoreBad.iso", NewQEMUAllErrors(), true),
		table.Entry("successfully copy streaming image", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(errors.New("should not be called"), nil, nil), false),
		table.Entry("streaming image qemu validation fails", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(nil, nil, errors.New("invalid image")), true),
		table.Entry("streaming image qemu convert fails", "cirros-qcow2.raw", "cirros-qcow2.img", NewFakeQEMUOperations(nil, errors.New("exit 1"), nil), true),
	)

	table.DescribeTable("internal copy", func(r io.Reader, out string, qemu bool, qemuOperations image.QEMUOperations, wantErr bool) {
		defer os.Remove(out)
		By("Replacing QEMU Operations")
		replaceQEMUOperations(qemuOperations, func() {
			err := copy(r, out, qemu)
			if !wantErr {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		})
	},
		table.Entry("successfully copy reader", stringRdr, "testoutfile", false, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil), false),
		table.Entry("successfully copy qcow2 reader", testFileRdrs, "testqcow2file", true, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil), false),
		table.Entry("expect error trying to copy invalid format", testFileRdrs, "testinvalidfile", true, NewFakeQEMUOperations(nil, nil, errors.New("invalid format")), true),
		table.Entry("expect error trying to copy qemu process fails", testFileRdrs, "testinvalidfile", true, NewFakeQEMUOperations(errors.New("exit 1"), nil, nil), true),
	)
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
	baseTestImage := filepath.Join(imageDir, "tinyCore.iso")
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
		testTarget := filepath.Join(tmpTestDir, common.ImporterWriteFile)
		By(fmt.Sprintf("Importing %q to %q", tempTestServer.URL, testTarget))
		err = CopyImage(testTarget, tempTestServer.URL+"/"+testBase, "", "")
		Expect(err).NotTo(HaveOccurred())

		By(fmt.Sprintf("Checking size of the output file %q", testTarget))
		var targetSize int64
		if useVirtSize {
			By("... using output image's virtual size")
			targetSize = getImageVirtualSize(testTarget)
			Expect(targetSize).To(Equal(sourceSize))
		} else {
			By("... using stat()")
			finfo, err = os.Stat(testTarget)
			Expect(err).NotTo(HaveOccurred())
			targetSize = finfo.Size()
			Expect(targetSize).To(Equal(sourceSize))
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
		ds, err := NewDataStream(tempTestServer.URL+"/"+testBase, "", "")

		Expect(err).NotTo(HaveOccurred())
		defer ds.Close()
		Expect(ds.Size).To(Equal(sourceSize))

		fmt.Fprintf(GinkgoWriter, "End test on test file %q\n", testSample)
	},
		table.Entry("should decompress gzip", baseTestImage, false, image.ExtGz),
		table.Entry("should decompress xz", baseTestImage, false, image.ExtXz),
		table.Entry("should unarchive tar", baseTestImage, false, image.ExtTar),
		table.Entry("should unpack .tar.gz", baseTestImage, false, image.ExtTar, image.ExtGz),
		// Disabled until issue 335 is resolved
		// https://github.com/kubevirt/containerized-data-importer/issues/335
		//table.Entry("should unpack .tar.xz", baseTestImage, false, image.ExtTar, image.ExtXz),
		table.Entry("should convert .qcow2", baseTestImage, true, image.ExtQcow2),
		table.Entry("should convert and unpack .qcow2.gz", baseTestImage, false, image.ExtQcow2, image.ExtGz),
		table.Entry("should convert and unpack .qcow2.xz", baseTestImage, false, image.ExtQcow2, image.ExtXz),
		table.Entry("should convert and untar .qcow2.tar", baseTestImage, false, image.ExtQcow2, image.ExtTar),
		table.Entry("should convert and untar and unpack .qcow2.tar.gz", baseTestImage, false, image.ExtQcow2, image.ExtTar, image.ExtGz),
		table.Entry("should convert and untar and unpack .qcow2.tar.xz", baseTestImage, false, image.ExtQcow2, image.ExtTar, image.ExtXz),
		table.Entry("should pass through unformatted data", baseTestImage, false, ""),
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
	return NewFakeQEMUOperations(err, err, err)
}

func NewFakeQEMUOperations(e1, e2, e3 error) image.QEMUOperations {
	return &fakeQEMUOperations{e1, e2, e3}
}

func (o *fakeQEMUOperations) ConvertQcow2ToRaw(string, string) error {
	return o.e1
}

func (o *fakeQEMUOperations) ConvertQcow2ToRawStream(*url.URL, string) error {
	return o.e2
}

func (o *fakeQEMUOperations) Validate(string, string) error {
	return o.e3
}

func createTestData() map[string]string {
	xzfile, _ := utils.FormatTestData(filepath.Join(imageDir, "tinyCore.iso"), os.TempDir(), image.ExtXz)
	gzfile, _ := utils.FormatTestData(filepath.Join(imageDir, "tinyCore.iso"), os.TempDir(), image.ExtGz)
	xztarfile, _ := utils.FormatTestData(filepath.Join(imageDir, "tinyCore.iso"), os.TempDir(), []string{image.ExtTar, image.ExtGz}...)

	return map[string]string{
		".xz":     xzfile,
		".gz":     gzfile,
		".tar.xz": xztarfile,
	}
}
