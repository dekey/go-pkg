package dir1

import (
	"errors"
	"fmt"

	"github.com/dekey/go-pkg/tracerr"
)

var ErrSomeError = errors.New("some error")

func DoSomething1() error {
	errWithStackTrace := tracerr.Wrap(ErrSomeError)

	return fmt.Errorf(
		"wrap it if you want to save stacktrace: %w",
		errWithStackTrace,
	)
}
