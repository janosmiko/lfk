package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// executeSessionCommand handles `:session save <name>` / `:session delete <name>`.
// Bare `:session` (or any other subcommand) opens the picker overlay.
func (m Model) executeSessionCommand(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m.openSessionsOverlay()
	}
	sub := args[0]
	name := strings.TrimSpace(strings.Join(args[1:], " "))
	switch sub {
	case "save":
		return m.sessionSave(name)
	case "delete", "rm":
		return m.sessionDelete(name)
	default:
		return m.openSessionsOverlay()
	}
}

func (m Model) sessionSave(name string) (tea.Model, tea.Cmd) {
	if m.unionMode {
		m.setStatusMessage(":session save is disabled in union view", true)
		return m, scheduleStatusClear()
	}
	if sanitizeSessionName(name) == "" {
		m.setStatusMessage("Usage: :session save <name>", true)
		return m, scheduleStatusClear()
	}
	ns := NamedSession{Name: name, SavedAt: time.Now(), State: m.buildSessionState()}
	if err := saveNamedSession(ns); err != nil {
		m.setStatusMessage("Save failed: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Saved session: "+name, false)
	return m, scheduleStatusClear()
}

func (m Model) sessionDelete(name string) (tea.Model, tea.Cmd) {
	if sanitizeSessionName(name) == "" {
		m.setStatusMessage("Usage: :session delete <name>", true)
		return m, scheduleStatusClear()
	}
	existed, err := deleteNamedSession(name)
	if err != nil {
		m.setStatusMessage("Delete failed: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	if !existed {
		m.setStatusMessage("Session not found: "+name, true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Deleted session: "+name, false)
	return m, scheduleStatusClear()
}

// sessionsOverlayState holds the sessions-picker overlay runtime fields,
// embedded into Model to keep app.go under the file-length cap.
type sessionsOverlayState struct {
	sessionsList       []NamedSession
	sessionsFilter     TextInput // filter text (/ mode) for sessions overlay
	sessionsFilterMode bool
	// activeSession is the named session that auto-save writes to and that
	// startup restored. "" means the built-in default workspace (session.yaml).
	activeSession string
}

// openSessionsOverlay loads the saved sessions and opens the picker overlay.
func (m Model) openSessionsOverlay() (tea.Model, tea.Cmd) {
	m.sessionsList = listNamedSessions()
	m.sessionsFilter.Clear()
	m.sessionsFilterMode = false
	m.overlayCursor = 0
	m.overlay = overlaySessions
	return m, nil
}

// filteredNamedSessions returns m.sessionsList filtered by sessionsFilter.
func (m Model) filteredNamedSessions() []NamedSession {
	if m.sessionsFilter.Value == "" {
		return m.sessionsList
	}
	var out []NamedSession
	for _, s := range m.sessionsList {
		if ui.MatchLine(s.Name, m.sessionsFilter.Value) {
			out = append(out, s)
		}
	}
	return out
}

// handleSessionsOverlayKey handles keys for the sessions picker overlay:
// enter=switch, d=delete, /=filter, esc/q=close.
func (m Model) handleSessionsOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.sessionsFilterMode {
		return m.handleSessionsFilterMode(msg)
	}
	items := m.filteredNamedSessions()
	switch msg.String() {
	case "esc", "q":
		if m.sessionsFilter.Value != "" {
			m.sessionsFilter.Clear()
			m.overlayCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			return m.switchToNamedSession(items[m.overlayCursor])
		}
		return m, nil
	case "d":
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			name := items[m.overlayCursor].Name
			if _, err := deleteNamedSession(name); err != nil {
				m.setStatusMessage("Delete failed: "+err.Error(), true)
				return m, scheduleStatusClear()
			}
			m.sessionsList = listNamedSessions()
			m.overlayCursor = clampOverlayCursor(m.overlayCursor, 0, len(m.filteredNamedSessions())-1)
			m.setStatusMessage("Deleted session: "+name, false)
			return m, scheduleStatusClear()
		}
		return m, nil
	case "/":
		m.sessionsFilterMode = true
		m.sessionsFilter.Clear()
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil
	}
	return m, nil
}

// handleSessionsFilterMode mirrors handleNamespaceFilterMode: type to narrow,
// Enter/Esc leave filter mode (they do NOT switch — switching is an explicit
// Enter in normal mode).
func (m Model) handleSessionsFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch handleFilterKey(&m.sessionsFilter, msg.String()) {
	case filterEscape, filterAccept:
		m.sessionsFilterMode = false
		m.overlayCursor = 0
		return m, nil
	case filterClose:
		m.overlay = overlayNone
		return m, nil
	case filterContinue:
		m.overlayCursor = 0
		return m, nil
	}
	return m, nil
}

// switchToNamedSession replaces the current workspace with ns by riding the
// startup restore path: set pendingSession and reload contexts so
// updateContextsLoaded fires restoreSession.
func (m Model) switchToNamedSession(ns NamedSession) (tea.Model, tea.Cmd) {
	if m.unionMode {
		m.setStatusMessage("Exit union view before switching sessions", true)
		return m, scheduleStatusClear()
	}
	m.saveCurrentSession() // preserve the current unnamed workspace
	state := ns.State
	m.pendingSession = &state
	m.sessionRestored = false
	m.overlay = overlayNone
	m.setStatusMessage("Switched to session: "+ns.Name, false)
	return m, tea.Batch(m.loadContexts(), scheduleStatusClear())
}

// overlayHintBarSessions returns the hint bar for the sessions picker overlay.
func (m Model) overlayHintBarSessions() string {
	return m.renderHints([]ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "enter", Desc: "switch"},
		{Key: "d", Desc: "delete"},
		{Key: "/", Desc: "filter"},
		{Key: "esc", Desc: "close"},
	})
}
