package filesystem_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	fs "github.com/dekey/go-pkg/filesystem"
	"github.com/dekey/go-pkg/tracerr"
	"github.com/stretchr/testify/require"
)

func TestAbsoluteKeepsPathComponents(t *testing.T) {
	testCases := []struct {
		name   string
		path   func(t *testing.T, root string) string
		assert func(t *testing.T, root string, path string, got string, err error)
	}{
		{
			name: "absolute path keeps parent components and repeated slashes",
			path: func(_ *testing.T, _ string) string {
				return "/given/link/../folder//"
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "/given/link/../folder//", got, "Absolute(%q)", path)
			},
		},
		{
			name: "absolute path is returned without requiring it to exist",
			path: func(_ *testing.T, _ string) string {
				return "/missing/folder"
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "/missing/folder", got, "Absolute(%q)", path)
			},
		},
		{
			name: "empty path gives the real working folder",
			path: func(t *testing.T, root string) string {
				t.Chdir(root)
				return ""
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root, got, "Absolute(%q)", path)
			},
		},
		{
			name: "relative path is joined to the real working folder with parent components and repeated slashes",
			path: func(t *testing.T, root string) string {
				t.Chdir(root)
				return "link/../folder//"
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root+"/link/../folder//", got, "Absolute(%q)", path)
			},
		},
		{
			name: "relative dot is joined to the real working folder without cleaning",
			path: func(t *testing.T, root string) string {
				t.Chdir(root)
				return "."
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root+"/.", got, "Absolute(%q)", path)
			},
		},
		{
			name: "relative path from the file system root is joined with one separator",
			path: func(t *testing.T, _ string) string {
				t.Chdir("/")
				return "x"
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "/x", got, "Absolute(%q) with the working folder /", path)
			},
		},
		{
			name: "absolute file path stays unchanged",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file")
				require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
				return path
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(root, "file"), got, "Absolute(%q)", path)
			},
		},
		{
			name: "dangling absolute symlink stays unchanged without resolution",
			path: func(t *testing.T, root string) string {
				link := filepath.Join(root, "link")
				require.NoError(t, os.Symlink(filepath.Join(root, "missing"), link))
				return link
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(root, "link"), got, "Absolute(%q)", path)
			},
		},
		{
			name: "relative path resolves only the symlinked working folder in front of it",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "deep"), 0o755))
				require.NoError(t, os.Symlink(filepath.Join(root, "real", "deep"), filepath.Join(root, "link")))
				t.Chdir(filepath.Join(root, "link"))
				return "../missing//"
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root+"/real/deep/../missing//", got, "Absolute(%q)", path)
			},
		},
		{
			name: "relative path from a removed working folder gives the not exist error from Real",
			path: func(t *testing.T, root string) string {
				dir := filepath.Join(root, "removed")
				require.NoError(t, os.Mkdir(dir, 0o755))
				t.Chdir(dir)
				require.NoError(t, os.Remove(dir))
				return "missing"
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "Absolute(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "Absolute(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Path).Real",
					frames[0].Func,
					"Absolute(%q)",
					path,
				)
				require.Empty(t, got, "Absolute(%q)", path)
			},
		},
		{
			name: "absolute path does not read a removed working folder",
			path: func(t *testing.T, root string) string {
				dir := filepath.Join(root, "removed")
				require.NoError(t, os.Mkdir(dir, 0o755))
				t.Chdir(dir)
				require.NoError(t, os.Remove(dir))
				return root + "/missing/../folder//"
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root+"/missing/../folder//", got, "Absolute(%q)", path)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			path := tc.path(t, root)
			fsPath := fs.NewPath()
			got, err := fsPath.Absolute(path)
			tc.assert(t, root, path, got, err)
		})
	}
}

