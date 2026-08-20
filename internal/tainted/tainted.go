// Package tainted carries cluster-sourced strings in a wrapper that cannot
// reach the terminal without an explicit, named sanitizer call.
//
// Everything lfk renders comes from the cluster, so any principal able to
// create an object in a watched namespace controls those strings. A raw
// escape sequence on the terminal means OSC-52 clipboard writes, screen
// rewrites via CSI, and text reordering via bidi overrides. Reviews caught
// this four times and still missed sibling fields in the same Sprintf, so
// the guarantee is moved into the type system instead.
//
// The package is a leaf: it imports nothing from lfk, so internal/k8s,
// internal/model and internal/ui can all use it without an import cycle.
// The sanitizer implementations live here (see sanitize.go) rather than in
// internal/ui for the same reason. internal/ui re-exports them.
package tainted

import (
	"fmt"
	"strings"
)

// unsanitizedMarker is what a String renders as when it reaches fmt without
// an unwrap. It is deliberately loud and greppable: a leak becomes a visible
// bug report rather than silent terminal control.
const unsanitizedMarker = "[tainted:unsanitized]"

// String holds a cluster-controlled value. The payload is unexported, so the
// value cannot be concatenated, compared to a literal, or passed where a
// plain string is expected. Reaching the payload requires choosing one of
// the two unwraps below, which is the point: the choice between the
// single-line and the body sanitizer is a real decision that a default would
// hide.
type String struct {
	raw string
}

// Wrap marks a value as cluster-controlled. Call it at the boundary where
// the value enters lfk - the API response decode - not at the render site.
func Wrap(s string) String {
	return String{raw: s}
}

// Line unwraps for a single-line sink: a table cell, a title, a status
// message. Drops C0/C1 controls and bidi overrides entirely. No ANSI or tab
// survives, because a short cell has no legitimate use for either.
func (t String) Line() string {
	return SanitizeTerminalText(t.raw)
}

// Body unwraps for a multi-line sink: a log body, a describe pane, command
// output. Strips bidi overrides, then keeps SGR colour sequences when
// renderAnsi is set and expands tabs. Use Line for anything that lands in a
// single cell.
func (t String) Body(renderAnsi bool) string {
	return SanitizeLogBody(StripBidiOverrides(t.raw), renderAnsi)
}

// Is reports whether the payload equals a trusted literal. For branching on
// known API enum values (Event Type "Warning", a condition status) without
// unwrapping. The argument is a constant in lfk's own source, never another
// cluster value.
func (t String) Is(literal string) bool {
	return t.raw == literal
}

// IsEmpty reports whether the payload is empty, so "render this row only if
// the field is set" needs no unwrap.
func (t String) IsEmpty() bool {
	return t.raw == ""
}

// Compare orders two payloads like strings.Compare, for sorting without
// unwrapping.
func (t String) Compare(other String) int {
	return strings.Compare(t.raw, other.raw)
}

// Contains reports whether the payload contains sub, for filter matching
// without unwrapping.
func (t String) Contains(sub string) bool {
	return strings.Contains(t.raw, sub)
}

// Raw returns the payload unsanitized. It exists for the non-render paths
// that need the exact bytes the cluster sent: sending a value back to the
// API server, a map key, a file write. Never pass the result to anything
// that reaches the terminal - use Line or Body. Every call is a deliberate
// exception and should be greppable as such.
func (t String) Raw() string {
	return t.raw
}

// Format makes String print the marker under every verb, so a String that
// slips into a Sprintf leaks nothing. Implementing fmt.Formatter rather than
// only fmt.Stringer matters: %v, %q, %d and %#v all route here, whereas a
// bare String method leaves %#v printing the struct with its payload.
func (t String) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(unsanitizedMarker))
}

// String satisfies fmt.Stringer for the paths that call it directly rather
// than going through fmt.
func (t String) String() string {
	return unsanitizedMarker
}
