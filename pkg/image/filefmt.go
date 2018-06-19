package image

import (
	"bytes"
	"encoding/binary"

"github.com/golang/glog"
	"github.com/pkg/errors"
)

const MaxExpectedHdrSize = 1024 // 1kb

func MatchHeader(hdr *Header, b []byte) *Header {
	if hdr.match(b) {
		return hdr
	}
	return nil
}

var KnownHdrs = []*Header{
	{
		Format:      "gz",
		magicNumber: []byte{0x1F, 0x8B},
		// TODO: size not in hdr
		sizeOff:     0,
		sizeLen:     0,
	},
	{
		Format:      "qcow2",
		magicNumber: []byte{'Q', 'F', 'I', 0xfb},
		mgOffset:    0,
		sizeOff:     24,
		sizeLen:     8,
	},
	{
		Format:      "tar",
		magicNumber: []byte{0x75, 0x73, 0x74, 0x61, 0x72, 0x00},
		mgOffset:    0x101,
		sizeOff:     124,
		sizeLen:     8,
	},
	{
		Format:      "xz",
		magicNumber: []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00},
		// TODO: size not in hdr
		sizeOff:     0,
		sizeLen:     0,
	},
}

type Header struct {
	Format	    string
	magicNumber []byte
	mgOffset    int
	sizeOff	    int // in bytes
	sizeLen	    int // in bytes
}

func (h Header) match(b []byte) bool {
glog.Infof("\n***** match: h=%+v, len(magic)=%d\n",h,len(h.magicNumber))
	return bytes.Equal(b[h.mgOffset:h.mgOffset+len(h.magicNumber)], h.magicNumber)
}

// BIG OR LITTLE-ENDIAN?
func (h *Header) Size(b []byte) (int64, error) {
	sizeLen := h.sizeLen
	if sizeLen == 0 { // indicates no size is supported in this format's header
		return 0, nil
	}
	size, n := binary.Varint(b[h.sizeOff:h.sizeOff+sizeLen])
	if n != len(b) {
		return 0, errors.Errorf("internal Size error: number of bytes read (%d) != expected (%d)", n, len(b))
	}
	return size, nil
}