func TestRealResolvesCleanAbsolutePaths(t *testing.T) {
	testCases := []struct {
		name   string
		path   func(t *testing.T, root string) string
		assert func(t *testing.T, root string, path string, got string, err error)
	}{
		{
			name: "folder reached through a symlink gives its real absolute path",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "sub"), 0o755))
				require.NoError(t, os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")))
				return filepath.Join(root, "link", "sub")
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(root, "real", "sub"), got, "Real(%q)", path)
			},
		},
		{
			name: "relative folder gives its real absolute path",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "sub"), 0o755))
				workDir, err := os.Getwd()
				require.NoError(t, err)
				path, err := filepath.Rel(workDir, filepath.Join(root, "real", "sub"))
				require.NoError(t, err)
				return path
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(root, "real", "sub"), got, "Real(%q)", path)
			},
		},
		{
			name: "file reached through a symlink gives the real file path",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600))
				require.NoError(t, os.Symlink(filepath.Join(root, "file"), filepath.Join(root, "link")))
				return filepath.Join(root, "link")
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(root, "file"), got, "Real(%q)", path)
			},
		},
		{
			name: "parent component is removed by name before a symlink is followed",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "deep"), 0o755))
				require.NoError(t, os.Symlink(filepath.Join(root, "real", "deep"), filepath.Join(root, "link")))
				return root + "/link/.."
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root, got, "Real(%q)", path)
			},
		},
		{
			name: "empty path gives the real working folder",
			path: func(t *testing.T, root string) string {
				t.Chdir(root)
				return ""
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root, got, "Real(%q)", path)
			},
		},
		{
			name: "relative dot gives the real working folder without a trailing component",
			path: func(t *testing.T, root string) string {
				t.Chdir(root)
				return "."
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root, got, "Real(%q)", path)
			},
		},
		{
			name: "trailing separators and dot components are cleaned",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.Mkdir(filepath.Join(root, "real"), 0o755))
				return root + "/real//.//"
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(root, "real"), got, "Real(%q)", path)
			},
		},
		{
			name: "file path without links gives the same file path",
			path: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file")
				require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
				return path
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join(root, "file"), got, "Real(%q)", path)
			},
		},
		{
			name: "relative parent component is removed by name before a symlink is followed",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "deep"), 0o755))
				require.NoError(t, os.Symlink(filepath.Join(root, "real", "deep"), filepath.Join(root, "link")))
				t.Chdir(root)
				return "link/.."
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root, got, "Real(%q)", path)
			},
		},
		{
			name: "parent component removes a dangling symlink before it is followed",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "link")))
				return root + "/link/.."
			},
			assert: func(t *testing.T, root string, path string, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, root, got, "Real(%q)", path)
			},
		},
		{
			name: "relative path from a removed working folder gives the not exist error from Real",
			path: func(t *testing.T, root string) string {
				dir := filepath.Join(root, "removed")
				require.NoError(t, os.Mkdir(dir, 0o755))
				t.Chdir(dir)
				require.NoError(t, os.Remove(dir))
				return "missing"
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "Real(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "Real(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Path).Real",
					frames[0].Func,
					"Real(%q)",
					path,
				)
				require.Empty(t, got, "Real(%q)", path)
			},
		},
		{
			name: "file used as a parent folder gives the not a directory error from Real",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600))
				return filepath.Join(root, "file", "child")
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.ErrorIs(t, err, syscall.ENOTDIR, "Real(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "Real(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Path).Real",
					frames[0].Func,
					"Real(%q)",
					path,
				)
				require.Empty(t, got, "Real(%q)", path)
			},
		},
		{
			name: "missing path gives the not exist error from Real and an empty path",
			path: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing")
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "Real(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "Real(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Path).Real",
					frames[0].Func,
					"Real(%q)",
					path,
				)
				require.Empty(t, got, "Real(%q)", path)
			},
		},
		{
			name: "dangling symlink gives the not exist error from Real and an empty path",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "link")))
				return filepath.Join(root, "link")
			},
			assert: func(t *testing.T, _ string, path string, got string, err error) {
				require.ErrorIs(t, err, os.ErrNotExist, "Real(%q) error", path)
				frames := tracerr.StackTrace(err)
				require.NotEmpty(t, frames, "Real(%q) error %v has no stack trace", path, err)
				require.Equal(
					t,
					"github.com/dekey/go-pkg/filesystem.(*Path).Real",
					frames[0].Func,
					"Real(%q)",
					path,
				)
				require.Empty(t, got, "Real(%q)", path)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			path := tc.path(t, root)
			fsPath := fs.NewPath()
			got, err := fsPath.Real(path)
			tc.assert(t, root, path, got, err)
		})
	}
}

