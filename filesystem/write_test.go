package filesystem_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	fs "github.com/dekey/go-pkg/filesystem"
	"github.com/dekey/go-pkg/tracerr"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	syscall.Umask(0o002)
	m.Run()
}

func TestWriteTempWritesDataWithExactModeToNewFileInDir(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		dir    func(root string) string
		perm   os.FileMode
		assert func(t *testing.T, dir string, got string, err error)
	}{
		{
			name: "temp file gets the data, mode 644 and a dot name made from the pattern in dir",
			dir: func(root string) string {
				return root
			},
			perm: 0o644,
			assert: func(t *testing.T, dir string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, dir, filepath.Dir(got), "WriteTemp() folder of %q", got)
				random, found := strings.CutPrefix(filepath.Base(got), ".report.txt.")
				require.True(t, found, "WriteTemp(%q) name of %q does not start with the pattern", dir, got)
				require.NotEmpty(t, random, "WriteTemp(%q) name of %q has no random part", dir, got)
				require.NotEqual(t, "*", random, "WriteTemp(%q) name of %q keeps the * of the pattern", dir, got)
				info, statErr := os.Stat(got)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "WriteTemp() mode of %q", got)
				data, readErr := os.ReadFile(got)
				require.NoError(t, readErr)
				require.Equal(t, "line\n", string(data), "WriteTemp() content of %q", got)
			},
		},
		{
			name: "mode 666 is set in full, not the os.CreateTemp mode 600 and not 664 from the umask 002",
			dir: func(root string) string {
				return root
			},
			perm: 0o666,
			assert: func(t *testing.T, _ string, got string, err error) {
				require.NoError(t, err)
				info, statErr := os.Stat(got)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o666), info.Mode().Perm(), "WriteTemp() mode of %q", got)
			},
		},
		{
			name: "mode 400 narrows the new temp file and the data is still written",
			dir: func(root string) string {
				return root
			},
			perm: 0o400,
			assert: func(t *testing.T, _ string, got string, err error) {
				require.NoError(t, err)
				info, statErr := os.Stat(got)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o400), info.Mode().Perm(), "WriteTemp() mode of %q", got)
				data, readErr := os.ReadFile(got)
				require.NoError(t, readErr)
				require.Equal(t, "line\n", string(data), "WriteTemp() content of %q", got)
			},
		},
		{
			name: "missing folder gives ErrMakeTemp with a stack trace from WriteTemp and no path",
			dir: func(root string) string {
				return filepath.Join(root, "missing")
			},
			perm: 0o644,
			assert: func(t *testing.T, dir string, got string, err error) {
				require.ErrorIs(t, err, fs.ErrMakeTemp, "WriteTemp(%q) error", dir)
				require.ErrorIs(t, err, os.ErrNotExist, "WriteTemp(%q) error", dir)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "WriteTemp(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).WriteTemp",
					frames[0].Func,
					"WriteTemp(%q)",
					dir,
				)
				require.Empty(t, got, "WriteTemp(%q) path", dir)
				require.NoDirExists(t, dir, "WriteTemp(%q) made the missing folder", dir)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := tc.dir(t.TempDir())
			writer := fs.NewWriter(fs.NewReader())
			got, err := writer.WriteTemp(dir, ".report.txt.*", []byte("line\n"), tc.perm)
			tc.assert(t, dir, got, err)
		})
	}
}

func TestRemoveRemovesOnlyTheGivenFile(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		setup  func(t *testing.T, path string)
		assert func(t *testing.T, path string, err error)
	}{
		{
			name: "temp file that was not renamed is removed and the file it was meant to replace stays",
			setup: func(t *testing.T, path string) {
				target := filepath.Join(filepath.Dir(path), "report.txt")
				require.NoError(t, os.WriteFile(target, []byte("old\n"), 0o600))
				require.NoError(t, os.WriteFile(path, []byte("new\n"), 0o600))
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				require.NoFileExists(t, path)
				target := filepath.Join(filepath.Dir(path), "report.txt")
				data, readErr := os.ReadFile(target)
				require.NoError(t, readErr)
				require.Equal(t, "old\n", string(data), "Remove(%q) content of %q", path, target)
			},
		},
		{
			name:  "missing temp file gives the not exist error with a stack trace from Remove",
			setup: func(_ *testing.T, _ string) {},
			assert: func(t *testing.T, path string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "Remove(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "Remove(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).Remove",
					frames[0].Func,
					"Remove(%q)",
					path,
				)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), ".report.txt.1")
			tc.setup(t, path)
			writer := fs.NewWriter(fs.NewReader())
			err := writer.Remove(path)
			tc.assert(t, path, err)
		})
	}
}

