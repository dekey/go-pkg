# go-pkg

A collection of utility packages for Go development.

## Features

### Filesystem Locator
The `filesystem` package provides a `Locator` that can:
- Find the project's root directory by searching upwards for a `go.mod` file or any other specified file.
- Support finding the root directory starting from the caller's location or a given directory.
- Read the module path directly from a `go.mod` file.
- Calculate relative package paths from the module root.

## Installation

```bash
go get github.com/dekey/go-pkg
```

## Usage

### Filesystem Locator

```go
import "github.com/dekey/go-pkg/filesystem"

locator := filesystem.NewLocator()

// Find root dir by searching for go.mod, skipping 1 level of caller stack
rootDir, err := locator.FindRootDirWithGoMod(1)
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

- Go 1.26 or higher.

## License

[MIT](LICENSE)