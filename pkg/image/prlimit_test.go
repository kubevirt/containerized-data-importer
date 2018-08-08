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

package image

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
)

const killedByTestError = "Had to kill process"

func TestLimits(t *testing.T) {
	const twentyFiveMeg = (1 << 20) * 25
	type args struct {
		limitFunc   func(int)
		timeout     time.Duration
		command     string
		commandArgs []string
	}

	memConsumerScript, err := filepath.Abs("../../test/scripts/memory-consumer.sh")
	if err != nil {
		t.Error("Can't find mamory consumer script")
	}

	tests := []struct {
		name         string
		args         args
		errorMessage string
	}{
		{
			name:         "normal execution with cpu limit",
			args:         args{cpuLimitFunction(1), 2 * time.Second, "dd", []string{"if=/dev/zero", "of=/dev/null", "count=10"}},
			errorMessage: "",
		},
		{
			name:         "should get killed by cpu limit",
			args:         args{cpuLimitFunction(1), 2 * time.Second, "dd", []string{"if=/dev/zero", "of=/dev/null", "count=1000000000"}},
			errorMessage: "signal: killed",
		},

		{
			name:         "normal execution with memory limit",
			args:         args{memoryLimitFunction(twentyFiveMeg), 10 * time.Second, memConsumerScript, []string{"512"}},
			errorMessage: "",
		},
		{
			name:         "should get killed by memory limit",
			args:         args{memoryLimitFunction(twentyFiveMeg), 10 * time.Second, memConsumerScript, []string{"2048"}},
			errorMessage: "exit status 2",
		},
		{
			name:         "should get killed by test",
			args:         args{func(int) {}, 4 * time.Second, memConsumerScript, []string{"2048"}},
			errorMessage: killedByTestError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFail := false
			output, err := executeComand(tt.args.limitFunc, tt.args.timeout, tt.args.command, tt.args.commandArgs)
			if err != nil && (tt.errorMessage == "" || tt.errorMessage != err.Error()) {
				t.Logf("Got unexpected failure: '%s', '%s'", tt.errorMessage, err)
				testFail = true
			} else if err == nil && tt.errorMessage != "" {
				t.Logf("Got unexpected success, expected '%s'", tt.errorMessage)
				testFail = true
			}
			if testFail {
				t.Log(string(output))
				t.Errorf("'%s' test failed for %+v", tt.name, tt.args)
			}
		})
	}
}

func cpuLimitFunction(limit uint64) func(int) {
	return func(pid int) { setCPUTimeLimit(pid, limit) }
}

func memoryLimitFunction(limit uint64) func(int) {
	return func(pid int) { setAddressSpaceLimit(pid, limit) }
}

func executeComand(limitFunction func(int), maxTime time.Duration, command string, args []string) ([]byte, error) {
	var buf bytes.Buffer
	cmd := exec.Command(command, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	cmd.Start()

	limitFunction(cmd.Process.Pid)

	done := make(chan error)
	go func() { done <- cmd.Wait() }()

	timeout := time.After(maxTime)

	select {
	case <-timeout:
		cmd.Process.Kill()
		return nil, errors.New(killedByTestError)
	case err := <-done:
		return buf.Bytes(), err
	}
}
