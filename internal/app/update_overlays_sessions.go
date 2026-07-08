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

// handleSessionsOverlayKey is a temporary stub; Task 4 replaces it with the
// real key handler (switch/delete/filter).
func (m Model) handleSessionsOverlayKey(_ tea.KeyMsg) (tea.Model, tea.Cmd) { return m, nil }

// overlayHintBarSessions returns the hint bar for the sessions picker overlay.
func (m Model) overlayHintBarSessions() string {
	return m.renderHints([]ui.HintEntry{
		{Key: "enter", Desc: "switch"},
		{Key: "d", Desc: "delete"},
		{Key: "/", Desc: "filter"},
		{Key: "esc", Desc: "close"},
	})
}
