package image

import (
	"bytes"
	"fmt"
	"os/exec"
)

var qcowMagicNum = []byte{'Q', 'F', 'I', 0xfb}

func MatchQcow2MagicNum(match []byte) bool {
	return bytes.HasPrefix(match, qcowMagicNum)
}

func ConvertQcow2ToRaw(src, dest string) error {
	cmd := exec.Command("qemu-img", "convert", "-f", "qcow2", "-O", "raw", src, dest)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ConvertQcow2ToRaw: command failed: %v\n", err)
	}
	return nil
}
