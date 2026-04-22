//nolint:testpackage
package tracerr

import (
	"errors"
	"maps"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func resetCache(t *testing.T, seed map[string][]string) {
	t.Helper()
	mutex.Lock()
	cache = make(map[string][]string)
	maps.Copy(cache, seed)
	mutex.Unlock()
	t.Cleanup(func() {
		mutex.Lock()
		cache = make(map[string][]string)
		mutex.Unlock()
	})
}

func assertReadLines(t *testing.T, got []string, err error, wantLines []string, wantErrSub string) {
	t.Helper()
	if wantErrSub != "" {
		require.ErrorContains(t, err, wantErrSub)
		return
	}
	require.NoError(t, err)
	require.Equal(t, wantLines, got)
}

func TestReadLines(t *testing.T) {
	tmpFile := func(t *testing.T, content string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "*.go")
		require.NoError(t, err)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		f.Close()
		return f.Name()
	}

	tests := map[string]struct {
		path       func(t *testing.T) string
		seedCache  map[string][]string
		wantLines  []string
		wantErrSub string
	}{
		"file not found": {
			path:       func(_ *testing.T) string { return "/nonexistent/tracerr_test_file.go" },
			wantErrSub: "not found",
		},
		"reads file and populates cache": {
			path:      func(t *testing.T) string { return tmpFile(t, "line1\nline2\nline3") },
			wantLines: []string{"line1", "line2", "line3"},
		},
		"returns from cache, skips disk read": {
			path: func(_ *testing.T) string { return "/fake/cached.go" },
			seedCache: map[string][]string{
				"/fake/cached.go": {"cached_line1", "cached_line2"},
			},
			wantLines: []string{"cached_line1", "cached_line2"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resetCache(t, tc.seedCache)
			got, err := readLines(tc.path(t))
			assertReadLines(t, got, err, tc.wantLines, tc.wantErrSub)
		})
	}
}

