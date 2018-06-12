package image

import "testing"

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
	// TODO: Add test cases.
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
	// TODO: Add test cases.
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
	// TODO: Add test cases.
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
	// TODO: Add test cases.
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
	// TODO: Add test cases.
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
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrimString(tt.args.s); got != tt.want {
				t.Errorf("TrimString() = %v, want %v", got, tt.want)
			}
		})
	}
}
