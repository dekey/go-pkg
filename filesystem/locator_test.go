package filesystem_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	fs "github.com/dekey/go-pkg/filesystem"
	"github.com/stretchr/testify/require"
)

func TestLocator_FindRootDir(t *testing.T) {
	testCases := []struct {
		name   string
		file   string
		skip   int
		assert func(t *testing.T, dir string, err error)
	}{
		{
			name: "success: finds project root by go.mod",
			file: "go.mod",
			skip: 1,
			assert: func(t *testing.T, dir string, err error) {
				require.NoError(t, err)

				// Get current test file location
				_, thisFile, _, ok := runtime.Caller(0)
				require.True(t, ok)

				// Verify returned dir is a parent of this test file
				absTestFile, err := filepath.Abs(thisFile)
				require.NoError(t, err)
				require.True(t, strings.HasPrefix(absTestFile, dir))

				// Verify go.mod exists
				_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
				require.NoError(t, statErr)

				r, err := os.OpenRoot(dir)
				require.NoError(t, err)
				require.NotEmpty(t, r)

				require.NoError(t, r.Close())

				// Verify path is clean
				require.Equal(t, filepath.Clean(dir), dir)
				require.NotContains(t, dir, "..")
				require.NotContains(t, dir, string(filepath.Separator)+"."+string(filepath.Separator))
			},
		},
		{
			name: "error: file not found in any parent",
			file: "___definitely_not_existing___.txt",
			skip: 1,
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrFailToFindRootDir)
				require.Empty(t, got)
			},
		},
		{
			name: "error: invalid skip causes runtime.Caller failure",
			file: "go.mod",
			skip: 9999,
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrFailToGetCallerID)
				require.Empty(t, got)
			},
		},
		{
			name: "error: negative skip value",
			file: "go.mod",
			skip: -1,
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrCallerIDIsNegative)
				require.Empty(t, got)
			},
		},
		{
			name: "error: empty filename",
			file: "",
			skip: 1,
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrEmptyFileName)
				require.Empty(t, got)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			locator := fs.NewLocator()
			dir, err := locator.FindRootDir(tc.file, tc.skip)
			tc.assert(t, dir, err)
		})
	}
}

func TestLocator_FindRootDirWithGoMod(t *testing.T) {
	testCases := []struct {
		name   string
		skip   int
		assert func(t *testing.T, dir string, err error)
	}{
		{
			name: "success: finds project root containing go.mod",
			skip: 1,
			assert: func(t *testing.T, dir string, err error) {
				require.NoError(t, err)

				_, thisFile, _, ok := runtime.Caller(0)
				require.True(t, ok)

				// thisFile is locator_test.go inside filesystem/; two Dir calls reach the module root.
				wantRoot := filepath.Dir(filepath.Dir(thisFile))
				require.Equal(t, wantRoot, dir)

				_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
				require.NoError(t, statErr)

				require.Equal(t, filepath.Clean(dir), dir)
				require.NotContains(t, dir, "..")
			},
		},
		{
			name: "error: negative skip value",
			skip: -1,
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrCallerIDIsNegative)
				require.Empty(t, got)
			},
		},
		{
			name: "error: skip exceeds call stack depth",
			skip: 9999,
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrFailToGetCallerID)
				require.Empty(t, got)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			locator := fs.NewLocator()
			dir, err := locator.FindRootDirWithGoMod(tc.skip)
			tc.assert(t, dir, err)
		})
	}
}

