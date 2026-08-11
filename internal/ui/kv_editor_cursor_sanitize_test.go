package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSanitizeCursorText_StripsHostilePayloadsAndKeepsCursorOnChar guards
// overlayCursor (the K/V editor's Key-field cursor overlay): the edit
// buffer is seeded straight from a cluster value, so before this fix a
// bidi override, raw CSI, or OSC-52 clipboard write in a Secret/ConfigMap
// annotation key survived byte for byte the moment the user pressed edit
// on that row.
func TestSanitizeCursorText_StripsHostilePayloadsAndKeepsCursorOnChar(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"bidi override", "ab\u202eXcd"},
		{"raw csi", "ab\x1b[31mXcd"},
		{"csi screen erase", "ab\x1b[2JXcd"},
		{"osc52 clipboard write", "ab\x1b]52;c;aGF4\x07Xcd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursor := strings.IndexByte(tc.raw, 'X')
			require.GreaterOrEqual(t, cursor, 0)

			sanitized, newCursor := sanitizeCursorText(tc.raw, cursor)

			assert.NotContains(t, sanitized, "\x1b")
			assert.NotContains(t, sanitized, "\x07")
			assert.NotContains(t, sanitized, "\u202e")
			require.True(t, newCursor >= 0 && newCursor < len(sanitized), "cursor %d out of range for %q", newCursor, sanitized)
			assert.Equal(t, byte('X'), sanitized[newCursor], "cursor should still land on the marker character")
		})
	}
}

// TestSanitizeCursorBody_StripsHostilePayloadsAndKeepsCursorOnChar mirrors
// the above for overlayCursorMultiline (the K/V editor's Value-field
// cursor overlay), which handles multi-line Secret/ConfigMap bodies.
func TestSanitizeCursorBody_StripsHostilePayloadsAndKeepsCursorOnChar(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"bidi override", "line1\nab\u202eXcd"},
		{"raw csi", "line1\nab\x1b[31mXcd"},
		{"csi screen erase", "line1\nab\x1b[2JXcd"},
		{"osc52 clipboard write", "line1\nab\x1b]52;c;aGF4\x07Xcd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursor := strings.IndexByte(tc.raw, 'X')
			require.GreaterOrEqual(t, cursor, 0)

			sanitized, newCursor := sanitizeCursorBody(tc.raw, cursor)

			assert.NotContains(t, sanitized, "\x1b")
			assert.NotContains(t, sanitized, "\x07")
			assert.NotContains(t, sanitized, "\u202e")
			require.True(t, newCursor >= 0 && newCursor < len(sanitized), "cursor %d out of range for %q", newCursor, sanitized)
			assert.Equal(t, byte('X'), sanitized[newCursor], "cursor should still land on the marker character")
		})
	}
}

// TestOverlayCursor_SanitizesActiveAndInactive guards the actual render
// entry point: both branches (active field with a live cursor, and an
// inactive field rendered plain) must scrub the ESC byte out of the raw
// payload before Truncate ever measures the string's width. Checking the
// whole payload substring (ESC included), not a bare "\x1b", because the
// cursor's own reverse-video styling legitimately emits ESC elsewhere in
// the output; sanitizing only needs to break the ESC+body pairing, not
// scrub every ESC byte on the line.
func TestOverlayCursor_SanitizesActiveAndInactive(t *testing.T) {
	payload := "secret\x1b]52;c;aGF4\x07value"

	// Assert on the OSC introducer, not the whole payload. The cursor's
	// styling is inserted at offset 3, which splits the payload string, so a
	// whole-payload check passes even when the sequence comes through intact.
	// "\x1b]" opens an OS command and never belongs in rendered output, while
	// the cursor's own reverse-video SGR legitimately carries ESC.
	const oscIntroducer = "\x1b]"

	active := overlayCursor(payload, 3, true, 80)
	assert.NotContains(t, active, oscIntroducer)
	assert.NotContains(t, active, "\a", "BEL terminates the sequence")

	inactive := overlayCursor(payload, 3, false, 80)
	assert.NotContains(t, inactive, oscIntroducer)
	assert.NotContains(t, inactive, "\a")
}

// TestOverlayCursorMultiline_SanitizesHostilePayload guards the Value
// field's multi-line cursor overlay the same way.
func TestOverlayCursorMultiline_SanitizesHostilePayload(t *testing.T) {
	csi := "\x1b[31m"
	osc52 := "\x1b]52;c;aGF4\x07"
	bidi := "\u202e"
	payload := "line one\nsecret" + csi + bidi + "value" + osc52

	out := overlayCursorMultiline(payload, 5, true, 0, 40, 10)
	assert.NotContains(t, out, csi)
	assert.NotContains(t, out, osc52)
	assert.NotContains(t, out, bidi)
}
