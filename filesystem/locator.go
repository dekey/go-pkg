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

// Locator finds the project root folder.
type Locator struct{}

// NewLocator returns a new Locator.
func NewLocator() *Locator {
	return &Locator{}
}

// FindRootDirWithGoMod is FindRootDir with "go.mod". It adds a stack frame, so skipCaller 2
// starts from the file of its caller.
func (l *Locator) FindRootDirWithGoMod(skipCaller int) (string, error) {
	result, err := l.FindRootDir(goModFilename, skipCaller)
	if err != nil {
		return "", err
	}

	return result, nil
}

// FindRootDirFrom returns the nearest folder that holds file, walking up from startDir (checked
// first when it is an existing folder). It never checks "/" or GOPATH; a relative startDir gives
// a relative result. Errors match ErrEmptyFileName or ErrFailToFindRootDir.
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

// FindRootDir is FindRootDirFrom started at the source file of a caller. skipCaller counts frames
// as in runtime.Caller: 1 is the function that calls FindRootDir.
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

// ReadModulePath returns the module path from the go.mod file in root.
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

// RelativePackagePath returns fullPath relative to the module root modRoot.
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
