package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

// --- wrapLine ---

func TestWrapLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		width    int
		expected []string
	}{
		{
			name:     "zero width returns whole line",
			line:     "hello",
			width:    0,
			expected: []string{"hello"},
		},
		{
			name:     "negative width returns whole line",
			line:     "hello",
			width:    -1,
			expected: []string{"hello"},
		},
		{
			name:     "empty line",
			line:     "",
			width:    10,
			expected: []string{""},
		},
		{
			name:     "fits exactly",
			line:     "hello",
			width:    5,
			expected: []string{"hello"},
		},
		{
			name:     "fits with room",
			line:     "hi",
			width:    10,
			expected: []string{"hi"},
		},
		{
			name:     "splits into two parts",
			line:     "abcdef",
			width:    3,
			expected: []string{"abc", "def"},
		},
		{
			name:     "splits into three parts",
			line:     "abcdefgh",
			width:    3,
			expected: []string{"abc", "def", "gh"},
		},
		{
			name:     "width 1 splits every rune",
			line:     "abc",
			width:    1,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "unicode runes",
			line:     "héllo wörld",
			width:    5,
			expected: []string{"héllo", " wörl", "d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, wrapLine(tt.line, tt.width))
		})
	}
}

// Regression: producer-colored kyverno-style log rows in wrap mode lost
// text mid-line. wrapLine rune-sliced the input, but ANSI escape sequences
// occupy several rune slots while contributing zero visual width. A wrap
// at `width` runes regularly cut mid-SGR (e.g. between `\x1b[` and `90m`)
// or chopped real text early because the rune budget was eaten by the
// invisible escape bytes. Both cases manifested as missing characters or
// raw "[NNm" leaks once the terminal hit the malformed sequence.
//
// Wrap must split by visual columns and never break ANSI sequences.
func TestWrapLine_PreservesANSIAndSplitsByVisualWidth(t *testing.T) {
	// 24-char visible content: "timestamp level message1" plus producer
	// SGR runs around each field. ANSI bytes are zero-width and must not
	// consume the wrap budget.
	line := "\x1b[90mtimestamp\x1b[0m \x1b[34mlevel\x1b[0m message1"
	parts := wrapLine(line, 16)

	// Stripped concatenation of the wrap parts must equal the stripped
	// original (no characters lost mid-line).
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(ansi.Strip(p))
	}
	assert.Equal(t, ansi.Strip(line), sb.String(),
		"wrapping must preserve every visible character (got parts %q)", parts)

	// Each wrap part must visually fit the limit.
	for _, p := range parts {
		assert.LessOrEqual(t, ansi.StringWidth(p), 16,
			"wrap part exceeds visual width budget: %q", p)
	}
}

// --- stripTimestampRaw ---

func TestStripTimestampRaw(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no timestamp",
			input:    "just a regular log line",
			expected: "just a regular log line",
		},
		{
			name:     "too short",
			input:    "short",
			expected: "short",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "RFC3339Nano timestamp stripped",
			input:    "2024-01-15T10:30:00.000000000Z log content here",
			expected: "log content here",
		},
		{
			name:     "RFC3339 timestamp stripped",
			input:    "2024-01-15T10:30:00Z log content here",
			expected: "log content here",
		},
		{
			name:     "timestamp with timezone offset",
			input:    "2024-01-15T10:30:00+05:30 log content",
			expected: "log content",
		},
		{
			name:     "no T at position 10 not stripped",
			input:    "2024-01-15X10:30:00Z log content here",
			expected: "2024-01-15X10:30:00Z log content here",
		},
		{
			name:     "no dash at position 4 not stripped",
			input:    "20240115T10:30:00.000000000Z log content here",
			expected: "20240115T10:30:00.000000000Z log content here",
		},
		{
			name:     "space too early not stripped",
			input:    "2024-01-15T10:30 remainder",
			expected: "2024-01-15T10:30 remainder",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, stripTimestampRaw(tt.input))
		})
	}
}

// --- stripTimestamp ---

func TestStripTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain log line no timestamp",
			input:    "just a regular log line",
			expected: "just a regular log line",
		},
		{
			name:     "plain timestamp stripped",
			input:    "2024-01-15T10:30:00.000000000Z log content here",
			expected: "log content here",
		},
		{
			name:     "prefixed line with timestamp",
			input:    "[pod/nginx main] 2024-01-15T10:30:00.000000000Z log content here",
			expected: "[pod/nginx main] log content here",
		},
		{
			name:     "prefixed line without timestamp",
			input:    "[pod/nginx main] no timestamp here",
			expected: "[pod/nginx main] no timestamp here",
		},
		{
			name:     "bracket without closing keeps as plain",
			input:    "[unclosed bracket 2024-01-15T10:30:00Z rest",
			expected: "[unclosed bracket 2024-01-15T10:30:00Z rest",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StripTimestamp(tt.input))
		})
	}
}

// --- sanitizeLogLine ---

