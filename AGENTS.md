# Project Overview

This is a multi-package utility library. Each top-level directory is its own package. A package may import `tracerr` to give its errors a stack trace; packages do not import each other in any other way.

## Important Caveats

- `tracerr` — errors with stack traces; imports no other package of this repo

- `filesystem` — four types: `Locator` (find the project root), `Path` (absolute and real paths), `Reader` (open a file read-only, last lines), `Writer` (temp files, link, remove, open, create, append a line). No method makes folders. Imports `tracerr`.

## Commands

- `make test`: `go test -parallel=1 -count=1 ./...`
- `make lint`: golangci-lint with `.golangci.yml`
- `make init`: installs the pinned golangci-lint
- `make format`: `golines`, `gofumpt` and `go fix ./...`. It can rewrite files your change does not own (for example `tracerr/errors.go`); check `git diff --stat` after it.

## Conventions

- Wrap an error with `tracerr` once, in the exported function or method where it starts, so the first frame of the stack trace is that function or method. Forward errors from this repo's own functions as they are.
- Sentinel errors are `var ErrXxx = errors.New("...")` with no doc comment; the name and the test names say what failed. Join a sentinel with its cause as `tracerr.Errorf("%w: %w", ErrXxx, err)`, so `errors.Is` matches both.
- Error texts and names carry no words of a caller (say "no free name", not "no free item name").
- Tests: `package <pkg>_test` (the `testpackage` linter), `filesystem` imported as `fs`, table tests whose rows are named after the rule they check. `lll` also checks tests. Runnable `Example` funcs sit in the test file of their source file and use the `fs` alias.
- `filesystem` tests set the umask to 002 in `TestMain`. Modes without a write bit for group or others (755, 644, 600, 400) stay exact; a new 666 file or 777 folder comes out as 664 or 775. The `WriteTemp` 666 row checks that `WriteTemp` sets the full mode itself (`Chmod`), past the umask.
- depguard allows only the standard library, this module and testify.
- A new exported name goes into the package `readme.md` and into Features in the root `README.md`.

## Versions (keep in sync)

- Go: the `go` line in `go.mod` = `run.go` in `.golangci.yml` = `GOLANG_VERSION` in `.github/workflows/run-lint.yml` and `run-test.yml` = Requirements in `README.md`.
- golangci-lint: the version in the `Makefile` `init` target = `GOLANGCI_LINT_VERSION` in `.github/workflows/run-lint.yml`.

## Release

- Every push to `main` makes a tag (`.github/workflows/release.yml`, `default_bump: patch`). It does not wait for lint or test.
- Pull requests are squash-merged, so the PR title decides the version bump: a title that starts with `feat` gives a minor bump, anything else a patch bump.
- A released tag can not be changed.
- `.agent/` (plans and task logs) is git-ignored.
