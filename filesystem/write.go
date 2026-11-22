package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dekey/go-pkg/tracerr"
)

var (
	ErrMakeTemp   = errors.New("make temp file")
	ErrNoFreeName = errors.New("no free name")
)

// Writer writes temp files, links them to a free name, removes files, opens or makes files and
// appends lines to files. It holds only its Opener, which CreateFile and AppendLine open files
// with.
type Writer struct {
	opener Opener
}

// NewWriter returns a new Writer that opens files with opener. Pass NewReader() unless you need
// another Opener.
func NewWriter(opener Opener) *Writer {
	return &Writer{
		opener: opener,
	}
}

// WriteTemp writes data to a new file in dir, syncs it to the disk and returns the file's path. An
// empty dir means os.TempDir(). The file name comes from pattern as in os.CreateTemp. The file gets
// mode perm exactly: the umask (the mask of the process that takes bits off the mode of new files)
// does not change it. A file that can not be made gives ErrMakeTemp. When the write, the mode
// change, the sync or the close fails, WriteTemp removes the file and returns that error; a failed
// remove is joined into it.
//
// To replace a file in one step, give WriteTemp the folder of that file and os.Rename the temp file
// over it. os.Rename works only within one file system; across file systems it fails. When the
// rename fails, remove the temp file with Remove. For the rename itself to survive a power loss,
// also open the folder and call Sync on it. A pattern that starts with a dot hides a left temp file
// from tools that skip dot names.
func (w *Writer) WriteTemp(dir string, pattern string, data []byte, perm os.FileMode) (string, error) {
	temp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", tracerr.Errorf("%w: %w", ErrMakeTemp, err)
	}
	_, writeErr := temp.Write(data)
	chmodErr := temp.Chmod(perm)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if err := errors.Join(writeErr, chmodErr, syncErr, closeErr); err != nil {
		// os.Remove, not Remove: tracerr.Wrap would take the stack trace of the error of Remove from
		// inside the join, and the stack trace must start in WriteTemp.
		removeErr := os.Remove(temp.Name())
		return "", tracerr.Wrap(errors.Join(err, removeErr))
	}
	return temp.Name(), nil
}

// LinkFirstFree hard links the file temp to the first free name in dir and returns the path it
// made. It tries base+ext, then base-2+ext, base-3+ext and so on up to base-<limit>+ext. A link never
// replaces a file and never follows a link at the target name, so a name that holds a file, a folder
// or a link counts as taken. When every name up to limit is taken, or limit is below 1, it gives
// ErrNoFreeName. Any other link error stops at once, for example one that matches os.ErrNotExist
// when dir is missing.
//
// base+ext must be a plain file name: an empty name, ".", ".." or a name with a path separator
// gives an error that matches os.ErrInvalid, and nothing is linked. Every link error, this one too,
// is an *os.LinkError with temp as Old; for a refused name, New is base+ext as given.
//
// LinkFirstFree does not remove temp: the caller owns it. Written with WriteTemp in the same folder
// (a hard link works only within one file system), the new file never shows up half written and
// never overwrites another one.
func (w *Writer) LinkFirstFree(temp string, dir string, base string, ext string, limit int) (string, error) {
	first := base + ext
	plainName := first != "" && first != "." && first != ".." && !strings.ContainsRune(first, filepath.Separator)
	if !plainName {
		return "", tracerr.Wrap(&os.LinkError{Op: "link", Old: temp, New: first, Err: os.ErrInvalid})
	}
	for counter := 1; counter <= limit; counter++ {
		name := first
		if counter > 1 {
			name = fmt.Sprintf("%s-%d%s", base, counter, ext)
		}
		path := filepath.Join(dir, name)
		err := os.Link(temp, path)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", tracerr.Wrap(err)
		}
		return path, nil
	}
	return "", tracerr.Wrap(ErrNoFreeName)
}

// Remove wraps os.Remove only to add a tracerr stack trace to the error, so a failed cleanup
// shows where it happened, like the errors of the other helpers in this package.
func (w *Writer) Remove(path string) error {
	if err := os.Remove(path); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

// CreateFile opens the file at path for reading and writing through the Opener, with
// os.O_RDWR|os.O_CREATE. With a Reader as the Opener (see [Reader.OpenFile]), a missing file is made
// with mode 644, less the bits the umask of the process takes off, and a file that is there is
// opened as it is, with its content and mode kept; no folder is made: a missing folder gives an
// error that matches os.ErrNotExist, and nothing is made. The caller closes the file.
func (w *Writer) CreateFile(path string) (*os.File, error) {
	file, err := w.opener.OpenFile(path, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	return file, nil
}

// AppendLine appends line to the end of the file at path. It opens the file through the Opener with
// os.O_APPEND|os.O_WRONLY|os.O_CREATE, so with a Reader as the Opener (see [Reader.OpenFile]) a
// missing file is made and no folder is. It writes line as given and adds no newline: end it with
// \n.
//
// The file is opened with os.O_APPEND and the line goes out in one write, so on a local file system
// lines of processes that append to the same file at the same time do not mix. This does not hold
// on a network file system such as NFS, or for a line longer than 1 GiB, which Go writes in parts.
func (w *Writer) AppendLine(path string, line string) error {
	file, err := w.opener.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE)
	if err != nil {
		return tracerr.Wrap(err)
	}
	_, writeErr := file.WriteString(line)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}
