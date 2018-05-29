package image

import (
	"io"
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
	// TODO: Add test cases.
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
	// TODO: Add test cases.
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
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ConvertQcow2ToRaw(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("ConvertQcow2ToRaw() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
