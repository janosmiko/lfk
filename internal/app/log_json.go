package app

import (
	"encoding/json"
	"io"
	"strings"
)

// JSONLine is the cached result of a JSON-detection pass on a raw log line.
// It is intentionally a value type: the cache stores copies so later consumers
// (pretty-print renderer, field-query evaluator) can read without coordinating
// on a pointer.
type JSONLine struct {
	// IsJSON reports whether, after prefix stripping, the line parses as a
	// single top-level JSON object or array.
	IsJSON bool
	// Value is nil when IsJSON is false. When true, it holds the parsed
	// top-level JSON object/array as a generic map[string]any or []any.
	// Numbers use json.Number (preserving precision) so field filters
	// can compare big ints without float drift.
	Value any
	// Payload is the substring of the original line that was parsed as
	// JSON (i.e. minus any leading pod/container prefix, RFC3339
	// timestamp, or klog header). Empty when IsJSON is false.
	Payload string
}

// DetectJSONLine inspects a raw log line and returns its JSONLine. Never
// returns an error — malformed JSON is reported as IsJSON=false. The
// detector strips a leading "[pod/container] " prefix, a leading RFC3339
// timestamp (with or without nanoseconds), and/or a leading klog header
// before attempting to parse, so kubectl --timestamps output classifies
// correctly.
//
// Prefix stripping is applied in order, first match wins per layer:
//  1. Pod/container bracket prefix: "[<pod>/<container>] "
//  2. RFC3339 timestamp: "2026-04-16T10:00:30.123456789Z "
//  3. klog header: "I0412 12:34:56.789012       1 main.go:123] "
//
// After stripping, the payload must start with '{' or '[' and end with
// the matching '}' or ']' (allowing trailing whitespace) before decode
// is even attempted. JSON embedded mid-line is intentionally NOT
// recognised — that is a separate parsing problem for a later phase.
func DetectJSONLine(line string) JSONLine {
	payload := stripJSONLinePrefixes(line)

	// Cheap gates before decode.
	if len(payload) < 2 {
		return JSONLine{}
	}
	first := payload[0]
	if first != '{' && first != '[' {
		return JSONLine{}
	}
	// Verify the trailing (non-whitespace) character matches.
	last := lastNonSpaceByte(payload)
	switch first {
	case '{':
		if last != '}' {
			return JSONLine{}
		}
	case '[':
		if last != ']' {
			return JSONLine{}
		}
	}

	// Decode with UseNumber so numeric fields retain their exact form.
	dec := json.NewDecoder(strings.NewReader(payload))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return JSONLine{}
	}
	// Reject multi-value payloads: a single trailing token means the
	// payload was not a single top-level value.
	if _, err := dec.Token(); err != io.EOF {
		return JSONLine{}
	}
	switch v.(type) {
	case map[string]any, []any:
		return JSONLine{IsJSON: true, Value: v, Payload: payload}
	default:
		// Bare numbers, strings, booleans, nulls — not what we want here.
		return JSONLine{}
	}
}

// stripJSONLinePrefixes removes a leading "[pod/container] " prefix, a
// leading RFC3339 timestamp, and/or a leading klog header in that order.
// Any combination is allowed; each layer is only stripped if it actually
// matches.
func stripJSONLinePrefixes(line string) string {
	line = stripPodContainerPrefix(line)
	line = strings.TrimLeft(line, " \t")
	line = stripRFC3339Prefix(line)
	line = strings.TrimLeft(line, " \t")
	line = stripKlogPrefix(line)
	line = strings.TrimLeft(line, " \t")
	return line
}

// stripPodContainerPrefix removes a leading "[<pod>/<container>] " prefix
// (as produced by kubectl logs --prefix). The bracket segment MUST contain
// a '/' to avoid mis-stripping bracketed level markers like "[WARN]".
func stripPodContainerPrefix(line string) string {
	if len(line) == 0 || line[0] != '[' {
		return line
	}
	end := strings.Index(line, "] ")
	if end <= 0 {
		return line
	}
	if !strings.Contains(line[1:end], "/") {
		return line
	}
	return line[end+2:]
}

// stripRFC3339Prefix removes a leading RFC3339 timestamp and trailing
// space. Both second and nanosecond precision are accepted. Returns the
// original string when no timestamp is present.
//
// Kept as a small inline check (rather than calling time.Parse on every
// line) because DetectJSONLine runs on every streamed log line. A bad
// candidate bails out in constant time after the length/format gates.
func stripRFC3339Prefix(s string) string {
	// Minimum form: "2024-01-15T10:30:00Z " (21 chars).
	if len(s) < 21 {
		return s
	}
	// YYYY-MM-DDTHH:MM:SS shape.
	if s[4] != '-' || s[7] != '-' || s[10] != 'T' || s[13] != ':' || s[16] != ':' {
		return s
	}
	for _, i := range []int{0, 1, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15, 17, 18} {
		if s[i] < '0' || s[i] > '9' {
			return s
		}
	}
	// Now scan past the optional fractional seconds + zone offset until
	// we hit a space.
	spaceIdx := strings.IndexByte(s, ' ')
	if spaceIdx < 20 || spaceIdx > 40 {
		return s
	}
	// Zone: must end in 'Z' or have a '+'/'-' sign somewhere after index 19.
	end := spaceIdx - 1
	tz := s[end]
	if tz != 'Z' && tz != '+' && tz != '-' {
		// Offsets like "+00:00" or "-07:00" — zone sign is not at the
		// last position. Walk back to find the zone marker.
		found := false
		for i := end; i >= 19; i-- {
			if s[i] == '+' || s[i] == '-' || s[i] == 'Z' {
				found = true
				break
			}
		}
		if !found {
			return s
		}
	}
	return s[spaceIdx+1:]
}

// stripKlogPrefix removes a leading klog/glog header of the form
// "I0412 12:34:56.789012       1 main.go:123] ". First char is one of
// IWEF (info/warn/error/fatal), followed by 4 digits (MMDD), space,
// HH:MM:SS[.microseconds], whitespace, goroutine/thread id, whitespace,
// file:line, then "] ". Returns the original string on miss.
func stripKlogPrefix(s string) string {
	// Minimum form: "I0412 12:34:56.000000       1 a.go:1] " (~38 chars).
	if len(s) < 30 {
		return s
	}
	switch s[0] {
	case 'I', 'W', 'E', 'F':
	default:
		return s
	}
	// MMDD.
	for i := 1; i <= 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return s
		}
	}
	if s[5] != ' ' {
		return s
	}
	// HH:MM:SS.
	if len(s) < 14 || s[8] != ':' || s[11] != ':' {
		return s
	}
	// The header always ends with "] " somewhere later in the line.
	end := strings.Index(s, "] ")
	if end <= 0 {
		return s
	}
	return s[end+2:]
}

// lastNonSpaceByte returns the last non-ASCII-whitespace byte of s.
// Returns 0 when s is empty or all whitespace — which will fail the
// subsequent bracket-match check, correctly rejecting the line.
func lastNonSpaceByte(s string) byte {
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return s[i]
		}
	}
	return 0
}
