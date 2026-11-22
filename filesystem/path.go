package filesystem

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dekey/go-pkg/tracerr"
)

// Path makes paths absolute and resolves their links.
type Path struct{}

// NewPath returns a new Path.
func NewPath() *Path {
	return &Path{}
}

// Absolute joins a relative path to the real working folder (not cleaned) and returns an absolute
// path as given. The target may be missing; os.ErrNotExist means the working folder is gone.
func (p *Path) Absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	workDir, err := p.Real(".")
	if err != nil {
		return "", err
	}
	if path == "" {
		return workDir, nil
	}
	separator := string(filepath.Separator)
	return strings.TrimSuffix(workDir, separator) + separator + path, nil
}

// Real returns the absolute path with every link resolved; ".." is removed before links are
// followed. A missing target or a dangling link gives an error matching os.ErrNotExist.
func (*Path) Real(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", tracerr.Wrap(err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", tracerr.Wrap(err)
	}
	return resolved, nil
}

// RealDir returns the real path and true when path is absolute and a folder, else "" and false.
func (p *Path) RealDir(path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	resolved, err := p.Real(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return resolved, true
}
