package filesystem

import "os"

// filePerm is the mode of a file that CreateFile, AppendLine or OpenFile with os.O_CREATE makes,
// less the umask.
const filePerm os.FileMode = 0o644

type reader interface {
	OpenFile(path string, flag int) (*os.File, error)
}
