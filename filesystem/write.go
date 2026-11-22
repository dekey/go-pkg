package filesystem

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/dekey/go-pkg/tracerr"
)

var (
	ErrMakeTemp   = errors.New("make temp file")
	ErrNoFreeName = errors.New("no free name")
)

// Writer writes, links, removes, creates and appends to files. No method makes folders.
type Writer struct {
	opener Opener
}

// NewWriter returns a Writer that opens files with opener.
func NewWriter(opener Opener) *Writer {
	return &Writer{
		opener: opener,
	}
}

// WriteTemp writes data to a new file made like os.CreateTemp(dir, pattern), syncs it and returns
// its path. The file gets mode perm exactly, past the umask. A failed create gives ErrMakeTemp;
// when a later step fails, WriteTemp removes the file.
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
		// os.Remove, not w.Remove: the stack trace must start in WriteTemp.
		removeErr := os.Remove(temp.Name())
		return "", tracerr.Wrap(errors.Join(err, removeErr))
	}
	return temp.Name(), nil
}

// LinkFirstFree hard links temp into dir as the first free name of base+ext, base-2+ext, ...,
// base-<limit>+ext and returns its path. It never replaces a file and does not remove temp.
// Errors match ErrNoFreeName, os.ErrInvalid (base+ext is not a plain name) or os.ErrNotExist.
func (w *Writer) LinkFirstFree(temp string, dir string, base string, ext string, limit int) (string, error) {
	first := base + ext
	dotName := first == "" || first == "." || first == ".."
	hasSeparator := slices.ContainsFunc([]byte(first), os.IsPathSeparator)
	if dotName || hasSeparator || filepath.VolumeName(first) != "" {
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

// Remove is os.Remove with a tracerr stack trace on the error.
func (w *Writer) Remove(path string) error {
	if err := os.Remove(path); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

// CreateFile opens or makes the file at path for reading and writing, without truncating it.
// The caller closes the file.
func (w *Writer) CreateFile(path string) (*os.File, error) {
	file, err := w.opener.OpenFile(path, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	return file, nil
}

// AppendLine appends line to the file at path in one write, making the file when missing.
// It adds no newline.
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
