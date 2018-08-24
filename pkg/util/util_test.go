package util

import (
	"regexp"
	"testing"
)

func TestRandAlphaNum(t *testing.T) {
	type args struct {
		n int
	}

	const pattern = "^[a-zA-Z0-9]+$"

	tests := []struct {
		name        string
		args        args
		wantMatches string
		expectErr   bool
	}{
		{
			name: "Test expected input",
			args: args{
				n: 8,
			},
			wantMatches: pattern,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RandAlphaNum(tt.args.n)
			if len(got) != tt.args.n {
				t.Errorf("len(RandAlphaNum()) = %v, want %v", len(got), tt.args.n)
			}
			if !regexp.MustCompile(tt.wantMatches).Match([]byte(got)) {
				t.Errorf("RandAlphaNum() = %v, want match %v", got, tt.wantMatches)
			}
		})
	}
}
