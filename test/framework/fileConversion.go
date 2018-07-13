package framework

import (
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"path/filepath"
	"fmt"
)

var formatTable = map[string]func(string, string) (string, error){
	image.ExtGz:    gzCmd,
	image.ExtXz:    xzCmd,
	image.ExtTar:   tarCmd,
	image.ExtQcow2: qcow2Cmd,
}

// create file based on targetFormat extensions and return created file's name.
// note: intermediate files are removed.
// TODO write the formatted file somewhere useful
// TODO the path is retuning with the first section /Users/ missing.  I think the URL package is considering /Users/ as the server
func FormatTestData(srcFile, tgtDir string, targetFormats ...string) (string, error) {

	if len(targetFormats) == 0 {
		return srcFile, nil
	}

	var err error

	for _, tf := range targetFormats {
		f, ok := formatTable[tf]
		if !ok {
			return "", errors.Errorf("format extension %q not recognized", tf)
		}
		// invoke conversion func
		srcFile, err = f(srcFile, tgtDir)
		if err != nil {
			return "", errors.Wrap(err, "could not format test data")
		}
	}
	return srcFile, nil
}

func tarCmd(src, tgtDir string) (string, error) {
	tgt := filepath.Join(tgtDir, src+image.ExtTar)
	args := []string{"-cf", tgt, src}

	if err := doCmdAndVerifyFile(tgt, "tar", args...); err != nil {
		return "", err
	}
	return tgt, nil
}

func gzCmd(src, tgtDir string) (string, error) {
	if err := doCmd("cp", []string{src, tgtDir}...); err != nil {
		return "", err
	}
	base := filepath.Base(src)
	fmt.Printf("[fileConversion.go:L61] %s<%T>: %+v\n", "base", base, base)
	src = filepath.Join(tgtDir, base)
	fmt.Printf("[fileConversion.go:L63] %s<%T>: %+v\n", "src", src, src)
	tgt := filepath.Join(tgtDir, base + image.ExtGz)
	fmt.Printf("[fileConversion.go:L65] %s<%T>: %+v\n", "tgt", tgt, tgt)
	if err := doCmdAndVerifyFile(tgt, "gzip", src); err != nil {
		return "", err
	}
	return tgt, nil
}

func xzCmd(srcFile, tgtDir string) (string, error) {
	tgt := filepath.Join(srcFile, image.ExtXz)
	args := []string{"xz", "-c", srcFile, ">", tgt}

	if err := doCmdAndVerifyFile(tgt, "gzip", args...); err != nil {
		return "", err
	}
	return tgt, nil
}

func qcow2Cmd(srcfile, tgtDir string) (string, error) {
	tgt := strings.Replace(filepath.Base(srcfile), ".iso", image.ExtQcow2, 1)
	tgt = filepath.Join(tgtDir, tgt)
	args := []string{"convert", "-f", "raw", "-O", "qcow2", srcfile, tgt}

	if err := doCmdAndVerifyFile(tgt, "gzip", args...); err != nil {
		return "", err
	}
	return tgt, nil
}

func doCmdAndVerifyFile(tgt, cmd string, args ...string) error {
	if err := doCmd(cmd, args...); err != nil {
		return err
	}
	fmt.Printf("Verifying file creation\n")
	if _, err := os.Stat(tgt); err != nil {
		return errors.Wrapf(err, "Failed to stat file %q", tgt)
	}
	return nil
}

func doCmd(osCmd string, osArgs ...string) error {
	fmt.Printf("command: %s %s\n", osCmd, osArgs)
	cmd := exec.Command(osCmd, osArgs...)
	fmt.Printf("Command:\n%#v\n\n", cmd)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "OS command `%s %v` errored: %v", osCmd, strings.Join(osArgs, " "), err)
	}
	fmt.Printf("Command succeeded\n")
	return nil
}
