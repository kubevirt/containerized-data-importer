package errors

import (
	"fmt"
	"go/build"
	"runtime"
	"strings"

	"github.com/golang/glog"
)

const repository = "containerized-data-importer"

type error struct {
	reason         string
	reasonAndTrace string
	file           string
	funcTrace      string
	function       string
	lineNumber     int
	traceFrames    []runtime.Frame
}

func Err(errMsg string) *error {
	pc := make([]uintptr, 256)
	runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc)
	var tr []runtime.Frame

	for f, more := frames.Next(); strings.Contains(f.File, repository) || !more; f, more = frames.Next() {
		tr = append(tr, f)
	}
	currentFrame := tr[0]
	e := &error{
		reason:      errMsg,
		traceFrames: tr,
		file:        strings.TrimLeft(currentFrame.File, build.Default.GOPATH),
		function:    currentFrame.Function,
		lineNumber:  currentFrame.Line,
	}
	e.trace()
	return e
}

func Errf(pattern string, args ...interface{}) *error {
	return Err(fmt.Sprintf(pattern, args...))
}

func (e error) Error() string {
	return fmt.Sprintf("ERROR: %v\n\t%s(L%d)\n%s\n", e.reason, e.file, e.lineNumber, e.funcTrace)
}

func (e error) String() string {
	return e.Error()
}

func (e error) Log() {
	glog.Error()
}

func (e *error) trace() {
	ftrace := "Trace:"
	for i := len(e.traceFrames) - 1; i >= 0; i-- {
		ftrace = fmt.Sprintf("%s\n\t=> %s(L%d)", ftrace, e.traceFrames[i].Function, e.traceFrames[i].Line)
	}
	e.funcTrace = ftrace
	return
}
