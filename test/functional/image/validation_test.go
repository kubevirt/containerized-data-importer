package image

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"

	. "github.com/kubevirt/containerized-data-importer/pkg/image"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validate file and extensions", func() {
	type testT struct {
		name    string
		expctOK bool
	}

	Context("various file extensions", func() {
		tests := []testT{
			{
				name:    "test.qcow2",
				expctOK: true,
			},
			{
				name:    "test.tar",
				expctOK: true,
			},
			{
				name:    "test.gz",
				expctOK: true,
			},
			{
				name:    "test.xz",
				expctOK: true,
			},
			{
				name:    "test.img",
				expctOK: true,
			},
			{
				name:    "test.iso",
				expctOK: true,
			},
			{
				name:    "xyz.abc",
				expctOK: false,
			},
			{
				name:    "xxx",
				expctOK: false,
			},
			{
				name:    "",
				expctOK: false,
			},
		}

		for _, t := range tests {
			name := t.name
			expctOK := t.expctOK
			ext := filepath.Ext(name)

			It(fmt.Sprintf("checking filename %q", name), func() {
				supported := IsSupporedFileType(name)
				if expctOK {
					Expect(supported).To(BeTrue(), fmt.Sprintf("%q should be a supported extension", ext))
				} else {
					Expect(supported).ToNot(BeTrue(), fmt.Sprintf("%q should not be a supported extension", ext))
				}
			})
		}
	})

	Context("QCOW2 magic number", func() {
		type testT struct {
			size       int
			expctMatch bool
		}
		tests := []testT{
			{
				size:       512,
				expctMatch: true,
			},
			{
				size:       QCOW2MagicStrSize,
				expctMatch: true,
			},
			{
				size:       1,
				expctMatch: false,
			},
		}

		for _, t := range tests {
			size := t.size
			expctQcow2 := t.expctMatch
			It(fmt.Sprintf("%d byte buffer", size), func() {
				f := createQCOW2(size)
				magic, err := GetMagicNumber(f)
				Expect(err).ToNot(HaveOccurred())
				match := MatchQcow2MagicNum(magic)
				if expctQcow2 {
					Expect(match).To(BeTrue(), fmt.Sprintf("magic num %q did not match expected %q", magic, QCOW2MagicStr))
				} else {
					Expect(match).ToNot(BeTrue(), fmt.Sprintf("magic num %q should not have been found", QCOW2MagicStr))
				}
			})
		}
	})
})

// Return a reader for the magic number. Size is how big to make the buffer.
func createQCOW2(size int) io.Reader {
	if size <= 0 {
		return nil
	}
	buf := make([]byte, size)
	_, err := rand.Read(buf) // fill buf with random stuff
	Expect(err).ToNot(HaveOccurred(), "could not generate random number")
	// set Qcow2 magic num
	for i := 0; i < QCOW2MagicStrSize && i < size; i++ {
		buf[i] = QCOW2MagicStr[i]
	}
	return bytes.NewReader(buf)
}
