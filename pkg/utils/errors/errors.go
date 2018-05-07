package errors

import (
	"fmt"
	"runtime"
	"strings"
)

const repository = "containerized-data-importer"

type error struct {
	message       string
	funcTrace     string
	reverseFrames []runtime.Frame
}

func newError(errMsg string, skip int) *error {
	pc := make([]uintptr, 256)
	runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc)

	var tr []runtime.Frame
	for f, more := frames.Next(); strings.Contains(f.File, repository) || !more; f, more = frames.Next() {
		tr = append(tr, f)
	}

	ft := "Trace:"
	for i := len(tr) - 1; i >= 0; i-- {
		ft = fmt.Sprintf("%s\n\t=> %s(L%d)", ft, tr[i].Function, tr[i].Line)
	}

	return 	&error{
		message:       errMsg,
		reverseFrames: tr,
		funcTrace:     ft,
	}
}

func Err(message string) *error {
	return newError(message, 3)
}

func Errf(pattern string, args ...interface{}) *error {
	return newError(fmt.Sprintf(pattern, args...), 3)
}

func (e error) Error() string {
	return fmt.Sprintf("ERROR: %v\n%s:%s(L%d)\n%s", e.message, e.reverseFrames[0].File, e.reverseFrames[0].Function, e.reverseFrames[0].Line, e.funcTrace)
}

func (e error) String() string {
	return e.Error()
}
