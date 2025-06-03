package image

import (
	"fmt"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

func convertTo(format, src, dest string, preallocate bool) error {
	switch format {
	case "qcow2", "raw":
		// Do nothing.
	default:
		return errors.Errorf("unknown format: %s", format)
	}
	args := []string{"convert", "-t", "writeback", "-p", "-O", format, src, dest}
	var err error

	if preallocate {
		err = addPreallocation(args, convertPreallocationMethods, func(args []string) ([]byte, error) {
			return qemuExecFunction(nil, reportProgress, "qemu-img", args...)
		})
	} else {
		klog.V(1).Infof("Running qemu-img with args: %v", args)
		_, err = qemuExecFunction(nil, reportProgress, "qemu-img", args...)
	}
	if err != nil {
		os.Remove(dest)
		errorMsg := fmt.Sprintf("could not convert image to %s", format)
		if nbdkitLog, err := os.ReadFile(common.NbdkitLogPath); err == nil {
			errorMsg += " " + string(nbdkitLog)
		}
		return errors.Wrap(err, errorMsg)
	}

	return nil
}

func (o *qemuOperations) ConvertToFormatStream(url *url.URL, format, dest string, preallocate bool) error {
	if len(url.Scheme) > 0 && url.Scheme != "nbd+unix" {
		return fmt.Errorf("not valid schema %s", url.Scheme)
	}
	return convertTo(format, url.String(), dest, preallocate)
}
