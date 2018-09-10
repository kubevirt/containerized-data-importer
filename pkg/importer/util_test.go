package importer

import (
	"io"
	"os"
	"strings"
	"testing"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

func TestParseEnvVar(t *testing.T) {
	type args struct {
		envVarName string
		decode     bool
	}

	os.Setenv("TESTKEY", "KEYVALUE")
	os.Setenv("ENCODEDKEY", "RU5DT0RFRFZBTFVF")
	os.Setenv("BADKEY", "")

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "successfully get EnvVar",
			args:    args{"TESTKEY", false},
			want:    "KEYVALUE",
			wantErr: false,
		},
		{
			name:    "successfully get encoded EnvVar",
			args:    args{"ENCODEDKEY", true},
			want:    "ENCODEDVALUE",
			wantErr: false,
		},
		{
			name:    "invalid EnvVar",
			args:    args{"FAKETESTKEY", false},
			want:    "",
			wantErr: false,
		},
		{
			name:    "produce error when trying to decode non-encoded EnvVar",
			args:    args{"BADKEY", true},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEnvVar(tt.args.envVarName, tt.args.decode)
			if err != nil && !tt.wantErr {
				t.Errorf("ParseEnvVar() do not expect error but got %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
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
		want    bool
		setEnv  bool
		wantErr bool
	}{
		{
			name:    "successfully get url object from endpoint",
			args:    args{"http://www.bing.com"},
			want:    true,
			setEnv:  true,
			wantErr: false,
		},
		{
			name:    "successfully get url object from default value",
			args:    args{""},
			want:    true,
			setEnv:  true,
			wantErr: false,
		},
		{
			name:    "return error with empty endpoint and no default endpoint set",
			args:    args{""},
			want:    false,
			setEnv:  false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv(common.ImporterEndpoint, "www.google.com")
			if !tt.setEnv {
				os.Unsetenv(common.ImporterEndpoint)
			}
			got, err := ParseEndpoint(tt.args.endpt)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil && tt.want {
				t.Errorf("ParseEndpoint() did not get url object and expected it = %v, want %v", got, tt.want)
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
		{
			name:    "successfully streamDataToFile",
			args:    args{strings.NewReader("test data for reader 1"), "/tmp/testoutfile"},
			wantErr: false,
		},
		{
			name:    "expect error when trying to open an invalid out file",
			args:    args{strings.NewReader("test data for reader 1"), ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.Remove(tt.args.filePath)
			if err := StreamDataToFile(tt.args.dataReader, tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("StreamDataToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
