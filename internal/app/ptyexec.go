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
	err  error
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
		m.execPTY.Close()
		m.execPTY = nil
	}
	m.execTerm = nil
	m.execDone = nil
}

// handleExecKey forwards key presses to the embedded PTY.
func (m Model) handleExecKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+] exits the embedded terminal.
	if msg.String() == "ctrl+]" {
		m.cleanupExecPTY()
		m.mode = modeExplorer
		return m, nil
	}

	// If process has exited, any key returns to explorer.
	if m.execDone != nil && m.execDone.Load() {
		m.cleanupExecPTY()
		m.mode = modeExplorer
		return m, nil
	}

	if m.execPTY == nil {
		return m, nil
	}

	// Convert Bubbletea key to raw bytes for PTY.
	raw := keyToBytes(msg)
	if len(raw) > 0 {
		_, _ = m.execPTY.Write(raw)
	}
	return m, nil
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

// keyToBytes converts a Bubbletea key message to raw terminal bytes.
func keyToBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeyBackspace:
		return []byte{'\x7f'}
	case tea.KeyDelete:
		return []byte{'\x1b', '[', '3', '~'}
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyEscape:
		return []byte{'\x1b'}
	case tea.KeyUp:
		return []byte{'\x1b', '[', 'A'}
	case tea.KeyDown:
		return []byte{'\x1b', '[', 'B'}
	case tea.KeyRight:
		return []byte{'\x1b', '[', 'C'}
	case tea.KeyLeft:
		return []byte{'\x1b', '[', 'D'}
	case tea.KeyHome:
		return []byte{'\x1b', '[', 'H'}
	case tea.KeyEnd:
		return []byte{'\x1b', '[', 'F'}
	case tea.KeyPgUp:
		return []byte{'\x1b', '[', '5', '~'}
	case tea.KeyPgDown:
		return []byte{'\x1b', '[', '6', '~'}
	case tea.KeyCtrlA:
		return []byte{'\x01'}
	case tea.KeyCtrlB:
		return []byte{'\x02'}
	case tea.KeyCtrlC:
		return []byte{'\x03'}
	case tea.KeyCtrlD:
		return []byte{'\x04'}
	case tea.KeyCtrlE:
		return []byte{'\x05'}
	case tea.KeyCtrlF:
		return []byte{'\x06'}
	case tea.KeyCtrlG:
		return []byte{'\x07'}
	case tea.KeyCtrlH:
		return []byte{'\x08'}
	case tea.KeyCtrlK:
		return []byte{'\x0b'}
	case tea.KeyCtrlL:
		return []byte{'\x0c'}
	case tea.KeyCtrlN:
		return []byte{'\x0e'}
	case tea.KeyCtrlO:
		return []byte{'\x0f'}
	case tea.KeyCtrlP:
		return []byte{'\x10'}
	case tea.KeyCtrlQ:
		return []byte{'\x11'}
	case tea.KeyCtrlR:
		return []byte{'\x12'}
	case tea.KeyCtrlS:
		return []byte{'\x13'}
	case tea.KeyCtrlT:
		return []byte{'\x14'}
	case tea.KeyCtrlU:
		return []byte{'\x15'}
	case tea.KeyCtrlV:
		return []byte{'\x16'}
	case tea.KeyCtrlW:
		return []byte{'\x17'}
	case tea.KeyCtrlX:
		return []byte{'\x18'}
	case tea.KeyCtrlY:
		return []byte{'\x19'}
	case tea.KeyCtrlZ:
		return []byte{'\x1a'}
	}
	// Fallback: try the raw string representation.
	s := msg.String()
	if len(s) == 1 {
		return []byte(s)
	}
	return nil
}
