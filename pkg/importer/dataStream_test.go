package importer

import (
	"io"
	"net/url"
	"reflect"
	"testing"

	"kubevirt.io/containerized-data-importer/pkg/image"
	"path/filepath"
)

func createDataStream(ep, accKey, secKey string) *dataStream {
	url, _ := ParseEndpoint(ep)

	ds := &dataStream{
		Url:         url,
		buf:         make([]byte, image.MaxExpectedHdrSize),
		accessKeyId: accKey,
		secretKey:   secKey,
	}

	ds.constructReaders()

	return ds
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
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		Qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}

	type args struct {
		buf []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.Qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, err := d.Read(tt.args.buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("dataStream.Read() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dataStream_Close(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if err := d.Close(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_dataStreamSelector(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if err := d.dataStreamSelector(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.dataStreamSelector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_s3(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Reader
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, err := d.s3()
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.s3() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.s3() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dataStream_http(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Reader
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, err := d.http()
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.http() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.http() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dataStream_local(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Reader
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, err := d.local()
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.local() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.local() = %v, want %v", got, tt.want)
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
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CopyImage(tt.args.dest, tt.args.endpoint, tt.args.accessKey, tt.args.secKey); (err != nil) != tt.wantErr {
				t.Errorf("CopyImage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_constructReaders(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if err := d.constructReaders(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.constructReaders() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_topReader(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name   string
		fields fields
		want   io.ReadCloser
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if got := d.topReader(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.topReader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dataStream_fileFormatSelector(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	type args struct {
		hdr *image.Header
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if err := d.fileFormatSelector(tt.args.hdr); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.fileFormatSelector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_gzReader(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Reader
		want1   int64
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, got1, err := d.gzReader()
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.gzReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.gzReader() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dataStream.gzReader() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_dataStream_qcow2NopReader(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	type args struct {
		h *image.Header
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Reader
		want1   int64
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, got1, err := d.qcow2NopReader(tt.args.h)
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.qcow2NopReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.qcow2NopReader() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dataStream.qcow2NopReader() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_dataStream_xzReader(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Reader
		want1   int64
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, got1, err := d.xzReader()
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.xzReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.xzReader() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dataStream.xzReader() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_dataStream_tarReader(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *Reader
		want1   int64
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, got1, err := d.tarReader()
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.tarReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.tarReader() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dataStream.tarReader() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_dataStream_matchHeader(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	type args struct {
		knownHdrs *image.Headers
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *image.Header
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, err := d.matchHeader(tt.args.knownHdrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.matchHeader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.matchHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_closeReaders(t *testing.T) {
	type args struct {
		readers []Reader
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := closeReaders(tt.args.readers); (err != nil) != tt.wantErr {
				t.Errorf("closeReaders() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_dataStream_copy(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	type args struct {
		dest string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if err := d.copy(tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.copy() error = %v, wantErr %v", err, tt.wantErr)
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
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := copy(tt.args.r, tt.args.out, tt.args.qemu); (err != nil) != tt.wantErr {
				t.Errorf("copy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_randTmpName(t *testing.T) {
	type args struct {
		src string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := randTmpName(tt.args.src); got != tt.want {
				t.Errorf("randTmpName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dataStream_parseDataPath(t *testing.T) {
	type fields struct {
		Url         *url.URL
		Readers     []Reader
		buf         []byte
		qemu        bool
		Size        int64
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
		want1  string
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dataStream{
				Url:         tt.fields.Url,
				Readers:     tt.fields.Readers,
				buf:         tt.fields.buf,
				qemu:        tt.fields.qemu,
				Size:        tt.fields.Size,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, got1 := d.parseDataPath()
			if got != tt.want {
				t.Errorf("dataStream.parseDataPath() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("dataStream.parseDataPath() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
