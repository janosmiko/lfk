package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/ui"
)

// nowFunc is the time source used by security state updates. Tests override it.
var nowFunc = time.Now

// updateSecurityMsg dispatches security-related messages. It is split out from
// updateResourceMsg to keep the main dispatch switch below the gocyclo limit.
// Returns the updated Model and true if the message was handled. All current
// security messages are fire-and-forget (no follow-up tea.Cmd), so the caller
// does not need a command channel.
func (m Model) updateSecurityMsg(msg tea.Msg) (Model, bool) {
	switch msg := msg.(type) {
	case securityFindingsLoadedMsg:
		return m.handleSecurityFindingsLoaded(msg), true
	case securityFetchErrorMsg:
		m.securityView.Loading = false
		m.securityView.LastError = msg.err
		return m, true
	case securityAvailabilityLoadedMsg:
		return m.handleSecurityAvailabilityLoaded(msg), true
	}
	return m, false
}

// handleSecurityKey handles key events when the security preview is focused
// (i.e., selectedMiddleItem().Extra == "__security__"). Returns the updated
// Model and an optional command.
func (m Model) handleSecurityKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.securityView = cycleSecurityCategory(m.securityView, +1)
		return m, nil
	case tea.KeyShiftTab:
		m.securityView = cycleSecurityCategory(m.securityView, -1)
		return m, nil
	case tea.KeyEnter:
		m.securityView.ShowDetail = !m.securityView.ShowDetail
		return m, nil
	}
	if len(msg.Runes) == 0 {
		return m, nil
	}
	switch msg.Runes[0] {
	case 'j':
		m = securityCursorDown(m)
	case 'k':
		m = securityCursorUp(m)
	case 'g':
		m.securityView.Cursor = 0
		m.securityView.Scroll = 0
	case 'G':
		visible := m.securityView.VisibleFindings()
		if len(visible) > 0 {
			m.securityView.Cursor = len(visible) - 1
		}
	case 'r':
		m.securityView.Loading = true
		return m, m.loadSecurityDashboard()
	case 'C':
		m.securityView.ResourceFilter = nil
	case '1':
		m = activateSecurityCategoryByIndex(m, 0)
	case '2':
		m = activateSecurityCategoryByIndex(m, 1)
	case '3':
		m = activateSecurityCategoryByIndex(m, 2)
	case '4':
		m = activateSecurityCategoryByIndex(m, 3)
	}
	return m, nil
}

func cycleSecurityCategory(state ui.SecurityViewState, delta int) ui.SecurityViewState {
	if len(state.AvailableCategories) == 0 {
		return state
	}
	idx := 0
	for i, c := range state.AvailableCategories {
		if c == state.ActiveCategory {
			idx = i
			break
		}
	}
	n := len(state.AvailableCategories)
	idx = (idx + delta + n) % n
	state.ActiveCategory = state.AvailableCategories[idx]
	state.Cursor = 0
	state.Scroll = 0
	return state
}

func activateSecurityCategoryByIndex(m Model, idx int) Model {
	if idx < 0 || idx >= len(m.securityView.AvailableCategories) {
		return m
	}
	m.securityView.ActiveCategory = m.securityView.AvailableCategories[idx]
	m.securityView.Cursor = 0
	m.securityView.Scroll = 0
	return m
}

func securityCursorDown(m Model) Model {
	visible := m.securityView.VisibleFindings()
	if m.securityView.Cursor < len(visible)-1 {
		m.securityView.Cursor++
	}
	return m
}

func securityCursorUp(m Model) Model {
	if m.securityView.Cursor > 0 {
		m.securityView.Cursor--
	}
	return m
}

// handleSecurityFindingsLoaded processes a securityFindingsLoadedMsg.
func (m Model) handleSecurityFindingsLoaded(msg securityFindingsLoadedMsg) Model {
	// Discard stale results from a previous context.
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m
	}
	m.securityView.Findings = msg.result.Findings
	m.securityView.Loading = false
	m.securityView.LastError = nil
	m.securityView.LastFetch = nowFunc()

	// Recompute available categories from the source status list.
	catSet := map[security.Category]bool{}
	for _, s := range msg.result.Sources {
		if !s.Available {
			continue
		}
		if m.securityManager == nil {
			continue
		}
		for _, src := range m.securityManager.Sources() {
			if src.Name() != s.Name {
				continue
			}
			for _, c := range src.Categories() {
				catSet[c] = true
			}
		}
	}
	m.securityView.AvailableCategories = nil
	for _, c := range []security.Category{
		security.CategoryVuln, security.CategoryMisconfig,
		security.CategoryPolicy, security.CategoryCompliance,
	} {
		if catSet[c] {
			m.securityView.AvailableCategories = append(m.securityView.AvailableCategories, c)
		}
	}
	if m.securityView.ActiveCategory == "" && len(m.securityView.AvailableCategories) > 0 {
		m.securityView.ActiveCategory = m.securityView.AvailableCategories[0]
	}

	// Rebuild the manager's FindingIndex from the async message payload so
	// the SEC column and per-resource lookups stay fresh. The manager's
	// FetchAll cache is bypassed when findings arrive via tea.Msg.
	if m.securityManager != nil {
		m.securityManager.SetIndex(security.BuildFindingIndex(msg.result.Findings))
	}

	return m
}

// handleSecurityAvailabilityLoaded updates the cached availability flag.
func (m Model) handleSecurityAvailabilityLoaded(msg securityAvailabilityLoadedMsg) Model {
	if msg.context != m.nav.Context && m.nav.Context != "" {
		return m
	}
	m.securityAvailable = msg.available
	return m
}
