# Error With stack trace

This package represents a simple way to handle errors with stack trace. It is useful for logging errors with stack trace.

## How to use

Error should be wrapped with: `Wrap(ErrSome)`, `Errorf("message %s", strVar)` or created new one with `New()` functions.
More info in `tracerr/errors.go`. Best way to have full stack trace is to wrap error in one places. Placing `Wrap()`
on errors that connect external code with internal code will show full stack trace from entry point to error wrapping.

Example:

```go
package example

import (
	"errors"

	"github.com/dekey/go-pkg/tracerr"
)

func DoSomething() error {
	errSomeError := errors.New("some error")
	
	return error_tracer.Wrap(errSomeError)
}
```
