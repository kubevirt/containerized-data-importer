package errors

import (
	"fmt"
	"runtime"
	"strings"
)

type prettyError struct {
	message    string
	traceStack *trace
	frameStack []runtime.Frame
}

func (e prettyError) Error() string {
	return fmt.Sprintf("ERROR: %v\n%s", e.message, e.traceStack)
}

func (e prettyError) String() string {
	return e.Error()
}

func Err(msg interface{}) *prettyError {
	e := fmt.Sprintf("%v", msg)
	e = strings.TrimRight(e, "\n")
	return newError(e, 3)
}

func Errf(pattern string, args ...interface{}) *prettyError {
	return newError(strings.TrimRight(fmt.Sprintf(pattern, args...), "\n"), 3)
}

func newError(errMsg string, skip int) *prettyError {
	pc := make([]uintptr, 32)
	numPCs := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:numPCs])

	fs := frameStack(frames)
	st := newTrace(fs)

	return &prettyError{
		message:    errMsg,
		frameStack: fs,
		traceStack: st,
	}
}

func frameStack(frames *runtime.Frames) []runtime.Frame {
	var tr []runtime.Frame
	for {
		f, more := frames.Next()
		if ! strings.Contains(f.File, "containerized-data-importer") {
			break
		} else if ! more {
			break
		}
		tr = append(tr, f)
	}
	return tr
}

type trace struct {
	traceSlice []string
}

func (t *trace) String() string {
	var outputTrace string
	for _, line := range t.traceSlice {
		outputTrace = fmt.Sprintf("%s%s", outputTrace, line)
	}
	return outputTrace
}

func newTrace(fs []runtime.Frame) *trace {
	var ts = []string{"\tStack Trace:\n"}
	for i := len(fs) - 1; i >= 0; i-- {
		ts = append(ts, fmt.Sprintf("\t=> %s(L%d)\n", fs[i].Function, fs[i].Line))
	}
	return &trace{
		traceSlice: ts,
	}
}
