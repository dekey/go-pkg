# File System package

Utilities for locating Go project directories in the filesystem.

This package provides a stateless helper (`Locator`) to:

- Traverse upwards from a calling file to find a project root containing a specific file (e.g., `go.mod`).
- Read the module path declared in `go.mod`.
- Derive a package path relative to the module root.

The logic is designed to work in typical Go module projects and can be used by tooling that needs to resolve paths 
reliably from the caller's location.

---

## Types and Errors

- **type Locator**: Stateless helper with methods for locating project roots and computing paths.
- **Errors:**
  - `ErrFailToGetCallerID`: `runtime.Caller` failed to obtain caller info.
  - `ErrFailToFindRootDir`: Upward traversal could not find the requested file.
  - `ErrModulePathNotFound`: `go.mod` does not contain a `module` directive.

---

## Examples

### Find module root and read module path

```go
package main

import (
    "fmt"
    "log"
    "path/filepath"

    "github.com/dekey/go-pkg/filesystem"
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

---
