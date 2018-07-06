package image

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestCopyKnownHdrs(t *testing.T) {

	tests := []struct {
		name    string
		want    Headers
		wantErr bool
	}{
		{
			name:    "map known headers",
			want:    knownHeaders,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CopyKnownHdrs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CopyKnownHdrs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeader_Match(t *testing.T) {
	type fields struct {
		Format      string
		magicNumber []byte
		mgOffset    int
		SizeOff     int
		SizeLen     int
	}

	//tar bytes and offset
	token := make([]byte, 257)
	tarheader := []byte{0x75, 0x73, 0x74, 0x61, 0x72, 0x20}
	rand.Read(token)
	tarbyte := append(token, tarheader...)

	type args struct {
		b []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "match gz",
			fields: fields{"gz", []byte{0x1F, 0x8B}, 0, 0, 0},
			args:   args{[]byte{0x1F, 0x8B}},
			want:   true,
		},
		{
			name:   "match qcow2",
			fields: fields{"qcow2", []byte{'Q', 'F', 'I', 0xfb}, 0, 24, 8},
			args:   args{[]byte{'Q', 'F', 'I', 0xfb}},
			want:   true,
		},
		{
			name:   "match tar",
			fields: fields{"tar", []byte{0x75, 0x73, 0x74, 0x61, 0x72, 0x20}, 0x101, 124, 8},
			args:   args{tarbyte},
			want:   true,
		},
		{
			name:   "match xz",
			fields: fields{"xz", []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}, 0, 0, 0},
			args:   args{[]byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}},
			want:   true,
		},
		{
			name:   "failed match",
			fields: fields{"gz", []byte{0x1F, 0x8B}, 0, 0, 0},
			args:   args{[]byte{'Q', 'F', 'I', 0xfb}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Header{
				Format:      tt.fields.Format,
				magicNumber: tt.fields.magicNumber,
				mgOffset:    tt.fields.mgOffset,
				SizeOff:     tt.fields.SizeOff,
				SizeLen:     tt.fields.SizeLen,
			}
			if got := h.Match(tt.args.b); got != tt.want {
				t.Errorf("Header.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeader_Size(t *testing.T) {
	type fields struct {
		Format      string
		magicNumber []byte
		mgOffset    int
		SizeOff     int
		SizeLen     int
	}

	//tar bytes and offset
	token := make([]byte, 20)
	qcowMagic := []byte{'Q', 'F', 'I', 0xfb}
	qcowSize := []byte("10561056")
	rand.Read(token)
	qcowbyte := append(qcowMagic, token...)
	qcowbyte = append(qcowbyte, qcowSize...)

	type args struct {
		b []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int64
		wantErr bool
	}{
		{
			name:    "get size of qcow2",
			fields:  fields{"qcow2", []byte{'Q', 'F', 'I', 0xfb}, 0, 24, 8},
			args:    args{qcowbyte},
			want:    3544391413610329398,
			wantErr: false,
		},
		{
			name:    "does not implement size",
			fields:  fields{"gz", []byte{0x1F, 0x8B}, 0, 0, 0},
			args:    args{[]byte{0x1F, 0x8B}},
			want:    0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := Header{
				Format:      tt.fields.Format,
				magicNumber: tt.fields.magicNumber,
				mgOffset:    tt.fields.mgOffset,
				SizeOff:     tt.fields.SizeOff,
				SizeLen:     tt.fields.SizeLen,
			}
			got, err := h.Size(tt.args.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("Header.Size() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Header.Size() = %v, want %v", got, tt.want)
			}
		})
	}
}
