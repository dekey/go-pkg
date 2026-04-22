package tracerr

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	// DefaultLinesAfter is number of source lines after traced line to display.
	DefaultLinesAfter = 2

	// DefaultLinesBefore is number of source lines before traced line to display.
	DefaultLinesBefore = 3
)

var (
	mutex sync.RWMutex
	cache = map[string][]string{}
)

// Sprint returns error message with stack trace as string.
func Sprint(err error) string {
	return sprint(err, []int{0}, false)
}

// SprintSourceColor returns error message with stack trace and source fragments.
//
// By default, 6 lines of source code will be printed,
// see DefaultLinesAfter and DefaultLinesBefore.
//
// Pass a single number to specify a total number of source lines.
//
// Pass two numbers to specify exactly how many lines should be shown
// before and after traced line.
func SprintSourceColor(err error, nums ...int) string {
	return sprint(err, nums, true)
}

func sprint(err error, nums []int, colorized bool) string {
	if err == nil {
		return ""
	}

	var e Error
	ok := errors.As(err, &e)
	if !ok {
		return err.Error()
	}

	before, after, withSource := calcRows(nums)
	frames := e.StackTrace()
	expectedRows := len(frames) + 1
	if withSource {
		expectedRows = (before+after+3)*len(frames) + 2
	}
	rows := make([]string, 0, expectedRows)
	// will return first error message, it should be built by fmt.Errorf to see the whole message
	rows = append(rows, err.Error())
	if withSource {
		rows = append(rows, "")
	}
	for _, frame := range frames {
		message := frame.String()
		if colorized {
			message = bold(message)
		}
		rows = append(rows, message)
		if withSource {
			rows = sourceRows(rows, frame, before, after, colorized)
		}
	}

	rows = append(rows, "\n\r")
	return strings.Join(rows, "\n\r")
}

func calcRows(nums []int) (before, after int, withSource bool) {
	before = DefaultLinesBefore
	after = DefaultLinesAfter
	withSource = true
	if len(nums) > 1 {
		before = nums[0]
		after = nums[1]
		withSource = true
	} else if len(nums) == 1 {
		if nums[0] > 0 {
			// Extra line goes to "before" rather than "after".
			after = (nums[0] - 1) / 2
			before = nums[0] - after - 1
		} else {
			after = 0
			before = 0
			withSource = false
		}
	}
	if before < 0 {
		before = 0
	}
	if after < 0 {
		after = 0
	}
	return before, after, withSource
}

func sourceRows(rows []string, frame Frame, before, after int, colorized bool) []string {
	lines, err := readLines(frame.Path)
	if err != nil {
		message := err.Error()
		if colorized {
			message = yellow(message)
		}
		return append(rows, message, "")
	}
	if len(lines) < frame.Line {
		message := fmt.Sprintf(
			"tracerr: too few lines, got %d, want %d",
			len(lines), frame.Line,
		)
		if colorized {
			message = yellow(message)
		}
		return append(rows, message, "")
	}
	current := frame.Line - 1
	start := current - before
	end := current + after
	for i := start; i <= end; i++ {
		if i < 0 || i >= len(lines) {
			continue
		}
		line := lines[i]
		var message string
		if i == frame.Line-1 {
			message = fmt.Sprintf("%d\t%s", i+1, line)
			if colorized {
				message = red(message)
			}
		} else if colorized {
			message = fmt.Sprintf("%s\t%s", black(strconv.Itoa(i+1)), line)
		} else {
			message = fmt.Sprintf("%d\t%s", i+1, line)
		}
		rows = append(rows, message)
	}
	return append(rows, "")
}

func readLines(path string) ([]string, error) {
	mutex.RLock()
	lines, ok := cache[path]
	mutex.RUnlock()
	if ok {
		return lines, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tracerr: file %s not found", path)
	}
	lines = strings.Split(string(b), "\n")
	mutex.Lock()
	defer mutex.Unlock()
	if _, ok := cache[path]; !ok {
		cache[path] = lines
	}

	return cache[path], nil
}
