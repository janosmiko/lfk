package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/janosmiko/lfk/internal/logger"
)

// execPTYTickMsg triggers a re-render of the embedded terminal.
// The ptmx field identifies which tab's terminal this tick belongs to.
type execPTYTickMsg struct {
	ptmx *os.File
}

// execPTYExitMsg signals that the PTY process has exited.
// The ptmx field identifies which tab's terminal exited.
type execPTYExitMsg struct {
	ptmx *os.File
}

// execPTYStartMsg carries the PTY state from the tea.Cmd back to the Update handler.
type execPTYStartMsg struct {
	ptmx  *os.File
	term  vt10x.Terminal
	title string
	cmd   *exec.Cmd
}

// startPTYExecCmd launches a command in an embedded PTY terminal.
// It returns a tea.Cmd that starts the PTY and sends an execPTYStartMsg.
// The cmd must NOT have been started yet (pty.StartWithSize handles that).
func startPTYExecCmd(cmd *exec.Cmd, title string, cols, rows int) tea.Cmd {
	return func() tea.Msg {
		term := vt10x.New(vt10x.WithSize(cols, rows))

		ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
		if err != nil {
			logger.Error("Failed to start PTY", "error", err)
			return actionResultMsg{err: fmt.Errorf("failed to start PTY: %w", err)}
		}

		return execPTYStartMsg{ptmx: ptmx, term: term, title: title, cmd: cmd}
	}
}

// scheduleExecTick schedules the next terminal refresh tick.
func (m Model) scheduleExecTick() tea.Cmd {
	ptmx := m.execPTY
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return execPTYTickMsg{ptmx: ptmx}
	})
}

// cleanupExecPTY closes the PTY and cleans up exec state.
func (m *Model) cleanupExecPTY() {
	if m.execPTY != nil {
		_ = m.execPTY.Close()
		m.execPTY = nil
	}
	m.execTerm = nil
	m.execDone = nil
}

// handleExecKey forwards key presses to the embedded PTY.
// Ctrl+] is a prefix key (like tmux's Ctrl+b):
//   - Ctrl+] then ] = next tab
//   - Ctrl+] then [ = previous tab
//   - Ctrl+] then t = new tab
//   - Ctrl+] then Ctrl+] = exit terminal
//   - Ctrl+] then any other key = cancel, return to PTY
func (m Model) handleExecKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If process has exited, any key returns to explorer.
	if m.execDone != nil && m.execDone.Load() {
		m.cleanupExecPTY()
		m.execEscPressed = false
		m.mode = modeExplorer
		return m, nil
	}

	// Handle follow-up key after Ctrl+] prefix.
	if m.execEscPressed {
		m.execEscPressed = false
		switch msg.String() {
		case "ctrl+]":
			// Double Ctrl+] exits the terminal.
			m.cleanupExecPTY()
			m.mode = modeExplorer
			return m, nil
		case "]":
			return m.execSwitchTab((m.activeTab + 1) % len(m.tabs))
		case "[":
			return m.execSwitchTab((m.activeTab - 1 + len(m.tabs)) % len(m.tabs))
		case "t":
			return m.execNewTab()
		default:
			// Cancel prefix — key is swallowed (not forwarded).
			return m, nil
		}
	}

	// Ctrl+] pressed: set prefix flag and show hint.
	if msg.String() == "ctrl+]" {
		m.execEscPressed = true
		m.setStatusMessage("Ctrl+]: ]/[ switch tab, t new tab, Ctrl+] exit", false)
		return m, nil
	}

	if m.execPTY == nil {
		return m, nil
	}

	// Convert Bubbletea key to raw bytes for PTY.
	// Pass the terminal's application cursor mode so arrow keys send the
	// correct sequences (\x1bO vs \x1b[ depending on DECCKM state).
	appCursor := m.execTerm != nil && m.execTerm.Mode()&vt10x.ModeAppCursor != 0
	raw := keyToBytes(msg, appCursor)
	if len(raw) > 0 {
		_, _ = m.execPTY.Write(raw)
	}
	return m, nil
}

// execSwitchTab saves the current tab and switches to the target tab index.
func (m Model) execSwitchTab(target int) (tea.Model, tea.Cmd) {
	if len(m.tabs) <= 1 {
		return m, nil
	}
	m.saveCurrentTab()
	if cmd := m.loadTab(target); cmd != nil {
		return m, cmd
	}
	switch m.mode {
	case modeExplorer:
		return m, m.loadPreview()
	case modeLogs:
		if m.logCh != nil {
			return m, m.waitForLogLine()
		}
	case modeExec:
		if m.execPTY != nil {
			return m, m.scheduleExecTick()
		}
	}
	return m, nil
}

