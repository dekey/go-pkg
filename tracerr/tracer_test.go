package tracerr_test

import (
	"errors"
	"testing"

	"github.com/dekey/go-pkg/tracerr"
	"github.com/stretchr/testify/require"
)

func TestErrorDataUnwrap(t *testing.T) {
	cause := errors.New("cause")

	tests := map[string]struct {
		err    tracerr.Error
		assert func(t *testing.T, result error)
	}{
		"New returns underlying plain error": {
			err: tracerr.New("msg"),
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "msg", result.Error())
				require.False(t, tracerr.IsTraceableError(result))
			},
		},
		"Errorf without wrapping returns plain error": {
			err: tracerr.Errorf("value %d", 1),
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "value 1", result.Error())
				require.False(t, tracerr.IsTraceableError(result))
			},
		},
		"Errorf with %w preserves chain": {
			err: tracerr.Errorf("context: %w", cause),
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "context: cause", result.Error())
				require.ErrorIs(t, result, cause)
			},
		},
		"Wrap returns the original error": {
			err: tracerr.Wrap(cause),
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.NotNil(t, result)
				require.ErrorIs(t, result, cause)
				require.False(t, tracerr.IsTraceableError(result))
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.assert(t, tt.err.Unwrap())
		})
	}
}

func TestErrorDataStackTrace(t *testing.T) {
	tests := map[string]struct {
		err tracerr.Error
	}{
		"New":    {err: tracerr.New("msg")},
		"Errorf": {err: tracerr.Errorf("msg %d", 1)},
		"Wrap":   {err: tracerr.Wrap(errors.New("msg"))},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			frames := tt.err.StackTrace()
			require.NotEmpty(t, frames)
			require.Contains(t, frames[0].Func, "TestErrorDataStackTrace")
			require.NotEmpty(t, frames[0].Path)
			require.Positive(t, frames[0].Line)
		})
	}
}

func TestErrorDataError(t *testing.T) {
	tests := map[string]struct {
		err  tracerr.Error
		want string
	}{
		"New": {
			err:  tracerr.New("plain message"),
			want: "plain message",
		},
		"Errorf single arg": {
			err:  tracerr.Errorf("value is %d", 42),
			want: "value is 42",
		},
		"Errorf with %w wrapping": {
			err:  tracerr.Errorf("context: %w", errors.New("cause")),
			want: "context: cause",
		},
		"Wrap": {
			err:  tracerr.Wrap(errors.New("wrapped")),
			want: "wrapped",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestFrameString(t *testing.T) {
	tests := map[string]struct {
		frame tracerr.Frame
		want  string
	}{
		"all fields set": {
			frame: tracerr.Frame{Func: "pkg.Func", Line: 42, Path: "path/to/file.go"},
			want:  "path/to/file.go:42 pkg.Func()",
		},
		"zero line": {
			frame: tracerr.Frame{Func: "pkg.Func", Line: 0, Path: "path/to/file.go"},
			want:  "path/to/file.go:0 pkg.Func()",
		},
		"empty path": {
			frame: tracerr.Frame{Func: "pkg.Func", Line: 42, Path: ""},
			want:  ":42 pkg.Func()",
		},
		"empty func": {
			frame: tracerr.Frame{Func: "", Line: 42, Path: "path/to/file.go"},
			want:  "path/to/file.go:42 ()",
		},
		"zero value": {
			frame: tracerr.Frame{},
			want:  ":0 ()",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.frame.String())
		})
	}
}
