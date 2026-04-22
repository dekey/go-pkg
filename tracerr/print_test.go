package tracerr

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func resetCache(t *testing.T, seed map[string][]string) {
	t.Helper()
	mutex.Lock()
	cache = make(map[string][]string)
	for k, v := range seed {
		cache[k] = v
	}
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
			path:       func(t *testing.T) string { return "/nonexistent/tracerr_test_file.go" },
			wantErrSub: "not found",
		},
		"reads file and populates cache": {
			path:      func(t *testing.T) string { return tmpFile(t, "line1\nline2\nline3") },
			wantLines: []string{"line1", "line2", "line3"},
		},
		"returns from cache, skips disk read": {
			path: func(t *testing.T) string { return "/fake/cached.go" },
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
