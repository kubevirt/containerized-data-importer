package importer

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/glog"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

const (
	TestImagesDir = "../../tests/images"
	defaultPort   = 9999
)

var imageDir, _ = filepath.Abs(TestImagesDir)
var httpImageBase = fmt.Sprintf("http://localhost:%d/", defaultPort)

type fakeQEMUOperations struct {
	e1 error
	e2 error
	e3 error
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

func NewFakeQEMUOperations(e1, e2, e3 error) image.QEMUOperations {
	return &fakeQEMUOperations{e1, e2, e3}
}

func NewQEMUAllErrors() image.QEMUOperations {
	err := errors.New("qemu should not be called from this test override with replaceQEMUOperations")
	return NewFakeQEMUOperations(err, err, err)
}

func replaceQEMUOperations(replacement image.QEMUOperations, f func()) {
	orig := qemuOperations
	if replacement != nil {
		qemuOperations = replacement
		defer func() { qemuOperations = orig }()
	}
	f()
}

// Parses the endpoint but does not call the constructReaders() method, and thus Close() is
// not necessary.
func FakeNewDataStream(ep, accKey, secKey string) (*DataStream, error) {
	dsurl, err := ParseEndpoint(ep)
	if err != nil {
		return nil, fmt.Errorf("FakeNewDataStream parse error: %v", err)
	}
	ds := &DataStream{
		url:         dsurl,
		buf:         make([]byte, image.MaxExpectedHdrSize),
		accessKeyID: accKey,
		secretKey:   secKey,
	}
	return ds, nil
}

// Creates a fake dataStream and returns the byte content of testfile, if it's passed.
// Note: if ep is passed then ":port" is expected.
func createDataStreamBytes(testfile, ep string, defaultBuf bool) (*DataStream, []byte, error) {
	var urlPath string
	if len(ep) == 0 {
		urlPath = getURLPath(testfile)
	} else {
		urlPath = ep + "/" + testfile
	}
	testFilePath := getTestFilePath(testfile)

	ds, err := FakeNewDataStream(urlPath, "", "")
	if err != nil {
		return nil, nil, err
	}

	// if no file supplied just return empty byte stream
	if len(testfile) == 0 {
		if defaultBuf {
			return ds, []byte{'T', 'E', 'S', 'T'}, nil
		}
		return ds, nil, nil
	}

	f, err := os.Open(testFilePath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	testBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to read datastream buffer")
	}

	return ds, testBytes, nil
}

func getFileSize(testfile string) (int, error) {
	f, err := os.Open(getTestFilePath(testfile))
	defer f.Close()
	if err != nil {
		return 0, fmt.Errorf("Unable to open source datastream file %s", getTestFilePath(testfile))
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("Unable to stat file %v", f.Name())
	}

	return int(fi.Size()), nil
}

func getTestFilePath(testfile string) string {
	// CWD is set within go test to the directory of the test
	// being executed, so using relative path
	return filepath.Join(imageDir, testfile)
}

// Return the web root path to `f`.
func getURLPath(f string) string {
	// CWD is set within go test to the directory of the test
	// being executed, so using relative path
	if len(f) == 0 {
		return httpImageBase
	}
	if f[len(f)-1:] != "/" {
		f += "/"
	}
	return httpImageBase + f
}

func startHTTPServer(port int, dir string) (*http.Server, error) {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.FileServer(http.Dir(dir)),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			// cannot panic, because this probably is an intentional close
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	started := false
	for i := 0; i < 10; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		resp.Body.Close()
		started = true
		break
	}

	if !started {
		server.Shutdown(nil)
		return nil, errors.New("Couldn't start http server")
	}

	return server, nil
}

