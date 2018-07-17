package importer

import (
	"archive/tar"
	"bufio"
	"bytes"
	"encoding/hex"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xi2/xz"
	"kubevirt.io/containerized-data-importer/pkg/image"
)

func createSimpleDataStream(ep, accKey, secKey string) *dataStream {
	dsurl, _ := ParseEndpoint(ep)

	ds := &dataStream{
		Url:         dsurl,
		buf:         make([]byte, image.MaxExpectedHdrSize),
		accessKeyId: accKey,
		secretKey:   secKey,
	}

	return ds
}

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

func createDataStreamMulti(testfile, ep string) *dataStream {
	// set our directory for images
	imageDir, _ := filepath.Abs("../../test/images")
	urlPath := filepath.Join(ep, testfile)
	testFilePath := filepath.Join(imageDir, testfile)

	dsurl, _ := ParseEndpoint(urlPath)

	ds := &dataStream{
		Url:         dsurl,
		buf:         make([]byte, image.MaxExpectedHdrSize),
		accessKeyId: "",
		secretKey:   "",
	}

	ds.constructReaders()

	buf := bytes.NewBuffer(nil)
	f, _ := os.Open(testFilePath)
	io.Copy(buf, f)
	f.Close()
	testBytes, _ := ioutil.ReadAll(buf)

	//append readers
	xzreader, _ := xz.NewReader(bytes.NewReader(testBytes), 0)
	tarreader := tar.NewReader(xzreader)

	ds.appendReader(RdrXz, xzreader)
	ds.appendReader(RdrTar, tarreader)

	return ds
}

func createDataStreamBuf(ep, accKey, secKey, testfile string) (*dataStream, []byte, int) {

	// set our directory for images
	imageDir, _ := filepath.Abs("../../test/images")
	localImageBase := filepath.Join("file://", imageDir)
	urlPath := filepath.Join(localImageBase, testfile)
	testFilePath := filepath.Join(imageDir, testfile)
	if len(ep) == 0 {
		ep = urlPath
	}
	dsurl, _ := ParseEndpoint(ep)

	ds := &dataStream{
		Url:         dsurl,
		buf:         make([]byte, image.MaxExpectedHdrSize),
		accessKeyId: accKey,
		secretKey:   secKey,
	}

	ds.constructReaders()
	// if no file supplied just return empty byte stream
	if len(testfile) == 0 {
		return ds, []byte{'T', 'E', 'S', 'T'}, 0
	}

	buf := bytes.NewBuffer(nil)
	f, _ := os.Open(testFilePath)
	fi, _ := f.Stat()
	expectedSize := int(fi.Size())
	io.Copy(buf, f)
	f.Close()
	testBytes, _ := ioutil.ReadAll(buf)

	//append readers
	ds.appendReader(RdrXz, bytes.NewReader(testBytes))

	return ds, testBytes, expectedSize
}

func TestNewDataStream(t *testing.T) {
	type args struct {
		endpt  string
		accKey string
		secKey string
	}

	imageDir, _ := filepath.Abs("../../test/images")
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
			args:    args{filepath.Join(localImageBase, "image.bad"), "", ""},
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
			testFile: "sample.tar.xz",
			wantErr:  false,
		},
		{
			name:     "failure on invalid read of empty datastream buffer",
			testFile: "",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, testBytes, expectedSize := createDataStreamBuf("", "", "", tt.testFile)
			defer ds.Close()

			got, err := ds.Read(testBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Read() error = %v, wantErr %v %v %v", err, tt.wantErr, testBytes, expectedSize)
				return
			}
			if got != expectedSize {
				t.Errorf("dataStream.Read() = %v, want %v", got, expectedSize)
			}
		})
	}
}

func Test_dataStream_Close(t *testing.T) {
	imageDir, _ := filepath.Abs("../../test/images")
	localImageBase := filepath.Join("file://", imageDir)
	ds := createDataStreamMulti("sample.tar.xz", localImageBase)
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
			if err := ds.Close(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_dataStreamSelector(t *testing.T) {
	imageDir, _ := filepath.Abs("../../test/images")
	localImageBase := filepath.Join("file://", imageDir)

	tests := []struct {
		name     string
		testFile string
		ep       string
		wantErr  bool
	}{
		{
			name:     "success building selector for file",
			testFile: "sample.tar.xz",
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
			testFile: "sample.tar.xz",
			ep:       "fake://somefakefile",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := createDataStreamMulti(tt.testFile, tt.ep)
			defer ds.Close()

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
	imageDir, _ := filepath.Abs("../../test/images")
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

func Test_dataStream_constructReaders(t *testing.T) {
	imageDir, _ := filepath.Abs("../../test/images")
	localImageBase := filepath.Join("file://", imageDir)

	tests := []struct {
		name    string
		ds      *dataStream
		wantErr bool
	}{
		{
			name:    "successfully construct tar.gz reader",
			ds:      createSimpleDataStream(filepath.Join(localImageBase, "sample.tar.gz"), "", ""),
			wantErr: false,
		},
		{
			name:    "successfully construct tar.xz reader",
			ds:      createSimpleDataStream(filepath.Join(localImageBase, "sample.tar.xz"), "", ""),
			wantErr: false,
		},
		{
			name:    "successfully construct .iso reader",
			ds:      createSimpleDataStream(filepath.Join(localImageBase, "tinyCore.iso"), "", ""),
			wantErr: false,
		},
		{
			name:    "fail constructing reader for invalid file path",
			ds:      createSimpleDataStream(filepath.Join(localImageBase, "tinyCorebad.iso"), "", ""),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.ds.Close()
			if err := tt.ds.constructReaders(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.constructReaders() error = %v, wantErr %v", err, tt.wantErr)
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

	imageDir, _ := filepath.Abs("../../test/images")
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
		want string
	}{
		{
			name: "create expected random name",
			args: args{"testfile.img"},
			want: "/tmp/testfile.img",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			randName := make([]byte, numbyte)
			rand.Read(randName)
			wantString := hex.EncodeToString(randName)

			got := randTmpName(tt.args.src)
			if len(got) != len(tt.want)+len(wantString) {
				t.Errorf("randTmpName() length does not match = %v, want %v", len(got), len(tt.want)+len(wantString))
			}
		})
	}
}
