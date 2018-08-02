package framework

import (
	"os"
	"os/exec"
	"strings"
	"archive/tar"

	"github.com/pkg/errors"
	"kubevirt.io/containerized-data-importer/pkg/image"
	"path/filepath"
	"io"
)

var formatTable = map[string]func(string, string) (string, error){
	image.ExtGz:    gzCmd,
	image.ExtXz:    xzCmd,
	image.ExtTar:   tarCmd,
	image.ExtQcow2: qcow2Cmd,
	"":             noopCmd,
}

// create file based on targetFormat extensions and return created file's name.
// note: intermediate files are removed.
// TODO the path is retuning with the first section /Users/ missing.  I think the URL package is considering /Users/ as the server
func FormatTestData(srcFile, tgtDir string, targetFormats ...string) (string, error) {
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
	base := filepath.Base(src)
	tgt := filepath.Join(tgtDir, base+image.ExtTar)
	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0660)
	if err != nil {
		return "", errors.Wrap(err, "Erred opening file")
	}
	defer srcFile.Close()
	tarBall, err := os.Create(tgt)
	if err != nil {
		return "", errors.Wrap(err, "Erred opening file")
	}
	defer tarBall.Close()

	tw := tar.NewWriter(tarBall)
	defer tw.Close()

	srcFileInfo, err := srcFile.Stat()
	if err != nil {
		return "", errors.Wrap(err, "Erred stating file")
	}

	hdr, err := tar.FileInfoHeader(srcFileInfo, "")
	if err != nil {
		return "", errors.Wrap(err, "Erred generating tar file header")
	}

	err = tw.WriteHeader(hdr)
	if err != nil {
		return "", errors.Wrap(err, "Erred writing header")
	}

	_, err = io.Copy(tw, srcFile)
	if err != nil {
		return "", errors.Wrap(err, "Err writing to file")
	}
	return tgt, nil
}

func gzCmd(src, tgtDir string) (string, error) {
	tmpDir := os.TempDir()
	src, err := copyIfNotPresent(src, tmpDir)
	if err != nil {
		return "", err
	}
	if err = doCmd("gzip", src); err != nil {
		return "", err
	}
	return copyIfNotPresent(src+image.ExtGz, tgtDir)
}

func xzCmd(src, tgtDir string) (string, error) {
	tmpDir := os.TempDir()
	src, err := copyIfNotPresent(src, tmpDir)
	if err != nil {
		return "", err
	}
	if err = doCmd("xz", src); err != nil {
		return "", err
	}
	return copyIfNotPresent(src+image.ExtXz, tgtDir)
}

func qcow2Cmd(srcfile, tgtDir string) (string, error) {
	tgt := strings.Replace(filepath.Base(srcfile), ".iso", image.ExtQcow2, 1)
	tgt = filepath.Join(tgtDir, tgt)
	args := []string{"convert", "-f", "raw", "-O", "qcow2", srcfile, tgt}

	if err := doCmdAndVerifyFile(tgt, "qemu-img", args...); err != nil {
		return "", err
	}
	return tgt, nil
}

func noopCmd(src, tgtDir string) (string, error) {
	newSrc, err := copyIfNotPresent(src, tgtDir)
	if err != nil {
		return "", err
	}
	return newSrc, nil
}

func doCmdAndVerifyFile(tgt, cmd string, args ...string) error {
	if err := doCmd(cmd, args...); err != nil {
		return err
	}
	if _, err := os.Stat(tgt); err != nil {
		return errors.Wrapf(err, "Failed to stat file %q", tgt)
	}
	return nil
}

func doCmd(osCmd string, osArgs ...string) error {
	cmd := exec.Command(osCmd, osArgs...)
	cout, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "OS command `%s %v` errored: %v\nStdout/Stderr: %s", osCmd, strings.Join(osArgs, " "), err, string(cout))
	}
	return nil
}

// copyIfNotPresent checks for the src file in the tgtDir.  If it is not there, it attempts to copy it from src to tgtdir.
// If a copy is performed, the path to the copy is returned.
// If no copy is performed, the original src string is returned.
func copyIfNotPresent(src, tgtDir string) (string, error) {
	tgt := filepath.Join(tgtDir, filepath.Base(src))
	// Only copy the source image if it does not exist in the temp directory
	_, err := os.Stat(tgt)
	if os.IsNotExist(err) {
		if err := doCmd("cp", src, tgtDir); err == nil {
			return "", err
		}
		return tgt, nil
	}
	return src, nil
}
