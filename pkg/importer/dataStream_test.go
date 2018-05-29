package importer

import (
	"io"
	"net/url"
	"reflect"
	"testing"

)

// comment
// comment
func Test_dataStream_Read(t *testing.T) {
	type fields struct {
		DataRdr     io.ReadCloser
		url         *url.URL
		accessKeyId string
		secretKey   string
	}
	type args struct {
		p []byte
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
			d := &dataStream{
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, err := d.Read(tt.args.p)
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
		DataRdr     io.ReadCloser
		url         *url.URL
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
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if err := d.Close(); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewDataStream(t *testing.T) {
	type args struct {
		ep     *url.URL
		accKey string
		secKey string
	}
	tests := []struct {
		name string
		args args
		want *dataStream
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDataStream(tt.args.ep, tt.args.accKey, tt.args.secKey); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDataStream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dataStream_dataStreamSelector(t *testing.T) {
	type fields struct {
		DataRdr     io.ReadCloser
		url         *url.URL
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    io.ReadCloser
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dataStream{
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			got, err := d.dataStreamSelector()
			if (err != nil) != tt.wantErr {
				t.Errorf("dataStream.dataStreamSelector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dataStream.dataStreamSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dataStream_s3(t *testing.T) {
	type fields struct {
		DataRdr     io.ReadCloser
		url         *url.URL
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    io.ReadCloser
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dataStream{
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
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
		DataRdr     io.ReadCloser
		url         *url.URL
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    io.ReadCloser
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dataStream{
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
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
		DataRdr     io.ReadCloser
		url         *url.URL
		accessKeyId string
		secretKey   string
	}
	tests := []struct {
		name    string
		fields  fields
		want    io.ReadCloser
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &dataStream{
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
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

func Test_dataStream_Copy(t *testing.T) {
	type fields struct {
		DataRdr     io.ReadCloser
		url         *url.URL
		accessKeyId string
		secretKey   string
	}
	type args struct {
		outPath string
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
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
				accessKeyId: tt.fields.accessKeyId,
				secretKey:   tt.fields.secretKey,
			}
			if err := d.Copy(tt.args.outPath); (err != nil) != tt.wantErr {
				t.Errorf("dataStream.Copy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_copyImage(t *testing.T) {
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
			if err := copyImage(tt.args.r, tt.args.out, tt.args.qemu); (err != nil) != tt.wantErr {
				t.Errorf("copyImage() error = %v, wantErr %v", err, tt.wantErr)
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
		DataRdr     io.ReadCloser
		url         *url.URL
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
			d := &dataStream{
				DataRdr:     tt.fields.DataRdr,
				url:         tt.fields.url,
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
