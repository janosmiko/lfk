package app

import (
	"sync"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/stretchr/testify/assert"
)

// --- keyToBytes ---

func TestKeyToBytes(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.KeyMsg
		expected []byte
	}{
		{
			name:     "runes",
			msg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}},
			expected: []byte("a"),
		},
		{
			name:     "enter",
			msg:      tea.KeyMsg{Type: tea.KeyEnter},
			expected: []byte{'\r'},
		},
		{
			name:     "tab",
			msg:      tea.KeyMsg{Type: tea.KeyTab},
			expected: []byte{'\t'},
		},
		{
			name:     "backspace",
			msg:      tea.KeyMsg{Type: tea.KeyBackspace},
			expected: []byte{'\x7f'},
		},
		{
			name:     "delete",
			msg:      tea.KeyMsg{Type: tea.KeyDelete},
			expected: []byte{'\x1b', '[', '3', '~'},
		},
		{
			name:     "space",
			msg:      tea.KeyMsg{Type: tea.KeySpace},
			expected: []byte{' '},
		},
		{
			name:     "escape",
			msg:      tea.KeyMsg{Type: tea.KeyEscape},
			expected: []byte{'\x1b'},
		},
		{
			name:     "up arrow",
			msg:      tea.KeyMsg{Type: tea.KeyUp},
			expected: []byte{'\x1b', '[', 'A'},
		},
		{
			name:     "down arrow",
			msg:      tea.KeyMsg{Type: tea.KeyDown},
			expected: []byte{'\x1b', '[', 'B'},
		},
		{
			name:     "right arrow",
			msg:      tea.KeyMsg{Type: tea.KeyRight},
			expected: []byte{'\x1b', '[', 'C'},
		},
		{
			name:     "left arrow",
			msg:      tea.KeyMsg{Type: tea.KeyLeft},
			expected: []byte{'\x1b', '[', 'D'},
		},
		{
			name:     "home",
			msg:      tea.KeyMsg{Type: tea.KeyHome},
			expected: []byte{'\x1b', '[', 'H'},
		},
		{
			name:     "end",
			msg:      tea.KeyMsg{Type: tea.KeyEnd},
			expected: []byte{'\x1b', '[', 'F'},
		},
		{
			name:     "page up",
			msg:      tea.KeyMsg{Type: tea.KeyPgUp},
			expected: []byte{'\x1b', '[', '5', '~'},
		},
		{
			name:     "page down",
			msg:      tea.KeyMsg{Type: tea.KeyPgDown},
			expected: []byte{'\x1b', '[', '6', '~'},
		},
		{
			name:     "ctrl+c",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlC},
			expected: []byte{'\x03'},
		},
		{
			name:     "ctrl+d",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlD},
			expected: []byte{'\x04'},
		},
		{
			name:     "ctrl+z",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlZ},
			expected: []byte{'\x1a'},
		},
		{
			name:     "ctrl+a",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlA},
			expected: []byte{'\x01'},
		},
		{
			name:     "ctrl+l",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlL},
			expected: []byte{'\x0c'},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keyToBytes(tt.msg, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeyToBytesAppCursorMode(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.KeyMsg
		expected []byte
	}{
		{name: "up", msg: tea.KeyMsg{Type: tea.KeyUp}, expected: []byte{'\x1b', 'O', 'A'}},
		{name: "down", msg: tea.KeyMsg{Type: tea.KeyDown}, expected: []byte{'\x1b', 'O', 'B'}},
		{name: "right", msg: tea.KeyMsg{Type: tea.KeyRight}, expected: []byte{'\x1b', 'O', 'C'}},
		{name: "left", msg: tea.KeyMsg{Type: tea.KeyLeft}, expected: []byte{'\x1b', 'O', 'D'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keyToBytes(tt.msg, true)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestKeyToBytesAppCursorFallthrough verifies that when appCursor=true but the
// key is not an arrow key, keyToBytes falls through to keyBytesMap and returns
// the normal sequence (DECCKM only affects cursor keys).
func TestKeyToBytesAppCursorFallthrough(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.KeyMsg
		expected []byte
	}{
		{name: "runes unchanged", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, expected: []byte("a")},
		{name: "enter unchanged", msg: tea.KeyMsg{Type: tea.KeyEnter}, expected: []byte{'\r'}},
		{name: "tab unchanged", msg: tea.KeyMsg{Type: tea.KeyTab}, expected: []byte{'\t'}},
		{name: "home unchanged", msg: tea.KeyMsg{Type: tea.KeyHome}, expected: []byte{'\x1b', '[', 'H'}},
		{name: "end unchanged", msg: tea.KeyMsg{Type: tea.KeyEnd}, expected: []byte{'\x1b', '[', 'F'}},
		{name: "pgup unchanged", msg: tea.KeyMsg{Type: tea.KeyPgUp}, expected: []byte{'\x1b', '[', '5', '~'}},
		{name: "pgdown unchanged", msg: tea.KeyMsg{Type: tea.KeyPgDown}, expected: []byte{'\x1b', '[', '6', '~'}},
		{name: "delete unchanged", msg: tea.KeyMsg{Type: tea.KeyDelete}, expected: []byte{'\x1b', '[', '3', '~'}},
		{name: "ctrl+c unchanged", msg: tea.KeyMsg{Type: tea.KeyCtrlC}, expected: []byte{'\x03'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keyToBytes(tt.msg, true)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestP4ExecKeyQ(t *testing.T) {
	m := bp4()
	m.mode = modeExec
	done := &sync.Once{}
	_ = done
	m.execDone = new(atomic.Bool)
	m.execDone.Store(true)
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovCleanANSI(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"empty", "", ""},
		{"no ansi", "hello world", "hello world"},
		{"simple esc", "\x1b[31mred\x1b[0m", "red"},
		{"bold", "\x1b[1mbold\x1b[0m", "bold"},
		{"multiple", "\x1b[31mred\x1b[0m and \x1b[32mgreen\x1b[0m", "red and green"},
		{"nested", "\x1b[1;31mbold red\x1b[0m", "bold red"},
		{"partial esc no letter", "text\x1b[", "text"},
		{"cursor movement", "\x1b[2Amove up", "move up"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, cleanANSI(tt.input))
		})
	}
}

func TestCovParseFirstJSONField(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		field  string
		suffix string
		expect string
	}{
		{"exact match", `[{"name":"cilium"}]`, "name", "cilium", "cilium"},
		{"repo prefixed", `[{"name":"myrepo/cilium"}]`, "name", "cilium", "myrepo/cilium"},
		{"no match", `[{"name":"nginx"}]`, "name", "cilium", ""},
		{"empty json", `[]`, "name", "cilium", ""},
		{"multiple entries", `[{"name":"other"},{"name":"repo/cilium"}]`, "name", "cilium", "repo/cilium"},
		{"missing field", `[{"version":"1.0"}]`, "name", "cilium", ""},
		{"broken json", `{"name":"unclosed`, "name", "cilium", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, parseFirstJSONField(tt.json, tt.field, tt.suffix))
		})
	}
}

func TestCovRandomSuffix(t *testing.T) {
	s1 := randomSuffix(5)
	assert.Len(t, s1, 5)

	s2 := randomSuffix(10)
	assert.Len(t, s2, 10)

	// All characters should be lowercase alphanumeric.
	for _, c := range s1 {
		assert.True(t, (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'))
	}
}

func TestCovFinalizerMatchKey(t *testing.T) {
	m := k8s.FinalizerMatch{
		Namespace: "default",
		Kind:      "Pod",
		Name:      "my-pod",
	}
	assert.Equal(t, "default/Pod/my-pod", finalizerMatchKey(m))

	m2 := k8s.FinalizerMatch{Kind: "Node", Name: "node-1"}
	assert.Equal(t, "/Node/node-1", finalizerMatchKey(m2))
}

func TestFinalRandomSuffix(t *testing.T) {
	s1 := randomSuffix(5)
	s2 := randomSuffix(5)
	assert.Len(t, s1, 5)
	assert.Len(t, s2, 5)
	// Extremely unlikely to be the same.
	if s1 == s2 {
		t.Log("random suffixes happened to match (extremely unlikely)")
	}
}

func TestFinalRandomSuffixLength(t *testing.T) {
	for _, n := range []int{0, 1, 3, 10, 20} {
		assert.Len(t, randomSuffix(n), n)
	}
}

func TestFinalCleanANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"no ansi", "no ansi"},
		{"\x1b[38;5;200mextended\x1b[0m", "extended"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, cleanANSI(tt.input))
	}
}

func TestFinalParseFirstJSONField(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		field  string
		suffix string
		want   string
	}{
		{"exact match", `[{"name":"cilium"}]`, "name", "cilium", "cilium"},
		{"repo prefix", `[{"name":"repo/cilium"}]`, "name", "cilium", "repo/cilium"},
		{"no match", `[{"name":"nginx"}]`, "name", "cilium", ""},
		{"empty json", `[]`, "name", "cilium", ""},
		{"multiple entries", `[{"name":"other"},{"name":"repo/cilium"}]`, "name", "cilium", "repo/cilium"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseFirstJSONField(tt.json, tt.field, tt.suffix))
		})
	}
}