func TestLinkFirstFreeLinksTempToTheFirstFreeNameAndNeverReplacesAFile(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		base   string
		ext    string
		limit  int
		setup  func(t *testing.T, dir string)
		assert func(t *testing.T, temp string, dir string, got string, err error)
	}{
		{
			name:  "free first name becomes a hard link of temp with its content",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(dir, "report.txt"), got, "LinkFirstFree(%q) path", dir)
				data, readErr := os.ReadFile(got)
				require.NoError(t, readErr)
				require.Equal(t, "new\n", string(data), "LinkFirstFree(%q) content of %q", dir, got)
				gotInfo, gotErr := os.Stat(got)
				require.NoError(t, gotErr)
				tempInfo, tempErr := os.Stat(temp)
				require.NoError(t, tempErr)
				require.True(
					t,
					os.SameFile(tempInfo, gotInfo),
					"LinkFirstFree(%q) %q is not a link of %q",
					dir,
					got,
					temp,
				)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Len(t, entries, 1, "LinkFirstFree(%q) files in the folder", dir)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "taken first name gives base-2 and the taken file keeps its content",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "report.txt"), []byte("old\n"), 0o600))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(dir, "report-2.txt"), got, "LinkFirstFree(%q) path", dir)
				data, readErr := os.ReadFile(got)
				require.NoError(t, readErr)
				require.Equal(t, "new\n", string(data), "LinkFirstFree(%q) content of %q", dir, got)
				taken := filepath.Join(dir, "report.txt")
				takenData, takenErr := os.ReadFile(taken)
				require.NoError(t, takenErr)
				require.Equal(t, "old\n", string(takenData), "LinkFirstFree(%q) content of %q", dir, taken)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "gap in the counters is used before a later free name",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "report.txt"), []byte("old\n"), 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "report-3.txt"), []byte("old\n"), 0o600))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(dir, "report-2.txt"), got, "LinkFirstFree(%q) path", dir)
				taken := filepath.Join(dir, "report-3.txt")
				takenData, takenErr := os.ReadFile(taken)
				require.NoError(t, takenErr)
				require.Equal(t, "old\n", string(takenData), "LinkFirstFree(%q) content of %q", dir, taken)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "folder at the first name counts as taken and nothing is linked into it",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "report.txt"), 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(dir, "report-2.txt"), got, "LinkFirstFree(%q) path", dir)
				taken := filepath.Join(dir, "report.txt")
				takenEntries, takenErr := os.ReadDir(taken)
				require.NoError(t, takenErr)
				require.Empty(t, takenEntries, "LinkFirstFree(%q) files in the folder %q", dir, taken)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "dangling link at the first name counts as taken and its target is not made",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
				target := filepath.Join(filepath.Dir(dir), "target")
				require.NoError(t, os.Symlink(target, filepath.Join(dir, "report.txt")))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(dir, "report-2.txt"), got, "LinkFirstFree(%q) path", dir)
				target := filepath.Join(filepath.Dir(dir), "target")
				_, targetErr := os.Lstat(target)
				require.ErrorIs(t, targetErr, os.ErrNotExist, "LinkFirstFree(%q) made the link target %q", dir, target)
				taken := filepath.Join(dir, "report.txt")
				takenInfo, takenErr := os.Lstat(taken)
				require.NoError(t, takenErr)
				require.Equal(t, os.ModeSymlink, takenInfo.Mode().Type(), "LinkFirstFree(%q) type of %q", dir, taken)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "98 taken names leave the last name at the limit 99",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "report.txt"), nil, 0o600))
				for counter := 2; counter <= 98; counter++ {
					name := "report-" + strconv.Itoa(counter) + ".txt"
					require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
				}
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(dir, "report-99.txt"), got, "LinkFirstFree(%q) path", dir)
				data, readErr := os.ReadFile(got)
				require.NoError(t, readErr)
				require.Equal(t, "new\n", string(data), "LinkFirstFree(%q) content of %q", dir, got)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "all 99 names up to the limit taken give ErrNoFreeName traced to LinkFirstFree and no new file",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "report.txt"), nil, 0o600))
				for counter := 2; counter <= 99; counter++ {
					name := "report-" + strconv.Itoa(counter) + ".txt"
					require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
				}
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, fs.ErrNoFreeName, "LinkFirstFree(%q) error", dir)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LinkFirstFree(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).LinkFirstFree",
					frames[0].Func,
					"LinkFirstFree(%q)",
					dir,
				)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				require.NoFileExists(t, filepath.Join(dir, "report-100.txt"))
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Len(t, entries, 99, "LinkFirstFree(%q) files in the folder", dir)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "limit 1 tries only base and ext and gives ErrNoFreeName when it is taken",
			base:  "report",
			ext:   ".txt",
			limit: 1,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "report.txt"), []byte("old\n"), 0o600))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, fs.ErrNoFreeName, "LinkFirstFree(%q) error", dir)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LinkFirstFree(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).LinkFirstFree",
					frames[0].Func,
					"LinkFirstFree(%q)",
					dir,
				)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				require.NoFileExists(t, filepath.Join(dir, "report-2.txt"))
				taken := filepath.Join(dir, "report.txt")
				takenData, takenErr := os.ReadFile(taken)
				require.NoError(t, takenErr)
				require.Equal(t, "old\n", string(takenData), "LinkFirstFree(%q) content of %q", dir, taken)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "limit 0 gives ErrNoFreeName traced to LinkFirstFree and links nothing",
			base:  "report",
			ext:   ".txt",
			limit: 0,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, fs.ErrNoFreeName, "LinkFirstFree(%q) error", dir)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LinkFirstFree(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).LinkFirstFree",
					frames[0].Func,
					"LinkFirstFree(%q)",
					dir,
				)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the folder", dir)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "negative limit gives ErrNoFreeName traced to LinkFirstFree and links nothing",
			base:  "report",
			ext:   ".txt",
			limit: -1,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, fs.ErrNoFreeName, "LinkFirstFree(%q) error", dir)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LinkFirstFree(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).LinkFirstFree",
					frames[0].Func,
					"LinkFirstFree(%q)",
					dir,
				)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the folder", dir)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "missing folder gives the link not exist error at once and not ErrNoFreeName",
			base:  "report",
			ext:   ".txt",
			limit: 99,
			setup: func(_ *testing.T, _ string) {},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "LinkFirstFree(%q) error", dir)
				require.NotErrorIs(t, err, fs.ErrNoFreeName, "LinkFirstFree(%q) error", dir)
				var linkErr *os.LinkError
				require.ErrorAs(t, err, &linkErr, "LinkFirstFree(%q) error", dir)
				require.Equal(t, "link", linkErr.Op, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, temp, linkErr.Old, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, filepath.Join(dir, "report.txt"), linkErr.New, "LinkFirstFree(%q) error %v", dir, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LinkFirstFree(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).LinkFirstFree",
					frames[0].Func,
					"LinkFirstFree(%q)",
					dir,
				)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				require.NoDirExists(t, dir, "LinkFirstFree(%q) made the missing folder", dir)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "base with a parent folder part gives the invalid argument error and links nothing outside dir",
			base:  "../escape",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				require.NotErrorIs(t, err, fs.ErrNoFreeName, "LinkFirstFree(%q) error", dir)
				var linkErr *os.LinkError
				require.ErrorAs(t, err, &linkErr, "LinkFirstFree(%q) error", dir)
				require.Equal(t, "link", linkErr.Op, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, temp, linkErr.Old, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, "../escape.txt", linkErr.New, "LinkFirstFree(%q) error %v", dir, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LinkFirstFree(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).LinkFirstFree",
					frames[0].Func,
					"LinkFirstFree(%q)",
					dir,
				)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				escaped := filepath.Join(filepath.Dir(dir), "escape.txt")
				require.NoFileExists(t, escaped, "LinkFirstFree(%q) linked outside dir", dir)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "base with a subfolder part gives the invalid argument error and links nothing in the subfolder",
			base:  "sub/report",
			ext:   ".txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				var linkErr *os.LinkError
				require.ErrorAs(t, err, &linkErr, "LinkFirstFree(%q) error", dir)
				require.Equal(t, "link", linkErr.Op, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, temp, linkErr.Old, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, "sub/report.txt", linkErr.New, "LinkFirstFree(%q) error %v", dir, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "LinkFirstFree(%q) error %v has no stack trace", dir, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).LinkFirstFree",
					frames[0].Func,
					"LinkFirstFree(%q)",
					dir,
				)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(filepath.Join(dir, "sub"))
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the subfolder", dir)
				tempData, tempReadErr := os.ReadFile(temp)
				require.NoError(t, tempReadErr)
				require.Equal(t, "new\n", string(tempData), "LinkFirstFree(%q) content of temp %q", dir, temp)
			},
		},
		{
			name:  "ext with a folder part gives the invalid argument error before any link is tried",
			base:  "report",
			ext:   "/x.txt",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				var linkErr *os.LinkError
				require.ErrorAs(t, err, &linkErr, "LinkFirstFree(%q) error", dir)
				require.Equal(t, temp, linkErr.Old, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, "report/x.txt", linkErr.New, "LinkFirstFree(%q) error %v", dir, err)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
			},
		},
		{
			name:  "base of two dots gives the invalid argument error and links nothing",
			base:  "..",
			ext:   "",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, _ string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the folder", dir)
			},
		},
		{
			name:  "base of one dot gives the invalid argument error and links nothing",
			base:  ".",
			ext:   "",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				var linkErr *os.LinkError
				require.ErrorAs(t, err, &linkErr, "LinkFirstFree(%q) error", dir)
				require.Equal(t, temp, linkErr.Old, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, ".", linkErr.New, "LinkFirstFree(%q) error %v", dir, err)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the folder", dir)
			},
		},
		{
			name:  "empty base and ext give the invalid argument error and link nothing",
			base:  "",
			ext:   "",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, _ string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the folder", dir)
			},
		},
		{
			name:  "base of only a slash gives the invalid argument error and links nothing in dir",
			base:  "/",
			ext:   "",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				var linkErr *os.LinkError
				require.ErrorAs(t, err, &linkErr, "LinkFirstFree(%q) error", dir)
				require.Equal(t, temp, linkErr.Old, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, "/", linkErr.New, "LinkFirstFree(%q) error %v", dir, err)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the folder", dir)
			},
		},
		{
			name:  "empty base with an ext of only a slash gives the invalid argument error and links nothing in dir",
			base:  "",
			ext:   "/",
			limit: 99,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
			assert: func(t *testing.T, temp string, dir string, got string, err error) {
				require.ErrorIs(t, err, os.ErrInvalid, "LinkFirstFree(%q) error", dir)
				var linkErr *os.LinkError
				require.ErrorAs(t, err, &linkErr, "LinkFirstFree(%q) error", dir)
				require.Equal(t, temp, linkErr.Old, "LinkFirstFree(%q) error %v", dir, err)
				require.Equal(t, "/", linkErr.New, "LinkFirstFree(%q) error %v", dir, err)
				require.Empty(t, got, "LinkFirstFree(%q) path", dir)
				entries, dirErr := os.ReadDir(dir)
				require.NoError(t, dirErr)
				require.Empty(t, entries, "LinkFirstFree(%q) files in the folder", dir)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			temp := filepath.Join(root, ".report.txt.1")
			require.NoError(t, os.WriteFile(temp, []byte("new\n"), 0o600))
			dir := filepath.Join(root, "data")
			tc.setup(t, dir)
			writer := fs.NewWriter(fs.NewReader())
			got, err := writer.LinkFirstFree(temp, dir, tc.base, tc.ext, tc.limit)
			tc.assert(t, temp, dir, got, err)
		})
	}
}

