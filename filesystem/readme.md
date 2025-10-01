# filesystem package

Utilities for locating Go project directories in the filesystem.

This package provides a small helper (Locator) to:

- Traverse upwards from a calling file to find a project root containing a specific file (e.g., go.mod).
- Read the module path declared in go.mod.
- Derive a package path relative to the module root.

The logic is designed to work in typical Go module projects and can be used by  tooling that needs to resolve paths 
reliably from the caller's location.

## Installation

The package is part of this repository under `pkg/filesystem` and is intended for internal use within the project.
If you reuse it externally, import it accordingly after publishing your module.

## Types and errors

- type Locator: stateless helper with methods for locating project roots and computing paths.
- Errors:
  - ErrFailToGetCallerID: runtime.Caller failed to obtain caller info.
  - ErrFailToFindRootDir: upward traversal could not find the requested file.
  - ErrModulePathNotFound: go.mod does not contain a `module` directive.

## API

- NewLocator() *Locator: constructor.
- (*Locator) FindRootDirWithGoMod(skipCaller int) (string, error): find the root directory that contains `go.mod`, walking up from the calling file identified by runtime.Caller(skipCaller).
- (*Locator) FindRootDir(file string, skipCaller int) (string, error): generalized root finder for any file name.
- (*Locator) ReadModulePath(root string) (string, error): read the module path from `<root>/go.mod`.
- (*Locator) RelativePackagePath(modRoot, fullPath string) (string, error): compute `fullPath` relative to `modRoot` and return its directory path (package path).

## Notes on skipCaller

The `skipCaller` parameter is passed to `runtime.Caller(skipCaller)` to select which stack frame to treat as the starting point for the search:

- 0: the function where `FindRootDir`/`FindRootDirWithGoMod` is called.
- 1: the caller of that function, and so on.

If you're writing a helper wrapper around Locator, you typically want to add +1 (or more) to ensure the reported file path corresponds to your wrapper's caller rather than the wrapper itself.

## Behavior and traversal boundaries

- Upward traversal stops at:
  - The filesystem root `/`.
  - The cleaned value of the `GOPATH` environment variable (if set). This prevents accidentally walking out of the GOPATH workspace in GOPATH-mode setups.
- A directory is considered the root if it contains the target file name (e.g., `go.mod`).

## Examples

### Find module root and read module path

```go
package main

import (
    "fmt"
    "log"
    "path/filepath"

    "your/module/pkg/filesystem"
)

func main() {
    l := filesystem.NewLocator()

    // Use skipCaller=1 to start from this function's frame.
    modRoot, err := l.FindRootDirWithGoMod(1)
    if err != nil {
        log.Fatal(err)
    }

    modPath, err := l.ReadModulePath(modRoot)
    if err != nil {
        log.Fatal(err)
    }

    // e.g., derive a package path relative to the module root
    pkgFullPath := filepath.Join(modRoot, "pkg", "filesystem")
    relPkgPath, err := l.RelativePackagePath(modRoot, pkgFullPath)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Module root:", modRoot)
    fmt.Println("Module path:", modPath)
    fmt.Println("Relative package path:", relPkgPath)
}
```

### Find a custom root marker file

```go
modRoot, err := l.FindRootDir("go.work", 1) // or any other marker file name
if err != nil {
    // handle error
}
```

## Error handling tips

- ErrFailToGetCallerID: verify your `skipCaller` value and the execution environment (optimizations or unusual build flags can sometimes affect `runtime.Caller`).
- ErrFailToFindRootDir: ensure the marker file (e.g., `go.mod`) exists in a parent directory of the resolved caller file. Also confirm that the search is not blocked by GOPATH boundary.
- ErrModulePathNotFound: check that your `go.mod` contains a valid `module` directive and is readable.

## Logging

`RelativePackagePath` emits debug logs using the standard library `log/slog` package. Configure a logger with an appropriate level if you want to observe these messages during development.

## License

This package is distributed under the repository's license. See the LICENSE file at the repo root.
