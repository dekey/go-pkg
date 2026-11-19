# go-pkg

A collection of utility packages for Go development.

## Features

### Errors with stack traces
The `tracerr` package wraps errors with a stack trace and keeps `errors.Is` and `errors.As` working. See [tracerr/readme.md](tracerr/readme.md).

### Filesystem Locator
The `filesystem` package provides a `Locator` that can:
- Find the project's root directory by searching upwards for a `go.mod` file or any other specified file.
- Support finding the root directory starting from the caller's location or a given path; a given folder is checked first (where the walk starts and stops: [filesystem/readme.md](filesystem/readme.md#project-root-locator)).
- Read the module path directly from a `go.mod` file.
- Calculate relative package paths from the module root.

### File Helpers
The `filesystem` package also has small helpers for files and paths, made for Linux and macOS. They need only the standard library and `tracerr`. Their errors carry a stack trace and keep the `os` error inside when there is one, so `errors.Is` works with errors such as `os.ErrNotExist` and with the `Err` values of the package.
- `Writer` (from `NewWriter`) with the methods `WriteTemp`, `Remove`: write data to a new temp file in a folder with an exact file mode, and remove it again.
- `Writer` methods `CreateFile`, `OpenFile`, `AppendLine`: open a file for reading and writing and make it when it is missing (a file that is there keeps its content), open a file with your own flag used as given, or append one line to a file in one write (a missing file is made). None of them makes folders.
- `Reader` (from `NewReader`) with the methods `OpenFile`, `LastLines`: open a file for reading only (a missing file is never made), or read the last n lines of a file, like `tail -n`.
- `Path` (from `NewPath`) with the methods `Absolute`, `Real`, `RealDir`: make a path absolute, or absolute with every link resolved.
- `Writer` method `LinkFirstFree`: hard link a file to the first free name (`report.txt`, `report-2.txt`, ...), never over another file.

Errors and examples: [filesystem/readme.md](filesystem/readme.md).

## Installation

```bash
go get github.com/dekey/go-pkg
```

## Usage

### Filesystem Locator

```go
import "github.com/dekey/go-pkg/filesystem"

locator := filesystem.NewLocator()

// Find root dir by searching for go.mod, starting from this file.
// Pass 2: FindRootDirWithGoMod adds its own stack frame, and 1 would start from go-pkg's own file.
rootDir, err := locator.FindRootDirWithGoMod(2)
if err != nil {
    // handle error
}

// Read module path from go.mod in the root directory
modulePath, err := locator.ReadModulePath(rootDir)
if err != nil {
    // handle error
}
```

## Development

The project includes a `Makefile` for common development tasks:

- `make test`: Run all tests.
- `make format`: Format the code using `golines`, `gofumpt`, and `go fix`.
- `make lint`: Run `golangci-lint`.
- `make add-vendor`: Tidy the module and update the `vendor` directory.

## Requirements

- Go 1.27 or higher.
- Linux or macOS for the `filesystem` file helpers.

## License

[MIT](LICENSE)
