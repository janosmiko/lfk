package tainted_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/tainted"
)

// hostile carries every escape class the threat model cares about: OSC-52
// clipboard write, a CSI screen erase, and a bidi override that reorders the
// text after it.
const hostile = "ok\x1b]52;c;aGF4\x07\x1b[2Jsafe\u202etxet"

// TestFormatVerbsNeverEmitPayload is the core guarantee: a String that
// reaches fmt without an unwrap must emit the marker, not the payload. It
// covers %v and %#v specifically because a plain String() method leaves
// those printing the struct with its payload intact.
func TestFormatVerbsNeverEmitPayload(t *testing.T) {
	v := tainted.Wrap(hostile)
	for _, verb := range []string{"%s", "%v", "%q", "%d", "%#v", "%+v"} {
		out := fmt.Sprintf(verb, v)
		assert.NotContains(t, out, "\x1b", "verb %s emitted a raw ESC", verb)
		assert.NotContains(t, out, "\u202e", "verb %s emitted a bidi override", verb)
		assert.Contains(t, out, "[tainted:unsanitized]", "verb %s should emit the marker", verb)
	}
}

// TestFormatInsideStructNeverEmitsPayload covers the failure that actually
// shipped four times: a sibling field printed raw in the same Sprintf as
// sanitized ones. Printing the whole struct must not leak either.
func TestFormatInsideStructNeverEmitsPayload(t *testing.T) {
	s := struct {
		Reason  tainted.String
		Message tainted.String
	}{tainted.Wrap(hostile), tainted.Wrap(hostile)}

	// %d is in the list on purpose: a String with only a Stringer method
	// survives %v but leaks the payload through fmt's wrong-verb reporting.
	for _, verb := range []string{"%v", "%+v", "%d"} {
		out := fmt.Sprintf(verb, s)
		assert.NotContains(t, out, "\x1b", "verb %s leaked an ESC from a struct field", verb)
		assert.NotContains(t, out, "\u202e", "verb %s leaked a bidi override from a struct field", verb)
	}
}

func TestStringerEmitsMarker(t *testing.T) {
	assert.Equal(t, "[tainted:unsanitized]", tainted.Wrap(hostile).String())
}

// TestLineStripsEverySinkUnsafeRune checks the single-line unwrap drops
// controls, ESC and bidi overrides while keeping ordinary text.
func TestLineStripsEverySinkUnsafeRune(t *testing.T) {
	out := tainted.Wrap(hostile).Line()
	assert.NotContains(t, out, "\x1b")
	assert.NotContains(t, out, "\u202e")
	assert.NotContains(t, out, "\x07")
	assert.Contains(t, out, "safe")
}

// TestBodyStripsBidiAndKeepsSGR checks the body unwrap makes the other
// trade-off: colour survives, reordering does not.
func TestBodyStripsBidiAndKeepsSGR(t *testing.T) {
	out := tainted.Wrap("\x1b[32mgreen\x1b[0m\u202eflip").Body(true)
	assert.Contains(t, out, "\x1b[32m", "SGR colour should survive the body unwrap")
	assert.NotContains(t, out, "\u202e", "bidi override must not survive the body unwrap")

	plain := tainted.Wrap("\x1b[2Jerase").Body(false)
	assert.NotContains(t, plain, "\x1b", "a non-SGR CSI must not survive even with renderAnsi off")
}

// TestNonRenderAccessorsAvoidUnwrapping documents that branching, sorting
// and filtering do not need a sanitizer, so nobody reaches for Raw to do
// them.
func TestNonRenderAccessorsAvoidUnwrapping(t *testing.T) {
	warn := tainted.Wrap("Warning")
	assert.True(t, warn.Is("Warning"))
	assert.False(t, warn.Is("Normal"))
	assert.False(t, warn.IsEmpty())
	assert.True(t, tainted.Wrap("").IsEmpty())
	assert.True(t, warn.Contains("arn"))
	assert.Negative(t, tainted.Wrap("a").Compare(tainted.Wrap("b")))
}

func TestRawReturnsPayloadVerbatim(t *testing.T) {
	assert.Equal(t, hostile, tainted.Wrap(hostile).Raw())
}

func TestWrapAllPreservesOrder(t *testing.T) {
	got := tainted.WrapAll([]string{"a", "b", "c"})
	var b strings.Builder
	for _, v := range got {
		b.WriteString(v.Raw())
	}
	assert.Equal(t, "abc", b.String())
}
