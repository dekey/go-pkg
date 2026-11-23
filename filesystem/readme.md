# File System package

Helpers for Go project folders, files and paths:

- `Locator` finds the root folder of a Go project and reads the module path from `go.mod`.
- The [file helpers](#file-helpers) write temp files, append lines, read the last lines of a file,
  build absolute and real paths, and link a file to the first free name.

---

## Project-root locator

This package provides a stateless helper (`Locator`) to:

- Walk up from a start path to the first folder that holds a given file (e.g., `go.mod`). The start
  path is the caller's file (`FindRootDir`, `FindRootDirWithGoMod`) or a path you give
  (`FindRootDirFrom`).
- Read the module path declared in `go.mod`.
- Derive a package path relative to the module root.

Where the walk starts: when the start path is an existing folder, the walk checks that folder first
(cleaned, so a trailing `/` or `/.` makes no difference); for a file, or a path `os.Stat` fails on,
it starts at `filepath.Dir` of the path (for a file, its folder). Then it checks each parent folder
in turn and returns the first one that holds the file, with no trailing separator. Only a successful
`os.Stat` of the file counts as found; any other error, such as permission denied, keeps the walk
going.

Where the walk stops: before the file system root (`/` on Linux and macOS), which it never checks,
and before the `GOPATH` folder when `GOPATH` names one folder (a `GOPATH` that lists several folders
stops the walk at none of them). On Windows the volume root, such as `C:\`, is the last folder
checked. When the walk ends without the file, the error wraps `ErrFailToFindRootDir`.

`skipCaller` counts stack frames as in `runtime.Caller`. To start from the file of the function that
makes the call, pass 1 to `FindRootDir` and 2 to `FindRootDirWithGoMod`, because
`FindRootDirWithGoMod` adds its own frame. With 1, `FindRootDirWithGoMod` starts from go-pkg's own
`locator.go`, so in a module that imports go-pkg it finds the go-pkg folder in the module cache.

A binary built with `-trimpath` has relative file paths, so `FindRootDir` and
`FindRootDirWithGoMod` then work like `FindRootDirFrom` with a relative path (see
[Start from a path you give](#start-from-a-path-you-give)).

---

## Locator types and errors

- **type Locator**: Stateless helper with methods for locating project roots and computing paths.
- **Errors:**
  - `ErrFailToGetCallerID`: `runtime.Caller` failed to obtain caller info.
  - `ErrCallerIDIsNegative`: `skipCaller` is below 0.
  - `ErrEmptyFileName`: the name of the file to look for is empty.
  - `ErrFailToFindRootDir`: the walk up did not find the requested file.
  - `ErrFailToReadGoMod`: `go.mod` could not be read.
  - `ErrModulePathNotFound`: `go.mod` does not contain a `module` directive.
  - `ErrFailToGetRelPackagePath`: the package path could not be made relative to the module root.

---

## Locator examples

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

    // 2 starts from this file: FindRootDirWithGoMod adds its own stack frame.
    modRoot, err := l.FindRootDirWithGoMod(2)
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

### Start from a path you give

`FindRootDirFrom` walks as described in [Project-root locator](#project-root-locator). A relative
path is not made absolute: the walk goes up the path text only, ends after `.` (the working folder)
and returns a relative folder. Pass an absolute path to walk up past the working folder.

```go
start, err := filepath.Abs(dir) // dir itself is checked first
if err != nil {
    // handle error
}
modRoot, err := l.FindRootDirFrom(start, "go.mod")
if err != nil {
    // handle error
}
```

---

## File helpers

The file helpers take plain paths, data and limits from the caller. They are made for Linux and
macOS. Every error they return has a `tracerr` stack trace and keeps the `os` error inside when
there is one, so `errors.Is` works with errors such as `os.ErrNotExist` and with the `Err` values
below.

The umask, used below, is the mask of the process that takes bits off the mode of new files (often
`022`, which takes write for group and others off).

Runnable examples: `ExampleWriter_WriteTemp`, `ExampleWriter_LinkFirstFree` and
`ExampleWriter_AppendLine` in [write_test.go](write_test.go), `ExampleReader_LastLines` in
[read_test.go](read_test.go). `go test` runs them and checks their output. `go doc` does not show
them: it does not read test files.

### Make a Writer: `NewWriter`, `Opener`

`NewWriter(opener)` returns a `*Writer` that opens files through `opener`. Pass
`filesystem.NewReader()`:

```go
w := filesystem.NewWriter(filesystem.NewReader())
```

`Opener` is an interface with one method, `OpenFile(path, flag)`, which opens a file like
`os.OpenFile` and returns the `*os.File`. A `*Reader` is one (see
[`Reader`](#open-a-file-and-read-its-last-lines-reader)). `CreateFile` and `AppendLine` open files
only through the `Opener`; `WriteTemp`, `Remove` and `LinkFirstFree` do not use it. Apart from its
`Opener`, a `Writer` holds no state.

Pass your own `Opener` only when you need other open behaviour, for example in a test. When its
error has no `tracerr` stack trace, `CreateFile` or `AppendLine` adds one. The text below describes
a `Writer` with `NewReader()` as its `Opener`.

### Temp files: `Writer.WriteTemp`, `Writer.Remove`

- `w.WriteTemp(dir, pattern, data, perm)` writes `data` to a new file in `dir`, syncs it to the disk
  and returns its path. An empty `dir` means `os.TempDir()`. The file name comes from `pattern` as
  in `os.CreateTemp`. The file gets mode `perm` exactly: the umask does not change it. When the
  write, the mode change, the sync or the close fails, the file is removed and the error returned;
  a failed remove is joined into it.
- `w.Remove(path)` removes the file, for example a temp file that was not renamed. It is
  `os.Remove` with a `tracerr` stack trace on the error.

Errors:

- `ErrMakeTemp`: the temp file could not be made, for example because `dir` is missing (the error
  then also matches `os.ErrNotExist`).
- Other `WriteTemp` errors come from the write, the mode change, the sync or the close.
- `Remove` gives the remove error, for example one that matches `os.ErrNotExist` for a missing
  file.

To replace a file in one step, write the temp file in the same folder and rename it over the old
file. `os.Rename` works only within one file system; across file systems it fails. For the rename
itself to survive a power loss, also open the folder and call `Sync` on it.

```go
w := filesystem.NewWriter(filesystem.NewReader())
temp, err := w.WriteTemp(dir, ".config.json.*", data, 0o644)
if err != nil {
    return err
}
if err := os.Rename(temp, filepath.Join(dir, "config.json")); err != nil {
    removeErr := w.Remove(temp)
    return errors.Join(err, removeErr)
}
return nil
```

### Open or make a file: `Writer.CreateFile`

`w.CreateFile(path)` opens the file for reading and writing, through its `Opener` with
`os.O_RDWR|os.O_CREATE`. So it works like
[`r.OpenFile`](#open-a-file-and-read-its-last-lines-reader) with that flag: a missing file is made
with mode 644, less the bits the umask takes off; a file that is there is opened as it is, with its
content and mode kept.

The folder of `path` must exist: `CreateFile` never makes folders. Make a new folder first, for
example with `os.MkdirAll`. The caller closes the file.

Errors: no `Err` value of its own.

- A missing folder gives an error that matches `os.ErrNotExist`, and nothing is made.
- A file in place of the folder gives an error that matches `syscall.ENOTDIR`.
- Any other open error comes as the `os` package gives it, for example
  `open <path>: is a directory`.

### Append a line: `Writer.AppendLine`

`w.AppendLine(path, line)` adds `line` to the end of the file. It writes `line` as given and adds
no newline, so end it with `\n`. The file is opened with `os.O_APPEND` and the line goes out in one
write, so on a local file system lines of processes that append to the same file at the same time
do not mix. This does not hold on a network file system such as NFS, or for a line longer than
1 GiB, which Go writes in parts.

`AppendLine` opens the file through its `Opener` with `os.O_APPEND|os.O_WRONLY|os.O_CREATE`, so a
missing file is made with mode 644, less the bits the umask takes off. Like `CreateFile`, it never
makes folders.

Errors: no `Err` value of its own. An open error is the same as for `CreateFile` (see
[Open or make a file](#open-or-make-a-file-writercreatefile)); `AppendLine` passes it on.
`AppendLine` also gives the write or close error as the `os` package gives it.

```go
w := filesystem.NewWriter(filesystem.NewReader())
path := filepath.Join(dir, "history", "events.txt")
if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
    return err
}
if err := w.AppendLine(path, "user signed in\n"); err != nil {
    return err
}
return nil
```

### Open a file and read its last lines: `Reader`

`NewReader()` returns a `*Reader`; like `Locator` and `Path`, it holds no state. It is the usual
`Opener` of a `Writer`.

`r.OpenFile(path, flag)` opens the file with `flag` as given, like `os.OpenFile`: for example
`os.O_RDONLY` to read, or `os.O_APPEND|os.O_WRONLY|os.O_CREATE` to append. With `os.O_CREATE` a
missing file is made with mode 644, less the bits the umask takes off; a file that is there keeps
its mode. Without `os.O_CREATE` a missing file gives an error that matches `os.ErrNotExist` (no
`Err` value of its own), and nothing is made. It never makes folders: a missing folder gives an
error that matches `os.ErrNotExist`. The caller closes the file.

`r.LastLines(path, n)` returns the last `n` lines of the file, like `tail -n`. Lines end at `\n` and
come back without it; a `\r` before the `\n` stays. A final newline does not start a new line. An
empty file, or `n` of 0 or less, gives no lines (`nil`). The file is read backwards from its end, so
a long file costs only its tail. `LastLines` trusts the size the file reports: a file that reports
size 0, such as a named pipe, a device or a file in `/proc`, gives no lines. `LastLines` waits as
long as opening the file waits: on a named pipe with no writer it waits until a writer opens the
pipe.

Errors: no `Err` value of its own. A missing file gives an error that matches `os.ErrNotExist`, and
a folder in place of the file gives one that matches `syscall.EISDIR`. When the file gets shorter
while `LastLines` reads it, the error matches `io.EOF`.

```go
r := filesystem.NewReader()
lines, err := r.LastLines(path, 10)
if err != nil {
    return err
}
for _, line := range lines {
    fmt.Println(line)
}
return nil
```

### Absolute and real paths: `Path`

`NewPath()` returns a `*Path`; like `Locator`, it holds no state.

| Method | Returns | Missing target |
|---|---|---|
| `p.Absolute(path)` | `path` as an absolute path. An absolute path stays as given. A relative path is joined to the real working folder with `filepath.Separator` (`/` on Linux and macOS) and not cleaned; an empty path gives the real working folder. | allowed |
| `p.Real(path)` | an absolute path with every link resolved, like `cd <path> && pwd -P`. `.` and `..` are removed by name first, so `/a/link/..` is `/a` wherever `link` points. | error that matches `os.ErrNotExist` |
| `p.RealDir(path)` | the real path of a folder (as `Real` gives it) and `true`. It gives `""` and `false` when `path` is not absolute, when the target is missing, or when it is not a folder. | `""` and `false` |

Errors: no `Err` values. `Absolute` fails only when the working folder can not be read, for example
when it was removed. `RealDir` gives no error, only `false`, also for errors such as a missing
permission.

```go
p := filesystem.NewPath()
root, err := p.Real(".")
if err != nil {
    return err
}
fmt.Println("working folder:", root)

