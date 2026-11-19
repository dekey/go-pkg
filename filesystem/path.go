package filesystem

import (
	"os"
	"path/filepath"

	"github.com/dekey/go-pkg/tracerr"
)

// Path makes paths absolute and resolves their links.
type Path struct{}

// NewPath returns a new Path.
func NewPath() *Path {
	return &Path{}
}

// Absolute returns path as an absolute path. An absolute path stays as given, and an empty path
// gives the real working folder (see Real). A relative path is joined to the real working
// folder with filepath.Separator ("/" on Linux and macOS) and not cleaned, so a ".." in it still
// follows a link when the caller checks the result with os.Stat. The target may be missing. An
// error comes only from reading the working folder, for example one that matches os.ErrNotExist
// when the working folder was removed.
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
	return workDir + string(filepath.Separator) + path, nil
}

// Real returns path as an absolute path with every link resolved, like `cd <path> && pwd -P`.
// "." and ".." are removed by name first, so "/a/link/.." is "/a" wherever link points, then links
// are followed. A relative path starts at the working folder. The target must exist and may be a
// file: a missing target or a dangling link gives an error that matches os.ErrNotExist.
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

// RealDir returns the real path of the folder at path, as Real does, and true. It returns ""
// and false when path is not absolute (an empty path too), when the target is missing, or when it
// is not a folder, also when path is a link to a file. Any other error, such as a missing
// permission, also gives "" and false.
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
