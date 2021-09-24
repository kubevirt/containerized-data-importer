# Unified logging interface for Go client libraries

This repository contains a simple unified logging interface for all oVirt Go client libraries. This is *not* a logger itself, just an interface definition coupled with default loggers.

This library is used as a dependency, so you will most likely not need to rely on it. However, if you need to fetch it you can do so using `go get`:

```bash
go get github.com/ovirt/go-ovirt-client-log/v2
```

You can then reference this library using the `ovirtclientlog` package name.

## Default loggers

This library providers 3 default loggers:

- Go logging
- Go test logging
- "NOOP" logging

### Go logging

A Go logger can be created using the `NewGoLogger()` function.

```go
logger := ovirtclientlog.NewGoLogger()
```

Optionally, a [`*log.Logger`](https://pkg.go.dev/log#Logger) instance can be passed. If it is not passed, the log is written to the globally configured log destination.

```go
buf := &bytes.Buffer{}
backingLogger := log.New(buf, "", 0)
logger := ovirtclientlog.NewGoLogger(backingLogger)
```

### Test logging

This library also contains the ability to log via [`Logf` in `*testing.T`](https://pkg.go.dev/testing#T.Logf). You can create a logger like this:

```go
func TestYourFeature(t *testing.T) {
	logger := ovirtclientlog.NewTestLogger(t)
}
```

Using the test logger will have the benefit that the log messages will be properly attributed to the test that wrote them even if multiple tests are executed in parallel.

### NOOP logging

If you need a logger that doesn't do anything simply use `ovirtclientlog.NewNOOPLogger()`.

## Adding your own logger

You can easily integrate your own logger too. Loggers must satisfy the following interface:

```go
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}
```

For example, you can adapt logging to [klog](https://github.com/kubernetes/klog) like this:

```go
type klogLogger struct {
}

func (k klogLogger) Debugf(format string, args ...interface{}) {
	// klog doesn't have a debug level
	klog.Infof(format, args...)
}

func (k klogLogger) Infof(format string, args ...interface{}) {
	klog.Infof(format, args...)
}

func (k klogLogger) Warningf(format string, args ...interface{}) {
	klog.Warningf(format, args...)
}

func (k klogLogger) Errorf(format string, args ...interface{}) {
	klog.Errorf(format, args...)
}
```

You can then create a new logger copy like this:

```go
logger := &klogLogger{}
```
