package ui

import (
	"testing"

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

func TestSanitizeLogLine(t *testing.T) {
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
			name:     "tab preserved",
			input:    "key\tvalue",
			expected: "key\tvalue",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeLogLine(tt.input))
		})
	}
}
