package filesystem

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strings"
	"syscall"

	"github.com/dekey/go-pkg/tracerr"
)

const tailChunk = 64 << 10

// Reader opens files and reads their last lines.
type Reader struct{}

// NewReader returns a new Reader.
func NewReader() *Reader {
	return &Reader{}
}

// OpenFile opens the file at path with flag as given; the caller closes it. os.O_CREATE makes a
// missing file with mode 644 less the umask; other missing paths give os.ErrNotExist.
func (r *Reader) OpenFile(path string, flag int) (*os.File, error) {
	var perm os.FileMode
	if flag&os.O_CREATE != 0 {
		perm = filePerm
	}

	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	return file, nil
}

// LastLines returns the last n lines of the file at path without their "\n" ("\r" stays).
// A folder gives syscall.EISDIR; a file that reports size 0, such as one in /proc, gives no lines.
func (r *Reader) LastLines(path string, n int) ([]string, error) {
	file, err := r.OpenFile(path, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	tail, readErr := r.readTail(file, n)
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, tracerr.Wrap(err)
	}
	if len(tail) == 0 {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSuffix(string(tail), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func (r *Reader) readTail(file *os.File, n int) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, &os.PathError{Op: "read", Path: file.Name(), Err: syscall.EISDIR}
	}
	// Joined once at the end: prepending each chunk would cost the tail size squared.
	var chunks [][]byte
	newlines := 0
	offset := info.Size()
	for n > 0 && offset > 0 && newlines <= n {
		size := min(tailChunk, offset)
		offset -= size
		chunk := make([]byte, size)
		if _, err := file.ReadAt(chunk, offset); err != nil {
			return nil, err
		}
		newlines += bytes.Count(chunk, []byte("\n"))
		chunks = append(chunks, chunk)
	}
	slices.Reverse(chunks)
	return slices.Concat(chunks...), nil
}
