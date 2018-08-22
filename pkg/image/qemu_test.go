package image

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConvertQcow2ToRaw(t *testing.T) {
	type args struct {
		src  string
		dest string
	}
	// for local tests use repo path
	// for dockerized tests use depFile name
	// also having an issue with the makefile unit tests
	// if running in docker it all works fine
	imageDir, _ := filepath.Abs("../../tests/images")
	goodImage := filepath.Join(imageDir, "cirros-qcow2.img")
	badImage := filepath.Join(imageDir, "tinyCore.iso")
	if _, err := os.Stat(goodImage); os.IsNotExist(err) {
		goodImage = "cirros-qcow2.img"
	}
	if _, err := os.Stat(badImage); os.IsNotExist(err) {
		badImage = "tinyCore.iso"
	}

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
