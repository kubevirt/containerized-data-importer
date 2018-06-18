package image

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

const MaxExpectedHdrSize = 1024 // 1kb

func MatchHeader(hdr *Header, r io.Reader, b []byte) (*Header, error) {
	if b == nil {
		b = make([]byte, MaxExpectedHdrSize)
	}
	n, err := r.Read(b)
	if err != nil {
		return nil, errors.Wrap(err, "could not read file header")
	}
	if n != MaxExpectedHdrSize {
		return nil, errors.Errorf("could not read all %d bytes of file header", MaxExpectedHdrSize)
	}
	if hdr.match(b) {
		return hdr, nil
	}
	return nil, nil
}

var KnownHdrs = []*Header{
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
		// TODO size
		sizeOff:     0,
		sizeLen:     8,
	},
}

type Header struct {
	Format	    string
	magicNumber []byte
	mgOffset    int
	sizeOff	    int // in bytes
	sizeLen	    int // in bytes
}

func (h *Header) match(b []byte) bool {
	return bytes.Equal(b[h.mgOffset-1:h.mgOffset-1+len(h.magicNumber)], h.magicNumber)
}

// BIG OR LITTLE-ENDIAN?
func (h *Header) Size(b []byte) (int64, error) {
	size, n := binary.Varint(b[h.sizeOff-1:h.sizeOff-1+h.sizeLen])
	if n != len(b) {
		return 0, errors.Errorf("internal Size error: number of bytes read (%d) != expected (%d)", n, len(b))
	}
	return size, nil
}
