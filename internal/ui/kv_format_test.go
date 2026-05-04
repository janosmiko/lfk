package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNeedsYAMLQuote_SpecialScalarsForceQuoting pins the
// CodeRabbit-flagged regression: YAML special scalar values
// (booleans / null / numbers) that *look* like strings but parse
// as bool / nil / numeric must be quoted so the YAML round-trip
// preserves string-ness — otherwise a configmap value like "true"
// or "8080" silently changes type when re-parsed.
func TestNeedsYAMLQuote_SpecialScalarsForceQuoting(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		// Booleans (YAML 1.1 + 1.2).
		{"lower true", "true"},
		{"title True", "True"},
		{"upper TRUE", "TRUE"},
		{"lower false", "false"},
		{"yes", "yes"},
		{"no", "no"},
		{"on", "on"},
		{"off", "off"},
		{"y", "y"},
		{"n", "n"},
		// Null variants.
		{"null", "null"},
		{"Null", "Null"},
		{"tilde", "~"},
		// Numeric strings — port numbers, timeouts, version pins.
		{"int", "8080"},
		{"negative int", "-1"},
		{"float", "1.5"},
		{"int with sign", "+42"},
		{"scientific", "1e5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, needsYAMLQuote(tc.val),
				"value %q must be quoted to round-trip as a string", tc.val)
		})
	}
}

// TestNeedsYAMLQuote_PlainStringsStayUnquoted keeps the
// over-quoting in check — values that read unambiguously as plain
// strings should NOT be wrapped.
func TestNeedsYAMLQuote_PlainStringsStayUnquoted(t *testing.T) {
	cases := []string{
		"hello",
		"some-name",
		"v1.2.3-alpha",
		"https//example.com", // no colon → safe
		"abc123",
		"Mixed_Case",
	}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			assert.False(t, needsYAMLQuote(v),
				"plain string %q should not need quoting", v)
		})
	}
}

// TestFormatKVPairs_YAMLQuotesSpecialValues exercises the
// integration path: a value that needs quoting must come out of
// FormatKVPairs wrapped in %q so a YAML parser sees a string.
func TestFormatKVPairs_YAMLQuotesSpecialValues(t *testing.T) {
	out, label := FormatKVPairs(
		[]KVPair{
			{Key: "ENABLED", Value: "true"},
			{Key: "PORT", Value: "8080"},
			{Key: "NAME", Value: "frontend"},
		},
		KVFormatYAML,
	)
	assert.Equal(t, "yaml", label)
	// Quoted (forced by needsYAMLQuote): true and 8080 are wrapped.
	assert.Contains(t, out, `ENABLED: "true"`,
		"boolean-string must round-trip as quoted string, not unquoted bool")
	assert.Contains(t, out, `PORT: "8080"`,
		"numeric-string must round-trip as quoted string, not unquoted int")
	// Unquoted: plain string passes through.
	assert.Contains(t, out, "NAME: frontend",
		"plain string must NOT be over-quoted")
	// Sanity: no stray escape artefacts.
	assert.False(t, strings.Contains(out, "ENABLED: true\n"),
		"unquoted 'true' must not appear (would parse as boolean)")
}
