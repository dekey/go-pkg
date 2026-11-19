package filesystem

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dekey/go-pkg/tracerr"
)

const (
	goModFilename = "go.mod"
)

var (
	ErrFailToGetCallerID       = errors.New("failed to get caller info")
	ErrCallerIDIsNegative      = errors.New("caller is negative but it should be positive")
	ErrEmptyFileName           = errors.New("file name is empty")
	ErrFailToFindRootDir       = errors.New("failed to find root dir")
	ErrModulePathNotFound      = errors.New("module path not found in go.mod")
	ErrFailToGetRelPackagePath = errors.New("failed to get relative package path")
	ErrFailToReadGoMod         = errors.New("failed to read go.mod")
)

// Locator locates the project root folder by walking up the file system.
type Locator struct{}

// NewLocator returns a new Locator.
func NewLocator() *Locator {
	return &Locator{}
}

// FindRootDirWithGoMod is a convenience method that calls FindRootDir with the "go.mod" filename.
// It adds its own stack frame: skipCaller 1 starts from the file of the function that calls
// FindRootDir, which is this package's own file, so pass 2 to start from the file of the function
// that calls FindRootDirWithGoMod.
func (l *Locator) FindRootDirWithGoMod(skipCaller int) (string, error) {
	result, err := l.FindRootDir(goModFilename, skipCaller)
	if err != nil {
		return "", err
	}

	return result, nil
}

// FindRootDirFrom returns the first folder that holds file, walking up from startDir. When startDir
// is an existing folder, the walk checks that folder first; for a file, or any path os.Stat fails
// on, it starts at filepath.Dir(startDir). Then it checks each parent folder in turn. A folder path
// is cleaned first, so a trailing "/" or "/." makes no difference and the result has no trailing
// separator. Only a successful os.Stat of the file counts as found; any other Stat error, such as
// permission denied, means "not here" and the walk goes on.
//
// The walk stops before the file system root ("/" on Linux and macOS) and never checks it. It also
// stops before the GOPATH folder when GOPATH names one folder; a GOPATH that lists several folders
// stops the walk at none of them. On Windows the volume root, such as C:\, is the last folder
// checked. When the walk ends without the file, the error wraps ErrFailToFindRootDir.
//
// A relative startDir is not made absolute: the walk goes up the path text only, ends after "."
// (the working folder) and returns a relative folder.
func (l *Locator) FindRootDirFrom(startDir string, file string) (string, error) {
	if file == "" {
		return "", tracerr.Wrap(ErrEmptyFileName)
	}

	dir := l.findRootDir(startDir, file)
	if dir == "" {
		return "", tracerr.Errorf(
			"cannot find root dir for file %q in filepath %q: %w",
			file,
			startDir,
			ErrFailToFindRootDir,
		)
	}

	return dir, nil
}

// FindRootDir returns the first folder that holds file, walking up from the folder of the caller's
// source file as FindRootDirFrom does. skipCaller counts stack frames as in runtime.Caller: 1
// starts from the file of the function that calls FindRootDir. A binary built with -trimpath has
// relative file paths, so the walk then works as for a relative startDir in FindRootDirFrom.
func (l *Locator) FindRootDir(file string, skipCaller int) (string, error) {
	if skipCaller < 0 {
		return "", tracerr.Wrap(ErrCallerIDIsNegative)
	}
	if file == "" {
		return "", tracerr.Wrap(ErrEmptyFileName)
	}

	_, currentFilepath, _, ok := runtime.Caller(skipCaller)
	if !ok {
		return "", tracerr.Wrap(ErrFailToGetCallerID)
	}

	dir := l.findRootDir(currentFilepath, file)
	if dir == "" {
		return "", tracerr.Errorf(
			"cannot find root dir for file %q in filepath %q: %w",
			file,
			currentFilepath,
			ErrFailToFindRootDir,
		)
	}

	return dir, nil
}

// findRootDir walks up as FindRootDirFrom describes and returns the folder that holds file, or ""
// when the walk ends without it.
func (l *Locator) findRootDir(from string, file string) string {
	dir := filepath.Dir(from)
	if info, err := os.Stat(from); err == nil && info.IsDir() {
		dir = filepath.Clean(from)
	}
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		gopath = filepath.Clean(gopath)
	}
	for dir != string(filepath.Separator) && dir != gopath {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

// ReadModulePath reads and returns the module path from the go.mod file at the given root folder.
func (l *Locator) ReadModulePath(root string) (string, error) {
	goModFilePath := filepath.Join(root, goModFilename)

	fileContentBytes, err := os.ReadFile(goModFilePath)
	if err != nil {
		return "", tracerr.Errorf("%w: %w", ErrFailToReadGoMod, err)
	}

	lines := strings.SplitSeq(string(fileContentBytes), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			mod := strings.TrimSpace(after)
			if i := strings.Index(mod, "//"); i != -1 {
				mod = strings.TrimSpace(mod[:i])
			}
			mod = strings.Trim(mod, "\"`")
			mod = strings.TrimSuffix(mod, ".git")

			return mod, nil
		}
	}
	return "", tracerr.Wrap(ErrModulePathNotFound)
}

// RelativePackagePath returns the package folder path relative to the module root.
// modRoot is the absolute path to the module root, e.g. `/Users/username/project`.
// fullPath is the absolute path to the package folder,
// e.g. `/Users/username/project/pkg/destination`.
// It returns the relative package path, e.g. `pkg/destination`.
func (l *Locator) RelativePackagePath(modRoot string, fullPath string) (string, error) {
	slog.Debug(
		"RelativePackagePath",
		slog.String("modRoot", modRoot),
		slog.String("fullPath", fullPath),
	)
	result, err := filepath.Rel(modRoot, fullPath)
	if err != nil {
		return "", tracerr.Errorf(
			"modRoot %q, fullPath %q: %w: %w",
			modRoot,
			fullPath,
			ErrFailToGetRelPackagePath,
			err,
		)
	}
	slog.Debug(
		"RelativePackagePath",
		slog.String("result", result),
	)

	return result, nil
}
