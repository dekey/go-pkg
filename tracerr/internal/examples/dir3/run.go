package dir3

import (
	"github.com/dekey/go-pkg/tracerr/internal/examples/dir2"
)

// DoSomething3 returns the error of dir2.DoSomething2.
func DoSomething3() error {
	return dir2.DoSomething2()
}
