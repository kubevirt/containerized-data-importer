package ovirtclientlog

import (
	"testing"
)

// NewTestLogger returns a logger that logs via the Go test facility
func NewTestLogger(t *testing.T) Logger {
	return &testLogger{
		t: t,
	}
}

type testLogger struct {
	t *testing.T
}

func (t *testLogger) Debugf(format string, args ...interface{}) {
	t.t.Logf(format, args...)
}

func (t *testLogger) Infof(format string, args ...interface{}) {
	t.t.Logf(format, args...)
}

func (t *testLogger) Warningf(format string, args ...interface{}) {
	t.t.Logf(format, args...)
}

func (t *testLogger) Errorf(format string, args ...interface{}) {
	t.t.Logf(format, args...)
}