func TestCreateFileOpensForReadAndWriteAndMakesOnlyTheMissingFile(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		path   func(t *testing.T, root string) string
		assert func(t *testing.T, path string, file *os.File, err error)
	}{
		{
			name: "missing file is made empty with mode 644 and the returned file writes and reads it back",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				return path
			},
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.NoError(t, err)
				data, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				require.Empty(t, data, "CreateFile(%q) content of the new file", path)
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "CreateFile(%q) file mode", path)
				_, writeErr := file.WriteString("line\n")
				require.NoError(t, writeErr)
				_, seekErr := file.Seek(0, io.SeekStart)
				require.NoError(t, seekErr)
				got, readBackErr := io.ReadAll(file)
				require.NoError(t, readBackErr)
				require.Equal(t, "line\n", string(got), "CreateFile(%q) content read back through the file", path)
				require.NoError(t, file.Close())
			},
		},
		{
			name: "file that is there keeps its content and its mode 600 and the returned file reads and writes it",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("earlier\n"), 0o600))
				return path
			},
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.NoError(t, err)
				got, readErr := io.ReadAll(file)
				require.NoError(t, readErr)
				require.Equal(t, "earlier\n", string(got), "CreateFile(%q) content read through the file", path)
				_, writeErr := file.WriteString("line\n")
				require.NoError(t, writeErr)
				require.NoError(t, file.Close())
				data, readFileErr := os.ReadFile(path)
				require.NoError(t, readFileErr)
				require.Equal(t, "earlier\nline\n", string(data), "CreateFile(%q) content", path)
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "CreateFile(%q) file mode", path)
			},
		},
		{
			name: "missing folder gives the not exist error with a stack trace from OpenFile and makes nothing",
			path: func(_ *testing.T, root string) string {
				return filepath.Join(root, "data", "notes.txt")
			},
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "CreateFile(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "CreateFile(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "CreateFile(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "CreateFile(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "CreateFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"CreateFile(%q)",
					path,
				)
				require.Nil(t, file, "CreateFile(%q) file", path)
				entries, readErr := os.ReadDir(filepath.Dir(filepath.Dir(path)))
				require.NoError(t, readErr)
				require.Empty(t, entries, "CreateFile(%q) made a folder or file in the root folder", path)
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
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, syscall.EISDIR, "CreateFile(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "CreateFile(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "CreateFile(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "CreateFile(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "CreateFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"CreateFile(%q)",
					path,
				)
				require.Nil(t, file, "CreateFile(%q) file", path)
				data, readErr := os.ReadFile(filepath.Join(path, "notes.txt"))
				require.NoError(t, readErr)
				require.Equal(t, "earlier\n", string(data), "CreateFile(%q) content of the file in the folder", path)
			},
		},
		{
			name: "file in place of the folder gives the not a directory error with a stack trace from OpenFile",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "file"), nil, 0o600))
				return filepath.Join(root, "file", "data", "notes.txt")
			},
			assert: func(t *testing.T, path string, file *os.File, err error) {
				require.ErrorIs(t, err, syscall.ENOTDIR, "CreateFile(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "CreateFile(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"CreateFile(%q)",
					path,
				)
				require.Nil(t, file, "CreateFile(%q) file", path)
				data, readErr := os.ReadFile(filepath.Dir(filepath.Dir(path)))
				require.NoError(t, readErr)
				require.Empty(t, data, "CreateFile(%q) wrote to the file in place of the folder", path)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.path(t, t.TempDir())
			writer := fs.NewWriter(fs.NewReader())
			file, err := writer.CreateFile(path)
			tc.assert(t, path, file, err)
		})
	}
}

