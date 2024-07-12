/*
Copyright 2023 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package image

import (
	"io"
	"os"
	"syscall"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
)

// DirectIOChecker checks if a certain destination supports direct I/O (bypassing page cache)
type DirectIOChecker interface {
	CheckBlockDevice(path string) (bool, error)
	CheckFile(path string) (bool, error)
}

type directIOChecker struct{}

// NewDirectIOChecker returns a new direct I/O checker
func NewDirectIOChecker() DirectIOChecker {
	return &directIOChecker{}
}

func (c *directIOChecker) CheckBlockDevice(path string) (bool, error) {
	return c.check(path, syscall.O_RDONLY)
}

func (c *directIOChecker) CheckFile(path string) (bool, error) {
	flags := syscall.O_RDONLY
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// try to create the file and perform the check
		flags = flags | syscall.O_CREAT
		defer os.Remove(path)
	}
	return c.check(path, flags)
}

// based on https://github.com/kubevirt/kubevirt/blob/c4fc4ab72a868399f5331438f35b8c33e7dd0720/pkg/virt-launcher/virtwrap/converter/converter.go#L346
func (c *directIOChecker) check(path string, flags int) (bool, error) {
	// #nosec No risk for path injection as we only open the file, not read from it. The function leaks only whether the directory to `path` exists.
	f, err := os.OpenFile(path, flags|syscall.O_DIRECT, 0600)
	if err != nil {
		// EINVAL is returned if the filesystem does not support the O_DIRECT flag
		if perr := (&os.PathError{}); errors.As(err, &perr) && errors.Is(perr, syscall.EINVAL) {
			// #nosec No risk for path injection as we only open the file, not read from it. The function leaks only whether the directory to `path` exists.
			f, err := os.OpenFile(path, flags & ^syscall.O_DIRECT, 0600)
			if err == nil {
				defer closeIOAndCheckErr(f)
				return false, nil
			}
		}
		return false, err
	}
	defer closeIOAndCheckErr(f)
	return true, nil
}

func closeIOAndCheckErr(c io.Closer) {
	if ferr := c.Close(); ferr != nil {
		klog.Errorf("Error when closing file: \n%s\n", ferr)
	}
}
