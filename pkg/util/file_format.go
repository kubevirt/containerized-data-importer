package util

import "os"

func GetFormat(path string) (string, error) {
	const (
		formatQcow2 = "qcow2"
		formatRaw   = "raw"
	)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return formatQcow2, nil
		}
		return "", err
	}
	mode := info.Mode()
	if mode&os.ModeDevice != 0 {
		return formatRaw, nil
	}
	return formatQcow2, nil
}
