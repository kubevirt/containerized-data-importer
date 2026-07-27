package image

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"time"

	"k8s.io/klog/v2"
)

const (
	qemuInfoTimeout = 30 * time.Second
	qemuImg         = "qemu-img"
)

// scanLinesWithCR splits on both '\r' and '\n'.
// This is needed because qemu-img -p outputs progress updates separated by '\r'.
func scanLinesWithCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

type cmdExecError struct {
	name   string
	stderr string
	err    error
}

func (e *cmdExecError) Error() string {
	return fmt.Sprintf("%s execution failed: %s: %s", e.name, e.stderr, e.err)
}

func (e *cmdExecError) Unwrap() error {
	return e.err
}

// qemuCmd provides a domain-specific API for executing qemu-img (and other)commands
type qemuCmd struct {
	run    func(ctx context.Context, name string, args ...string) ([]byte, error)
	stream func(ctx context.Context, callback func(string), name string, args ...string) error
}

func newQemuCmd() *qemuCmd {
	q := &qemuCmd{}
	q.run = q.defaultRun
	q.stream = q.defaultRunWithStream
	return q
}

// Exec runs qemu-img with the given args, discarding stdout
func (q *qemuCmd) Exec(args ...string) error {
	_, err := q.run(context.Background(), qemuImg, args...)
	return err
}

// Info runs qemu-img info with a timeout and returns stdout (for JSON parsing)
func (q *qemuCmd) Info(timeout time.Duration, url *url.URL) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return q.run(ctx, qemuImg, "info", "--output=json", url.String())
}

// ExecWithProgress runs qemu-img streaming stdout through reportProgress
func (q *qemuCmd) ExecWithProgress(args ...string) error {
	return q.stream(context.Background(), reportProgress, qemuImg, args...)
}

// ExecRaw runs an arbitrary command (e.g. dd), discarding stdout
func (q *qemuCmd) ExecRaw(name string, args ...string) error {
	_, err := q.run(context.Background(), name, args...)
	return err
}

func (q *qemuCmd) defaultRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	output, err := c.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			klog.Errorf("%s failed output is:\n%s\n%s", name, string(output), string(exitErr.Stderr))
			return exitErr.Stderr, &cmdExecError{name: name, stderr: string(exitErr.Stderr), err: err}
		}
		return nil, &cmdExecError{name: name, err: err}
	}
	return output, nil
}

func (q *qemuCmd) defaultRunWithStream(ctx context.Context, callback func(string), name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)

	var errBuf bytes.Buffer
	c.Stderr = &errBuf

	stdout, err := c.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe for %s: %w", name, err)
	}

	if err := c.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(scanLinesWithCR)
	for scanner.Scan() {
		if callback != nil {
			callback(scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil {
		klog.Warningf("%s: error reading stdout: %v", name, err)
	}

	err = c.Wait()
	if err != nil {
		klog.Errorf("%s failed, stderr:\n%s", name, errBuf.String())
		return &cmdExecError{name: name, stderr: errBuf.String(), err: err}
	}
	return nil
}