func TestNewDataStream(t *testing.T) {
	type args struct {
		endpt  string
		accKey string
		secKey string
	}

	tests := []struct {
		name    string
		args    args
		qemu    bool
		rdrCnt  int
		wantErr bool
	}{
		{
			name:    "new DataStream for qcow2",
			args:    args{"cirros-qcow2.img", "", ""},
			qemu:    true,
			rdrCnt:  2,
			wantErr: false,
		},
		{
			name:    "new DataStream for iso",
			args:    args{"tinyCore.iso", "", ""},
			qemu:    false,
			rdrCnt:  3,
			wantErr: false,
		},
		{
			name:    "new DataStream should fail with missing endpoint",
			args:    args{"missingFooImage.iso", "", ""},
			wantErr: true,
		},
		{
			name:    "new DataStream should fail with empty endpoint",
			args:    args{"", "", ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := getURLPath(tt.args.endpt)
			t.Logf("NewDataStream testing endpoint: %q...", ep)
			got, err := NewDataStream(ep, tt.args.accKey, tt.args.secKey)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("NewDataStream error: %v", err)
				}
				return
			}
			if got == nil {
				t.Error("NewDataStream returned nil ptr and no error")
				return
			}

			// check various DataStream fields
			if got.accessKeyID != tt.args.accKey {
				t.Errorf("NewDataStream access key (%q) != expected (%q)", got.accessKeyID, tt.args.accKey)
				return
			}
			if got.secretKey != tt.args.secKey {
				t.Errorf("NewDataStream secret key (%q) != expected (%q)", got.secretKey, tt.args.secKey)
				return
			}
			if got.qemu != tt.qemu {
				t.Errorf("NewDataStream qemu (%v) does not match expected (%v)", got.qemu, tt.qemu)
				return
			}
			if len(got.Readers) != tt.rdrCnt {
				t.Errorf("NewDataStream number of Readers (%d) does not match expected (%d)", len(got.Readers), tt.rdrCnt)
				return
			}
			if got.Size <= 0 {
				t.Error("NewDataStream Size field is zero")
				return
			}
		})
	}
}

func Test_dataStream_Read(t *testing.T) {
	tests := []struct {
		name     string
		testFile string
		wantErr  bool
	}{
		{
			name:     "successful read of dataStream buffer",
			testFile: "tinyCore.iso",
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := getURLPath(tt.testFile)
			t.Logf("by creating a dataStream on endpoint %q...", ep)
			ds, err := NewDataStream(ep, "", "")
			if err != nil {
				t.Errorf("NewDataStreamerror: %v", err)
				return
			}
			if ds == nil {
				t.Error("NewateDataStream returned nil ptr and no error")
				return
			}

			// read test file
			var readbuf []byte
t.Logf("len Readers=%d", len(ds.Readers))
			n, err := ds.Read(readbuf)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("dataStream.Read error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			// size check
			expectedSize, err := getFileSize(tt.testFile)
			if err != nil {
				t.Errorf("error getting test file size: %v", err)
				return
			}
			if n != expectedSize {
				t.Errorf("dataStream.Read return cnt (%d) does not match %q's Stat size (%d)", n, tt.testFile, expectedSize)
				return
			}
		})
	}
}

func Test_dataStream_Close(t *testing.T) {
	t.Run("successfully close all readers", func(t *testing.T) {
		ep := getURLPath("tinyCore.iso")
		ds, err := NewDataStream(ep, "", "")
		if err != nil {
			t.Errorf("NewDataStream error on %q: %v", ep, err)
			return
		}
		if err := ds.Close(); err != nil {
			t.Errorf("dataStream.Close error on %q: %v", ep, err)
		}
	})
}

func Test_dataStream_dataStreamSelector(t *testing.T) {
	tests := []struct {
		name     string
		testFile string
		ep       string
		wantErr  bool
	}{
		{
			name:     "success building selector for http file",
			testFile: "tinyCore.iso",
			ep:       httpImageBase,
			wantErr:  false,
		},
		{
			name:     "fail trying to build selector for invalid http endpoint",
			testFile: "",
			ep:       "http://www.google.com",
			wantErr:  true,
		},
		{
			name:     "fail trying to build invalid selector scheme",
			testFile: "tinyCore.iso",
			ep:       "fake://somefakefile",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := tt.ep + tt.testFile
			ds, err := FakeNewDataStream(ep, "", "")
			if err != nil {
				t.Errorf("FakeNewDataStreamBytes error on %q: %v", ep, err)
				return
			}
			if ds == nil {
				t.Error("FakeNewDataStreamBytes returned nil ptr and no error")
				return
			}

			err = ds.dataStreamSelector()
			if err != nil {
				if !tt.wantErr {
					t.Errorf("dataStream.dataStreamSelector error: %v", err)
				}
				return
			}
			defer ds.Close()
		})
	}
}

