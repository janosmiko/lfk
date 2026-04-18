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
//
// Note: the display string is what the user typed (e.g. "30d"). That
// is NOT a valid kubectl --since argument — kubectl doesn't understand
// the "d" suffix. Use kubectlSinceArg below when building kubectl args.
func parseLogSinceDuration(s string) (time.Duration, string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, "", fmt.Errorf("duration is empty")
	}

	parseInput, err := convertDaysSuffixToHours(trimmed)
	if err != nil {
		return 0, "", fmt.Errorf("invalid duration %q: %w", s, err)
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

// kubectlSinceArg returns the kubectl-compatible form of a user-typed
// duration string. kubectl's --since only understands s/m/h (no "d"),
// so "30d" is expanded to "720h". Other inputs pass through unchanged.
// Returns an empty string (and no error) when the input is empty —
// callers should treat empty as "no --since".
func kubectlSinceArg(userInput string) string {
	trimmed := strings.TrimSpace(userInput)
	if trimmed == "" {
		return ""
	}
	out, err := convertDaysSuffixToHours(trimmed)
	if err != nil {
		// Fall back to the raw input; kubectl will reject it with a
		// readable error that the user can see in the status bar.
		return trimmed
	}
	return out
}

// convertDaysSuffixToHours converts an optional "d" / fractional-days
// suffix to hours (e.g. "2d" → "48h", "1.5d" → "36h"). Inputs without
// a "d" suffix pass through unchanged. "d" mixed with explicit h/m/s
// units (e.g. "1d12h") is unsupported and rejected — kubectl's own
// parser doesn't support compound day strings either.
func convertDaysSuffixToHours(s string) (string, error) {
	if !strings.HasSuffix(s, "d") {
		return s, nil
	}
	if strings.ContainsAny(s, "hms") {
		return "", fmt.Errorf("combined day + h/m/s not supported: %q", s)
	}
	numPart := strings.TrimSuffix(s, "d")
	n, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(n*24, 'f', -1, 64) + "h", nil
}
