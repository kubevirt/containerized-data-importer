package importer

import (
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	kubevirtEnvPrefix   = "KUBEVIRT_IO_"
	kubevirtLabelPrefix = "kubevirt.io/"
)

// FsyncWriter is a custom writer that wraps an os.File
// and calls fsync after writing a certain amount of data.
type FsyncWriter struct {
    file      *os.File
    bytesWritten int64 // Keeps track of how many bytes have been written
    threshold    int64 // Threshold for calling fsync
}

// NewFsyncWriter initializes a new FsyncWriter
func NewFsyncWriter(file *os.File, threshold int64) *FsyncWriter {
    return &FsyncWriter{
        file:      file,
        threshold: threshold,
    }
}

// Write writes data to the underlying file and calls fsync
// after the threshold is exceeded.
func (w *FsyncWriter) Write(p []byte) (n int, err error) {
    n, err = w.file.Write(p)
    if err != nil {
        return n, err
    }
    w.bytesWritten += int64(n)
    if w.bytesWritten >= w.threshold {
        err = w.file.Sync() // Call fsync after threshold is reached
        if err != nil {
            return n, err
        }
        w.bytesWritten = 0 // Reset counter after sync
    }
    return n, nil
}

// ParseEndpoint parses the required endpoint and return the url struct.
func ParseEndpoint(endpt string) (*url.URL, error) {
	if endpt == "" {
		// Because we are passing false, we won't decode anything and there is no way to error.
		endpt, _ = util.ParseEnvVar(common.ImporterEndpoint, false)
		if endpt == "" {
			return nil, errors.Errorf("endpoint %q is missing or blank", common.ImporterEndpoint)
		}
	}
	return url.Parse(endpt)
}

// CleanAll deletes all files at specified paths (recursively)
func CleanAll(paths ...string) error {
	for _, p := range paths {
		isDevice, err := util.IsDevice(p)
		if err != nil {
			return err
		}

		if !isDevice {
			// Remove handles p not existing
			if err := os.RemoveAll(p); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetTerminationChannel returns a channel that listens for SIGTERM
func GetTerminationChannel() <-chan os.Signal {
	terminationChannel := make(chan os.Signal, 1)
	signal.Notify(terminationChannel, os.Interrupt, syscall.SIGTERM)
	return terminationChannel
}

// newTerminationChannel should be overridden for unit tests
var newTerminationChannel = GetTerminationChannel

func envsToLabels(envs []string) map[string]string {
	labels := map[string]string{}
	for _, env := range envs {
		k, v, found := strings.Cut(env, "=")
		if !found || !strings.Contains(k, kubevirtEnvPrefix) {
			continue
		}
		labels[envToLabel(k)] = v
	}

	return labels
}

func envToLabel(env string) string {
	label := ""
	before, after, _ := strings.Cut(env, kubevirtEnvPrefix)
	if elems := strings.Split(strings.TrimSuffix(before, "_"), "_"); len(elems) > 0 && elems[0] != "" {
		label += strings.Join(elems, ".") + "."
	}
	label += kubevirtLabelPrefix
	label += strings.Join(strings.Split(after, "_"), "-")

	return strings.ToLower(label)
}

// streamDataToFile provides a function to stream the specified io.Reader to the specified local file
func streamDataToFile(r io.Reader, fileName string) error {
	outFile, err := util.OpenFileOrBlockDevice(fileName)
	if err != nil {
		return err
	}
	defer outFile.Close()
	// Wrap the file with FsyncWriter
    threshold := int64(32 * 1024 * 1024) // Set threshold to 32 MiB
	fsyncWriter := NewFsyncWriter(outFile, threshold)
	klog.V(1).Infof("Writing data...\n")
	if _, err = io.Copy(fsyncWriter, r); err != nil {
		klog.Errorf("Unable to write file from dataReader: %v\n", err)
		os.Remove(outFile.Name())
		if strings.Contains(err.Error(), "no space left on device") {
			return errors.Wrapf(err, "unable to write to file")
		}
		return NewImagePullFailedError(err)
	}
	err = outFile.Sync()
	return err
}
