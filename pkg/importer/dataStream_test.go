package importer

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/glog"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"kubevirt.io/containerized-data-importer/test/framework"
)

const testImagesDir = "../../test/images"

func createDataStream(ep, accKey, secKey string) *dataStream {
	dsurl, _ := ParseEndpoint(ep)

	ds := &dataStream{
		Url:         dsurl,
		buf:         make([]byte, image.MaxExpectedHdrSize),
		accessKeyId: accKey,
		secretKey:   secKey,
	}

	ds.constructReaders()

	return ds
}

func createDataStreamBytes(testfile, ep string, defaultBuf, singlereader bool) (*dataStream, []byte, error) {
	urlPath := filepath.Join(ep, testfile)
	if len(ep) == 0 {
		urlPath = getUrlPath(testfile)
	}
	testFilePath := getTestFilePath(testfile)
	ds := createDataStream(urlPath, "", "")

	// if no file supplied just return empty byte stream
	if len(testfile) == 0 && !defaultBuf {
		return ds, nil, nil
	}
	if len(testfile) == 0 && defaultBuf {
		return ds, []byte{'T', 'E', 'S', 'T'}, nil
	}

	f, _ := os.Open(testFilePath)
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
	imageDir, _ := filepath.Abs(testImagesDir)
	return filepath.Join(imageDir, testfile)
}

func getUrlPath(testfile string) string {
	// CWD is set within go test to the directory of the test
	// being executed, so using relative path
	imageDir, _ := filepath.Abs(testImagesDir)
	return "file://" + imageDir + "/" + testfile
}

