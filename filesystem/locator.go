package filesystem

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	goModFilename = "go.mod"
)

var (
	ErrFailToGetCallerID  = errors.New("failed to get caller info")
	ErrCallerIDIsNegative = errors.New("caller is negative but it should be positive")
	ErrEmptyFileName      = errors.New("file name is empty")
	ErrFailToFindRootDir  = errors.New("failed to find root dir")
	ErrModulePathNotFound = errors.New("module path not found in go.mod")
)

type Locator struct{}

func NewLocator() *Locator {
	return &Locator{}
}

// FindRootDirWithGoMod is a convenience method that calls FindRootDir with the "go.mod" filename.
func (l *Locator) FindRootDirWithGoMod(skipCaller int) (string, error) {
	result, err := l.FindRootDir(goModFilename, skipCaller)
	if err != nil {
		return "", err
	}

	return result, nil
}

// FindRootDirFrom searches for the root directory containing the specified file starting from the given directory and
// moving upwards.
func (l *Locator) FindRootDirFrom(startDir string, file string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("%w", ErrEmptyFileName)
	}

	dir := l.findRootDir(startDir, file)
	if dir == "" {
		return "", fmt.Errorf(
			"cannot find root dir for file [%s] in filepath [%s] %w",
			file,
			startDir,
			ErrFailToFindRootDir,
		)
	}

	return dir, nil
}

// FindRootDir searches for the root directory containing the specified file starting from the caller's file path
// and moving upwards.
func (l *Locator) FindRootDir(file string, skipCaller int) (string, error) {
	if skipCaller < 0 {
		return "", fmt.Errorf("%w", ErrCallerIDIsNegative)
	}
	if file == "" {
		return "", fmt.Errorf("%w", ErrEmptyFileName)
	}

	_, currentFilepath, _, ok := runtime.Caller(skipCaller)
	if !ok {
		return "", fmt.Errorf("%w", ErrFailToGetCallerID)
	}

	dir := l.findRootDir(currentFilepath, file)
	if dir == "" {
		return "", fmt.Errorf(
			"cannot find root dir for file [%s] in filepath [%s] %w",
			file,
			currentFilepath,
			ErrFailToFindRootDir,
		)
	}

	return dir, nil
}

func (l *Locator) findRootDir(from string, file string) string {
	dir := filepath.Dir(from)
	gopath := filepath.Clean(os.Getenv("GOPATH"))
	for dir != "/" && dir != gopath {
		envFile := filepath.Join(dir, file)
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			dir = filepath.Dir(dir)
			continue
		}
		return dir
	}
	return ""
}

func (*Locator) ReadModulePath(root string) (string, error) {
	goModFilePath := filepath.Join(root, goModFilename)

	fileContentBytes, err := os.ReadFile(goModFilePath)
	if err != nil {
		return "", err
	}

	lines := strings.SplitSeq(string(fileContentBytes), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			mod := strings.TrimSpace(after)
			// strip quotes if any
			mod = strings.Trim(mod, "\"`")
			// drop trailing .git if present
			mod = strings.TrimSuffix(mod, ".git")

			return mod, nil
		}
	}
	return "", fmt.Errorf("%w", ErrModulePathNotFound)
}

// RelativePackagePath returns the package path relative to the module root.
// modRoot is a path from root dir to this project like: `/Users/username/project`
// fullPath is a full path to package  like `/Users/username/project/pkg/destination`
// returns relative path to package project/pkg/destination
func (*Locator) RelativePackagePath(modRoot string, fullPath string) (string, error) {
	slog.Debug("RelativePackagePath", slog.String("modRoot", modRoot), slog.String("fullPath", fullPath))
	result, err := filepath.Rel(modRoot, fullPath)
	if err != nil {
		return "", err
	}
	slog.Debug(
		"RelativePackagePath",
		slog.String("result", result),
	)

	return filepath.Dir(result), nil
}
