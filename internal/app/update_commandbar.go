package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleCommandBarKey processes key events when the command bar is active.
func (m Model) handleCommandBarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Paste {
		return m.handleCommandBarPaste(msg)
	}

	key := msg.String()

	// Right or Space accepts ghost preview if active (not loading placeholders).
	if (key == "right" || key == " ") && m.commandBarPreview != "" && m.commandBarPreview != "loading..." {
		m.commandBarInput.Set(m.commandBarApplySuggestion(m.commandBarPreview))
		m.commandBarPreview = ""
		return m.commandBarRefreshSuggestions()
	}

	// Tab/Shift+Tab/Ctrl+N/P don't clear preview (they cycle).
	// All other keys clear the preview.
	if key != "tab" && key != "shift+tab" && key != "ctrl+n" && key != "ctrl+p" {
		m.commandBarPreview = ""
	}

	switch key {
	case "ctrl+@", "ctrl+space":
		return m.commandBarRefreshSuggestions()
	case "esc":
		return m.commandBarHandleEsc()
	case "ctrl+c":
		return m.commandBarClose(), nil
	case "enter":
		return m.commandBarEnter()
	case "tab", "ctrl+n", "down":
		return m.commandBarCycleForward(key)
	case "shift+tab", "ctrl+p", "up":
		return m.commandBarCycleBackward(key)
	case "ctrl+d", "ctrl+u":
		return m.commandBarScrollSuggestions(key, 5)
	case "ctrl+f", "ctrl+b":
		return m.commandBarScrollSuggestions(key, 10)
	case "ctrl+a":
		m.commandBarInput.Home()
		return m, nil
	case "ctrl+e":
		m.commandBarInput.End()
		return m, nil
	case "backspace":
		if len(m.commandBarInput.Value) > 0 {
			m.commandBarInput.Backspace()
		}
		return m.commandBarRefreshSuggestions()
	case "ctrl+w":
		m.commandBarInput.DeleteWord()
		return m.commandBarRefreshSuggestions()
	case "left":
		m.commandBarInput.Left()
		return m, nil
	case "right":
		m.commandBarInput.Right()
		return m, nil
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.commandBarInput.Insert(key)
		}
		return m.commandBarRefreshSuggestions()
	}
}

func (m Model) commandBarHandleEsc() (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) > 0 {
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		m.commandBarPreview = ""
		return m, nil
	}
	return m.commandBarClose(), nil
}

func (m Model) commandBarCycleForward(key string) (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) > 0 {
		sel := m.commandBarSuggestions[m.commandBarSelectedSuggestion]
		if sel.Category == "status" {
			return m, nil
		}
		if key == "tab" && m.commandBarActionableSuggestionCount() == 1 {
			m.commandBarInput.Set(m.commandBarApplySuggestion(sel.Text))
			m.commandBarPreview = ""
			return m.commandBarRefreshSuggestions()
		}
		m.commandBarCycleSuggestion(1)
		for m.commandBarSuggestions[m.commandBarSelectedSuggestion].Category == "status" {
			m.commandBarCycleSuggestion(1)
		}
		m.commandBarPreview = m.commandBarSuggestions[m.commandBarSelectedSuggestion].Text
		return m, nil
	}
	if key == "down" {
		m.commandBarInput.Set(m.commandHistory.down())
	}
	return m, nil
}

func (m Model) commandBarCycleBackward(key string) (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) > 0 {
		m.commandBarCycleSuggestion(-1)
		for m.commandBarSuggestions[m.commandBarSelectedSuggestion].Category == "status" {
			m.commandBarCycleSuggestion(-1)
		}
		m.commandBarPreview = m.commandBarSuggestions[m.commandBarSelectedSuggestion].Text
		return m, nil
	}
	if key == "up" {
		m.commandBarInput.Set(m.commandHistory.up(m.commandBarInput.Value))
	}
	return m, nil
}

