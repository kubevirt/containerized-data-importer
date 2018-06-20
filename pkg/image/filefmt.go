package image

import (
	"bytes"
	"encoding/hex"
	"strconv"

	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/pkg/errors"
	"github.com/golang/glog"
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
		SizeOff:     0,
		SizeLen:     0,
	},
	{
		Format:      "qcow2",
		magicNumber: []byte{'Q', 'F', 'I', 0xfb},
		mgOffset:    0,
		SizeOff:     24,
		SizeLen:     8,
	},
	{
		Format:      "tar",
		magicNumber: []byte{0x75, 0x73, 0x74, 0x61, 0x72, 0x20},
		mgOffset:    0x101,
		SizeOff:     124,
		SizeLen:     8,
	},
	{
		Format:      "xz",
		magicNumber: []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00},
		// TODO: size not in hdr
		SizeOff:     0,
		SizeLen:     0,
	},
}

type Header struct {
	Format	    string
	magicNumber []byte
	mgOffset    int
	SizeOff	    int // in bytes
	SizeLen	    int // in bytes
}

func (h Header) match(b []byte) bool {
glog.Infof("\n***** match: len(b)=%d, h=%+v, slice=%+v\n",len(b),h,b[h.mgOffset:h.mgOffset+len(h.magicNumber)])
	return bytes.Equal(b[h.mgOffset:h.mgOffset+len(h.magicNumber)], h.magicNumber)
}

func (h *Header) Size(b []byte) (int64, error) {
	if h.SizeLen == 0 { // no size is supported in this format's header
		return 0, nil
	}
	s := hex.EncodeToString(b[h.SizeOff:h.SizeOff+h.SizeLen])
	size, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to determine original file size from %+v", s)
	}
	glog.V(Vdebug).Infof("Size: %q size in bytes: %d", h.Format, size)
	return size, nil
}
