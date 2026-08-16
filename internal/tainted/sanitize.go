package tainted

// The sanitizers below were moved verbatim from internal/ui so that
// internal/k8s and internal/model - which must not import internal/ui - can
// reach them through tainted.String. internal/ui re-exports each one, so its
// existing call sites are unchanged.

import (
	"strings"
	"unicode/utf8"
)

// SanitizeTerminalText strips the control characters described above from any
// cluster-controlled string before it reaches the screen. Exported so every
// such string passes through one implementation: the YAML blame gutter shows
// field manager names, which a non-core apiserver can set to anything.
//
// It also drops the bidi embedding, override, and isolate characters
// (U+202A-U+202E, U+2066-U+2069). Those reorder the text that follows them,
// so a hostile name can make one value read as another on screen. The plain
// direction marks U+200E and U+200F stay, because they only hint at direction
// and legitimate right-to-left text uses them.
func SanitizeTerminalText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) || isBidiOverride(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// StripBidiOverrides removes only the bidi embedding/override/isolate
// characters SanitizeTerminalText drops (see its doc comment), leaving
// everything else - including ESC, tabs, and SGR sequences - untouched.
// For sinks that need SanitizeLogBody's ANSI/tab handling but still must
// guard against a hostile value reordering the rendered text.
func StripBidiOverrides(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isBidiOverride(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isBidiOverride(r rune) bool {
	return (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

// logTabWidth is the column step used when expanding tab characters in
// log lines. 8 matches the default tab stop on virtually every terminal,
// so the post-expansion text aligns the way a user pasting the same line
// into their shell would expect.
const logTabWidth = 8

// SanitizeLogBody is the exported entry point to sanitizeLogLine for sinks
// outside this file (describe content, command-bar output) that render a
// BODY rather than a name or title: unlike SanitizeTerminalText, it keeps
// SGR colour sequences and expands tabs instead of dropping them. See
// sanitizeLogLine for the full behaviour.
func SanitizeLogBody(s string, renderAnsi bool) string {
	return sanitizeLogLine(s, renderAnsi)
}

// sanitizeLogLine replaces non-printable control bytes (NUL, DEL, the C0
// control range minus tab, and the C1 range U+0080-U+009F) with the
// Unicode replacement character and expands tab characters to spaces
// using a logTabWidth-column tab stop. Binary data from processes like
// MySQL handshakes contains bytes that break terminal width calculations
// and corrupt the viewer layout. C1 controls (raw or UTF-8-encoded, e.g.
// U+009B "CSI") let a log line hijack the terminal on emulators that
// honour 8-bit C1 (VTE, Linux console).
//
// Tab expansion is required because lipgloss.Width treats '\t' as
// zero-width while the terminal renders it as a jump to the next tab
// stop. The viewer's contentWidth-overflow guard (in RenderLogViewer)
// uses lipgloss.Width to decide when to truncate. Without expansion,
// tab-bearing lines slip through with an undercounted width, get
// re-wrapped internally by lipgloss, and push the bottom border off the
// visible area. Reported on dragonfly-operator (controller-runtime/zap)
// logs in particular - those use tabs to separate timestamp / level /
// logger / message.
//
// When renderAnsi is true, valid CSI SGR sequences (ESC [ params m — the
// ones that set colour, bold, underline, etc.) are preserved verbatim so
// log producers that emit ANSI colours render as intended. Non-SGR CSI
// sequences (cursor movement, screen erase) remain unsafe for an inline
// viewer and are still replaced. A bare ESC with no valid CSI introducer
// is replaced too. Leaving it would cause terminals to wait for a
// follow-up byte and mis-interpret subsequent output.
//
// C1 detection is decode-aware, not a raw byte-range check: a byte-level
// test for 0x80-0x9F would also catch UTF-8 continuation bytes of
// ordinary non-ASCII runes (many common accented Latin, CJK, emoji, and
// box-drawing characters have a continuation byte in that range) and
// mangle them. Every non-ASCII byte is instead decoded to its full rune;
// only a rune that actually equals U+0080-U+009F is replaced, whether it
// arrived as a raw invalid byte or a valid two-byte UTF-8 encoding (e.g.
// 0xC2 0x9B for U+009B). Anything else - including genuinely invalid
// UTF-8 outside the C1 range - copies through as before.
func sanitizeLogLine(s string, renderAnsi bool) string {
	// Fast path: no control bytes, tabs needing expansion, or non-ASCII
	// bytes means no work to do. Any byte >= 0x80 must go through the
	// slow path because it might decode to a C1 control character.
	needsSanitize := false
	for i := range len(s) {
		c := s[i]
		if c < 32 || c == 127 || c >= 0x80 {
			needsSanitize = true
			break
		}
	}
	if !needsSanitize {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	col := 0
	i := 0
	for i < len(s) {
		c := s[i]
		if renderAnsi && c == 0x1b {
			if end := parseSGRSequence(s, i); end > i {
				// SGR sequences are zero-width. Do not advance col.
				b.WriteString(s[i:end])
				i = end
				continue
			}
		}
		if c == '\t' {
			n := logTabWidth - col%logTabWidth
			for range n {
				b.WriteByte(' ')
			}
			col += n
			i++
			continue
		}
		if c < 0x80 {
			if c >= 32 && c != 127 {
				b.WriteByte(c)
				col++
			} else {
				// Control byte (< 32 and not tab, or DEL).
				b.WriteRune('\ufffd')
			}
			i++
			continue
		}
		// Non-ASCII byte: decode the full rune so a C1 control encoded
		// as two valid UTF-8 bytes is judged by its decoded value, not
		// by the raw continuation byte (see the C1 detection note
		// above).
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size <= 1:
			// Invalid UTF-8. A lone byte in the C1 range is still a
			// control character regardless of encoding. Anything else
			// invalid passes through unchanged, matching legacy
			// behaviour for non-UTF-8 binary payloads. Column tracking
			// mirrors the old approximation: only a would-be leading
			// byte (>= 0xC0) counts as one cell.
			if c >= 0x80 && c <= 0x9f {
				b.WriteRune('\ufffd')
			} else {
				b.WriteByte(c)
				if c >= 0xC0 {
					col++
				}
			}
			i++
		case r >= 0x80 && r <= 0x9f:
			// Valid UTF-8 encoding of a C1 control character.
			b.WriteRune('\ufffd')
			i += size
		default:
			// Ordinary multi-byte rune: copy through untouched.
			b.WriteString(s[i : i+size])
			col++
			i += size
		}
	}
	return b.String()
}

// parseSGRSequence returns the index after a valid ESC [ ... m sequence
// starting at s[i], or i if no valid sequence is present. Only SGR
// (Select Graphic Rendition) finals are accepted because they set
// colour and text attributes without moving the cursor or clearing the
// screen — preserving them is safe in an inline viewer, whereas other
// CSI finals would corrupt the layout.
func parseSGRSequence(s string, i int) int {
	if i+1 >= len(s) || s[i] != 0x1b || s[i+1] != '[' {
		return i
	}
	j := i + 2
	// Parameter bytes: digits, ';' and ':' only (truecolour uses both
	// separators). This deliberately excludes the private markers < = > ?
	// and the CSI intermediate bytes 0x20-0x2F: CSI > Ps . Ps m is
	// XTMODKEYS, which reprograms xterm's keyboard-modifier reporting, so
	// forwarding a private marker let a cluster-controlled line change
	// terminal state after rendering (TASK-885).
	for j < len(s) && (s[j] == ';' || s[j] == ':' || (s[j] >= '0' && s[j] <= '9')) {
		j++
	}
	// SGR final byte is lowercase 'm'. Anything else - including a private
	// marker or intermediate byte that stopped the loop above - is a CSI
	// we can't safely forward to an inline viewer.
	if j < len(s) && s[j] == 'm' {
		return j + 1
	}
	return i
}