func TestNewDataStream(t *testing.T) {
	type args struct {
		endpt  string
		accKey string
		secKey string
	}

	imageDir, _ := filepath.Abs(testImagesDir)
	localImageBase := filepath.Join("file://", imageDir)

	tests := []struct {
		name    string
		args    args
		want    *dataStream
		wantErr bool
	}{
		{
			name:    "expect new DataStream to succeed with matching buffers",
			args:    args{filepath.Join(localImageBase, "cirros-qcow2.img"), "", ""},
			want:    createDataStream(filepath.Join(localImageBase, "cirros-qcow2.img"), "", ""),
			wantErr: false,
		},
		{
			name:    "expect new DataStream to fail with unsupported file type",
			args:    args{filepath.Join(localImageBase, "badimage.iso"), "", ""},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "expect new DataStream to fail with invalid or missing endpoint",
			args:    args{"", "", ""},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDataStream(tt.args.endpt, tt.args.accKey, tt.args.secKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDataStream() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// do not deep reflect on expected error when got is nil
			if got == nil && tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got.buf, tt.want.buf) {
				t.Errorf("NewDataStream() = %v, want %v", got, tt.want)
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
			name:     "successful read of datastream buffer",
			testFile: "tinyCore.iso",
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, testBytes, errDs := createDataStreamBytes(tt.testFile, "", true, true)
			if errDs != nil {
				t.Errorf("error setting up test infrastructure %v", errDs)
			}
			defer ds.Close()

			got, err := ds.Read(testBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			expectedSize, errFs := getFileSize(tt.testFile)
			if errFs != nil {
				t.Errorf("error getting test file size %v", errFs)
				return
			}
			if got != expectedSize {
				t.Errorf("dataStream.Read() sizes do not match = %v, want %v", got, expectedSize)
			}
		})
	}
}

func Test_dataStream_Close(t *testing.T) {
	ds, _, errDs := createDataStreamBytes("tinyCore.iso", "", false, false)
	defer ds.Close()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successfully close all readers",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if errDs != nil {
				t.Errorf("error setting up test infrastructure %v", errDs)
				return
			}
			if err := ds.Close(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_dataStreamSelector(t *testing.T) {
	imageDir, _ := filepath.Abs(testImagesDir)
	localImageBase := filepath.Join("file://", imageDir)
	glog.V(2).Infof("Scott - localImageBase %s", localImageBase)

	tests := []struct {
		name     string
		testFile string
		ep       string
		wantErr  bool
	}{
		{
			name:     "success building selector for file",
			testFile: "tinyCore.iso",
			ep:       localImageBase,
			wantErr:  false,
		},
		{
			name:     "fail trying to build selector for invalid http endpoint",
			testFile: "",
			ep:       "http://www.google.com",
			wantErr:  true,
		},
		{
			name:     "fail trying to build invalid selector",
			testFile: "tinyCore.iso",
			ep:       "fake://somefakefile",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, _, errDs := createDataStreamBytes(tt.testFile, tt.ep, false, false)
			defer ds.Close()
			if errDs != nil {
				t.Errorf("error setting up test infrastructure %v", errDs)
				return
			}
			if err := ds.dataStreamSelector(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.dataStreamSelector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCopyImage(t *testing.T) {
	type args struct {
		dest      string
		endpoint  string
		accessKey string
		secKey    string
	}
	imageDir, _ := filepath.Abs(testImagesDir)
	localImageBase := filepath.Join("file://", imageDir)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "successfully copy local image",
			args:    args{filepath.Join(os.TempDir(), "cdi-testcopy"), filepath.Join(localImageBase, "tinyCore.iso"), "", ""},
			wantErr: false,
		},
		{
			name:    "expect failure trying to copy non-existing local image",
			args:    args{filepath.Join(os.TempDir(), "cdi-testcopy"), filepath.Join(localImageBase, "tinyCoreBad.iso"), "", ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		defer os.RemoveAll(tt.args.dest)
		t.Run(tt.name, func(t *testing.T) {
			if err := CopyImage(tt.args.dest, tt.args.endpoint, tt.args.accessKey, tt.args.secKey); (err != nil) != tt.wantErr {
				t.Errorf("CopyImage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func createTestData() map[string]string {
	imageDir, _ := filepath.Abs(testImagesDir)

	// xz data
	xzfile, _ := framework.FormatTestData(filepath.Join(imageDir, "tinyCore.iso"), os.TempDir(), image.ExtXz)
	gzfile, _ := framework.FormatTestData(filepath.Join(imageDir, "tinyCore.iso"), os.TempDir(), image.ExtGz)
	xztarfile, _ := framework.FormatTestData(filepath.Join(imageDir, "tinyCore.iso"), os.TempDir(), []string{image.ExtTar, image.ExtGz}...)

	return map[string]string{
		".xz":     xzfile,
		".gz":     gzfile,
		".tar.xz": xztarfile,
	}
}

func Test_dataStream_constructReaders(t *testing.T) {
	imageDir, _ := filepath.Abs(testImagesDir)
	localImageBase := filepath.Join("file://", imageDir)

	testfiles := createTestData()

	tests := []struct {
		name    string
		outfile string
		ds      *dataStream
		wantErr bool
	}{
		{
			name:    "successfully construct a xz reader",
			outfile: "tinyCore.iso.xz",
			ds:      createDataStream(filepath.Join("file://", testfiles[".xz"]), "", ""),
			wantErr: false,
		},
		{
			name:    "successfully construct a gz reader",
			outfile: "tinyCore.iso.gz",
			ds:      createDataStream(filepath.Join("file://", testfiles[".gz"]), "", ""),
			wantErr: false,
		},
		{
			name:    "successfully construct qcow2 reader",
			outfile: "",
			ds:      createDataStream(filepath.Join(localImageBase, "cirros-qcow2.img"), "", ""),
			wantErr: false,
		},
		{
			name:    "successfully construct .iso reader",
			outfile: "",
			ds:      createDataStream(filepath.Join(localImageBase, "tinyCore.iso"), "", ""),
			wantErr: false,
		},
		{
			name:    "fail constructing reader for invalid file path",
			outfile: "",
			ds:      createDataStream(filepath.Join(localImageBase, "tinyCorebad.iso"), "", ""),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ds.constructReaders(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.constructReaders() error = %v, wantErr %v", err, tt.wantErr)
			}
			defer tt.ds.Close()
			if len(tt.outfile) > 0 {
				os.Remove(filepath.Join(os.TempDir(), tt.outfile))
			}
		})
	}
}

func Test_closeReaders(t *testing.T) {
	type args struct {
		readers []Reader
	}

	rdrs1 := ioutil.NopCloser(strings.NewReader("test data for reader 1"))
	rdrs2 := ioutil.NopCloser(strings.NewReader("test data for reader 2"))
	rdrA := Reader{4, rdrs1}
	rdrB := Reader{7, rdrs2}

	rdrsTest := []Reader{rdrA, rdrB}

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
		r    io.Reader
		out  string
		qemu bool
	}
	rdrs1 := strings.NewReader("test data for reader 1")

	imageDir, _ := filepath.Abs(testImagesDir)
	file := filepath.Join(imageDir, "cirros-qcow2.img")
	rdrfile, _ := os.Open(file)
	rdrs2 := bufio.NewReader(rdrfile)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "successfully copy reader",
			args:    args{rdrs1, "testoutfile", false},
			wantErr: false,
		},
		{
			name:    "successfully copy qcow2 reader",
			args:    args{rdrs2, "testqcow2file", true},
			wantErr: false,
		},
		{
			name:    "expect error trying to copy invalid format",
			args:    args{rdrs2, "testinvalidfile", true},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tt.args.out)
			if err := copy(tt.args.r, tt.args.out, tt.args.qemu); (err != nil) != tt.wantErr {
				t.Errorf("copy() error = %v, wantErr %v", err, tt.wantErr)
			}
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
