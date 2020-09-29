/*
Copyright 2018 The CDI Authors.

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

package system

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

// ProcessLimiter defines the methods limiting resources of a Process
type ProcessLimiter interface {
	SetAddressSpaceLimit(pid int, value uint64) error
	SetCPUTimeLimit(pid int, value uint64) error
}

// ProcessLimitValues specifies the resource limits available to a process
type ProcessLimitValues struct {
	AddressSpaceLimit uint64
	CPUTimeLimit      uint64
}

type processLimiter struct{}

var execCommand = exec.Command
var execCommandContext = exec.CommandContext

var limiter = NewProcessLimiter()

// NewProcessLimiter returns a new ProcessLimiter
func NewProcessLimiter() ProcessLimiter {
	return &processLimiter{}
}

func (p *processLimiter) SetAddressSpaceLimit(pid int, value uint64) error {
	return prlimit(pid, unix.RLIMIT_AS, &syscall.Rlimit{Cur: value, Max: value})
}

func (p *processLimiter) SetCPUTimeLimit(pid int, value uint64) error {
	return prlimit(pid, unix.RLIMIT_CPU, &syscall.Rlimit{Cur: value, Max: value})
}

// SetAddressSpaceLimit sets a limit on total address space of a process
func SetAddressSpaceLimit(pid int, value uint64) error {
	return limiter.SetAddressSpaceLimit(pid, value)
}

// SetCPUTimeLimit sets a limit on the total cpu time a process may have
func SetCPUTimeLimit(pid int, value uint64) error {
	return limiter.SetCPUTimeLimit(pid, value)
}

// scanLinesWithCR is an alternate split function that works with carriage returns as well
// as new lines.
func scanLinesWithCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		// We have a full carriage return-terminated line.
		return i + 1, data[0:i], nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func processScanner(scanner *bufio.Scanner, buf *bytes.Buffer, done chan bool, callback func(string)) {
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		buf.WriteString("\n")
		if callback != nil {
			callback(line)
		}
	}
	done <- true
}

// ExecWithLimits executes a command with process limits
func ExecWithLimits(limits *ProcessLimitValues, callback func(string), command string, args ...string) ([]byte, error) {
	return executeWithLimits(limits, callback, true, command, args...)
}

// ExecWithLimitsSilently executes a command with process limits and do not print output on error
func ExecWithLimitsSilently(limits *ProcessLimitValues, callback func(string), command string, args ...string) ([]byte, error) {
	return executeWithLimits(limits, callback, false, command, args...)
}

func executeWithLimits(limits *ProcessLimitValues, callback func(string), logErr bool, command string, args ...string) ([]byte, error) {
	// Args can potentially contain sensitive information, make sure NOT to write args to the logs.
	var buf, errBuf bytes.Buffer
	var cmd *exec.Cmd

	stdoutDone := make(chan bool)
	stderrDone := make(chan bool)

	if limits != nil && limits.CPUTimeLimit > 0 {
		klog.V(3).Infof("Setting CPU limit to %d\n", limits.CPUTimeLimit)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(limits.CPUTimeLimit)*time.Second)
		defer cancel()
		cmd = execCommandContext(ctx, command, args...)
	} else {
		cmd = execCommand(command, args...)
	}
	stdoutIn, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrapf(err, "Couldn't get stdout for %s", command)
	}
	stderrIn, err := cmd.StderrPipe()
	if err != nil {
		return nil, errors.Wrapf(err, "Couldn't get stderr for %s", command)
	}

	scanner := bufio.NewScanner(stdoutIn)
	scanner.Split(scanLinesWithCR)
	errScanner := bufio.NewScanner(stderrIn)
	errScanner.Split(scanLinesWithCR)

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrapf(err, "Couldn't start %s", command)
	}
	defer cmd.Process.Kill()

	go processScanner(scanner, &buf, stdoutDone, callback)
	go processScanner(errScanner, &errBuf, stderrDone, callback)

	if limits != nil && limits.AddressSpaceLimit > 0 {
		klog.V(3).Infof("Setting Address space limit to %d\n", limits.AddressSpaceLimit)
		err = SetAddressSpaceLimit(cmd.Process.Pid, limits.AddressSpaceLimit)
		if err != nil {
			return nil, errors.Wrap(err, "Couldn't set address space limit")
		}
	}
	<-stdoutDone
	<-stderrDone
	// The wait has to be after the reading channels are finished otherwise there is a race where the wait completes and closes stdout/err before anything
	// is read from it.
	err = cmd.Wait()

	output := buf.Bytes()
	if err != nil {
		if logErr {
			klog.Errorf("%s failed output is:\n", command)
			klog.Errorf("%s\n", string(output))
			klog.Errorf("%s\n", string(errBuf.Bytes()))
		}
		return errBuf.Bytes(), errors.Wrapf(err, "%s execution failed", command)
	}
	return output, nil
}

func prlimit(pid int, limit int, value *syscall.Rlimit) error {
	_, _, e1 := syscall.RawSyscall6(syscall.SYS_PRLIMIT64, uintptr(pid), uintptr(limit), uintptr(unsafe.Pointer(value)), 0, 0, 0)
	if e1 != 0 {
		return errors.Wrapf(e1, "error setting prlimit on %d with value %d on pid %d", limit, value, pid)
	}
	return nil
}
