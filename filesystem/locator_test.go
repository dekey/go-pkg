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
		{
			name: "success: returned path is clean (no ../ or ./ segments)",
			file: "go.mod",
			skip: 1,
			assert: func(t *testing.T, dir string, err error) {
				require.NoError(t, err)

				// Verify path is clean
				cleanDir := filepath.Clean(dir)
				require.Equal(t, cleanDir, dir)

				// Verify no relative segments
				require.NotContains(t, dir, "..")
				require.NotContains(t, dir, string(filepath.Separator)+"."+string(filepath.Separator))
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
