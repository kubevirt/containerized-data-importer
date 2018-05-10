package framework

import (
	"os"
	"os/exec"
	"strings"

	"github.com/kubevirt/containerized-data-importer/pkg/image"
	"github.com/pkg/errors"
)

var formatTable = map[string]func(string) (string, error){
	image.ExtGz:    transformGz,
	image.ExtXz:    transformXz,
	image.ExtTar:   transformTar,
	image.ExtQcow2: transformQcow2,
	"":             transformNoop,
}

func FormatTestData(srcFile string, targetFormats ...string) (string, error) {

	var (
		err     error
		outFile = srcFile
	)

	for _, tf := range targetFormats {
		if err != nil {
			break
		}
		i := 0
		for ftkey, ffunc := range formatTable {
			if tf == ftkey {
				outFile, err = ffunc(outFile)
				break
			} else if i == len(formatTable)-1 {
				err = errors.Errorf("format extension %q not recognized", tf)
			}
			i++
		}
	}

	if err != nil {
		return "", errors.Wrap(err, "could not format test data")
	}
	return outFile, nil
}

func transformFile(srcFile, outfileName, osCmd string, osArgs ...string) (string, error) {
	cmd := exec.Command(osCmd, osArgs...)
	cout, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "OS command %s %v errored with output: %v", osCmd, strings.Join(osArgs, " "), cout)
	}
	finfo, err := os.Stat(outfileName)
	if err != nil {
		return "", errors.Wrapf(err, "error stat-ing file")
	}
	return finfo.Name(), nil
}

func transformTar(srcFile string) (string, error) {
	args := []string{"-cf", srcFile + image.ExtTar, srcFile}
	return transformFile(srcFile, srcFile+image.ExtTar, "tar", args...)
}

func transformGz(srcFile string) (string, error) {
	return transformFile(srcFile, srcFile+image.ExtGz, "gzip", "-k", srcFile)
}

func transformXz(srcFile string) (string, error) {
	return transformFile(srcFile, srcFile+image.ExtXz, "xz", "-k", srcFile)
}

func transformQcow2(srcfile string) (string, error) {
	outFile := strings.Replace(srcfile, ".iso", image.ExtQcow2, 1)
	args := []string{"convert", "-f", "raw", "-O", "qcow2", srcfile, outFile}
	return transformFile(srcfile, outFile, "qemu-img", args...)
}

func transformNoop(srcFile string) (string, error) {
	return srcFile, nil
}
