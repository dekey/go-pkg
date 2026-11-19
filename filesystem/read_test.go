package filesystem_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	fs "github.com/dekey/go-pkg/filesystem"
	"github.com/dekey/go-pkg/tracerr"
	"github.com/stretchr/testify/require"
)

func TestReaderOpenFileUsesTheFlagAsGivenAndMakesNoFolders(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		path   func(t *testing.T, root string) string
		flag   int
		assert func(t *testing.T, path string, file *os.File, err error)
	}{
		{
			name: "read only flag on a file that is there reads its content and a write gives the bad file descriptor error",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\nb\n"), 0o600))
				return path
			},
			flag: os.O_RDONLY,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.NoError(t, err)
				got, readErr := io.ReadAll(file)
				require.NoError(t, readErr)
				require.Equal(t, "a\nb\n", string(got), "OpenFile(%q) content read through the file", path)
				_, writeErr := file.WriteString("c\n")
				require.ErrorIs(t, writeErr, syscall.EBADF, "OpenFile(%q) write error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, writeErr, &pathErr, "OpenFile(%q) write error", path)
				require.Equal(t, "write", pathErr.Op, "OpenFile(%q) write error %v", path, writeErr)
				require.NoError(t, file.Close())
				data, readFileErr := os.ReadFile(path)
				require.NoError(t, readFileErr)
				require.Equal(t, "a\nb\n", string(data), "OpenFile(%q) content after the write", path)
			},
		},
		{
			name: "read only flag on a missing file gives the not exist error from OpenFile and makes nothing",
			path: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing.txt")
			},
			flag: os.O_RDONLY,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "OpenFile(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "OpenFile(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "OpenFile(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "OpenFile(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "OpenFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"OpenFile(%q)",
					path,
				)
				require.Nil(t, file, "OpenFile(%q) file", path)
				entries, readErr := os.ReadDir(filepath.Dir(path))
				require.NoError(t, readErr)
				require.Empty(t, entries, "OpenFile(%q) made a file in the folder", path)
			},
		},
		{
			name: "O_CREATE makes the missing file in an existing folder with mode 644 and the write starts it",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				return path
			},
			flag: os.O_APPEND | os.O_WRONLY | os.O_CREATE,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.NoError(t, err)
				_, writeErr := file.WriteString("line\n")
				require.NoError(t, writeErr)
				require.NoError(t, file.Close())
				data, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				require.Equal(t, "line\n", string(data), "OpenFile(%q) content", path)
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "OpenFile(%q) file mode", path)
			},
		},
		{
			name: "write flag without O_CREATE on a missing file gives the not exist error from OpenFile and makes nothing",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				return path
			},
			flag: os.O_APPEND | os.O_WRONLY,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "OpenFile(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "OpenFile(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "OpenFile(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "OpenFile(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "OpenFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"OpenFile(%q)",
					path,
				)
				require.Nil(t, file, "OpenFile(%q) file", path)
				entries, readErr := os.ReadDir(filepath.Dir(path))
				require.NoError(t, readErr)
				require.Empty(t, entries, "OpenFile(%q) made a file in the folder", path)
			},
		},
		{
			name: "O_CREATE on a file that is there keeps its lines and its mode 600 and the append flag writes after them",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("earlier\n"), 0o600))
				return path
			},
			flag: os.O_APPEND | os.O_WRONLY | os.O_CREATE,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.NoError(t, err)
				_, writeErr := file.WriteString("line\n")
				require.NoError(t, writeErr)
				require.NoError(t, file.Close())
				data, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				require.Equal(t, "earlier\nline\n", string(data), "OpenFile(%q) content", path)
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "OpenFile(%q) file mode", path)
			},
		},
		{
			name: "truncate flag empties the file that is there before the write",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("earlier\n"), 0o600))
				return path
			},
			flag: os.O_TRUNC | os.O_WRONLY,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.NoError(t, err)
				_, writeErr := file.WriteString("line\n")
				require.NoError(t, writeErr)
				require.NoError(t, file.Close())
				data, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				require.Equal(t, "line\n", string(data), "OpenFile(%q) content", path)
			},
		},
		{
			name: "missing folder with O_CREATE gives the not exist error with a stack trace from OpenFile and makes nothing",
			path: func(_ *testing.T, root string) string {
				return filepath.Join(root, "data", "notes.txt")
			},
			flag: os.O_APPEND | os.O_WRONLY | os.O_CREATE,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "OpenFile(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "OpenFile(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "OpenFile(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "OpenFile(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "OpenFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"OpenFile(%q)",
					path,
				)
				require.Nil(t, file, "OpenFile(%q) file", path)
				entries, readErr := os.ReadDir(filepath.Dir(filepath.Dir(path)))
				require.NoError(t, readErr)
				require.Empty(t, entries, "OpenFile(%q) made a folder or file in the root folder", path)
			},
		},
		{
			name: "folder in place of the file gives the open is a directory error from OpenFile",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data")
				require.NoError(t, os.Mkdir(path, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(path, "notes.txt"), []byte("earlier\n"), 0o600))
				return path
			},
			flag: os.O_APPEND | os.O_WRONLY,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, syscall.EISDIR, "OpenFile(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "OpenFile(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "OpenFile(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "OpenFile(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "OpenFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"OpenFile(%q)",
					path,
				)
				require.Nil(t, file, "OpenFile(%q) file", path)
				data, readErr := os.ReadFile(filepath.Join(path, "notes.txt"))
				require.NoError(t, readErr)
				require.Equal(t, "earlier\n", string(data), "OpenFile(%q) content of the file in the folder", path)
			},
		},
		{
			name: "file in place of the folder gives the not a directory error with a stack trace from OpenFile",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "file"), nil, 0o600))
				return filepath.Join(root, "file", "data", "notes.txt")
			},
			flag: os.O_APPEND | os.O_WRONLY,
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, syscall.ENOTDIR, "OpenFile(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "OpenFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"OpenFile(%q)",
					path,
				)
				require.Nil(t, file, "OpenFile(%q) file", path)
				data, readErr := os.ReadFile(filepath.Dir(filepath.Dir(path)))
				require.NoError(t, readErr)
				require.Empty(t, data, "OpenFile(%q) wrote to the file in place of the folder", path)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.path(t, t.TempDir())
			reader := fs.NewReader()
			file, err := reader.OpenFile(path, tc.flag)
			tc.assert(t, path, file, err)
		})
	}
}

