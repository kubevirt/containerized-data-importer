package util

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"k8s.io/klog/v2"
)

// OpenFileOrBlockDevice opens the destination data file, whether it is a block device or regular file
func OpenFileOrBlockDevice(fileName string) (*os.File, error) {
	var outFile *os.File
	blockSize, err := GetAvailableSpaceBlock(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "error determining if block device exists")
	}
	if blockSize >= 0 {
		// Block device found and size determined.
		outFile, err = os.OpenFile(fileName, os.O_EXCL|os.O_WRONLY, os.ModePerm)
	} else {
		// Attempt to create the file with name filePath.  If it exists, fail.
		outFile, err = os.OpenFile(fileName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.ModePerm)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "could not open file %q", fileName)
	}
	return outFile, nil
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

// LinkFile symlinks the source to the target
func LinkFile(source, target string) error {
	out, err := exec.Command("/usr/bin/ln", "-s", source, target).CombinedOutput()
	if err != nil {
		fmt.Printf("out [%s]\n", string(out))
		return err
	}
	return nil
}

// CopyDir copies a dir from one location to another.
func CopyDir(source string, dest string) error {
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
	return err
}

// GetAvailableSpace gets the amount of available space at the path specified.
func GetAvailableSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return int64(-1), err
	}
	return int64(stat.Bavail) * stat.Bsize, nil
}

// GetAvailableSpaceBlock gets the amount of available space at the block device path specified.
func GetAvailableSpaceBlock(deviceName string) (int64, error) {
	// Check if the file exists and is a device file.
	if ok, err := IsDevice(deviceName); !ok || err != nil {
		return int64(-1), err
	}

	// Device exists, attempt to get size.
	cmd := exec.Command(blockdevFileName, "--getsize64", deviceName)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		return int64(-1), errors.Errorf("%v, %s", err, errBuf.String())
	}
	i, err := strconv.ParseInt(strings.TrimSpace(out.String()), 10, 64)
	if err != nil {
		return int64(-1), err
	}
	return i, nil
}

// IsDevice returns true if it's a device file
func IsDevice(deviceName string) (bool, error) {
	info, err := os.Stat(deviceName)
	if err == nil {
		return (info.Mode() & os.ModeDevice) != 0, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// Three functions for zeroing a range in the destination file:

// PunchHole attempts to zero a range in a file with fallocate, for block devices and pre-allocated files.
func PunchHole(outFile *os.File, start, length int64) error {
	klog.Infof("Punching %d-byte hole at offset %d", length, start)
	flags := uint32(unix.FALLOC_FL_PUNCH_HOLE | unix.FALLOC_FL_KEEP_SIZE)
	err := syscall.Fallocate(int(outFile.Fd()), flags, start, length)
	if err == nil {
		_, err = outFile.Seek(length, io.SeekCurrent) // Just to move current file position
	}
	return err
}

// AppendZeroWithTruncate resizes the file to append zeroes, meant only for newly-created (empty and zero-length) regular files.
func AppendZeroWithTruncate(outFile *os.File, start, length int64) error {
	klog.Infof("Truncating %d-bytes from offset %d", length, start)
	end, err := outFile.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if start != end {
		return errors.Errorf("starting offset %d does not match previous ending offset %d, cannot safely append zeroes to this file using truncate", start, end)
	}
	err = outFile.Truncate(start + length)
	if err != nil {
		return err
	}
	_, err = outFile.Seek(0, io.SeekEnd)
	return err
}

var zeroBuffer []byte

// AppendZeroWithWrite just does normal file writes to the destination, a slow but reliable fallback option.
func AppendZeroWithWrite(outFile *os.File, start, length int64) error {
	klog.Infof("Writing %d zero bytes at offset %d", length, start)
	offset, err := outFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if start != offset {
		return errors.Errorf("starting offset %d does not match previous ending offset %d, cannot safely append zeroes to this file using write", start, offset)
	}
	if zeroBuffer == nil { // No need to re-allocate this on every write
		zeroBuffer = bytes.Repeat([]byte{0}, 32<<20)
	}
	count := int64(0)
	for count < length {
		blockSize := int64(len(zeroBuffer))
		remaining := length - count
		if remaining < blockSize {
			blockSize = remaining
		}
		written, err := outFile.Write(zeroBuffer[:blockSize])
		if err != nil {
			return errors.Wrapf(err, "unable to write %d zeroes at offset %d: %v", length, start+count, err)
		}
		count += int64(written)
	}
	return nil
}
