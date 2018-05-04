package util

import (
	"fmt"
	"runtime"
)

type err struct {
	msg        error
	file       string
	function   string
	lineNumber int
}

func Err(msg error) *err {
	pc := make([]uintptr, 1) // Only store the PC for the caller of Err().
	runtime.Callers(2, pc)   // skip the 2 lowest PCs (Callers() and Err).
	frames := runtime.CallersFrames(pc)
	callerFrame, _ := frames.Next() // Get the frame of the caller of Err.

	return &err{
		msg:        msg,
		file:       callerFrame.File,
		function:   callerFrame.Function,
		lineNumber: callerFrame.Line,
	}
}

func (e err) Error() string {
	return fmt.Sprintf("ERROR: <%s>%s()L%d: %s", e.file, e.function, e.lineNumber, e.msg)
}

func (e err) String() string {
	return e.Error()
}
