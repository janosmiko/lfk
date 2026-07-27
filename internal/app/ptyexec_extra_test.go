package app

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestHandleExecKeyCtrlBracketSchedulesStatusClear(t *testing.T) {
	m := Model{
		mode:    modeExec,
		execPTY: nil,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	_, cmd := m.handleExecKey(tea.KeyMsg{Type: tea.KeyCtrlCloseBracket})
	// The prefix hint must expire instead of lingering in the status bar.
	assert.NotNil(t, cmd)
}

// PgUp/PgDown scroll lfk's scrollback for ordinary shell output, but must be
// forwarded to full-screen programs (vim, less, htop) that page themselves.
func TestHandleExecKeyPageKeysRespectAltScreen(t *testing.T) {
	newModel := func(t *testing.T, altScreen bool) (Model, *os.File) {
		t.Helper()
		r, w, err := os.Pipe()
		require.NoError(t, err)
		t.Cleanup(func() { _ = r.Close(); _ = w.Close() })

		term := vt10x.New(vt10x.WithSize(80, 24))
		if altScreen {
			_, err = term.Write([]byte("\x1b[?1049h"))
			require.NoError(t, err)
			require.NotZero(t, term.Mode()&vt10x.ModeAltScreen, "terminal should be on the alternate screen")
		}
		m := Model{
			mode:           modeExec,
			execPTY:        w,
			execTerm:       term,
			execScrollback: newScrollback(500),
			execMu:         &sync.Mutex{},
			tabs:           []TabState{{}},
			width:          80,
			height:         40,
		}
		// Deep enough history that a full-viewport scroll never clamps.
		_, err = m.execScrollback.Write([]byte(strings.Repeat("line\n", 400)))
		require.NoError(t, err)
		return m, r
	}

	// readForwarded returns what the handler wrote to the PTY, failing the
	// test rather than hanging when the key was swallowed instead.
	readForwarded := func(t *testing.T, r *os.File) []byte {
		t.Helper()
		require.NoError(t, r.SetReadDeadline(time.Now().Add(2*time.Second)))
		buf := make([]byte, 8)
		n, err := r.Read(buf)
		require.NoError(t, err, "key should reach the PTY process")
		return buf[:n]
	}

	// assertSwallowed fails if a key we handled ourselves also reached the
	// PTY, which would page the program and our scrollback at once.
	assertSwallowed := func(t *testing.T, r *os.File) {
		t.Helper()
		require.NoError(t, r.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
		buf := make([]byte, 8)
		_, err := r.Read(buf)
		assert.ErrorIs(t, err, os.ErrDeadlineExceeded, "scroll key must not also reach the PTY")
	}

	t.Run("normal screen scrolls by exactly one viewport", func(t *testing.T) {
		m, r := newModel(t, false)
		rows := m.execViewportRows()
		require.Positive(t, rows)

		ret, _ := m.handleExecKey(tea.KeyMsg{Type: tea.KeyPgUp})
		up := ret.(Model)
		assert.Equal(t, rows, up.execScrollOffset, "PgUp scrolls back a full viewport, not a half page")

		ret, _ = up.handleExecKey(tea.KeyMsg{Type: tea.KeyPgDown})
		assert.Zero(t, ret.(Model).execScrollOffset, "PgDown returns to live")
		assertSwallowed(t, r)
	})

	t.Run("normal screen PgDown steps forward from deep history", func(t *testing.T) {
		m, r := newModel(t, false)
		rows := m.execViewportRows()
		m.execScrollOffset = 3 * rows

		ret, _ := m.handleExecKey(tea.KeyMsg{Type: tea.KeyPgDown})
		assert.Equal(t, 2*rows, ret.(Model).execScrollOffset, "PgDown moves exactly one viewport toward live")
		assertSwallowed(t, r)
	})

	t.Run("alt screen forwards page keys and snaps to live", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			key  tea.KeyType
			want []byte
		}{
			{"pgup", tea.KeyPgUp, []byte{'\x1b', '[', '5', '~'}},
			{"pgdown", tea.KeyPgDown, []byte{'\x1b', '[', '6', '~'}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m, r := newModel(t, true)
				// Start scrolled back: forwarding must snap the view to live
				// rather than page our scrollback.
				m.execScrollOffset = 2 * m.execViewportRows()

				ret, _ := m.handleExecKey(tea.KeyMsg{Type: tc.key})
				assert.Zero(t, ret.(Model).execScrollOffset, "forwarding a key snaps back to live")
				assert.Equal(t, tc.want, readForwarded(t, r))
			})
		}
	})
}
