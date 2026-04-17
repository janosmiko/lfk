package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseLogSinceDuration parses a duration string suitable for kubectl's
// --since flag.  It accepts the same syntax that time.ParseDuration
// understands (e.g. "30s", "5m", "1h30m"), plus a convenience "d"
// suffix for whole days (e.g. "2d" becomes 48h).
//
// On success it returns:
//   - the parsed time.Duration,
//   - the canonical display string (the user's original, trimmed input),
//   - a nil error.
//
// On failure it returns the zero duration, an empty display string,
// and a non-nil error describing what went wrong.  An empty input is
// considered invalid — callers that want to *clear* the filter treat
// empty input explicitly instead of going through this helper.  A
// non-positive duration (e.g. "0s", "-5m") is also rejected because
// kubectl rejects them and the chip would be meaningless.
func parseLogSinceDuration(s string) (time.Duration, string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, "", fmt.Errorf("duration is empty")
	}

	// Accept a bare "d" suffix as shorthand for days.  kubectl doesn't
	// understand "d" on its own; we convert to hours before handing off
	// to time.ParseDuration so the parser does the rest of the work.
	parseInput := trimmed
	if strings.HasSuffix(parseInput, "d") && !strings.ContainsAny(parseInput, "hms") {
		// "2d" -> "48h".  The numeric prefix may be a float (e.g. "1.5d").
		numPart := strings.TrimSuffix(parseInput, "d")
		n, err := strconv.ParseFloat(numPart, 64)
		if err != nil {
			return 0, "", fmt.Errorf("invalid duration %q: %w", s, err)
		}
		parseInput = strconv.FormatFloat(n*24, 'f', -1, 64) + "h"
	}

	d, err := time.ParseDuration(parseInput)
	if err != nil {
		return 0, "", fmt.Errorf("invalid duration %q: %w", s, err)
	}
	if d <= 0 {
		return 0, "", fmt.Errorf("duration must be positive: %q", s)
	}

	return d, trimmed, nil
}
