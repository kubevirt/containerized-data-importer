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
	rand "crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
)

var _ = Describe("Process Limits", func() {
	limits := &ProcessLimitValues{1, 1}
	nullLimiter := newTestProcessLimiter(nil, nil)

	realLimits := &ProcessLimitValues{1 << 30, 10}

	//workaround for issue #3341 and #3467
	if runtime.GOARCH == "s390x" || runtime.GOARCH == "arm64" {
		realLimits = &ProcessLimitValues{1 << 31, 10}
	}

	DescribeTable("exec", func(commandOverride func(context.Context, string, ...string) *exec.Cmd, limiter ProcessLimiter, limits *ProcessLimitValues, command, output, errString string, args ...string) {
		replaceExecCommandContext(commandOverride, func() {
			replaceLimiter(limiter, func() {
				result, err := ExecWithLimits(limits, testProgress, command, args...)
				strOutput := string(result)

				Expect(strOutput).To(Equal(output))

				if err != nil {
					if errString == "" {
						Expect(err).NotTo(HaveOccurred())
					} else {
						Expect(errors.Cause(err).Error()).To(Equal(errString))
					}
				} else if errString != "" {
					Expect(err).To(HaveOccurred())
				}
			})
		})
	},
		Entry("command success with real limits", fakeCommandContext, nil, realLimits, "faker", "", "", "0", "", ""),
		Entry("command start fails", badCommand, nullLimiter, limits, "faker", "", "fork/exec /usr/bin/doesnotexist: no such file or directory", "", "", ""),
		Entry("address space limit fails", fakeCommandContext, newTestProcessLimiter(errors.New("Set address limit fails"), nil), limits, "faker", "", "Set address limit fails", "", "", ""),
		Entry("command exit bad", fakeCommandContext, nullLimiter, limits, "faker", "", "exit status 1", "1", "", ""),
	)

	DescribeTable("limits actually work", func(timeout time.Duration, f limitFunction, command, errString string, args ...string) {
		_, err := runFakeCommandWithTimeout(timeout, f, command, args...)
		Expect(err.Error()).To(Equal(errString))
	},
		Entry("killed by cpu time limit", 10*time.Second, func(p int) error { return SetCPUTimeLimit(p, 1) }, "spinner", "signal: killed"),
		Entry("killed by memory limit", 10*time.Second, func(p int) error { return SetAddressSpaceLimit(p, (1<<21)*10) }, "hog", "exit status 2"),
	)

	It("Carriage return split should work", func() {
		reader := strings.NewReader("This is a line\rThis is line two\nThis is line three")
		scanner := bufio.NewScanner(reader)
		scanner.Split(scanLinesWithCR)
		hasMore := scanner.Scan()
		Expect(hasMore).Should(BeTrue())
		Expect(scanner.Text()).To(Equal("This is a line"))
		hasMore = scanner.Scan()
		Expect(hasMore).Should(BeTrue())
		Expect(scanner.Text()).To(Equal("This is line two"))
		hasMore = scanner.Scan()
		Expect(hasMore).Should(BeTrue())
		Expect(scanner.Text()).To(Equal("This is line three"))
		hasMore = scanner.Scan()
		Expect(hasMore).Should(BeFalse())
	})
})

type testProcessLimiter struct {
	addressSpaceError error
	cpuTimeError      error
}

func newTestProcessLimiter(addressSpaceError, cpuTimeError error) ProcessLimiter {
	return &testProcessLimiter{addressSpaceError, cpuTimeError}
}

func (p *testProcessLimiter) SetAddressSpaceLimit(pid int, value uint64) error {
	return p.addressSpaceError
}

func (p *testProcessLimiter) SetCPUTimeLimit(pid int, value uint64) error {
	return p.cpuTimeError
}

func testProgress(line string) {
	// No-op
}

func fakeCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)

	//nolint:gosec // This is not production code
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func fakeCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)

	//nolint:gosec // This is not production code
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "GOCOVERDIR=/tmp"}
	return cmd
}

func badCommand(ctx context.Context, command string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "/usr/bin/doesnotexist")
}

type limitFunction func(int) error

func runFakeCommandWithTimeout(duration time.Duration, f limitFunction, command string, args ...string) ([]byte, error) {
	var buf bytes.Buffer
	cmd := fakeCommand(command, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Start()
	Expect(err).NotTo(HaveOccurred())
	defer func() {
		_ = cmd.Process.Kill()
	}()

	err = f(cmd.Process.Pid)
	Expect(err).NotTo(HaveOccurred())

	done := make(chan error)
	go func() { done <- cmd.Wait() }()

	timeout := time.After(duration)

	select {
	case <-timeout:
		Fail("Process was not killed")
	case err := <-done:
		return buf.Bytes(), err
	}

	// shouldn't get here
	return nil, errors.New("This shouldn't happen")
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) < 1 {
		os.Exit(1)
	}

	switch args[0] {
	case "faker":
		doFaker(args[1:])
	case "spinner":
		doSpinner(args[1:])
	case "hog":
		doHog(args[1:])
	}

	//shouldn't get here
	os.Exit(1)
}

func doFaker(args []string) {
	rc, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprint(os.Stdout, args[1])
	fmt.Fprint(os.Stderr, args[2])
	os.Exit(rc)
}

func doSpinner(args []string) {
	for { //nolint:staticcheck,revive

	}
}

func doHog(args []string) {
	var arrays [][]byte

	for i := 0; i < (1 << 20); i++ {
		bytes := make([]byte, 1024)
		_, err := rand.Read(bytes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		arrays = append(arrays, bytes) //nolint:staticcheck
	}
}

func replaceExecCommandContext(replacement func(context.Context, string, ...string) *exec.Cmd, f func()) {
	orig := execCommandContext
	if replacement != nil {
		execCommandContext = replacement
		defer func() { execCommandContext = orig }()
	}
	f()
}

func replaceLimiter(replacement ProcessLimiter, f func()) {
	orig := limiter
	if replacement != nil {
		limiter = replacement
		defer func() { limiter = orig }()
	}
	f()
}
