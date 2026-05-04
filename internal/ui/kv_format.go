package ui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// kv_format.go owns the Shift+Y "copy in chosen format" plumbing:
// the canonical format list, the format-picker chip-row renderer,
// and the FormatKVPairs serialiser used by the app layer to build
// the clipboard payload.

// KVFormat enumerates the formats the K/V editor can copy selected
// pairs as. Order is the picker's display order — keep YAML first
// since it's the most kubectl-friendly default.
type KVFormat int

const (
	KVFormatYAML KVFormat = iota
	KVFormatJSON
	KVFormatDotenv
	KVFormatKeyEqValue
	KVFormatValuesOnly
)

// KVFormatEntry pairs a format with its display label. Exported so
// the app-layer handlers can iterate the same list the picker shows
// (keeps cursor index <-> chosen format in sync without duplicating
// the list literal).
type KVFormatEntry struct {
	Format KVFormat
	Label  string
}

// KVFormats is the format list in display order. Adding an entry
// here surfaces it in the picker AND the format-cursor handler
// automatically.
var KVFormats = []KVFormatEntry{
	{KVFormatYAML, "YAML"},
	{KVFormatJSON, "JSON"},
	{KVFormatDotenv, "dotenv"},
	{KVFormatKeyEqValue, "key=value"},
	{KVFormatValuesOnly, "values"},
}

// KVPair carries a single key/value to FormatKVPairs. Pulled out as
// its own type so the app layer can build slices in any order
// (filtered by selection / cursor position) without re-exposing the
// editor's internal map shape.
type KVPair struct {
	Key   string
	Value string
}

// FormatKVPairs serialises pairs into the requested format. Returns
// the clipboard-ready string + a human label suitable for a status
// message ("yaml", "json", ...). Unknown formats fall back to YAML
// so a stale formatCursor can't produce empty output.
func FormatKVPairs(pairs []KVPair, format KVFormat) (string, string) {
	switch format {
	case KVFormatJSON:
		// json.Marshal preserves insertion order via a sorted-map
		// approximation: we stream pairs into a map[string]string and
		// accept Go's randomised iteration. For order-sensitive output
		// (rare for K/V copy), users can pick YAML / dotenv instead.
		out := make(map[string]string, len(pairs))
		for _, p := range pairs {
			out[p.Key] = p.Value
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return "", "json"
		}
		return string(b) + "\n", "json"
	case KVFormatDotenv:
		// dotenv: always-quoted values so whitespace, =, and special
		// chars survive `source` of the file. Conservative — over-
		// quoting is safe; under-quoting silently breaks downstream.
		var b strings.Builder
		for _, p := range pairs {
			fmt.Fprintf(&b, "%s=%q\n", p.Key, p.Value)
		}
		return b.String(), "dotenv"
	case KVFormatKeyEqValue:
		var b strings.Builder
		for _, p := range pairs {
			fmt.Fprintf(&b, "%s=%s\n", p.Key, p.Value)
		}
		return b.String(), "key=value"
	case KVFormatValuesOnly:
		var b strings.Builder
		for _, p := range pairs {
			b.WriteString(p.Value)
			b.WriteByte('\n')
		}
		return b.String(), "values"
	default: // KVFormatYAML
		// Plain `key: value` lines. Quote values that would be
		// ambiguous (collection / scalar markers, embedded colons /
		// hashes / newlines) so the result round-trips through any
		// YAML parser.
		var b strings.Builder
		for _, p := range pairs {
			if needsYAMLQuote(p.Value) {
				fmt.Fprintf(&b, "%s: %q\n", p.Key, p.Value)
			} else {
				fmt.Fprintf(&b, "%s: %s\n", p.Key, p.Value)
			}
		}
		return b.String(), "yaml"
	}
}

// needsYAMLQuote returns true when a value's first character or
// content would change YAML's parsing (collection / scalar markers,
// reserved indicators) AND when the value is a YAML special scalar
// — booleans, null, or a number — that would round-trip as a non-
// string type if emitted unquoted. K/V editor values are always
// strings (k8s configmap / secret / label data), so over-quoting is
// safe; a missed case silently changes the user's clipboard from
// `"true"` (string) to `true` (bool) when re-parsed.
func needsYAMLQuote(v string) bool {
	if v == "" {
		return false
	}
	// YAML 1.1/1.2 special scalar words that parse as bool / null
	// when unquoted (case-insensitive — YAML accepts "True", "TRUE"
	// and so on as the same token).
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "y", "n", "null", "~":
		return true
	}
	// Anything that parses as a YAML number (int, float, sign, exponent)
	// — quote so the round-trip preserves string-ness.
	if isYAMLNumber(v) {
		return true
	}
	switch v[0] {
	case '"', '\'', '{', '[', '|', '>', '*', '&', '%', '!', '@', '`', '#', '-', ' ':
		return true
	}
	return strings.ContainsAny(v, ":\n\r#")
}

// isYAMLNumber reports whether v parses as a YAML number — covers
// integers, signed integers, decimals, and scientific notation. A
// configmap / secret value of "8080" or "1.5" must come out of
// FormatKVPairs as a quoted string so a YAML parser keeps treating
// it as a string instead of an int / float.
func isYAMLNumber(v string) bool {
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	return false
}

// RenderKVFormatPicker paints the Shift+Y format chip row that sits
// above the editor's table when formatActive is set. The cursor chip
// uses OverlaySelectedStyle so the highlight is visible regardless
// of theme; idle chips render flat on the editor's baseBg. Trailing
// hint reminds the user of the apply/cancel keys so they don't have
// to leave the picker to look it up.
func RenderKVFormatPicker(cursor int) string {
	var b strings.Builder
	b.WriteString(BarDimStyle.Render("Copy as: "))
	for i, entry := range KVFormats {
		chip := " " + entry.Label + " "
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(chip))
		} else {
			b.WriteString(BarNormalStyle.Render(chip))
		}
		if i < len(KVFormats)-1 {
			b.WriteString(BarDimStyle.Render(" "))
		}
	}
	b.WriteString(BarDimStyle.Render("  ↵ apply  esc cancel"))
	return lipgloss.NewStyle().Background(BaseBg).Render(b.String())
}