func (m Model) commandBarScrollSuggestions(key string, amount int) (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) == 0 {
		// ctrl+u without suggestions: delete line.
		if key == "ctrl+u" {
			m.commandBarInput.DeleteLine()
			return m.commandBarRefreshSuggestions()
		}
		return m, nil
	}
	delta := amount
	if key == "ctrl+u" || key == "ctrl+b" {
		delta = -amount
	}
	m.commandBarCycleSuggestion(delta)
	m.commandBarPreview = m.commandBarSuggestions[m.commandBarSelectedSuggestion].Text
	return m, nil
}

// commandBarActionableSuggestionCount returns the number of suggestions
// that are not status placeholders (e.g., "loading...").
func (m Model) commandBarActionableSuggestionCount() int {
	count := 0
	for _, s := range m.commandBarSuggestions {
		if s.Category != "status" {
			count++
		}
	}
	return count
}

func (m Model) handleCommandBarPaste(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	text := strings.TrimRight(string(msg.Runes), "\n")
	if strings.Contains(text, "\n") {
		m.triggerPasteConfirm(text, pasteTargetCommandBar)
		return m, nil
	}
	if text != "" {
		m.commandBarInput.Insert(text)
	}
	return m.commandBarRefreshSuggestions()
}

func (m Model) commandBarClose() Model {
	m.commandBarActive = false
	m.commandBarInput.Clear()
	m.commandBarSuggestions = nil
	m.commandBarSelectedSuggestion = 0
	m.commandBarPreview = ""
	return m
}

func (m Model) commandBarEnter() (tea.Model, tea.Cmd) {
	// If suggestions are visible, accept the selected one and clear suggestions.
	if len(m.commandBarSuggestions) > 0 {
		suggestion := m.commandBarSuggestions[m.commandBarSelectedSuggestion]
		if suggestion.Category != "status" {
			m.commandBarInput.Set(m.commandBarApplySuggestion(suggestion.Text))
		}
		m.commandBarPreview = ""
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	}

	// No suggestions visible: execute the current input.
	m.commandBarActive = false
	input := m.commandBarInput.Value
	m.commandBarInput.Clear()
	m.commandBarSuggestions = nil
	m.commandBarSelectedSuggestion = 0
	m.commandBarPreview = ""
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return m, nil
	}
	m.commandHistory.add(trimmed)
	m.commandHistory.save()
	return m.executeCommandBarInput(input)
}

func (m *Model) commandBarCycleSuggestion(delta int) {
	if len(m.commandBarSuggestions) == 0 {
		return
	}
	m.commandBarSelectedSuggestion += delta
	n := len(m.commandBarSuggestions)
	m.commandBarSelectedSuggestion = ((m.commandBarSelectedSuggestion % n) + n) % n
}

func (m Model) commandBarRefreshSuggestions() (Model, tea.Cmd) {
	oldLoading := m.commandBarNameLoading
	m.commandBarSuggestions = m.generateCommandBarSuggestions()
	m.commandBarSelectedSuggestion = 0
	m.commandBarPreview = ""

	// If a new async fetch was triggered, add a loading placeholder and fire the fetch.
	if m.commandBarNameLoading != "" && m.commandBarNameLoading != oldLoading {
		m.commandBarSuggestions = append(m.commandBarSuggestions,
			ui.Suggestion{Text: "loading...", Category: "status"})
		// Parse cache key to get namespace and resource type.
		parts := strings.SplitN(m.commandBarNameLoading, "/", 3)
		if len(parts) == 3 {
			return m, m.fetchCommandBarResourceNames(parts[2], parts[1])
		}
	}
	return m, nil
}

// commandBarApplySuggestion replaces the current partial word in the input
// with the accepted suggestion, followed by a trailing space.
func (m Model) commandBarApplySuggestion(suggestion string) string {
	input := m.commandBarInput.Value
	// If input ends with a space, append the suggestion as a new word.
	if strings.HasSuffix(input, " ") || input == "" {
		return input + suggestion
	}
	// Otherwise replace the last partial word.
	if idx := strings.LastIndex(input, " "); idx >= 0 {
		return input[:idx+1] + suggestion
	}
	return suggestion
}
