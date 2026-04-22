package tracerr

import (
	"fmt"
	"runtime"
)

// DefaultCap is the initial capacity of the frames slice allocated in trace.
// Pre-allocating avoids repeated heap growth when the stack depth is typical;
// tune it to the expected call depth of your application for best performance.
const DefaultCap = 20

// DefaultFrameLimit is the maximum number of stack frames captured per error.
// Prevents unbounded loop iteration in trace when the call stack is unusually deep
// or runtime.Caller never returns ok=false (e.g. in tests or recursive code).
const DefaultFrameLimit = 50

// Frame is a single step in stack trace.
type Frame struct {
	// Func contains a function name.
	Func string
	// Line contains a line number.
	Line int
	// Path contains a file path.
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
	// err contains original error.
	err error
	// frames contains stack trace of an error.
	frames []Frame
}

// Error returns error message.
func (e *errorData) Error() string {
	return e.err.Error()
}

// StackTrace returns stack trace of an error.
func (e *errorData) StackTrace() []Frame {
	return e.frames
}

// Unwrap returns the original error.
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
