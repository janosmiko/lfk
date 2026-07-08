package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logger"
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
	// defaultSessionTabs is the tab count of the default workspace
	// (session.yaml), snapshotted at overlay-open so the "default" row can show
	// it without a per-frame disk read.
	defaultSessionTabs int
	// sessionsSaveMode is true while the overlay prompts for a name (the "s"
	// save-as key); sessionsSaveInput holds the typed name.
	sessionsSaveMode  bool
	sessionsSaveInput TextInput
}

// defaultSessionLabel is the display name of the built-in default workspace in
// the picker; it has no sessions/<name>.yaml file (it lives in session.yaml).
const defaultSessionLabel = "default"

// sessionRow is one row in the picker: the built-in default plus each saved
// named session. An empty name identifies the default workspace.
type sessionRow struct {
	name      string // "" for the default workspace
	label     string
	tabs      int
	savedAt   time.Time
	isDefault bool
}

// openSessionsOverlay loads the saved sessions and opens the picker overlay.
func (m Model) openSessionsOverlay() (tea.Model, tea.Cmd) {
	m.sessionsList = listNamedSessions()
	m.defaultSessionTabs = 0
	if s := loadSession(); s != nil {
		m.defaultSessionTabs = len(s.Tabs)
	}
	m.sessionsFilter.Clear()
	m.sessionsFilterMode = false
	m.sessionsSaveMode = false
	m.sessionsSaveInput.Clear()
	m.overlayCursor = 0
	m.overlay = overlaySessions
	return m, nil
}

// sessionRows returns the picker rows: the default workspace first, then the
// saved named sessions, filtered by the current filter text (matched on label).
func (m Model) sessionRows() []sessionRow {
	rows := make([]sessionRow, 0, len(m.sessionsList)+1)
	rows = append(rows, sessionRow{label: defaultSessionLabel, tabs: m.defaultSessionTabs, isDefault: true})
	for _, s := range m.sessionsList {
		rows = append(rows, sessionRow{name: s.Name, label: s.Name, tabs: len(s.State.Tabs), savedAt: s.SavedAt})
	}
	if m.sessionsFilter.Value == "" {
		return rows
	}
	out := rows[:0:0]
	for _, r := range rows {
		if ui.MatchLine(r.label, m.sessionsFilter.Value) {
			out = append(out, r)
		}
	}
	return out
}

// handleSessionsOverlayKey handles keys for the sessions picker overlay:
// enter=switch, s=save-as, d=delete, /=filter, esc/q=close.
func (m Model) handleSessionsOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.sessionsSaveMode {
		return m.handleSessionsSaveMode(msg)
	}
	if m.sessionsFilterMode {
		return m.handleSessionsFilterMode(msg)
	}
	rows := m.sessionRows()
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
		if m.overlayCursor >= 0 && m.overlayCursor < len(rows) {
			return m.switchToSession(rows[m.overlayCursor].name)
		}
		return m, nil
	case "s":
		m.sessionsSaveMode = true
		m.sessionsSaveInput.Clear()
		return m, nil
	case "d":
		return m.deleteSessionRow(rows)
	case "/":
		m.sessionsFilterMode = true
		m.sessionsFilter.Clear()
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(rows)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(rows)-1)
		return m, nil
	}
	return m, nil
}

// deleteSessionRow deletes the highlighted named session. The default row has
// no file and cannot be deleted.
func (m Model) deleteSessionRow(rows []sessionRow) (tea.Model, tea.Cmd) {
	if m.overlayCursor < 0 || m.overlayCursor >= len(rows) {
		return m, nil
	}
	row := rows[m.overlayCursor]
	if row.isDefault {
		m.setStatusMessage("The default session cannot be deleted", true)
		return m, scheduleStatusClear()
	}
	if _, err := deleteNamedSession(row.name); err != nil {
		m.setStatusMessage("Delete failed: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.sessionsList = listNamedSessions()
	m.overlayCursor = clampOverlayCursor(m.overlayCursor, 0, len(m.sessionRows())-1)
	m.setStatusMessage("Deleted session: "+row.name, false)
	return m, scheduleStatusClear()
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

// handleSessionsSaveMode drives the save-as name prompt: Enter commits, Esc
// cancels, other keys edit the name.
func (m Model) handleSessionsSaveMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch handleFilterKey(&m.sessionsSaveInput, msg.String()) {
	case filterAccept:
		return m.commitSessionSaveAs()
	case filterEscape, filterClose:
		m.sessionsSaveMode = false
		m.sessionsSaveInput.Clear()
		return m, nil
	case filterContinue:
		return m, nil
	}
	return m, nil
}

// commitSessionSaveAs saves the current workspace under the typed name and
// makes it the active session (the current tabs continue into the new session).
func (m Model) commitSessionSaveAs() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.sessionsSaveInput.Value)
	m.sessionsSaveMode = false
	m.sessionsSaveInput.Clear()
	if m.unionMode {
		m.setStatusMessage(":session save is disabled in union view", true)
		return m, scheduleStatusClear()
	}
	if sanitizeSessionName(name) == "" {
		m.setStatusMessage("Session name required", true)
		return m, scheduleStatusClear()
	}
	if err := saveNamedSession(NamedSession{Name: name, SavedAt: time.Now(), State: m.buildSessionState()}); err != nil {
		m.setStatusMessage("Save failed: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.activeSession = name
	if err := saveActiveSessionName(name); err != nil {
		logger.Warn("Failed to persist active session", "session", name, "error", err)
	}
	m.sessionsList = listNamedSessions()
	m.setStatusMessage("Saved session: "+name, false)
	return m, scheduleStatusClear()
}

// switchToSession replaces the current workspace with the named session (or the
// default when name is ""). It rides the startup restore path: save the current
// active session, set pendingSession, and reload contexts so
// updateContextsLoaded fires restoreSession.
func (m Model) switchToSession(name string) (tea.Model, tea.Cmd) {
	if m.unionMode {
		m.setStatusMessage("Exit union view before switching sessions", true)
		return m, scheduleStatusClear()
	}
	if name == m.activeSession {
		m.overlay = overlayNone
		return m, nil
	}
	m.saveCurrentSession() // persist the session we're leaving
	m.activeSession = name
	if err := saveActiveSessionName(name); err != nil {
		logger.Warn("Failed to persist active session", "session", name, "error", err)
	}

	var state *SessionState
	if name == "" {
		state = loadSession()
	} else if ns, err := loadNamedSession(name); err == nil {
		s := ns.State
		state = &s
	}
	m.pendingSession = state
	m.sessionRestored = false
	m.overlay = overlayNone

	label := name
	if label == "" {
		label = defaultSessionLabel
	}
	m.setStatusMessage("Switched to session: "+label, false)
	return m, tea.Batch(m.loadContexts(), scheduleStatusClear())
}

// overlayHintBarSessions returns the hint bar for the sessions picker overlay.
func (m Model) overlayHintBarSessions() string {
	if m.sessionsSaveMode {
		return m.renderHints([]ui.HintEntry{
			{Key: "enter", Desc: "save"},
			{Key: "esc", Desc: "cancel"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "enter", Desc: "switch"},
		{Key: "s", Desc: "save current"},
		{Key: "d", Desc: "delete"},
		{Key: "/", Desc: "filter"},
		{Key: "esc", Desc: "close"},
	})
}
