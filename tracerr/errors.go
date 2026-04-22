package tracerr

import (
	"errors"
	"fmt"
)

const DefaultSkip = 2

// Error is an error with stack trace.
type Error interface {
	Error() string
	StackTrace() []Frame
	Unwrap() error
}

// Errorf creates new error with stacktrace and formatted message.
// Formatting works the same way as in fmt.Errorf.
func Errorf(message string, args ...any) Error {
	return trace(
		fmt.Errorf(message, args...),
		DefaultSkip,
	)
}

// New creates new error with stacktrace.
func New(message string) Error {
	return trace(fmt.Errorf("%s", message), DefaultSkip)
}

// Wrap adds stacktrace to existing error.
func Wrap(err error) Error {
	if err == nil {
		return nil
	}
	var e Error
	ok := errors.As(err, &e)
	if ok {
		return e
	}
	return trace(err, DefaultSkip)
}

// StackTrace returns stack trace of an error.
// It will be empty if err is not of type Error.
func StackTrace(err error) []Frame {
	var e Error
	ok := errors.As(err, &e)
	if !ok {
		return nil
	}
	return e.StackTrace()
}

// Unwrap returns the original error.
func Unwrap(err error) error {
	if err == nil {
		return nil
	}
	var e Error
	ok := errors.As(err, &e)
	if !ok {
		return err
	}
	return e.Unwrap()
}

func IsTraceableError(err error) bool {
	if err == nil {
		return false
	}

	var traceableError Error

	return errors.As(err, &traceableError)
}