// execNewTab creates a new tab from exec mode (starts in explorer).
func (m Model) execNewTab() (tea.Model, tea.Cmd) {
	if len(m.tabs) >= 9 {
		m.setStatusMessage("Max 9 tabs", true)
		return m, scheduleStatusClear()
	}
	m.saveCurrentTab()
	newTab := m.cloneCurrentTab()
	newTab.mode = modeExplorer
	newTab.logLines = nil
	newTab.logCancel = nil
	newTab.logCh = nil
	insertAt := m.activeTab + 1
	m.tabs = append(m.tabs[:insertAt], append([]TabState{newTab}, m.tabs[insertAt:]...)...)
	m.activeTab = insertAt
	m.loadTab(m.activeTab)
	m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
	return m, scheduleStatusClear()
}

// startExecPTYReader launches the background goroutine that reads from PTY
// output and feeds it into the virtual terminal emulator. It also waits for
// the process to exit and sets the done flag.
// The done atomic and mu allow the goroutine to signal the correct tab's state.
func startExecPTYReader(ptmx *os.File, term vt10x.Terminal, cmd *exec.Cmd, mu *sync.Mutex, done *atomic.Bool) {
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				mu.Lock()
				_, _ = term.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				if err != io.EOF {
					logger.Error("PTY read error", "error", err)
				}
				break
			}
		}
		// Wait for the process to exit and collect status.
		_ = cmd.Wait()
		// Small delay to let final output drain into the terminal.
		time.Sleep(100 * time.Millisecond)
		done.Store(true)
	}()
}

// keyBytesMap maps tea.KeyType to raw terminal byte sequences (normal cursor mode).
var keyBytesMap = map[tea.KeyType][]byte{
	tea.KeyEnter:     {'\r'},
	tea.KeyTab:       {'\t'},
	tea.KeyBackspace: {'\x7f'},
	tea.KeyDelete:    {'\x1b', '[', '3', '~'},
	tea.KeySpace:     {' '},
	tea.KeyEscape:    {'\x1b'},
	tea.KeyUp:        {'\x1b', '[', 'A'},
	tea.KeyDown:      {'\x1b', '[', 'B'},
	tea.KeyRight:     {'\x1b', '[', 'C'},
	tea.KeyLeft:      {'\x1b', '[', 'D'},
	tea.KeyHome:      {'\x1b', '[', 'H'},
	tea.KeyEnd:       {'\x1b', '[', 'F'},
	tea.KeyPgUp:      {'\x1b', '[', '5', '~'},
	tea.KeyPgDown:    {'\x1b', '[', '6', '~'},
	tea.KeyCtrlA:     {'\x01'},
	tea.KeyCtrlB:     {'\x02'},
	tea.KeyCtrlC:     {'\x03'},
	tea.KeyCtrlD:     {'\x04'},
	tea.KeyCtrlE:     {'\x05'},
	tea.KeyCtrlF:     {'\x06'},
	tea.KeyCtrlG:     {'\x07'},
	tea.KeyCtrlH:     {'\x08'},
	tea.KeyCtrlK:     {'\x0b'},
	tea.KeyCtrlL:     {'\x0c'},
	tea.KeyCtrlN:     {'\x0e'},
	tea.KeyCtrlO:     {'\x0f'},
	tea.KeyCtrlP:     {'\x10'},
	tea.KeyCtrlQ:     {'\x11'},
	tea.KeyCtrlR:     {'\x12'},
	tea.KeyCtrlS:     {'\x13'},
	tea.KeyCtrlT:     {'\x14'},
	tea.KeyCtrlU:     {'\x15'},
	tea.KeyCtrlV:     {'\x16'},
	tea.KeyCtrlW:     {'\x17'},
	tea.KeyCtrlX:     {'\x18'},
	tea.KeyCtrlY:     {'\x19'},
	tea.KeyCtrlZ:     {'\x1a'},
}

// keyBytesAppCursorMap overrides cursor-key sequences for application cursor
// mode (DECCKM active): arrow keys send \x1bO_ instead of \x1b[_.
var keyBytesAppCursorMap = map[tea.KeyType][]byte{
	tea.KeyUp:    {'\x1b', 'O', 'A'},
	tea.KeyDown:  {'\x1b', 'O', 'B'},
	tea.KeyRight: {'\x1b', 'O', 'C'},
	tea.KeyLeft:  {'\x1b', 'O', 'D'},
}

// keyToBytes converts a Bubbletea key message to raw terminal bytes.
// appCursor should be true when the terminal has DECCKM active (ModeAppCursor).
func keyToBytes(msg tea.KeyMsg, appCursor bool) []byte {
	if msg.Type == tea.KeyRunes {
		return []byte(string(msg.Runes))
	}
	if appCursor {
		if b, ok := keyBytesAppCursorMap[msg.Type]; ok {
			return b
		}
	}
	if b, ok := keyBytesMap[msg.Type]; ok {
		return b
	}
	// Fallback: try the raw string representation.
	s := msg.String()
	if len(s) == 1 {
		return []byte(s)
	}
	return nil
}
