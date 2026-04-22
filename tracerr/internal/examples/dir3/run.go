package dir3

import (
	"github.com/dekey/go-pkg/tracerr/internal/examples/dir2"
)

func DoSomething3() error {
	return dir2.DoSomething2()
}