func TestSanitizeLogLine_RenderAnsiDisabled(t *testing.T) {
	// renderAnsi=false preserves the legacy behaviour: every non-tab
	// control byte (including the ESC that introduces ANSI sequences) is
	// replaced, so badly-behaved producers can't hijack the terminal.
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text unchanged",
			input:    "INFO: server started on port 8080",
			expected: "INFO: server started on port 8080",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			// Tabs are expanded to spaces because lipgloss.Width treats
			// '\t' as zero-width while the terminal renders it as a jump
			// to the next 8-column tab stop. The discrepancy let
			// tab-bearing lines slip past RenderLogViewer's contentWidth
			// truncation guard, force lipgloss to re-wrap them, and push
			// the bottom border off-screen — reported on dragonfly-operator
			// (controller-runtime/zap) logs which are tab-separated.
			name:     "tab expanded to next 8-column tab stop",
			input:    "key\tvalue",
			expected: "key     value", // 'key' (col 3) + 5 spaces -> col 8 + 'value'
		},
		{
			name:     "leading tab expands to full tab stop",
			input:    "\thello",
			expected: "        hello", // col 0 -> col 8 = 8 spaces
		},
		{
			name:     "consecutive tabs each advance to next stop",
			input:    "a\t\tb",
			expected: "a               b", // col 1 + 7 sp (to 8) + 8 sp (to 16) + 'b'
		},
		{
			name:     "dragonfly-style controller-runtime log",
			input:    "2026-04-27T16:06:59Z\tINFO\tsetup\tstarting manager",
			expected: "2026-04-27T16:06:59Z    INFO    setup   starting manager",
			// 20 chars -> col 24 (4 sp), +4 -> col 32 (4 sp), +5 -> col 40 (3 sp), +msg
		},
		{
			name:     "null bytes replaced",
			input:    "hello\x00world",
			expected: "hello\ufffdworld",
		},
		{
			name:     "control chars replaced",
			input:    "data\x01\x02\x03end",
			expected: "data\ufffd\ufffd\ufffdend",
		},
		{
			name:     "DEL char replaced",
			input:    "before\x7fafter",
			expected: "before\ufffdafter",
		},
		{
			name:     "mysql binary handshake",
			input:    "5.5.5-10.6.25-MariaDB\x00Z$~sD7*k]\x00\x00\x00o7cn",
			expected: "5.5.5-10.6.25-MariaDB\ufffdZ$~sD7*k]\ufffd\ufffd\ufffdo7cn",
		},
		{
			name:     "SGR escape is replaced when ANSI disabled",
			input:    "\x1b[0;33mWARNING\x1b[0m rest",
			expected: "\ufffd[0;33mWARNING\ufffd[0m rest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeLogLine(tt.input, false))
		})
	}
}

func TestSanitizeLogLine_RenderAnsiEnabled_PreservesSGR(t *testing.T) {
	// renderAnsi=true keeps complete CSI SGR sequences (the ones that
	// set colour / bold / underline) verbatim so the terminal can colour
	// the line as the log producer intended.
	input := "\x1b[0;33mWARNING: CONFIGURATION: Long polling issues detected.\x1b[0m"
	out := sanitizeLogLine(input, true)
	assert.Equal(t, input, out, "SGR sequences must pass through untouched")
}

func TestSanitizeLogLine_RenderAnsiEnabled_StripsNonSGRCSI(t *testing.T) {
	// Non-SGR CSI sequences (clear screen, cursor move) would corrupt
	// the viewer layout, so they are treated as stray ESC bytes and
	// replaced with U+FFFD even when renderAnsi is on.
	cases := []struct {
		name  string
		input string
	}{
		{"clear screen", "before\x1b[2Jafter"},
		{"cursor up", "line1\x1b[3Aline2"},
		{"erase to end of line", "foo\x1b[Kbar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := sanitizeLogLine(tc.input, true)
			assert.NotContains(t, out, "\x1b",
				"non-SGR escape bytes must be replaced, got %q", out)
			assert.Contains(t, out, "\ufffd",
				"replacement char must appear where ESC was, got %q", out)
		})
	}
}

func TestSanitizeLogLine_RenderAnsiEnabled_ReplacesBareESC(t *testing.T) {
	// A bare ESC with no CSI introducer is treated the same as any
	// other control byte; the replacement prevents the terminal from
	// waiting on the next byte and mis-interpreting user output.
	out := sanitizeLogLine("hello\x1bworld", true)
	assert.Equal(t, "hello\ufffdworld", out)
}

func TestSanitizeLogLine_RenderAnsiEnabled_KeepsNullAndBinaryAsReplacements(t *testing.T) {
	// Even with rendering on, actual binary (NUL, DEL, mid-range
	// controls) must still be replaced — that's what the sanitizer
	// exists for in the first place (MySQL handshake etc).
	out := sanitizeLogLine("\x1b[32mok\x1b[0m\x00tail", true)
	assert.Equal(t, "\x1b[32mok\x1b[0m\ufffdtail", out)
}

func TestSanitizeLogLine_RenderAnsiEnabled_PreservesMultibyte(t *testing.T) {
	// UTF-8 multi-byte runes adjacent to SGR sequences must round-trip
	// unchanged; a byte-oriented implementation mustn't fragment them.
	input := "\x1b[34m☂ rain \x1b[0m日本語"
	assert.Equal(t, input, sanitizeLogLine(input, true))
}
