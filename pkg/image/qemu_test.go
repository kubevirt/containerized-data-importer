package image

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGetMagicNumber(t *testing.T) {
	type args struct {
		f io.Reader
	}

	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name:    "return valid qcow2 magic number",
			args:    args{bytes.NewReader([]byte{'Q', 'F', 'I', 0xfb, 'T', 'H', 'I', 'S'})},
			want:    QCOW2MagicStr,
			wantErr: false,
		},
		{
			name:    "return invalid qcow2 magic number",
			args:    args{bytes.NewReader([]byte{'F', 'I', 0xfb, 'T', 'H', 'I', 'S'})},
			want:    []byte{'F', 'I', 0xfb, 'T'},
			wantErr: false,
		},
		{
			name:    "empty reader",
			args:    args{bytes.NewReader([]byte{})},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetMagicNumber(tt.args.f)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMagicNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMagicNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchQcow2MagicNum(t *testing.T) {
	type args struct {
		match []byte
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "does match magic number",
			args: args{[]byte{'Q', 'F', 'I', 0xfb, 'T', 'H', 'I', 'S'}},
			want: true,
		},
		{
			name: "does not match magic number",
			args: args{[]byte{'Q', 'T', 'I', 0xfb, 'T', 'H', 'I', 'S'}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchQcow2MagicNum(tt.args.match); got != tt.want {
				t.Errorf("MatchQcow2MagicNum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertQcow2ToRaw(t *testing.T) {
	type args struct {
		src  string
		dest string
	}
	imageDir, _ := filepath.Abs("../../test/images")
	goodImage := filepath.Join(imageDir, "cirros-qcow2.img")
	badImage := filepath.Join(imageDir, "tinyCore.iso")
	defer os.Remove("/tmp/cirros-test-good")
	defer os.Remove("/tmp/cirros-test-bad")

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "convert qcow2 image to Raw",
			args:    args{goodImage, filepath.Join(os.TempDir(), "cirros-test-good")},
			wantErr: false,
		},
		{
			name:    "failed to convert non qcow2 image to Raw",
			args:    args{badImage, filepath.Join(os.TempDir(), "cirros-test-bad")},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertQcow2ToRaw(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("ConvertQcow2ToRaw() error = %v, wantErr %v %v", err, tt.wantErr, imageDir)
			}
		})
	}
}
