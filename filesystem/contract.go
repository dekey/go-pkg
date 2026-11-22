package filesystem

import "os"

const filePerm os.FileMode = 0o644

//go:generate go run github.com/vektra/mockery/v3@v3.9.1

// Opener opens a file like os.OpenFile; the caller closes it.
// NewWriter takes your own Opener or the usual NewReader().
type Opener interface {
	OpenFile(path string, flag int) (*os.File, error)
}
