package importer

import (
	"io"
	"net/url"
	"reflect"
	"testing"
)

func TestParseEnvVar(t *testing.T) {
	type args struct {
		envVarName string
		decode     bool
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
			if got, _ := ParseEnvVar(tt.args.envVarName, tt.args.decode); got != tt.want {
				t.Errorf("ParseEnvVar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEndpoint(t *testing.T) {
	type args struct {
		endpt string
	}
	tests := []struct {
		name    string
		args    args
		want    *url.URL
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEndpoint(tt.args.endpt)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStreamDataToFile(t *testing.T) {
	type args struct {
		dataReader io.Reader
		filePath   string
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
			if err := StreamDataToFile(tt.args.dataReader, tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("StreamDataToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
