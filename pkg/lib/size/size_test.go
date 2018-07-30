package size

import (
	"fmt"
	"path/filepath"

	"testing"
)

const (
	baseImageRelPath = "../../../test/images"
	tinyCore	 = "tinyCore.iso"
	cirros		 = "cirros-qcow2.img"
)

func TestSize(t *testing.T) {
	testImg1, err := filepath.Abs(filepath.Join(baseImageRelPath, tinyCore))
	if err != nil {
		t.Fatalf("failed to get %q source image Abs path: %v\n", tinyCore, err)
	}
	testImg2, err := filepath.Abs(filepath.Join(baseImageRelPath, cirros))
	if err != nil {
		t.Fatalf("failed to get %q source image Abs path: %v\n", cirros, err)
	}

	type args struct {
		endpoint  string
		accessKey string
		secKey	  string
	}
	tests := []struct {
		name  string
		args  args
		low   int64 // lowest acceptable size due to variations in iso hdrs
		exact int64
		high  int64 // highest acceptable size due to variations in iso hdrs
	}{
		{
			name:  "tinyCore file size",
			args:  args{testImg1, "", ""},
			low:   17860000,
			exact: 18874368,
			high:  18900000,
		},
		{
			name:  "cirros-qcow2 file size",
			args:  args{testImg2, "", ""},
			low:   46137344,
			exact: 46137344,
			high:  46137344,
		},
	}
	for _, tt := range tests {
		ep := fmt.Sprintf("file://%s", tt.args.endpoint)
		descr := fmt.Sprintf("testing %q file size", ep)
		t.Log(descr)
		t.Run(descr, func(t *testing.T) {
			got, err := Size(ep, tt.args.accessKey, tt.args.secKey)
			if err != nil {
				t.Errorf("Size() error: %v", err)
				return
			}
			if got == tt.exact {
				t.Logf("** Size()=%d, exact match!\n", got)
				return
			}
			if got < tt.low || got > tt.high {
				t.Errorf("Size() %d is outside range of %d-%d", got, tt.low, tt.high)
				return
			}
			t.Logf("** Size() %d is within range of %d-%d", got, tt.low, tt.high)
		})
	}
}