func TestLastLinesGivesTheTailOfTheFileLikeTailN(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		path   func(t *testing.T, root string) string
		n      int
		assert func(t *testing.T, path string, got []string, err error)
	}{
		{
			name: "file with fewer lines than n gives all its lines",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\nb\n"), 0o600))
				return path
			},
			n: 5,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{"a", "b"}, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "final newline does not start a new line",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\nb\nc\n"), 0o600))
				return path
			},
			n: 2,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{"b", "c"}, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "last line without a final newline is kept",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\nb\nc"), 0o600))
				return path
			},
			n: 2,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{"b", "c"}, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "blank line inside the tail is kept as an empty line",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\n\nb\n"), 0o600))
				return path
			},
			n: 3,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{"a", "", "b"}, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "empty file gives no lines like tail",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, nil, 0o600))
				return path
			},
			n: 3,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Nil(t, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "file of one newline gives one empty line",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("\n"), 0o600))
				return path
			},
			n: 3,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{""}, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "n of 0 gives no lines like tail -n 0",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\nb\n"), 0o600))
				return path
			},
			n: 0,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Nil(t, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "negative n gives no lines and no panic",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\nb\n"), 0o600))
				return path
			},
			n: -1,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Nil(t, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "line longer than one read chunk is returned whole",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("a", 70000)+"\nb\n"), 0o600))
				return path
			},
			n: 2,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{strings.Repeat("a", 70000), "b"}, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "carriage return before the newline stays in the line like tail",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte("a\r\nb\r\n"), 0o600))
				return path
			},
			n: 2,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				require.Equal(t, []string{"a\r", "b\r"}, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "empty folder in place of the file with n of 0 gives the read is a directory error",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data")
				require.NoError(t, os.Mkdir(path, 0o755))
				return path
			},
			n: 0,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.ErrorIs(t, err, syscall.EISDIR, "LastLines(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "LastLines(%q) error", path)
				require.Equal(t, "read", pathErr.Op, "LastLines(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "LastLines(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LastLines(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).LastLines",
					frames[0].Func,
					"LastLines(%q)",
					path,
				)
				require.Nil(t, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "file over several read chunks gives the last n lines, also lines across a chunk border",
			path: func(t *testing.T, root string) string {
				lines := make([]string, 0, 1000)
				for i := range 1000 {
					lines = append(lines, strconv.Itoa(10000+i)+strings.Repeat("x", 295))
				}
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
				return path
			},
			n: 400,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				want := make([]string, 0, 400)
				for i := 600; i < 1000; i++ {
					want = append(want, strconv.Itoa(10000+i)+strings.Repeat("x", 295))
				}
				require.Equal(t, want, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "n larger than a file over several read chunks gives every line from the start",
			path: func(t *testing.T, root string) string {
				lines := make([]string, 0, 1000)
				for i := range 1000 {
					lines = append(lines, strconv.Itoa(10000+i)+strings.Repeat("x", 295))
				}
				path := filepath.Join(root, "file.txt")
				require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
				return path
			},
			n: 2000,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.NoError(t, err)
				want := make([]string, 0, 1000)
				for i := range 1000 {
					want = append(want, strconv.Itoa(10000+i)+strings.Repeat("x", 295))
				}
				require.Equal(t, want, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "missing file gives the not exist error with a stack trace from OpenFile and no lines",
			path: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing.txt")
			},
			n: 3,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "LastLines(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LastLines(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"LastLines(%q)",
					path,
				)
				require.Nil(t, got, "LastLines(%q) lines", path)
			},
		},
		{
			name: "folder in place of the file gives the read is a directory error from LastLines and no lines",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data")
				require.NoError(t, os.Mkdir(path, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(path, "file.txt"), []byte("a\n"), 0o600))
				return path
			},
			n: 3,
			assert: func(t *testing.T, path string, got []string, err error) {
				require.ErrorIs(t, err, syscall.EISDIR, "LastLines(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "LastLines(%q) error", path)
				require.Equal(t, "read", pathErr.Op, "LastLines(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "LastLines(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LastLines(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).LastLines",
					frames[0].Func,
					"LastLines(%q)",
					path,
				)
				require.Nil(t, got, "LastLines(%q) lines", path)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.path(t, t.TempDir())
			reader := fs.NewReader()
			got, err := reader.LastLines(path, tc.n)
			tc.assert(t, path, got, err)
		})
	}
}

func ExampleReader_LastLines() {
	dir, err := os.MkdirTemp("", "example")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Println(err)
		}
	}()
	path := filepath.Join(dir, "events.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		fmt.Println(err)
		return
	}
	reader := fs.NewReader()
	lines, err := reader.LastLines(path, 2)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	// Output:
	// three
	// four
}
