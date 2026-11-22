package filesystem

import "os"

// filePerm is the mode of a file that Reader.OpenFile makes for os.O_CREATE, less the umask.
const filePerm os.FileMode = 0o644

//go:generate go run github.com/vektra/mockery/v3@v3.9.1

// Opener opens the file at path with flag, as os.OpenFile does, and returns it; the caller closes
// the file. CreateFile and AppendLine open files only through the Opener of their Writer. The usual
// Opener is the *Reader from NewReader. An error that has no tracerr stack trace gets one from the
// Writer method that called OpenFile.
type Opener interface {
	OpenFile(path string, flag int) (*os.File, error)
}
