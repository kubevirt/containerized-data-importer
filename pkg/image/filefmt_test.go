package image

import (
	"math/rand"
	"reflect"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("File format tests", func() {
	It("Copy known headers - map known headers", func() {
		got := CopyKnownHdrs()
		Expect(reflect.DeepEqual(got, knownHeaders)).To(BeTrue())
	})

	//tar bytes and offset
	token := make([]byte, 257)
	tarheader := []byte{0x75, 0x73, 0x74, 0x61, 0x72, 0x20}
	rand.Read(token)
	tarbyte := append(token, tarheader...)

	table.DescribeTable("Header match", func(h Header, b []byte, want bool) {
		got := h.Match(b)
		Expect(got).To(Equal(want))
	},
		table.Entry("match gz",
			Header{"gz", []byte{0x1F, 0x8B}, 0, 0, 0},
			[]byte{0x1F, 0x8B},
			true),
		table.Entry("match qcow2",
			Header{"qcow2", []byte{'Q', 'F', 'I', 0xfb}, 0, 24, 8},
			[]byte{'Q', 'F', 'I', 0xfb},
			true),
		table.Entry("match tar",
			Header{"tar", []byte{0x75, 0x73, 0x74, 0x61, 0x72, 0x20}, 0x101, 124, 8},
			tarbyte,
			true),
		table.Entry("match xz",
			Header{"xz", []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}, 0, 0, 0},
			[]byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00},
			true),
		table.Entry("failed match",
			Header{"gz", []byte{0x1F, 0x8B}, 0, 0, 0},
			[]byte{'Q', 'F', 'I', 0xfb},
			false),
		table.Entry("match vmdk",
			Header{"vmdk", []byte{0x4B, 0x44, 0x4D, 0x56}, 0, 24, 8},
			[]byte{0x4B, 0x44, 0x4D, 0x56},
			true),
		table.Entry("match vdi",
			Header{"vdi", []byte{0x3C, 0x3C, 0x3C, 0x20}, 0, 24, 8},
			[]byte{0x3C, 0x3C, 0x3C, 0x20},
			true),
		table.Entry("match vhd",
			Header{"vpc", []byte{0x63, 0x6F, 0x6E, 0x6E, 0x65, 0x63, 0x74, 0x69, 0x78}, 0, 24, 8},
			[]byte{0x63, 0x6F, 0x6E, 0x6E, 0x65, 0x63, 0x74, 0x69, 0x78},
			true),
		table.Entry("match vhdx",
			Header{"vhdx", []byte{0x76, 0x68, 0x64, 0x78, 0x66, 0x69, 0x6C, 0x65}, 0, 24, 8},
			[]byte{0x76, 0x68, 0x64, 0x78, 0x66, 0x69, 0x6C, 0x65},
			true),
	)

	tokenQcow := make([]byte, 20)
	qcowMagic := []byte{'Q', 'F', 'I', 0xfb}
	qcowSize := []byte("10561056")
	rand.Read(tokenQcow)
	qcowbyte := append(qcowMagic, tokenQcow...)
	qcowbyte = append(qcowbyte, qcowSize...)

	table.DescribeTable("Header size", func(h Header, b []byte, want int64, wantErr bool) {
		got, err := h.Size(b)
		if wantErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
		Expect(got).To(Equal(want))
	},
		table.Entry("get size of qcow2",
			Header{"qcow2", []byte{'Q', 'F', 'I', 0xfb}, 0, 24, 8},
			qcowbyte,
			int64(3544391413610329398),
			false),
		table.Entry("does not implement size",
			Header{"gz", []byte{0x1F, 0x8B}, 0, 0, 0},
			[]byte{0x1F, 0x8B},
			int64(0),
			false),
	)
})