func TestAppendLineAddsTheLineAsGivenToTheEndInOneWrite(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name   string
		path   func(t *testing.T, root string) string
		run    func(writer *fs.Writer, path string) error
		assert func(t *testing.T, path string, err error)
	}{
		{
			name: "lines are added in order after the content that is there and no newline is added",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("earlier\n"), 0o600))
				return path
			},
			run: func(writer *fs.Writer, path string) error {
				if err := writer.AppendLine(path, "first\n"); err != nil {
					return err
				}
				if err := writer.AppendLine(path, "second"); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				data, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				require.Equal(t, "earlier\nfirst\nsecond", string(data), "AppendLine(%q) content", path)
			},
		},
		{
			name: "missing file in an existing folder is made with mode 644 and holds exactly the line",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				return path
			},
			run: func(writer *fs.Writer, path string) error {
				if err := writer.AppendLine(path, "line\n"); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				data, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				require.Equal(t, "line\n", string(data), "AppendLine(%q) content", path)
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "AppendLine(%q) file mode", path)
			},
		},
		{
			name: "file that is there keeps its mode 600 when the line is added",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "data", "notes.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, []byte("earlier\n"), 0o600))
				return path
			},
			run: func(writer *fs.Writer, path string) error {
				if err := writer.AppendLine(path, "line\n"); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "AppendLine(%q) file mode", path)
			},
		},
		{
			name: "missing folder gives the not exist error with a stack trace from OpenFile and makes nothing",
			path: func(_ *testing.T, root string) string {
				return filepath.Join(root, "data", "notes.txt")
			},
			run: func(writer *fs.Writer, path string) error {
				if err := writer.AppendLine(path, "line\n"); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "AppendLine(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "AppendLine(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "AppendLine(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "AppendLine(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "AppendLine(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"AppendLine(%q)",
					path,
				)
				entries, readErr := os.ReadDir(filepath.Dir(filepath.Dir(path)))
				require.NoError(t, readErr)
				require.Empty(t, entries, "AppendLine(%q) made a folder or file in the root folder", path)
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
			run: func(writer *fs.Writer, path string) error {
				if err := writer.AppendLine(path, "line\n"); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.ErrorIs(t, err, syscall.EISDIR, "AppendLine(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "AppendLine(%q) error", path)
				require.Equal(t, "open", pathErr.Op, "AppendLine(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "AppendLine(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "AppendLine(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"AppendLine(%q)",
					path,
				)
				data, readErr := os.ReadFile(filepath.Join(path, "notes.txt"))
				require.NoError(t, readErr)
				require.Equal(t, "earlier\n", string(data), "AppendLine(%q) content of the file in the folder", path)
			},
		},
		{
			name: "file in place of the folder gives the not a directory error with a stack trace from OpenFile",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "file"), nil, 0o600))
				return filepath.Join(root, "file", "data", "notes.txt")
			},
			run: func(writer *fs.Writer, path string) error {
				if err := writer.AppendLine(path, "line\n"); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.ErrorIs(t, err, syscall.ENOTDIR, "AppendLine(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "AppendLine(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Reader).OpenFile",
					frames[0].Func,
					"AppendLine(%q)",
					path,
				)
				data, readErr := os.ReadFile(filepath.Dir(filepath.Dir(path)))
				require.NoError(t, readErr)
				require.Empty(t, data, "AppendLine(%q) wrote to the file in place of the folder", path)
			},
		},
		{
			name: "full device gives the no space write error with a stack trace from AppendLine",
			path: func(t *testing.T, _ string) string {
				if _, err := os.Stat("/dev/full"); err != nil {
					t.Skip("no /dev/full on this system")
				}
				return "/dev/full"
			},
			run: func(writer *fs.Writer, path string) error {
				if err := writer.AppendLine(path, "line\n"); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.ErrorIs(t, err, syscall.ENOSPC, "AppendLine(%q) error", path)
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr, "AppendLine(%q) error", path)
				require.Equal(t, "write", pathErr.Op, "AppendLine(%q) error %v", path, err)
				require.Equal(t, path, pathErr.Path, "AppendLine(%q) error %v", path, err)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "AppendLine(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Writer).AppendLine",
					frames[0].Func,
					"AppendLine(%q)",
					path,
				)
			},
		},
		{
			name: "20 writers appending 100 lines each at once give only whole lines and 100 per writer",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "shared", "lines.txt")
				require.NoError(t, os.Mkdir(filepath.Dir(path), 0o755))
				return path
			},
			run: func(writer *fs.Writer, path string) error {
				payload := strings.Repeat("x", 512)
				errs := make([]error, 20)
				var wg sync.WaitGroup
				for worker := range 20 {
					wg.Go(func() {
						for line := range 100 {
							text := fmt.Sprintf("writer %02d line %03d %s\n", worker, line, payload)
							if err := writer.AppendLine(path, text); err != nil {
								errs[worker] = err
								return
							}
						}
					})
				}
				wg.Wait()
				if err := errors.Join(errs...); err != nil {
					return err
				}
				return nil
			},
			assert: func(t *testing.T, path string, err error) {
				require.NoError(t, err)
				data, readErr := os.ReadFile(path)
				require.NoError(t, readErr)
				content := string(data)
				require.True(t, strings.HasSuffix(content, "\n"), "AppendLine(%q) last line has no newline", path)
				wholeLine := regexp.MustCompile(`^writer (\d{2}) line \d{3} x{512}$`)
				counts := map[string]int{}
				distinct := map[string]bool{}
				for line := range strings.SplitSeq(strings.TrimSuffix(content, "\n"), "\n") {
					match := wholeLine.FindStringSubmatch(line)
					require.NotNil(t, match, "AppendLine(%q) gave a broken line %q", path, line)
					counts[match[1]]++
					distinct[line] = true
				}
				want := map[string]int{}
				for writer := range 20 {
					want[fmt.Sprintf("%02d", writer)] = 100
				}
				require.Equal(t, want, counts, "AppendLine(%q) lines per writer", path)
				require.Len(t, distinct, 2000, "AppendLine(%q) distinct lines", path)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.path(t, t.TempDir())
			writer := fs.NewWriter(fs.NewReader())
			err := tc.run(writer, path)
			tc.assert(t, path, err)
		})
	}
}

