package util

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	blockdevFileName = "/usr/sbin/blockdev"
)

// CountingReader is a reader that keeps track of how much has been read
type CountingReader struct {
	Reader  io.ReadCloser
	Current uint64
	Done    bool
}

// RandAlphaNum provides an implementation to generate a random alpha numeric string of the specified length
func RandAlphaNum(n int) string {
	rand.Seed(time.Now().UnixNano())
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

// GetNamespace returns the namespace the pod is executing in
func GetNamespace() string {
	return getNamespace("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
}

func getNamespace(path string) string {
	if data, err := ioutil.ReadFile(path); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}
	return "cdi"
}

// ParseEnvVar provides a wrapper to attempt to fetch the specified env var
func ParseEnvVar(envVarName string, decode bool) (string, error) {
	value := os.Getenv(envVarName)
	if decode {
		v, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return "", errors.Errorf("error decoding environment variable %q", envVarName)
		}
		value = fmt.Sprintf("%s", v)
	}
	return value, nil
}

// Read reads bytes from the stream and updates the prometheus clone_progress metric according to the progress.
func (r *CountingReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.Current += uint64(n)
	r.Done = err == io.EOF
	return n, err
}

// Close closes the stream
func (r *CountingReader) Close() error {
	return r.Reader.Close()
}

// GetAvailableSpaceByVolumeMode calls another method based on the volumeMode parameter to get the amount of
// available space at the path specified.
func GetAvailableSpaceByVolumeMode(volumeMode v1.PersistentVolumeMode) (int64, error) {
	if volumeMode == v1.PersistentVolumeBlock {
		return GetAvailableSpaceBlock(common.WriteBlockPath)
	}
	return GetAvailableSpace(common.ImporterVolumePath)
}

// GetAvailableSpace gets the amount of available space at the path specified.
func GetAvailableSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return int64(-1), err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// GetAvailableSpaceBlock gets the amount of available space at the block device path specified.
func GetAvailableSpaceBlock(deviceName string) (int64, error) {
	// Check if device exists.
	info, err := os.Stat(deviceName)
	if os.IsNotExist(err) {
		return int64(-1), nil
	}
	if info.IsDir() {
		return int64(-1), nil
	}
	// Device exists and is not a directory attempt to get size
	cmd := exec.Command(blockdevFileName, "--getsize64", deviceName)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if err != nil {
		return int64(-1), errors.Errorf("%v, %s", err, errBuf.String())
	}
	i, err := strconv.ParseInt(strings.TrimSpace(out.String()), 10, 64)
	if err != nil {
		return int64(-1), err
	}
	return i, nil
}

// MinQuantity calculates the minimum of two quantities.
func MinQuantity(availableSpace, imageSize *resource.Quantity) resource.Quantity {
	if imageSize.Cmp(*availableSpace) == 1 {
		return *availableSpace
	}
	return *imageSize
}

// StreamDataToFile provides a function to stream the specified io.Reader to the specified local file
func StreamDataToFile(r io.Reader, fileName string) error {
	var outFile *os.File
	blockSize, err := GetAvailableSpaceBlock(fileName)
	if err != nil {
		return errors.Wrapf(err, "error determining if block device exists")
	}
	if blockSize >= 0 {
		// Block device found and size determined.
		outFile, err = os.OpenFile(fileName, os.O_EXCL|os.O_WRONLY, os.ModePerm)
	} else {
		// Attempt to create the file with name filePath.  If it exists, fail.
		outFile, err = os.OpenFile(fileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.ModePerm)
	}
	if err != nil {
		return errors.Wrapf(err, "could not open file %q", fileName)
	}
	defer outFile.Close()
	klog.V(1).Infof("Writing data...\n")
	if _, err = io.Copy(outFile, r); err != nil {
		klog.Errorf("Unable to write file from dataReader: %v\n", err)
		os.Remove(outFile.Name())
		return errors.Wrapf(err, "unable to write to file")
	}
	err = outFile.Sync()
	return err
}

// UnArchiveTar unarchives a tar file and streams its files
// using the specified io.Reader to the specified destination.
func UnArchiveTar(reader io.Reader, destDir string, arg ...string) error {
	klog.V(1).Infof("begin untar to %s...\n", destDir)

	var tarOptions string
	var args = arg
	if len(arg) > 0 {
		tarOptions = arg[0]
		args = arg[1:]
	}
	options := fmt.Sprintf("-%s%s", tarOptions, "xvC")
	untar := exec.Command("/usr/bin/tar", options, destDir, strings.Join(args, ""))
	untar.Stdin = reader
	var errBuf bytes.Buffer
	untar.Stderr = &errBuf
	err := untar.Start()
	if err != nil {
		return err
	}
	err = untar.Wait()
	if err != nil {
		klog.V(3).Infof("%s\n", errBuf.String())
		klog.Errorf("%s\n", err.Error())
		return err
	}
	return nil
}

// CopyFile copies a file from one location to another.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// WriteTerminationMessage writes the passed in message to the default termination message file
func WriteTerminationMessage(message string) error {
	return WriteTerminationMessageToFile(common.PodTerminationMessageFile, message)
}

// WriteTerminationMessageToFile writes the passed in message to the passed in message file
func WriteTerminationMessageToFile(file, message string) error {
	// Only write the first line of the message.
	scanner := bufio.NewScanner(strings.NewReader(message))
	if scanner.Scan() {
		err := ioutil.WriteFile(file, []byte(scanner.Text()), os.ModeAppend)
		if err != nil {
			return errors.Wrap(err, "could not create termination message file")
		}
	}
	return nil
}

// CopyDir copies a dir from one location to another.
func CopyDir(source string, dest string) (err error) {
	// get properties of source dir
	sourceinfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	// create dest dir
	err = os.MkdirAll(dest, sourceinfo.Mode())
	if err != nil {
		return err
	}

	directory, _ := os.Open(source)
	objects, err := directory.Readdir(-1)

	for _, obj := range objects {
		src := filepath.Join(source, obj.Name())
		dst := filepath.Join(dest, obj.Name())

		if obj.IsDir() {
			// create sub-directories - recursively
			err = CopyDir(src, dst)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			// perform copy
			err = CopyFile(src, dst)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	return
}

// RoundDown returns the number rounded down to the nearest multiple.
func RoundDown(number, multiple int64) int64 {
	return number / multiple * multiple
}
