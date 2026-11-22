# Project Overview

This is a multi-package utility library. Each top-level directory is its own package. A package may import `tracerr` to give its errors a stack trace; packages do not import each other in any other way.

## Important Caveats
- `tracerr` — errors with stack traces; imports no other package of this repo
- `filesystem` — four types and one interface: `Locator` (find the project root), `Path` (absolute and real paths), `Reader` (open a file with the flag given, last lines), `Writer` (temp files, link, remove, create, append a line), and `Opener` (`OpenFile(path, flag)`; `NewWriter` takes one, usually `NewReader()`, and `CreateFile` and `AppendLine` open files through it). No method makes folders. Imports `tracerr`.

## Commands

- `make test`: `go test -parallel=1 -count=1 ./...`
- `make lint`: golangci-lint with `.golangci.yml`
- `make init`: installs the pinned golangci-lint
- `make generate`: `go generate ./...`; remakes the mocks with mockery v3 and `.mockery.yml`
- `make format`: `golines`, `gofumpt` and `go fix ./...`. It can rewrite files your change does not own (for example `tracerr/errors.go`); check `git diff --stat` after it.

## Conventions

- Wrap an error with `tracerr` once, in the exported function or method where it starts, so the first frame of the stack trace is that function or method. Forward errors from this repo's own functions as they are.
- Sentinel errors are `var ErrXxx = errors.New("...")` with no doc comment; the name and the test names say what failed. Join a sentinel with its cause as `tracerr.Errorf("%w: %w", ErrXxx, err)`, so `errors.Is` matches both.
- Error texts and names carry no words of a caller (say "no free name", not "no free item name").
- Tests: `package <pkg>_test` (the `testpackage` linter), `filesystem` imported as `fs`, table tests whose rows are named after the rule they check. `lll` also checks tests. Runnable `Example` funcs sit in the test file of their source file and use the `fs` alias.
- `filesystem` tests set the umask to 002 in `TestMain`. Modes without a write bit for group or others (755, 644, 600, 400) stay exact; a new 666 file or 777 folder comes out as 664 or 775. The `WriteTemp` 666 row checks that `WriteTemp` sets the full mode itself (`Chmod`), past the umask.
- depguard allows only the standard library, this module and testify.
- Mocks come from mockery v3: list the interface in `.mockery.yml` and put `//go:generate go run github.com/vektra/mockery/v3@<version>` above it; the mock goes to `<pkg>/internal/mocks`. Never edit a mock by hand. Do not `go install` mockery v3: it replaces a global v2 `mockery` that other repos need. mockery v2 fails with Go 1.27.
- A new exported name goes into the package `readme.md` and into Features in the root `README.md`.

## Versions (keep in sync)

- Go: the `go` line in `go.mod` = `run.go` in `.golangci.yml` = `GOLANG_VERSION` in `.github/workflows/run-lint.yml` and `run-test.yml` = Requirements in `README.md`.
- golangci-lint: the version in the `Makefile` `init` target = `GOLANGCI_LINT_VERSION` in `.github/workflows/run-lint.yml`.
- mockery: the version in every `//go:generate go run github.com/vektra/mockery/v3@...` line.

## Release

- Every push to `main` makes a tag (`.github/workflows/release.yml`, `default_bump: patch`). It does not wait for lint or test.
- Pull requests are squash-merged, so the PR title decides the version bump: a title that starts with `feat` gives a minor bump, anything else a patch bump.
- A released tag can not be changed.
- `.agent/` (plans and task logs) is git-ignored.

## Tools
- The Bash tool runs zsh: quote every glob ('*.go'), never start an unquoted word with =, and add || true after a search where no match is normal. Wiki: /Users/dekey/ai-config/docs/wiki/workflow/tools.md.