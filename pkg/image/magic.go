package image

import (
	"fmt"
	"io"
)

const MagicSize = 4

// Return the magic number which is contained in the 1st `MagicSize` bytes of passed in file.
// If the file is too small then an empty magic string is returned.
// Error is returned if a non-eof io error occurs.
func GetMagicNumber(f io.Reader) ([]byte, error) {
	buff := make([]byte, MagicSize)
	cnt, err := f.Read(buff)
	if cnt < MagicSize {
		return []byte{}, nil
	}
	if err != nil && err != io.EOF {
		return []byte{}, fmt.Errorf("GetMagicNumber: read error: %v\n", err)
	}
	return buff, nil
}
