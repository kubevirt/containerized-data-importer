package image

import (
	"os/exec"

	"github.com/pkg/errors"
)

// ConvertQcow2ToRaw is a wrapper for qemu-img convert which takes a qcow2 file (specified by src) and converts
// it to a raw image (written to the provided dest file)
func ConvertQcow2ToRaw(src, dest string) error {
	cmd := exec.Command("qemu-img", "convert", "-f", "qcow2", "-O", "raw", src, dest)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "could not convert qcow2 image to raw")
	}
	return nil
}
