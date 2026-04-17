package ui

import (
	"fmt"
	"strings"
	"time"
)

// nowFunc returns the current time for relative-timestamp rendering.
// Tests swap this out to pin "now" to a deterministic value; the log
// viewer per-frame snapshot reads it exactly once so every line in a
// frame uses the same reference point.
var nowFunc = time.Now

// ParseLogLineTimestamp attempts to extract a leading RFC3339 or
// RFC3339Nano timestamp from a log line. It tolerates the optional
// kubectl `--prefix` envelope (e.g. `[pod/foo/bar] 2024-01-15T10:30:00Z ...`)
// by looking past the closing bracket before searching for the
// timestamp.
//
// On success it returns:
//   - the parsed time.Time,
//   - the exact leading substring that was consumed (including any
//     kubectl prefix and the trailing space, so the caller can slice
//     it off without re-computing the cut),
//   - true.
//
// On failure (no timestamp, malformed input, or not enough characters)
// it returns the zero time, an empty string, and false. Callers are
// expected to leave the original line untouched when ok is false.
//
// This mirrors the app-side helper (app.parseLogLineTimestamp) — the
// layering rule "internal/ui cannot import internal/app" forces a
// second copy here so the renderer can call it directly in the
// per-line loop without a package-crossing callback.
func ParseLogLineTimestamp(line string) (time.Time, string, bool) {
	if line == "" {
		return time.Time{}, "", false
	}

	// Walk past an optional kubectl prefix: `[pod/name/container] `. The
	// bracketed form always ends with `] ` — we preserve those two
	// characters in the consumed span so the caller's line[len(span):]
	// starts at the first character after the timestamp+space.
	prefixLen := 0
	rest := line
	if line[0] == '[' {
		close := strings.Index(line, "] ")
		if close < 0 {
			return time.Time{}, "", false
		}
		prefixLen = close + 2
		rest = line[prefixLen:]
	}

	// The timestamp terminates at the first space. kubectl writes
	// RFC3339Nano (`2024-01-15T10:30:00.123456789Z`) but non-kubectl
	// sources commonly emit RFC3339 without nanoseconds
	// (`2024-01-15T10:30:00Z`). Both end at the first space character.
	space := strings.IndexByte(rest, ' ')
	if space <= 0 {
		return time.Time{}, "", false
	}
	candidate := rest[:space]

	// A plausible RFC3339 timestamp needs a `T` at position 10
	// (`YYYY-MM-DDT...`). Checking this cheaply rules out most log
	// lines before we pay for the full parse.
	if len(candidate) < 20 || candidate[10] != 'T' {
		return time.Time{}, "", false
	}

	ts, err := time.Parse(time.RFC3339Nano, candidate)
	if err != nil {
		// Fall back to plain RFC3339 (no fractional seconds).
		ts, err = time.Parse(time.RFC3339, candidate)
		if err != nil {
			return time.Time{}, "", false
		}
	}

	// Consumed span: the kubectl prefix (if any) + the timestamp + the
	// trailing space. The caller uses len(consumed) as the slice index.
	consumed := line[:prefixLen+space+1]
	return ts, consumed, true
}

// FormatRelativeTimestamp renders ts relative to now as a compact
// human-readable string. Rules:
//
//   - |delta| < 1s            → "just now"
//   - delta within past 60s   → "{n}s ago"
//   - delta within past 60m   → "{n}m ago"
//   - delta within past 24h   → "{n}h ago"
//   - delta >= 24h in the past → "{n}d ago"
//   - future timestamp within 60s → "just now"
//   - future beyond 60s       → "in {n}s" / "in {n}m" / "in {n}h" / "in {n}d"
//
// The helper is pure: it reads neither the clock nor any package
// state, which keeps it trivially testable and independent from
// rendering concerns.
func FormatRelativeTimestamp(ts, now time.Time) string {
	delta := now.Sub(ts)

	if delta < 0 {
		future := -delta
		if future < time.Second {
			return "just now"
		}
		switch {
		case future < time.Minute:
			return fmt.Sprintf("in %ds", int(future.Seconds()))
		case future < time.Hour:
			return fmt.Sprintf("in %dm", int(future.Minutes()))
		case future < 24*time.Hour:
			return fmt.Sprintf("in %dh", int(future.Hours()))
		default:
			return fmt.Sprintf("in %dd", int(future.Hours()/24))
		}
	}

	if delta < time.Second {
		return "just now"
	}
	switch {
	case delta < time.Minute:
		return fmt.Sprintf("%ds ago", int(delta.Seconds()))
	case delta < time.Hour:
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
	}
}

// RewriteLogLineTimestamp rewrites the leading RFC3339 timestamp on
// line to its relative form (see FormatRelativeTimestamp). When line
// has no timestamp prefix (ok=false from ParseLogLineTimestamp) it is
// returned unchanged.
//
// The rewrite replaces the timestamp in place; any kubectl `[pod/...]`
// prefix and the trailing space are preserved so the rest of the
// layout (colorization, cursor columns) stays consistent.
func RewriteLogLineTimestamp(line string, now time.Time) string {
	ts, consumed, ok := ParseLogLineTimestamp(line)
	if !ok {
		return line
	}
	// `consumed` is "[prefix] TIMESTAMP " — if a kubectl prefix is
	// present it ends at the `] ` boundary we preserved. Everything
	// between that boundary and the trailing space is the RFC3339
	// timestamp we want to replace.
	prefixEnd := strings.Index(consumed, "] ")
	if prefixEnd >= 0 {
		prefixEnd += 2 // include "] "
	} else {
		prefixEnd = 0
	}
	return consumed[:prefixEnd] + FormatRelativeTimestamp(ts, now) + " " + line[len(consumed):]
}
