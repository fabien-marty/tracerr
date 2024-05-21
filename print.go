package tracerr

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// DefaultLinesAfter is number of source lines after traced line to display.
var DefaultLinesAfter = 2

// DefaultLinesBefore is number of source lines before traced line to display.
var DefaultLinesBefore = 3

// DefaultIgnoreFirstFrames is the number of first frames to ignore
var DefaultIgnoreFirstFrames = 0

// DefaultMaxFrames is the total number of frames to print/return (0 means "no limit")
var DefaultMaxFrames = 0

// DefaultIgnoreFirstFrames is the number of last frames to ignore
var DefaultIgnoreLastFrames = 0

var cache = map[string][]string{}

var mutex sync.RWMutex

// Print prints error message with stack trace.
func Print(err error) {
	fmt.Println(Sprint(err))
}

// PrintSource prints error message with stack trace and source fragments.
//
// By default, 6 lines of source code will be printed,
// see DefaultLinesAfter and DefaultLinesBefore.
//
// Pass a single number to specify a total number of source lines.
//
// Pass two numbers to specify exactly how many lines should be shown
// before and after traced line.
func PrintSource(err error, nums ...int) {
	fmt.Println(SprintSource(err, nums...))
}

// PrintSourceColor prints error message with stack trace and source fragments,
// which are in color.
// Output rules are the same as in PrintSource.
func PrintSourceColor(err error, nums ...int) {
	fmt.Println(SprintSourceColor(err, nums...))
}

// Sprint returns error output by the same rules as Print.
func Sprint(err error) string {
	return sprint(err, []int{0}, false)
}

// SprintSource returns error output by the same rules as PrintSource.
func SprintSource(err error, nums ...int) string {
	return sprint(err, nums, false)
}

// SprintSourceColor returns error output by the same rules as PrintSourceColor.
func SprintSourceColor(err error, nums ...int) string {
	return sprint(err, nums, true)
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
	cache[path] = lines
	return lines, nil
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
		// TODO Pad to the same length.
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

func sprint(err error, nums []int, colorized bool) string {
	if err == nil {
		return ""
	}
	e, ok := err.(Error)
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
	rows = append(rows, e.Error())
	if withSource {
		rows = append(rows, "")
	}
	i := 0
	appendedFrames := 0
	for _, frame := range frames {
		i++
		if i <= DefaultIgnoreFirstFrames {
			continue
		}
		message := frame.String()
		if colorized {
			message = bold(message)
		}
		rows = append(rows, message)
		if withSource {
			rows = sourceRows(rows, frame, before, after, colorized)
		}
		appendedFrames++
		if DefaultMaxFrames > 0 && appendedFrames >= DefaultMaxFrames {
			break
		}
		if DefaultIgnoreLastFrames > 0 && i + DefaultIgnoreLastFrames >= len(frames) {
			break
		}
	}
	return strings.Join(rows, "\n")
}
