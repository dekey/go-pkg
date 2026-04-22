package dir2

import (
	"github.com/dekey/go-pkg/tracerr/internal/examples/dir1"
)

func DoSomething2() error {
	return dir1.DoSomething1()
}