func TestRealDirRequiresExistingAbsoluteDirectory(t *testing.T) {
	testCases := []struct {
		name   string
		path   func(t *testing.T, root string) string
		assert func(t *testing.T, root string, path string, got string, ok bool)
	}{
		{
			name: "folder reached through a symlink gives its real path",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "sub"), 0o755))
				require.NoError(t, os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")))
				return filepath.Join(root, "link", "sub")
			},
			assert: func(t *testing.T, root string, path string, got string, ok bool) {
				require.True(t, ok, "RealDir(%q) ok", path)
				require.Equal(t, filepath.Join(root, "real", "sub"), got, "RealDir(%q)", path)
			},
		},
		{
			name: "relative folder is refused",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real"), 0o755))
				t.Chdir(root)
				return "real"
			},
			assert: func(t *testing.T, _ string, path string, got string, ok bool) {
				require.False(t, ok, "RealDir(%q) ok", path)
				require.Empty(t, got, "RealDir(%q)", path)
			},
		},
		{
			name: "empty path is refused instead of using the working folder",
			path: func(t *testing.T, root string) string {
				t.Chdir(root)
				return ""
			},
			assert: func(t *testing.T, _ string, path string, got string, ok bool) {
				require.False(t, ok, "RealDir(%q) ok", path)
				require.Empty(t, got, "RealDir(%q)", path)
			},
		},
		{
			name: "relative dot is refused although it names an existing folder",
			path: func(t *testing.T, root string) string {
				t.Chdir(root)
				return "."
			},
			assert: func(t *testing.T, _ string, path string, got string, ok bool) {
				require.False(t, ok, "RealDir(%q) ok", path)
				require.Empty(t, got, "RealDir(%q)", path)
			},
		},
		{
			name: "trailing separators and dot components of an absolute folder are cleaned",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.Mkdir(filepath.Join(root, "real"), 0o755))
				return root + "/real//.//"
			},
			assert: func(t *testing.T, root string, path string, got string, ok bool) {
				require.True(t, ok, "RealDir(%q) ok", path)
				require.Equal(t, filepath.Join(root, "real"), got, "RealDir(%q)", path)
			},
		},
		{
			name: "parent component is removed by name before a folder symlink is followed",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.MkdirAll(filepath.Join(root, "real", "deep"), 0o755))
				require.NoError(t, os.Symlink(filepath.Join(root, "real", "deep"), filepath.Join(root, "link")))
				return root + "/link/.."
			},
			assert: func(t *testing.T, root string, path string, got string, ok bool) {
				require.True(t, ok, "RealDir(%q) ok", path)
				require.Equal(t, root, got, "RealDir(%q)", path)
			},
		},
		{
			name: "symlink to a file is refused",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600))
				require.NoError(t, os.Symlink(filepath.Join(root, "file"), filepath.Join(root, "link")))
				return filepath.Join(root, "link")
			},
			assert: func(t *testing.T, _ string, path string, got string, ok bool) {
				require.False(t, ok, "RealDir(%q) ok", path)
				require.Empty(t, got, "RealDir(%q)", path)
			},
		},
		{
			name: "dangling symlink is refused",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "link")))
				return filepath.Join(root, "link")
			},
			assert: func(t *testing.T, _ string, path string, got string, ok bool) {
				require.False(t, ok, "RealDir(%q) ok", path)
				require.Empty(t, got, "RealDir(%q)", path)
			},
		},
		{
			name: "missing folder is refused",
			path: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing")
			},
			assert: func(t *testing.T, _ string, path string, got string, ok bool) {
				require.False(t, ok, "RealDir(%q) ok", path)
				require.Empty(t, got, "RealDir(%q)", path)
			},
		},
		{
			name: "file is refused",
			path: func(t *testing.T, root string) string {
				require.NoError(t, os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600))
				return filepath.Join(root, "file")
			},
			assert: func(t *testing.T, _ string, path string, got string, ok bool) {
				require.False(t, ok, "RealDir(%q) ok", path)
				require.Empty(t, got, "RealDir(%q)", path)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			path := tc.path(t, root)
			fsPath := fs.NewPath()
			got, ok := fsPath.RealDir(path)
			tc.assert(t, root, path, got, ok)
		})
	}
}
