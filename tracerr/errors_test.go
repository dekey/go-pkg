package tracerr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dekey/go-pkg/tracerr"
	"github.com/stretchr/testify/require"
)

func TestIsTraceableError(t *testing.T) {
	traced := tracerr.New("traced")

	tests := map[string]struct {
		err  error
		want bool
	}{
		"nil returns false":         {err: nil, want: false},
		"plain error returns false": {err: errors.New("plain"), want: false},
		"traceable returns true":    {err: traced, want: true},
		"traceable wrapped in fmt.Errorf returns true": {
			err:  fmt.Errorf("outer: %w", traced),
			want: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, tracerr.IsTraceableError(tt.err))
		})
	}
}

func TestUnwrap(t *testing.T) {
	plain := errors.New("plain error")
	traced := tracerr.New("traced error")

	tests := map[string]struct {
		err    error
		assert func(t *testing.T, result error)
	}{
		"nil returns nil": {
			err: nil,
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.Nil(t, result)
			},
		},
		"plain error returns itself": {
			err: plain,
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.ErrorIs(t, result, plain)
			},
		},
		"traceable returns underlying plain error": {
			err: traced,
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "traced error", result.Error())
				require.False(t, tracerr.IsTraceableError(result))
			},
		},
		"traceable wrapped in fmt.Errorf returns inner underlying error": {
			err: fmt.Errorf("outer: %w", traced),
			assert: func(t *testing.T, result error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "traced error", result.Error())
				require.False(t, tracerr.IsTraceableError(result))
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.assert(t, tracerr.Unwrap(tt.err))
		})
	}
}

func TestStackTrace(t *testing.T) {
	traced := tracerr.New("traced error")

	tests := map[string]struct {
		err    error
		assert func(t *testing.T, frames []tracerr.Frame)
	}{
		"nil returns nil": {
			err: nil,
			assert: func(t *testing.T, frames []tracerr.Frame) {
				t.Helper()
				require.Nil(t, frames)
			},
		},
		"plain error returns nil": {
			err: errors.New("plain"),
			assert: func(t *testing.T, frames []tracerr.Frame) {
				t.Helper()
				require.Nil(t, frames)
			},
		},
		"traceable error returns frames": {
			err: traced,
			assert: func(t *testing.T, frames []tracerr.Frame) {
				t.Helper()
				require.NotEmpty(t, frames)
				require.Contains(t, frames[0].Func, "TestStackTrace")
			},
		},
		"traceable wrapped in fmt.Errorf returns inner frames": {
			err: fmt.Errorf("outer: %w", traced),
			assert: func(t *testing.T, frames []tracerr.Frame) {
				t.Helper()
				require.NotEmpty(t, frames)
				require.Contains(t, frames[0].Func, "TestStackTrace")
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.assert(t, tracerr.StackTrace(tt.err))
		})
	}
}

func TestWrap(t *testing.T) {
	plain := errors.New("plain error")
	traced := tracerr.New("traced error")
	outerWrapped := fmt.Errorf("context: %w", traced)

	tests := map[string]struct {
		err    error
		assert func(t *testing.T, result tracerr.Error)
	}{
		"nil returns nil": {
			err: nil,
			assert: func(t *testing.T, result tracerr.Error) {
				t.Helper()
				require.Nil(t, result)
			},
		},
		"plain error gets stack trace": {
			err: plain,
			assert: func(t *testing.T, result tracerr.Error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "plain error", result.Error())
				require.NotEmpty(t, result.StackTrace())
				require.Contains(t, result.StackTrace()[0].Func, "TestWrap")
				require.ErrorIs(t, result, plain)
			},
		},
		"already traceable is returned unchanged": {
			err: traced,
			assert: func(t *testing.T, result tracerr.Error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "traced error", result.Error())
				require.NotEmpty(t, result.StackTrace())
				require.Contains(t, result.StackTrace()[0].Func, "TestWrap")
				require.ErrorIs(t, result, traced)
			},
		},
		"traceable wrapped in fmt.Errorf preserves outer context": {
			err: outerWrapped,
			assert: func(t *testing.T, result tracerr.Error) {
				t.Helper()
				require.NotNil(t, result)
				require.Equal(t, "context: traced error", result.Error())
				require.NotEmpty(t, result.StackTrace())
				require.Contains(t, result.StackTrace()[0].Func, "TestWrap")
				require.ErrorIs(t, result, traced)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.assert(t, tracerr.Wrap(tt.err))
		})
	}
}

func TestNew(t *testing.T) {
	tests := map[string]struct {
		message string
	}{
		"plain message":             {message: "plain message"},
		"empty string":              {message: ""},
		"message with percent sign": {message: "100% done"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tracerr.New(tt.message)

			require.NotNil(t, err)
			require.Equal(t, tt.message, err.Error())

			frames := err.StackTrace()
			require.NotEmpty(t, frames)
			require.Contains(t, frames[0].Func, "TestNew")
		})
	}
}

func TestErrorf(t *testing.T) {
	sentinel := errors.New("sentinel")

	tests := map[string]struct {
		message     string
		args        []any
		wantMsg     string
		wantIsWraps bool
	}{
		"plain message": {
			message: "plain message",
			wantMsg: "plain message",
		},
		"single format arg": {
			message: "value is %d",
			args:    []any{42},
			wantMsg: "value is 42",
		},
		"multiple format args": {
			message: "a=%d b=%s",
			args:    []any{1, "hello"},
			wantMsg: "a=1 b=hello",
		},
		"wraps error with %w": {
			message:     "context: %w",
			args:        []any{sentinel},
			wantMsg:     "context: sentinel",
			wantIsWraps: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tracerr.Errorf(tt.message, tt.args...)

			require.NotNil(t, err)
			require.Equal(t, tt.wantMsg, err.Error())

			frames := err.StackTrace()
			require.NotEmpty(t, frames)
			// DefaultSkip=2 must strip trace and Errorf; first frame is the caller.
			require.Contains(t, frames[0].Func, "TestErrorf")

			require.Equal(t, tt.wantIsWraps, errors.Is(err, sentinel))
		})
	}
}
