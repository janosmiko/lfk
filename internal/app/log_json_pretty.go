package app

import (
	"bytes"
	"encoding/json"
	"strings"
)

// PrettyPrintJSON renders the decoded JSON value v as a pretty-printed
// multi-line string with 2-space indentation. Returns an empty string when
// v is nil or cannot be marshalled.
//
// The pretty-printer preserves numeric precision by relying on the
// encoding/json MarshalIndent path with the decoder having used
// json.Number — if v carries json.Number values, they round-trip
// verbatim (no scientific notation, no float drift).
func PrettyPrintJSON(v any, indent int) string {
	if v == nil {
		return ""
	}
	prefix := ""
	unit := "  "
	// The `indent` parameter is the nesting level offset; we always use
	// 2 spaces per level. Callers pass 0 in the base case; >0 can be
	// used to shift the whole block right when embedding inside another
	// block (not used in v1 but retained for a future fold mode).
	if indent > 0 {
		prefix = strings.Repeat(unit, indent)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent(prefix, unit)
	if err := enc.Encode(v); err != nil {
		return ""
	}
	out := buf.String()
	// json.Encoder appends a trailing newline after the value; strip it
	// so the caller can choose whether the rendered line ends with a
	// newline (the expansion helper splits on "\n" so the trailing
	// newline would produce an empty tail line otherwise).
	out = strings.TrimRight(out, "\n")
	return out
}

// jsonLineLookup abstracts the JSON-detection lookup so the pretty helper
// can be exercised in tests without building a full Model. The production
// caller passes `(*Model).jsonLineAt`.
type jsonLineLookup func(idx int) JSONLine

// BuildPrettyVisibleLines returns a parallel slice where each element
// corresponds to one source-buffer index referenced by visibleIndices
// (or to logLines when visibleIndices is nil). For JSON lines, the
// returned string is the pretty-printed multi-line form (with embedded
// `\n`). For non-JSON lines, the returned string is the original line
// unchanged.
//
// The renderer is expected to split each returned element on `\n` and
// treat each fragment as a separate visual sub-line — the same way the
// wrap mode treats wrapped sub-lines. This piggybacks on the existing
// wrapping visual-line machinery so scroll/cursor math stays consistent
// without a bespoke visual-index map.
//
// The `lookup` argument is a closure over the JSON cache so we don't
// have to import Model's internals here (and tests can pass a stub).
func BuildPrettyVisibleLines(lines []string, visibleIndices []int, lookup jsonLineLookup) []string {
	if len(lines) == 0 {
		return nil
	}

	if visibleIndices == nil {
		out := make([]string, len(lines))
		for i, line := range lines {
			out[i] = prettifyOneLine(line, lookup(i))
		}
		return out
	}

	out := make([]string, len(visibleIndices))
	for i, srcIdx := range visibleIndices {
		if srcIdx < 0 || srcIdx >= len(lines) {
			out[i] = ""
			continue
		}
		out[i] = prettifyOneLine(lines[srcIdx], lookup(srcIdx))
	}
	return out
}

// prettifyOneLine returns the multi-line pretty form for a JSON line, or
// the unchanged original for a non-JSON line. The payload returned by
// the detector has the leading prefix (pod/container, RFC3339, klog)
// stripped, so the pretty-printed output is prefix-less — the caller
// preserves the original line for non-JSON cases, meaning any prefix
// stays intact for those.
//
// For JSON lines, if the original had a recognised prefix, we preserve
// it as the header of the first visual sub-line so the user can still
// tell which pod/container emitted the JSON blob.
func prettifyOneLine(raw string, detected JSONLine) string {
	if !detected.IsJSON {
		return raw
	}
	pretty := PrettyPrintJSON(detected.Value, 0)
	if pretty == "" {
		return raw
	}
	// Reconstruct any prefix that was stripped out by the detector. If
	// the original line ends with the payload, whatever comes before is
	// the prefix (which includes trailing whitespace). If not (e.g. the
	// payload had trailing whitespace trimmed differently), we fall
	// back to the bare pretty form.
	if detected.Payload != "" && strings.HasSuffix(raw, detected.Payload) {
		prefix := raw[:len(raw)-len(detected.Payload)]
		if prefix != "" {
			return prefix + pretty
		}
	}
	return pretty
}
