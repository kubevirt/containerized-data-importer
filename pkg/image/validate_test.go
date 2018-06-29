package image

import (
	"testing"
)

func TestIsSupportedType(t *testing.T) {
	type args struct {
		fn   string
		exts []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: ".gz is supported",
			args: args{"myfile.gz", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".xz is supported",
			args: args{"myfile.xz", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".tar is supported",
			args: args{"myfile.tar", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".qcow2 is supported",
			args: args{"myfile.qcow2", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".img is supported",
			args: args{"myfile.img", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".iso is supported",
			args: args{"myfile.iso", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".tar.gz is supported",
			args: args{"myfile.tar.gz", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".tar.xz is supported",
			args: args{"myfile.tar.xz", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".qcow2.tar.gz is supported",
			args: args{"myfile.qcow2.tar.xz", SupportedFileExtensions},
			want: true,
		},
		{
			name: ".fake is NOT supported",
			args: args{"myfile.fake", SupportedFileExtensions},
			want: false,
		},
		{
			name: "tar.fake is NOT supported",
			args: args{"myfile.tar.fake", SupportedFileExtensions},
			want: false,
		},
		{
			name: ".fake is supported based on custom extension",
			args: args{"myfile.tar.fake", []string{".fake"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedType(tt.args.fn, tt.args.exts); got != tt.want {
				t.Errorf("IsSupportedType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSupporedFileType(t *testing.T) {
	type args struct {
		fn string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: ".gz is supported",
			args: args{"myfile.gz"},
			want: true,
		},
		{
			name: ".xz is supported",
			args: args{"myfile.xz"},
			want: true,
		},
		{
			name: ".tar is supported",
			args: args{"myfile.tar"},
			want: true,
		},
		{
			name: ".qcow2 is supported",
			args: args{"myfile.qcow2"},
			want: true,
		},
		{
			name: ".img is supported",
			args: args{"myfile.img"},
			want: true,
		},
		{
			name: ".iso is supported",
			args: args{"myfile.iso"},
			want: true,
		},
		{
			name: ".tar.gz is supported",
			args: args{"myfile.tar.gz"},
			want: true,
		},
		{
			name: ".tar.xz is supported",
			args: args{"myfile.tar.xz"},
			want: true,
		},
		{
			name: ".qcow2.tar.gz is supported",
			args: args{"myfile.qcow2.tar.xz"},
			want: true,
		},
		{
			name: ".fake is NOT supported",
			args: args{"myfile.fake"},
			want: false,
		},
		{
			name: "tar.fake is NOT supported",
			args: args{"myfile.tar.fake"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedFileType(tt.args.fn); got != tt.want {
				t.Errorf("IsSupportedFileType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSupporedCompressionType(t *testing.T) {
	type args struct {
		fn string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: ".gz is supported",
			args: args{"myfile.gz"},
			want: true,
		},
		{
			name: ".xz is supported",
			args: args{"myfile.xz"},
			want: true,
		},
		{
			name: "gz.xz is supported",
			args: args{"myfile.gz.xz"},
			want: true,
		},
		{
			name: ".fake is NOT supported",
			args: args{"myfile.fake"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedCompressionType(tt.args.fn); got != tt.want {
				t.Errorf("IsSupportedCompressionType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSupporedArchiveType(t *testing.T) {
	type args struct {
		fn string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: ".tar is supported",
			args: args{"myfile.tar"},
			want: true,
		},
		{
			name: ".fake is NOT supported",
			args: args{"myfile.fake"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedArchiveType(tt.args.fn); got != tt.want {
				t.Errorf("IsSupportedArchiveType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSupporedCompressArchiveType(t *testing.T) {
	type args struct {
		fn string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: ".tar is supported",
			args: args{"myfile.tar"},
			want: true,
		},
		{
			name: ".tar.gz is supported",
			args: args{"myfile.tar.gz"},
			want: true,
		},
		{
			name: ".tar.xz is supported",
			args: args{"myfile.tar.xz"},
			want: true,
		},
		{
			name: ".fake is NOT supported",
			args: args{"myfile.fake"},
			want: false,
		},
		{
			name: ".tar.fake is NOT supported",
			args: args{"myfile.tar.fake"},
			want: false,
		},
		// TODO: shouldn't this test case fail?
		{
			name: ".fake.tar is NOT supported",
			args: args{"myfile.fake.tar"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedCompressArchiveType(tt.args.fn); got != tt.want {
				t.Errorf("IsSupportedCompressArchiveType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test conversion to lower case",
			args: args{"MyFile.Txt"},
			want: "myfile.txt",
		},
		{
			name: "test leading spaces",
			args: args{" myfile.txt"},
			want: "myfile.txt",
		},
		{
			name: "test ending spaces",
			args: args{"myfile.txt "},
			want: "myfile.txt",
		},
		{
			name: "test leading and trailing spaces and conversion to lower case",
			args: args{" MyFILE.Txt   "},
			want: "myfile.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimString(tt.args.s); got != tt.want {
				t.Errorf("TrimString() = %v, want %v", got, tt.want)
			}
		})
	}
}

