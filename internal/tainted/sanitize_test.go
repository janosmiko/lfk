package tainted_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/tainted"
)

// TestSGRParameterBytesRejectPrivateMarkersAndIntermediates is TASK-885: the
// CSI parameter scan in parseSGRSequence accepted the whole 0x30-0x3F range,
// which includes the private markers < = > ?. CSI > Ps ; Ps m is XTMODKEYS -
// xterm reads it as a request to change keyboard modifier reporting - so a
// cluster-controlled log line carrying that marker altered terminal state
// after rendering. This asserts each private marker, plus a CSI intermediate
// byte, is rejected rather than forwarded.
func TestSGRParameterBytesRejectPrivateMarkersAndIntermediates(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"private marker <", "pre\x1b[<4;2mpost"},
		{"private marker =", "pre\x1b[=4;2mpost"},
		{"private marker >", "pre\x1b[>4;2mpost"},
		{"private marker ?", "pre\x1b[?4;2mpost"},
		{"intermediate byte", "pre\x1b[ mpost"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tainted.Wrap(tc.payload).Body(true)
			assert.NotContains(t, out, "\x1b[", "payload %q should not survive as a CSI sequence", tc.payload)
		})
	}
}

// TestSGRParameterBytesKeepLegitimateColour guards the reason the parameter
// forwarding exists at all: plain SGR colour, including both truecolour
// separator forms, resets, and combined attributes, must still render when
// renderAnsi is true.
func TestSGRParameterBytesKeepLegitimateColour(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"8-colour", "pre\x1b[31mpost", "\x1b[31m"},
		{"256-colour", "pre\x1b[38;5;196mpost", "\x1b[38;5;196m"},
		{"truecolour semicolon", "pre\x1b[38;2;255;0;0mpost", "\x1b[38;2;255;0;0m"},
		{"truecolour colon", "pre\x1b[38:2:255:0:0mpost", "\x1b[38:2:255:0:0m"},
		{"reset", "pre\x1b[0mpost", "\x1b[0m"},
		{"combined attributes", "pre\x1b[1;4;31mpost", "\x1b[1;4;31m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tainted.Wrap(tc.payload).Body(true)
			assert.Contains(t, out, tc.want, "payload %q should keep its SGR sequence", tc.payload)
		})
	}
}

func TestSanitizeLogBodyDropsEscapesAndControls(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"progress-bar redraw", "\x1b[1A\x1b[2Kfrontend/en_AU", "frontend/en_AU"},
		{"bare ESC", "pre\x1bpost", "prepost"},
		{"ESC at end", "pre\x1b", "pre"},
		{"raw C1 CSI", "pre\x9b1A\x9b2Kpost", "prepost"},
		{"UTF-8 C1 CSI", "pre\u009b1A\u009b2Kpost", "prepost"},
		{"unterminated C1 CSI", "pre\u009b12", "pre"},
		{"raw C1 other", "pre\x85post", "prepost"},
		{"UTF-8 C1 other", "pre\u0085post", "prepost"},
		{"control bytes", "a\x00b\rc\x7fd", "abcd"},
		{"OSC title ended by BEL", "pre\x1b]0;title\apost", "prepost"},
		{"OSC hyperlink ended by ESC ST", "\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\", "link"},
		{"OSC unterminated", "pre\x1b]0;title", "pre"},
		{"DCS ended by ESC ST", "pre\x1bPq#0\x1b\\post", "prepost"},
		{"raw C1 OSC ended by raw ST", "pre\x9d0;title\x9cpost", "prepost"},
		{"UTF-8 C1 OSC ended by UTF-8 ST", "pre\u009d0;title\u009cpost", "prepost"},
		{"OSC payload with Ü is not cut early", "pre\x1b]0;Ü\apost", "prepost"},
		{"plain text", "plain", "plain"},
		{"tab expands", "a\tb", "a       b"},
		{"multibyte kept", "café 日本", "café 日本"},
		{"invalid non-C1 byte kept", "a\xe9b", "a\xe9b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, renderAnsi := range []bool{false, true} {
				assert.Equal(t, tc.want, tainted.SanitizeLogBody(tc.payload, renderAnsi))
			}
		})
	}
}
