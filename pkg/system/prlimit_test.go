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
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
)

var _ = Describe("Process Limits", func() {
	limits := &ProcessLimitValues{CPUTimeLimit: 1}

	DescribeTable("exec", func(commandOverride func(context.Context, string, ...string) *exec.Cmd, limits *ProcessLimitValues, command, output, errString string, args ...string) {
		replaceExecCommandContext(commandOverride, func() {
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
	},
		Entry("command success", fakeCommandContext, limits, "faker", "", "", "0", "", ""),
		Entry("command start fails", badCommand, limits, "faker", "", "fork/exec /usr/bin/doesnotexist: no such file or directory", "", "", ""),
		Entry("command exit bad", fakeCommandContext, limits, "faker", "", "exit status 1", "1", "", ""),
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

func testProgress(line string) {
	// No-op
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

func replaceExecCommandContext(replacement func(context.Context, string, ...string) *exec.Cmd, f func()) {
	orig := execCommandContext
	if replacement != nil {
		execCommandContext = replacement
		defer func() { execCommandContext = orig }()
	}
	f()
}