if dir, ok := p.RealDir(path); ok {
    fmt.Println("folder:", dir)
}
return nil
```

### Link to the first free name: `Writer.LinkFirstFree`

`w.LinkFirstFree(temp, dir, base, ext, limit)` hard links the file `temp` to the first free name in
`dir` and returns the new path. It tries `base+ext`, then `base-2+ext`, `base-3+ext` and so on up to
`base-<limit>+ext`, for example `report.txt`, `report-2.txt`, ..., `report-99.txt`. A link never
replaces a file and never follows a link at the target name, so a name that holds a file, a folder
or a link (a broken one too) counts as taken.

`base+ext` must be a plain file name. An empty name, `.`, `..` or a name with a `/` gives an error
that matches `os.ErrInvalid`, and nothing is linked, so a name from user input can not put the link
outside `dir`.

Every link error, the one for a refused name too, is an `*os.LinkError`, so `errors.As` gives `temp`
as `Old`. For a refused name, `New` is `base+ext` as given; for any other link error it is the name
joined to `dir`.

`LinkFirstFree` does not remove `temp`: the caller does. A hard link works only within one file
system, so write `temp` in `dir`.

Errors:

- `ErrNoFreeName`: every name up to `limit` is taken, or `limit` is below 1.
- An error that matches `os.ErrInvalid`: `base+ext` is not a plain file name.
- Any other link error stops at once, for example one that matches `os.ErrNotExist` when `dir` is
  missing.

With `WriteTemp`, the new file never shows up half written and never replaces another file. When
the link is made but the temp file can not be removed, the new file is still there, so keep its
path:

```go
w := filesystem.NewWriter(filesystem.NewReader())
temp, err := w.WriteTemp(dir, ".report.*", data, 0o644)
if err != nil {
    return "", err
}
path, err := w.LinkFirstFree(temp, dir, "report", ".txt", 99)
removeErr := w.Remove(temp)
if err != nil {
    return "", errors.Join(err, removeErr)
}
if removeErr != nil {
    log.Printf("temp file left: %v", removeErr)
}
return path, nil
```

---
