package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// openSessionsOverlay is a temporary stub; Task 3 replaces it with the real
// session-picker overlay.
func (m Model) openSessionsOverlay() (tea.Model, tea.Cmd) { return m, nil }