func TestCopyImage(t *testing.T) {
	const copyDir = "cdi-copy"
	tmpDir := filepath.Join(os.TempDir(), copyDir)

	type args struct {
		dest           string // full pathname, index counter appended for uniqueness
		endpoint       string // just image filename, not full endpt
		accessKey      string
		secKey         string
		qemuOperations image.QEMUOperations
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successfully copy iso image",
			args: args{
				"disk-img",
				"tinyCore.iso",
				"",
				"",
				NewQEMUAllErrors(),
			},
			wantErr: false,
		},
		{
			name: "fail copy non-existent iso image",
			args: args{
				"disk-img",
				"fooBar.iso",
				"",
				"",
				NewQEMUAllErrors(),
			},
			wantErr: false,
		},
		{
			name: "successfully copy qcow2 image",
			args: args{
				"disk-img",
				"cirros-qcow2.img",
				"",
				"",
				NewFakeQEMUOperations(errors.New("should not be called"), nil, nil),
			},
			wantErr: false,
		},
		{
			name: "streaming image qemu validation fails",
			args: args{
				"disk-img",
				"cirros-qcow2.img",
				"",
				"",
				NewFakeQEMUOperations(nil, nil, errors.New("invalid image")),
			},
			wantErr: true,
		},
		{
			name: "streaming image qemu convert fails",
			args: args{
				"disk-img",
				"cirros-qcow2.img",
				"",
				"",
				NewFakeQEMUOperations(nil, errors.New("exit 1"), nil),
			},
			wantErr: true,
		},
	}
	for i, tt := range tests {
		// create sequenced temp dir for destination image
		dest := filepath.Join(fmt.Sprintf("%s%d", tmpDir, i), tt.args.dest)
		err := os.MkdirAll(dest, os.ModePerm)
		if err != nil {
			t.Errorf("cannot create dir %q: %v", dest, err)
			continue
		}
		defer os.RemoveAll(dest)
		ep := httpImageBase + tt.args.endpoint

		t.Logf("CopyImage from %q to %q...", ep, dest)
		t.Run(tt.name, func(t *testing.T) {
			replaceQEMUOperations(tt.args.qemuOperations, func() {
				if err := CopyImage(dest, ep, tt.args.accessKey, tt.args.secKey); (err != nil) != tt.wantErr {
					t.Errorf("CopyImage() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		})
	}
}

func createTestData() map[string]string {
	xzfile, _ := utils.FormatTestData(getTestFilePath("tinyCore.iso"), os.TempDir(), image.ExtXz)
	gzfile, _ := utils.FormatTestData(getTestFilePath("tinyCore.iso"), os.TempDir(), image.ExtGz)
	xztarfile, _ := utils.FormatTestData(getTestFilePath("tinyCore.iso"), os.TempDir(), []string{image.ExtTar, image.ExtGz}...)

	return map[string]string{
		".xz":     xzfile,
		".gz":     gzfile,
		".tar.xz": xztarfile,
	}
}

func Test_dataStream_constructReaders(t *testing.T) {
	testfiles := createTestData()

	tests := []struct {
		name    string
		outfile string
		src     string
		numRdrs int
		wantErr bool
	}{
		{
			name:    "successfully construct a xz reader",
			outfile: "tinyCore.iso.xz",
			src:     testfiles[".xz"],
			numRdrs: 4, // [http, multi-r, xz, multi-r]
			wantErr: false,
		},
		{
			name:    "successfully construct a gz reader",
			outfile: "tinyCore.iso.gz",
			src:     testfiles[".gz"],
			numRdrs: 4, // [http, multi-r, gz, multi-r]
			wantErr: false,
		},
		{
			name:    "successfully construct qcow2 reader",
			outfile: "",
			src:     "cirros-qcow2.img",
			numRdrs: 2, // [http, multi-r]
			wantErr: false,
		},
		{
			name:    "successfully construct .iso reader",
			outfile: "",
			src:     "tinyCore.iso",
			numRdrs: 2, // [http, multi-r]
			wantErr: false,
		},
		{
			name:    "fail constructing reader for invalid file path",
			outfile: "",
			src:     "tinyMissing.iso",
			numRdrs: 0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := getURLPath(tt.src)
			ds, err := FakeNewDataStream(ep, "", "")
			if err != nil {
				t.Errorf("FakeNewDataStream error on endpoint %q: %v", ep, err)
				return
			}

			err = ds.constructReaders()
			if err != nil {
				t.Errorf("FakeNewDataStream.constructReaders error on endpoint %q: %v", ep, err)
				return
			}
			defer ds.Close()

			actualNumRdrs := len(ds.Readers)
			if tt.numRdrs != actualNumRdrs {
				t.Errorf("FakeNewDataStream.constructReaders: expect num-readers to be %d, got %d", tt.numRdrs, actualNumRdrs)
			}
			if len(tt.outfile) > 0 {
				os.Remove(filepath.Join(os.TempDir(), tt.outfile))
			}
		})
	}
}

func Test_closeReaders(t *testing.T) {
	type args struct {
		readers []reader
	}

	rdrs1 := ioutil.NopCloser(strings.NewReader("test data for reader 1"))
	rdrs2 := ioutil.NopCloser(strings.NewReader("test data for reader 2"))
	rdrA := reader{4, rdrs1}
	rdrB := reader{7, rdrs2}

	rdrsTest := []reader{rdrA, rdrB}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "successfully close reader",
			args:    args{rdrsTest},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := closeReaders(tt.args.readers); (err != nil) != tt.wantErr {
				t.Errorf("closeReaders() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_copy(t *testing.T) {
	type args struct {
		r              io.Reader
		out            string
		qemu           bool
		qemuOperations image.QEMUOperations
	}
	rdrs1 := strings.NewReader("test data for reader 1")

	file := getTestFilePath("cirros-qcow2.img")
	rdrfile, _ := os.Open(file)
	rdrs2 := bufio.NewReader(rdrfile)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "successfully copy reader",
			args:    args{rdrs1, "testoutfile", false, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil)},
			wantErr: false,
		},
		{
			name:    "successfully copy qcow2 reader",
			args:    args{rdrs2, "testqcow2file", true, NewFakeQEMUOperations(nil, errors.New("Shouldn't get this"), nil)},
			wantErr: false,
		},
		{
			name:    "expect error trying to copy invalid format",
			args:    args{rdrs2, "testinvalidfile", true, NewFakeQEMUOperations(nil, nil, errors.New("invalid format"))},
			wantErr: true,
		},
		{
			name:    "expect error trying to copy qemu process fails",
			args:    args{rdrs2, "testinvalidfile", true, NewFakeQEMUOperations(errors.New("exit 1"), nil, nil)},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tt.args.out)
			replaceQEMUOperations(tt.args.qemuOperations, func() {
				if err := copy(tt.args.r, tt.args.out, tt.args.qemu); (err != nil) != tt.wantErr {
					t.Errorf("copy() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		})
	}
}

func Test_randTmpName(t *testing.T) {
	const numbyte = 8

	type args struct {
		src string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "create expected random name",
			args: args{"testfile.img"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			randName := make([]byte, numbyte)
			rand.Read(randName)
			wantString := hex.EncodeToString(randName)

			got := randTmpName(tt.args.src)
			_, fn := filepath.Split(got)

			if len(fn) != len(tt.args.src)+len(wantString) {
				t.Errorf("randTmpName() length does not match = %v, want %v  -  %s   %s", len(fn), len(tt.args.src)+len(wantString), fn, tt.args.src+wantString)
			}
		})
	}
}

// testMainWrap: start the local http server in a separate goroutine and shut it down after all
// tests have run.
func testMainWrap(m *testing.M, port int) (retCode int) {
	server, err := startHTTPServer(port, imageDir)
	if err != nil {
		glog.Errorf("Error starting local HTTP server: %v", err)
		return 1
	}
	defer server.Shutdown(nil)

	replaceQEMUOperations(NewQEMUAllErrors(), func() {
		retCode = m.Run()
	})
	return
}

// Need to confirm: qwmu-img aborts when doing streaming conversion when run in travis.
func TestMain(m *testing.M) {
	// set flag so glog calls are seen in stdout
	flag.Set("alsologtostderr", fmt.Sprintf("%t", true))

	os.Exit(testMainWrap(m, defaultPort))
}
