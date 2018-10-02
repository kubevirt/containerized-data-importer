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
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
)

var _ = Describe("Process Limits", func() {
	limits := &ProcessLimitValues{1, 1}
	nullLimiter := newTestProcessLimiter(nil, nil)

	table.DescribeTable("exec", func(commandOverride func(string, ...string) *exec.Cmd, limiter ProcessLimiter, limits *ProcessLimitValues, command, output, errString string, args ...string) {
		replaceExecCommand(commandOverride, func() {
			replaceLimiter(limiter, func() {
				result, err := ExecWithLimits(limits, command, args...)
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
		table.Entry("command success with real limits", fakeCommand, nil, &ProcessLimitValues{1 << 30, 10}, "faker", "", "", "0", "", ""),
		table.Entry("command start fails", badCommand, nullLimiter, limits, "faker", "", "fork/exec /usr/bin/doesnotexist: no such file or directory", "", "", ""),
		table.Entry("address space limit fails", fakeCommand, newTestProcessLimiter(errors.New("Set address limit fails"), nil), limits, "faker", "", "Set address limit fails", "", "", ""),
		table.Entry("address space limit fails", fakeCommand, newTestProcessLimiter(nil, errors.New("Set CPU limit fails")), limits, "faker", "", "Set CPU limit fails", "", "", ""),
		table.Entry("command exit bad", fakeCommand, nullLimiter, limits, "faker", "", "exit status 1", "1", "", ""),
	)

	table.DescribeTable("limits actually work", func(timeout time.Duration, f limitFunction, command, errString string, args ...string) {
		_, err := runFakeCommandWithTimeout(timeout, f, command, args...)
		Expect(err.Error()).To(Equal(errString))
	},
		table.Entry("killed by cpu time limit", 10*time.Second, func(p int) error { return SetCPUTimeLimit(p, 1) }, "spinner", "signal: killed", ""),
		table.Entry("killed by memory limit", 10*time.Second, func(p int) error { return SetAddressSpaceLimit(p, (1<<21)*10) }, "hog", "exit status 2", ""),
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

func fakeCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)

	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func badCommand(string, ...string) *exec.Cmd {
	return exec.Command("/usr/bin/doesnotexist")
}

type limitFunction func(int) error

func runFakeCommandWithTimeout(duration time.Duration, f limitFunction, command string, args ...string) ([]byte, error) {
	var buf bytes.Buffer
	cmd := fakeCommand(command, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Start()
	Expect(err).NotTo(HaveOccurred())
	defer cmd.Process.Kill()

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
	for {

	}
}

func doHog(args []string) {
	var arrays [][]byte

	for i := 0; i < (1 << 20); i++ {
		bytes := make([]byte, 1024)
		rand.Read(bytes)
		arrays = append(arrays, bytes)
	}
}

func replaceExecCommand(replacement func(string, ...string) *exec.Cmd, f func()) {
	orig := execCommand
	if replacement != nil {
		execCommand = replacement
		defer func() { execCommand = orig }()
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
