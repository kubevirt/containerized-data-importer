package size

import (
	"testing"
)

func TestSize(t *testing.T) {
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
			name:  "small tinyCore.iso file size",
			args:  args{"file:///root/go/src/kubevirt.io/containerized-data-importer/tinyCore.iso", "", ""},
			low:   17860000,
			exact: 18874368,
			high:  18900000,
		},
		{
			name:  "large windows .iso.xz file size",
			args:  args{"file:///root/go/src/kubevirt.io/containerized-data-importer/en_windows_server_2016_updated_feb_2018_x64_dvd_11636692.iso.xz", "", ""},
			low:   6006580000,
			exact: 6006587392,
			high:  6006590000,
		},
	}
	for _, tt := range tests {
		ep := tt.args.endpoint
		t.Logf("** testing endpoint=%s\n", ep)
		t.Run(tt.name, func(t *testing.T) {
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