func ExampleWriter_WriteTemp() {
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
	w := fs.NewWriter(fs.NewReader())
	temp, err := w.WriteTemp(dir, ".config.json.*", []byte(`{"debug":true}`), 0o644)
	if err != nil {
		fmt.Println(err)
		return
	}
	path := filepath.Join(dir, "config.json")
	if err := os.Rename(temp, path); err != nil {
		removeErr := w.Remove(temp)
		fmt.Println(errors.Join(err, removeErr))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data))
	// Output: {"debug":true}
}

func ExampleWriter_LinkFirstFree() {
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
	if err := os.WriteFile(filepath.Join(dir, "report.txt"), []byte("old\n"), 0o600); err != nil {
		fmt.Println(err)
		return
	}
	w := fs.NewWriter(fs.NewReader())
	temp, err := w.WriteTemp(dir, ".report.*", []byte("new\n"), 0o644)
	if err != nil {
		fmt.Println(err)
		return
	}
	path, err := w.LinkFirstFree(temp, dir, "report", ".txt", 99)
	removeErr := w.Remove(temp)
	if err != nil {
		fmt.Println(errors.Join(err, removeErr))
		return
	}
	if removeErr != nil {
		fmt.Println("temp file left:", removeErr)
	}
	fmt.Println(filepath.Base(path))
	// Output: report-2.txt
}

func ExampleWriter_AppendLine() {
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
	path := filepath.Join(dir, "history", "events.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Println(err)
		return
	}
	w := fs.NewWriter(fs.NewReader())
	for _, line := range []string{"user signed in\n", "user signed out\n"} {
		if err := w.AppendLine(path, line); err != nil {
			fmt.Println(err)
			return
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(string(data))
	// Output:
	// user signed in
	// user signed out
}