func TestSprintFunc(t *testing.T) {
	frame1 := Frame{Func: "pkg.Func1", Line: 10, Path: "/fake/a.go"}
	frame2 := Frame{Func: "pkg.Func2", Line: 20, Path: "/fake/b.go"}

	join := func(parts ...string) string {
		return strings.Join(parts, "\n\r")
	}

	tests := map[string]struct {
		err  error
		want string
	}{
		"nil returns empty": {
			err:  nil,
			want: "",
		},
		"non-traceable returns message only": {
			err:  errors.New("plain error"),
			want: "plain error",
		},
		"no frames": {
			err:  &errorData{err: errors.New("test error"), frames: nil},
			want: join("test error", "\n\r"),
		},
		"single frame: plain text, no source, no color": {
			err:  &errorData{err: errors.New("test error"), frames: []Frame{frame1}},
			want: join("test error", frame1.String(), "\n\r"),
		},
		"multiple frames: all plain, no source between them": {
			err:  &errorData{err: errors.New("test error"), frames: []Frame{frame1, frame2}},
			want: join("test error", frame1.String(), frame2.String(), "\n\r"),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := Sprint(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSprintSourceColor(t *testing.T) {
	fakeFrame := Frame{Func: "pkg.TestFunc", Line: 42, Path: "/fake/path.go"}
	fakeErr := &errorData{err: errors.New("test error"), frames: []Frame{fakeFrame}}

	f, err := os.CreateTemp(t.TempDir(), "*.go")
	require.NoError(t, err)
	_, err = f.WriteString("line1\nline2\nline3\nline4\nline5")
	require.NoError(t, err)
	f.Close()
	realFrame := Frame{Func: "pkg.RealFunc", Line: 3, Path: f.Name()}
	realErr := &errorData{err: errors.New("real error"), frames: []Frame{realFrame}}

	join := func(parts ...string) string {
		return strings.Join(parts, "\n\r")
	}

	tests := map[string]struct {
		err  error
		nums []int
		want string
	}{
		"nil returns empty": {
			err:  nil,
			want: "",
		},
		"non-traceable returns message only": {
			err:  errors.New("plain error"),
			want: "plain error",
		},
		"no nums: with source, frame bold, file error yellow": {
			err: fakeErr,
			want: join(
				"test error",
				"",
				bold(fakeFrame.String()),
				yellow("tracerr: file /fake/path.go not found"),
				"",
				"\n\r",
			),
		},
		"zero: no source, frame bold": {
			err:  fakeErr,
			nums: []int{0},
			want: join("test error", bold(fakeFrame.String()), "\n\r"),
		},
		"two zero values: with source, current line red, no context": {
			err:  realErr,
			nums: []int{0, 0},
			want: join("real error", "", bold(realFrame.String()), red("3\tline3"), "", "\n\r"),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := SprintSourceColor(tt.err, tt.nums...)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSprint(t *testing.T) {
	fakeFrame := Frame{Func: "pkg.TestFunc", Line: 42, Path: "/fake/path.go"}
	fakeErr := &errorData{err: errors.New("test error"), frames: []Frame{fakeFrame}}

	join := func(parts ...string) string {
		return strings.Join(parts, "\n\r")
	}

	tests := map[string]struct {
		err       error
		nums      []int
		colorized bool
		want      string
	}{
		"nil returns empty": {
			err:  nil,
			want: "",
		},
		"non-traceable returns message only": {
			err:  errors.New("plain error"),
			want: "plain error",
		},
		"no frames no source": {
			err:  &errorData{err: errors.New("test error"), frames: nil},
			nums: []int{0},
			want: join("test error", "\n\r"),
		},
		"single frame no source": {
			err:  fakeErr,
			nums: []int{0},
			want: join("test error", fakeFrame.String(), "\n\r"),
		},
		"single frame with source invalid path": {
			err:  fakeErr,
			nums: []int{},
			want: join("test error", "", fakeFrame.String(), "tracerr: file /fake/path.go not found", "", "\n\r"),
		},
		"colorized frame no source": {
			err:       fakeErr,
			nums:      []int{0},
			colorized: true,
			want:      join("test error", bold(fakeFrame.String()), "\n\r"),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := sprint(tt.err, tt.nums, tt.colorized)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSourceRows(t *testing.T) {
	writeTmp := func(content string) string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "*.go")
		require.NoError(t, err)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		f.Close()
		return f.Name()
	}

	fiveLinesFile := writeTmp("line1\nline2\nline3\nline4\nline5")

	tests := map[string]struct {
		frame     Frame
		before    int
		after     int
		colorized bool
		want      []string
	}{
		"file not found": {
			frame: Frame{Path: "/nonexistent/tracerr_sourcerows_test.go", Line: 1},
			want:  []string{"tracerr: file /nonexistent/tracerr_sourcerows_test.go not found", ""},
		},
		"colorized file not found": {
			frame:     Frame{Path: "/nonexistent/tracerr_sourcerows_test.go", Line: 1},
			colorized: true,
			want:      []string{yellow("tracerr: file /nonexistent/tracerr_sourcerows_test.go not found"), ""},
		},
		"too few lines": {
			frame: Frame{Path: fiveLinesFile, Line: 99},
			want:  []string{"tracerr: too few lines, got 5, want 99", ""},
		},
		"colorized too few lines": {
			frame:     Frame{Path: fiveLinesFile, Line: 99},
			colorized: true,
			want:      []string{yellow("tracerr: too few lines, got 5, want 99"), ""},
		},
		"only current line": {
			frame:  Frame{Path: fiveLinesFile, Line: 3},
			before: 0,
			after:  0,
			want:   []string{"3\tline3", ""},
		},
		"context window": {
			frame:  Frame{Path: fiveLinesFile, Line: 3},
			before: 1,
			after:  1,
			want:   []string{"2\tline2", "3\tline3", "4\tline4", ""},
		},
		"clipped at start": {
			frame:  Frame{Path: fiveLinesFile, Line: 1},
			before: 3,
			after:  1,
			want:   []string{"1\tline1", "2\tline2", ""},
		},
		"clipped at end": {
			frame:  Frame{Path: fiveLinesFile, Line: 5},
			before: 1,
			after:  3,
			want:   []string{"4\tline4", "5\tline5", ""},
		},
		"colorized current line uses red": {
			frame:     Frame{Path: fiveLinesFile, Line: 3},
			before:    0,
			after:     0,
			colorized: true,
			want:      []string{red("3\tline3"), ""},
		},
		"colorized context uses black line numbers": {
			frame:     Frame{Path: fiveLinesFile, Line: 3},
			before:    1,
			after:     0,
			colorized: true,
			want:      []string{black("2") + "\tline2", red("3\tline3"), ""},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := sourceRows(nil, tt.frame, tt.before, tt.after, tt.colorized)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCalcRows(t *testing.T) {
	tests := map[string]struct {
		nums       []int
		wantBefore int
		wantAfter  int
		wantSource bool
	}{
		"no args uses defaults": {
			nums:       []int{},
			wantBefore: DefaultLinesBefore,
			wantAfter:  DefaultLinesAfter,
			wantSource: true,
		},
		"single zero disables source": {
			nums:       []int{0},
			wantBefore: 0,
			wantAfter:  0,
			wantSource: false,
		},
		"single negative disables source": {
			nums:       []int{-1},
			wantBefore: 0,
			wantAfter:  0,
			wantSource: false,
		},
		"single one shows only current line": {
			nums:       []int{1},
			wantBefore: 0,
			wantAfter:  0,
			wantSource: true,
		},
		"single three splits symmetrically": {
			nums:       []int{3},
			wantBefore: 1,
			wantAfter:  1,
			wantSource: true,
		},
		"single even total gives extra line to before": {
			nums:       []int{4},
			wantBefore: 2,
			wantAfter:  1,
			wantSource: true,
		},
		"two args set before and after explicitly": {
			nums:       []int{2, 3},
			wantBefore: 2,
			wantAfter:  3,
			wantSource: true,
		},
		"two negative args clamped to zero": {
			nums:       []int{-1, -2},
			wantBefore: 0,
			wantAfter:  0,
			wantSource: true,
		},
		"more than two args uses first two only": {
			nums:       []int{1, 2, 99},
			wantBefore: 1,
			wantAfter:  2,
			wantSource: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotBefore, gotAfter, gotSource := calcRows(tt.nums)
			require.Equal(t, tt.wantBefore, gotBefore, "before")
			require.Equal(t, tt.wantAfter, gotAfter, "after")
			require.Equal(t, tt.wantSource, gotSource, "withSource")
		})
	}
}

// TestReadLinesConcurrentCacheMiss exercises the double-checked locking path:
// multiple goroutines miss the cache simultaneously, call os.ReadFile concurrently,
// and then race to write. The re-check under the write lock must ensure all
// goroutines receive consistent results and exactly one entry lands in the cache.
func TestReadLinesConcurrentCacheMiss(t *testing.T) {
	tests := map[string]struct {
		content    string
		goroutines int
		wantLines  []string
	}{
		"few goroutines": {
			content:    "line1\nline2\nline3",
			goroutines: 5,
			wantLines:  []string{"line1", "line2", "line3"},
		},
		"many goroutines": {
			content:    "line1\nline2\nline3",
			goroutines: 50,
			wantLines:  []string{"line1", "line2", "line3"},
		},
		"single line file": {
			content:    "only one line",
			goroutines: 20,
			wantLines:  []string{"only one line"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			resetCache(t, nil)

			f, err := os.CreateTemp(t.TempDir(), "*.go")
			require.NoError(t, err)
			_, err = f.WriteString(tc.content)
			require.NoError(t, err)
			f.Close()
			path := f.Name()

			results := make([][]string, tc.goroutines)
			errs := make([]error, tc.goroutines)

			var wg sync.WaitGroup
			wg.Add(tc.goroutines)
			for i := range tc.goroutines {
				go func(idx int) {
					defer wg.Done()
					results[idx], errs[idx] = readLines(path)
				}(i)
			}
			wg.Wait()

			for i := range tc.goroutines {
				assertReadLines(t, results[i], errs[i], tc.wantLines, "")
			}

			mutex.RLock()
			cacheLen := len(cache)
			mutex.RUnlock()
			require.Equal(t, 1, cacheLen, "cache must have exactly one entry after concurrent writes")
		})
	}
}
