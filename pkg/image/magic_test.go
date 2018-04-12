package image_test

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"

	"github.com/kubevirt/containerized-data-importer/pkg/common"
	. "github.com/kubevirt/containerized-data-importer/pkg/image"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("QCOW2 magic number", func() {

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
			size:       common.QCOW2MagicStrSize,
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
			Expect(err).ToNot(HaveOccurred(), "GetMagicNumber error:")
			match := MatchQcow2MagicNum(magic)
			if expctQcow2 {
				Expect(match).To(BeTrue(), fmt.Sprintf("magic num %q did not match expected %q", magic, common.QCOW2MagicStr))
			} else {
				Expect(match).ToNot(BeTrue(), fmt.Sprintf("magic num %q should not have been found", common.QCOW2MagicStr))
			}
		})
	}
})

func createQCOW2(size int) io.Reader {
	if size <= 0 {
		return nil
	}
	buf := make([]byte, size)
	_, err := rand.Read(buf) // fill buf with random stuff
	Expect(err).ToNot(HaveOccurred(), "createQCOW2: rand.Read errror:")
	// set Qcow2 magic num
	for i := 0; i < common.QCOW2MagicStrSize && i < size; i++ {
		buf[i] = common.QCOW2MagicStr[i]
	}
	return bytes.NewReader(buf)
}