func TestLocator_FindRootDirFrom(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	thisDir := filepath.Dir(thisFile)

	testCases := []struct {
		name     string
		startDir string
		file     string
		assert   func(t *testing.T, dir string, err error)
	}{
		{
			name:     "success: finds project root by go.mod",
			startDir: thisDir,
			file:     "go.mod",
			assert: func(t *testing.T, dir string, err error) {
				require.NoError(t, err)
				require.True(t, strings.HasPrefix(thisFile, dir))
				_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
				require.NoError(t, statErr)

				require.Equal(t, filepath.Clean(dir), dir)
				require.NotContains(t, dir, "..")
			},
		},
		{
			name:     "error: file not found in any parent",
			startDir: thisDir,
			file:     "___definitely_not_existing___.txt",
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrFailToFindRootDir)
				require.Empty(t, got)
			},
		},
		{
			name:     "error: empty filename",
			startDir: thisDir,
			file:     "",
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrEmptyFileName)
				require.Empty(t, got)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			locator := fs.NewLocator()
			dir, err := locator.FindRootDirFrom(tc.startDir, tc.file)
			tc.assert(t, dir, err)
		})
	}
}

func TestLocator_RelativePackagePath(t *testing.T) {
	root := t.TempDir()

	testCases := []struct {
		name     string
		modRoot  string
		fullPath string
		assert   func(t *testing.T, got string, err error)
	}{
		{
			name:     "success: nested package directory",
			modRoot:  root,
			fullPath: filepath.Join(root, "pkg", "foo"),
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, filepath.Join("pkg", "foo"), got)
			},
		},
		{
			name:     "success: single-level package directory",
			modRoot:  root,
			fullPath: filepath.Join(root, "cmd"),
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "cmd", got)
			},
		},
		{
			name:     "success: root package returns dot",
			modRoot:  root,
			fullPath: root,
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, ".", got)
			},
		},
		{
			name:     "error: incompatible absolute and relative paths",
			modRoot:  "/absolute/mod/root",
			fullPath: "relative/pkg/foo.go",
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrFailToGetRelPackagePath)
				require.Empty(t, got)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			locator := fs.NewLocator()
			got, err := locator.RelativePackagePath(tc.modRoot, tc.fullPath)
			tc.assert(t, got, err)
		})
	}
}

func TestLocator_ReadModulePath(t *testing.T) {
	locator := fs.NewLocator()

	testCases := []struct {
		name   string
		setup  func(t *testing.T) string
		assert func(t *testing.T, got string, err error)
	}{
		{
			name: "success: standard module declaration",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "go.mod"),
					[]byte("module github.com/user/repo\n"),
					0o600,
				))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "github.com/user/repo", got)
			},
		},
		{
			name: "success: double-quoted module path",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "go.mod"),
					[]byte(`module "github.com/user/repo"`+"\n"),
					0o600,
				))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "github.com/user/repo", got)
			},
		},
		{
			name: "success: backtick-quoted module path",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "go.mod"),
					[]byte("module `github.com/user/repo`\n"),
					0o600,
				))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "github.com/user/repo", got)
			},
		},
		{
			name: "success: trailing .git stripped",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "go.mod"),
					[]byte("module github.com/user/repo.git\n"),
					0o600,
				))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "github.com/user/repo", got)
			},
		},
		{
			name: "success: module line preceded by go directive",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "go.mod"),
					[]byte("go 1.26\n\nmodule github.com/user/repo\n"),
					0o600,
				))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.NoError(t, err)
				require.Equal(t, "github.com/user/repo", got)
			},
		},
		{
			name: "error: no module declaration",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(root, "go.mod"),
					[]byte("go 1.26\n\nrequire some/dep v1.0.0\n"),
					0o600,
				))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrModulePathNotFound)
				require.Empty(t, got)
			},
		},
		{
			name: "error: empty file",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte{}, 0o600))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrModulePathNotFound)
				require.Empty(t, got)
			},
		},
		{
			name: "error: go.mod does not exist",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrFailToReadGoMod)
				require.ErrorIs(t, err, os.ErrNotExist)
				require.Empty(t, got)
			},
		},
		{
			name: "error: go.mod is a directory",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				require.NoError(t, os.Mkdir(filepath.Join(root, "go.mod"), 0o755))
				return root
			},
			assert: func(t *testing.T, got string, err error) {
				require.ErrorIs(t, err, fs.ErrFailToReadGoMod)
				require.Empty(t, got)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t)
			got, err := locator.ReadModulePath(root)
			tc.assert(t, got, err)
		})
	}
}
