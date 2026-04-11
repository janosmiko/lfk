package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// --- keyToBytes: additional key types not covered in ptyexec_test.go ---

func TestKeyToBytesCtrlKeys(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.KeyMsg
		expected []byte
	}{
		{name: "ctrl+b", msg: tea.KeyMsg{Type: tea.KeyCtrlB}, expected: []byte{'\x02'}},
		{name: "ctrl+e", msg: tea.KeyMsg{Type: tea.KeyCtrlE}, expected: []byte{'\x05'}},
		{name: "ctrl+f", msg: tea.KeyMsg{Type: tea.KeyCtrlF}, expected: []byte{'\x06'}},
		{name: "ctrl+g", msg: tea.KeyMsg{Type: tea.KeyCtrlG}, expected: []byte{'\x07'}},
		{name: "ctrl+h", msg: tea.KeyMsg{Type: tea.KeyCtrlH}, expected: []byte{'\x08'}},
		{name: "ctrl+k", msg: tea.KeyMsg{Type: tea.KeyCtrlK}, expected: []byte{'\x0b'}},
		{name: "ctrl+n", msg: tea.KeyMsg{Type: tea.KeyCtrlN}, expected: []byte{'\x0e'}},
		{name: "ctrl+o", msg: tea.KeyMsg{Type: tea.KeyCtrlO}, expected: []byte{'\x0f'}},
		{name: "ctrl+p", msg: tea.KeyMsg{Type: tea.KeyCtrlP}, expected: []byte{'\x10'}},
		{name: "ctrl+q", msg: tea.KeyMsg{Type: tea.KeyCtrlQ}, expected: []byte{'\x11'}},
		{name: "ctrl+r", msg: tea.KeyMsg{Type: tea.KeyCtrlR}, expected: []byte{'\x12'}},
		{name: "ctrl+s", msg: tea.KeyMsg{Type: tea.KeyCtrlS}, expected: []byte{'\x13'}},
		{name: "ctrl+t", msg: tea.KeyMsg{Type: tea.KeyCtrlT}, expected: []byte{'\x14'}},
		{name: "ctrl+u", msg: tea.KeyMsg{Type: tea.KeyCtrlU}, expected: []byte{'\x15'}},
		{name: "ctrl+v", msg: tea.KeyMsg{Type: tea.KeyCtrlV}, expected: []byte{'\x16'}},
		{name: "ctrl+w", msg: tea.KeyMsg{Type: tea.KeyCtrlW}, expected: []byte{'\x17'}},
		{name: "ctrl+x", msg: tea.KeyMsg{Type: tea.KeyCtrlX}, expected: []byte{'\x18'}},
		{name: "ctrl+y", msg: tea.KeyMsg{Type: tea.KeyCtrlY}, expected: []byte{'\x19'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keyToBytes(tt.msg, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKeyToBytesMultiCharRunes(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h', 'e', 'l', 'l', 'o'}}
	result := keyToBytes(msg, false)
	assert.Equal(t, []byte("hello"), result)
}

func TestKeyToBytesFallbackSingleChar(t *testing.T) {
	// Simulate an unknown key type with a single-char string representation.
	// This exercises the final fallback path.
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	result := keyToBytes(msg, false)
	assert.Equal(t, []byte("x"), result)
}

// --- handleExecKey ---

func TestHandleExecKeyCtrlBracketSetsPrefix(t *testing.T) {
	m := Model{
		mode:    modeExec,
		execPTY: nil, // no real PTY
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, _ := m.handleExecKey(tea.KeyMsg{Type: tea.KeyCtrlCloseBracket})
	result := ret.(Model)
	assert.True(t, result.execEscPressed)
}

func TestHandleExecKeyDoubleCtrlBracketExits(t *testing.T) {
	m := Model{
		mode:           modeExec,
		execPTY:        nil,
		execEscPressed: true,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleExecKey(tea.KeyMsg{Type: tea.KeyCtrlCloseBracket})
	result := ret.(Model)
	assert.Equal(t, modeExplorer, result.mode)
	assert.False(t, result.execEscPressed)
}

func TestHandleExecKeyPrefixThenOtherKeyCancels(t *testing.T) {
	m := Model{
		mode:           modeExec,
		execPTY:        nil,
		execEscPressed: true,
		tabs:           []TabState{{}},
		width:          80,
		height:         40,
	}
	ret, _ := m.handleExecKey(runeKey('x'))
	result := ret.(Model)
	assert.False(t, result.execEscPressed)
	// Mode stays exec since no action was taken
	assert.Equal(t, modeExec, result.mode)
}

func TestHandleExecKeyNoPTYNoOp(t *testing.T) {
	m := Model{
		mode:    modeExec,
		execPTY: nil,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, cmd := m.handleExecKey(runeKey('a'))
	result := ret.(Model)
	assert.Equal(t, modeExec, result.mode)
	assert.Nil(t, cmd)
}
