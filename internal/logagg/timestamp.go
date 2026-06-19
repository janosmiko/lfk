package logagg

import (
	"strings"
	"time"
)

// SplitTimestamp strips the leading RFC3339Nano timestamp that
// "kubectl logs --timestamps" prepends to each line. ok is false when the
// line has no parseable leading timestamp.
func SplitTimestamp(line string) (time.Time, string, bool) {
	idx := strings.IndexByte(line, ' ')
	if idx <= 0 {
		return time.Time{}, line, false
	}
	ts, err := time.Parse(time.RFC3339Nano, line[:idx])
	if err != nil {
		return time.Time{}, line, false
	}
	return ts, line[idx+1:], true
}
