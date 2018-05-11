package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// repoRoot is used to limit frame processing to only those created by this project.  See unstackFrames() for why it is hardcoded
const repoRoot = "containerized-data-importer"

// errorStack wraps a passed in message and the internally generated stack trace.  `message` may be
// a string or an error interface.
type errorStack struct {
	message    string
	traceStack *trace
}

// Error satisfies the built-in error interface
func (e errorStack) Error() string {
	return fmt.Sprintf("ERROR: %v\n%s", e.message, e.traceStack)
}

// String satisfies the built-in Stringer interface
func (e errorStack) String() string {
	return e.Error()
}


// Err is intended to behave as the built-in errors.New() function.  Use this function to generate simple prettyErrors.
// Some pre-formatting is performed by trimming trailing new lines.
func Err(msg string) *errorStack {
	return newError(strings.TrimRight(msg, "\n"), 3)
}

// Errf is intended to behave as the built-in fmt.Errorf() function. Use this function to generate formatted prettyErrors.
// Some pre-formatting is performed by trimming trailing new lines.
func Errf(pattern string, args ...interface{}) *errorStack {
	return newError(strings.TrimRight(fmt.Sprintf(pattern, args...), "\n"), 3)
}

// Wrap will accept errors and convert them into errorStack objects.  Remember that the stack trace will be generated at
// the PC for the caller of Wrap, and so will not reflect the full trace for the wrapped error.
func Wrap(e error) *errorStack {
	return newError(e.Error(), 3)
}

// newError is the central function of the errorStack package.  Using the runtime package, it will generate a stack trace
// from the PC of runtime.Callers() - skip.  E.g. when Err() passes skip=3 to newError, the lowest PC in the frame stack
// will be the caller of Err(). Caller(), newError(), and Err() will be skipped.
// After the stack frame is generated, a trace object is generated from the frames.
func newError(errMsg string, skip int) *errorStack {
	pc := make([]uintptr, 32)
	numPCs := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:numPCs])

	return &errorStack{
		message:    errMsg,
		traceStack: newTrace(frames),
	}
}

// trace stores the formatted lines of the stack trace for output.  It satisfies the Stringer interface so that it may
// be used directly in printing statements.
type trace struct {
	traceSlice []string
}

// String satisfies the Stringer interface
func (t *trace) String() string {
	var outputTrace string
	for _, line := range t.traceSlice {
		outputTrace = fmt.Sprintf("%s%s", outputTrace, line)
	}
	return outputTrace
}

// newTrace parses the funcion (which contains an absolute path as well) and the line number, formats them in a string
// and appends that to a string slice to be wrapped by a trace object.
func newTrace(frames *runtime.Frames) *trace {
	fs := unstackFrames(frames)
	var ts = []string{"\tStack Trace:\n"}
	for i := len(fs) - 1; i >= 0; i-- {
		ts = append(ts, fmt.Sprintf("\t=> %s(L%d)\n", fs[i].Function, fs[i].Line))
	}
	return &trace{
		traceSlice: ts,
	}
}

// unstackFrames pops frames from the Frames stack by iteratively calling Next() and storing those frames in a slice
// The process ends when a frame belongs to some external package.
//
// Note: we're currently using a hard coded constant to limit our frames to only this project.  This is not ideal.
// However, using the go/build package to attempt to dynamically get the project's root directory is dangerous and prone
// to error. This method would require substantial string parsing and some assumptions about the GOPATH and project directory.
func unstackFrames(frames *runtime.Frames) []runtime.Frame {
	var tr []runtime.Frame
	for {
		f, more := frames.Next()
		if ! strings.Contains(f.File, repoRoot) {
			break
		} else if ! more {
			break
		}
		tr = append(tr, f)
	}
	return tr
}
