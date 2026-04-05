package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/ui"
)

// --- Konami Code Easter Egg ---

// konamiSequence is the classic Konami Code key sequence.
var konamiSequence = []string{"up", "up", "down", "down", "left", "right", "left", "right", "b", "a"}

// konamiClearMsg signals that the Konami status message should be cleared.
type konamiClearMsg struct{}

// checkKonami advances or resets progress through the Konami Code sequence.
// When the full sequence is entered, it activates the konamiActive flag and
// sets a status message.
func (m Model) checkKonami(msg tea.KeyMsg) Model {
	expected := konamiSequence[m.konamiProgress]
	actual := msg.String()

	if actual == expected {
		m.konamiProgress++
		if m.konamiProgress >= len(konamiSequence) {
			m.konamiProgress = 0
			m.konamiActive = true
			m.setStatusMessage("Cheat code activated. +30 extra life!", false)
		}
	} else {
		m.konamiProgress = 0
		// Check if the wrong key happens to be the start of the sequence.
		if actual == konamiSequence[0] {
			m.konamiProgress = 1
		}
	}

	return m
}

// clearKonami resets the konamiActive flag.
func (m Model) clearKonami() Model {
	m.konamiActive = false
	return m
}

// scheduleKonamiClear returns a command that fires after 5 seconds
// to clear the Konami Code activation status.
func scheduleKonamiClear() tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return konamiClearMsg{}
	})
}

// --- Nyan Mode Easter Egg ---

// nyanTickMsg signals a nyan mode animation tick.
type nyanTickMsg struct{}

// toggleNyan toggles nyan mode on/off and sets an appropriate status message.
func (m Model) toggleNyan() (Model, tea.Cmd) {
	m.nyanMode = !m.nyanMode
	if m.nyanMode {
		m.nyanTick = 0
		m.setStatusMessage("[NYAN] Nyan mode activated!", false)
		return m, scheduleNyanTick()
	}
	m.nyanTick = 0
	m.setStatusMessage("Nyan mode deactivated", false)
	return m, nil
}

// scheduleNyanTick returns a command that fires a nyanTickMsg every 150ms.
func scheduleNyanTick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(_ time.Time) tea.Msg {
		return nyanTickMsg{}
	})
}

// --- kubectl explain life Easter Egg ---

// explainLifeContent returns a joke "kubectl explain" output for the "life" resource.
func explainLifeContent() string {
	return `KIND:     Life
VERSION:  v1

DESCRIPTION:
     Life is a non-deterministic, eventually consistent resource
     with no rollback support. Created automatically; deletion
     policy is "retain forever (hopefully)".

FIELDS:
   spec.coffee <string> -required-
     Minimum daily intake. Defaults to "3 cups". Values below 1
     trigger CrashLoopBackOff.

   spec.sleep <Duration>
     Recommended 8h. Actual value typically 5h30m.

   status.phase <string>
     One of: Monday, GettingBy, Almost, Friday, Weekend.

   status.meetings <int>
     Current count. Always higher than desired.

   metadata.labels
     friday: "true" - Applied automatically every 7 days.
     remote: "true" - Working from couch since 2020.

   metadata.annotations
     kubernetes.io/description: "It's not a bug, it's a feature"`
}

// --- Credits Easter Egg ---

// creditsTickMsg signals a scroll tick for the credits animation.
type creditsTickMsg struct{}

// creditsCloseMsg signals the credits screen should close after the pause.
type creditsCloseMsg struct{}

// creditsLines returns the lines of the credits screen.
func creditsLines(version string) []string {
	return []string{
		"",
		"",
		"LFK",
		"Lightning Fast Kubernetes Navigator",
		"",
		fmt.Sprintf("Version %s", version),
		"",
		"Created by",
		"Janos Miko",
		"",
		"Built with",
		"BubbleTea -- Terminal UI Framework",
		"Lipgloss -- Style Definitions",
		"Bubbles -- TUI Components",
		"client-go -- Kubernetes Client Library",
		"",
		"Inspired by",
		"yazi, k9s, lazydocker, lazygit and many more open source tools",
		"Thank you to all the awesome open source maintainers out there! 🙌",
		"",
		"Open Source",
		"github.com/janosmiko/lfk",
		"",
		"Licensed under the MIT License",
		"",
		"Thank you for using LFK.",
		"",
		"",
	}
}

// scheduleCreditsScroll returns a command that fires a creditsTickMsg
// after 250ms for scrolling animation.
func scheduleCreditsScroll() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(_ time.Time) tea.Msg {
		return creditsTickMsg{}
	})
}

// tickCredits decrements the credits scroll position by one line.
// Returns true when the content has reached the center and scrolling should stop.
func (m Model) tickCredits() (Model, bool) {
	lines := creditsLines(m.version)
	contentLen := len(lines)
	// Stop when the content is centered vertically on screen.
	centerStop := (m.height - contentLen) / 2
	if m.creditsScroll <= centerStop {
		m.creditsStopped = true
		return m, true
	}
	m.creditsScroll--
	return m, false
}

// scheduleCreditsClose returns a command that fires a creditsCloseMsg after 10 seconds.
func scheduleCreditsClose() tea.Cmd {
	return tea.Tick(10*time.Second, func(_ time.Time) tea.Msg {
		return creditsCloseMsg{}
	})
}

// viewCredits renders the credits screen with centered, scrolled content.
func (m Model) viewCredits() string {
	lines := creditsLines(m.version)
	contentHeight := m.height
	contentWidth := m.width

	// Build the visible area by placing lines at their scrolled position.
	visible := make([]string, contentHeight)
	for i := range visible {
		visible[i] = ""
	}

	for i, line := range lines {
		screenY := m.creditsScroll + i
		if screenY >= 0 && screenY < contentHeight {
			// Center the line horizontally.
			lineWidth := lipgloss.Width(line)
			pad := (contentWidth - lineWidth) / 2
			if pad < 0 {
				pad = 0
			}

			// Style the title line differently.
			var styled string
			if line == "LFK" {
				styled = ui.HelpKeyStyle.Render(line)
			} else {
				styled = ui.BarDimStyle.Render(line)
			}

			visible[screenY] = strings.Repeat(" ", pad) + styled
		}
	}

	return strings.Join(visible, "\n")
}
