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

// tailChunk is how many bytes are read per step from the end of the file.
const tailChunk = 64 << 10

// Reader opens files for reading and reads their last lines. It holds no state.
type Reader struct{}

// NewReader returns a new Reader.
func NewReader() *Reader {
	return &Reader{}
}

// OpenFile opens the file at path for reading only, like os.Open. It never makes a file: a missing
// file gives an error that matches os.ErrNotExist, and nothing is made. The caller closes the file.
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

// LastLines returns the last n lines of the file at path, like tail -n. Lines end at \n and come
// back without it; a \r before the \n stays. A final newline does not start a new line. An empty
// file, or n of 0 or less, gives no lines (nil). A folder gives an error that matches
// syscall.EISDIR.
//
// LastLines reads the file backwards from its end, so a long file costs only its tail. It trusts
// the size the file reports: a file that reports size 0, such as a named pipe, a device or a file
// in /proc, gives no lines. LastLines waits as long as opening the file waits: on a named pipe with
// no writer it waits until a writer opens the pipe. When the file gets shorter while LastLines
// reads it, the error matches io.EOF.
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

// readTail reads the file backwards in chunks until it holds more than n newlines or the whole
// file, so a long file costs only its tail. For n of 0 or less it reads nothing. A folder gives
// syscall.EISDIR also when its size reads as 0.
func (r *Reader) readTail(file *os.File, n int) ([]byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, &os.PathError{Op: "read", Path: file.Name(), Err: syscall.EISDIR}
	}
	// Chunks are kept last first and joined once, and only a new chunk is counted, so the cost
	// grows with the tail size, not with its square.
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
