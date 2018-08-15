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

package util

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/pkg/errors"
)

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

func TestExec(t *testing.T) {
	type args struct {
		commandOverride func(string, ...string) *exec.Cmd
		limiter         ProcessLimiter
		limits          *ProcessLimitValues
		command         string
		args            []string
	}

	limits := &ProcessLimitValues{1, 1}
	nullLimiter := newTestProcessLimiter(nil, nil)

	tests := []struct {
		name      string
		args      args
		output    string
		errString string
	}{
		{
			"command success no limits",
			args{fakeCommand, nil, nil, "faker", []string{"0", "", ""}},
			"",
			"",
		},
		{
			"command start fails",
			args{badCommand, nullLimiter, limits, "faker", []string{"", "", ""}},
			"",
			"fork/exec /usr/bin/doesnotexist: no such file or directory",
		},
		{
			"address space limit fails",
			args{fakeCommand, newTestProcessLimiter(errors.New("Set address limit fails"), nil), limits, "faker", []string{"", "", ""}},
			"",
			"Set address limit fails",
		},
		{
			"address space limit fails",
			args{fakeCommand, newTestProcessLimiter(nil, errors.New("Set CPU limit fails")), limits, "faker", []string{"", "", ""}},
			"",
			"Set CPU limit fails",
		},
		{
			"command exit bad",
			args{fakeCommand, nullLimiter, limits, "faker", []string{"1", "", ""}},
			"",
			"exit status 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replaceExecCommand(tt.args.commandOverride, func() {
				replaceLimiter(tt.args.limiter, func() {
					output, err := ExecWithLimits(tt.args.limits, tt.args.command, tt.args.args...)
					strOutput := string(output)

					if strOutput != tt.output {
						t.Errorf("Unexpected output: %s expected: %s", strOutput, tt.output)
					}

					if err != nil {
						if tt.errString == "" {
							t.Errorf("'%s' got unexpected failure: %s", tt.name, err)
						} else {
							rootErr := errors.Cause(err)
							if rootErr.Error() != tt.errString {
								t.Errorf("'%s' got wrong failure: %s, expected %s", tt.name, rootErr, tt.errString)
							}
						}

					} else if tt.errString != "" {
						t.Errorf("'%s' got unexpected success, expected: %s", tt.name, tt.errString)
					}
				})
			})
		})
	}
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

	if args[0] == "faker" {
		rc, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Fprint(os.Stdout, args[2])
		fmt.Fprint(os.Stderr, args[3])
		os.Exit(rc)
	}

	//shouldn't get here
	os.Exit(1)
}

func replaceExecCommand(replacement func(string, ...string) *exec.Cmd, f func()) {
	orig := execCommand
	if replacement != nil {
		execCommand = replacement
	}
	f()
	if replacement != nil {
		execCommand = orig
	}
}

func replaceLimiter(replacement ProcessLimiter, f func()) {
	orig := limiter
	if replacement != nil {
		limiter = replacement
	}
	f()
	if replacement != nil {
		limiter = orig
	}
}
