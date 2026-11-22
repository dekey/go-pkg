package tracerr

import (
	"fmt"
	"runtime"
)

// DefaultCap is the starting capacity of the frames slice of a trace.
const DefaultCap = 20

// DefaultFrameLimit is the most stack frames captured per error.
const DefaultFrameLimit = 50

// Frame is a single step in stack trace.
type Frame struct {
	Func string
	Line int
	Path string
}

// String formats Frame to string.
func (f Frame) String() string {
	return fmt.Sprintf(
		"%s:%d %s()",
		f.Path,
		f.Line,
		f.Func,
	)
}

type errorData struct {
	err    error
	frames []Frame
}

// Error returns the message of the wrapped error.
func (e *errorData) Error() string {
	return e.err.Error()
}

// StackTrace returns the stack trace saved for the error.
func (e *errorData) StackTrace() []Frame {
	return e.frames
}

// Unwrap returns the wrapped error.
func (e *errorData) Unwrap() error {
	return e.err
}

func trace(err error, skip int) Error {
	frames := make([]Frame, 0, DefaultCap)
	for range DefaultFrameLimit {
		pc, path, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		funcName := ""
		if fn != nil {
			funcName = fn.Name()
		}

		frame := Frame{
			Func: funcName,
			Line: line,
			Path: path,
		}
		frames = append(frames, frame)
		skip++
	}
	return &errorData{
		err:    err,
		frames: frames,
	}
}
