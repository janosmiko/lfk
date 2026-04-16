package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handleLogFilterOverlayKey dispatches keys when overlayLogFilter is active.
// Returns the updated model and an optional command.
//
//nolint:unparam // cmd return used by subsequent Phase E/F tasks
func (m Model) handleLogFilterOverlayKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Save-preset prompt intercepts all keys while active.
	if m.logSavePresetPrompt {
		return m.handleSavePresetPromptKey(msg), nil
	}
	// Load-preset picker intercepts all keys while active.
	if m.logLoadPresetOpen {
		return m.handleLoadPresetPickerKey(msg), nil
	}
	if msg.Type == tea.KeyTab {
		m.logFilterFocusInput = !m.logFilterFocusInput
		return m, nil
	}
	if m.logFilterFocusInput {
		if nm, handled := m.handleFilterInputKey(msg); handled {
			return nm, nil
		}
	}
	if !m.logFilterFocusInput {
		if nm, handled := m.handleFilterListKey(msg); handled {
			return nm, nil
		}
		// In list mode, only the explicit dispatch above (j/k/a/d/e/m/J/K/S/L/…)
		// can act. Any other key is a no-op so the user isn't surprised into
		// typing mode by an accidental keystroke. Use `a` to add a new rule.
	}
	// Esc behavior:
	//   - Typing mode (input focused): Esc exits typing mode → list mode.
	//     Any pending input is cleared and an in-progress edit is abandoned.
	//     The overlay stays open so the user can keep browsing rules.
	//   - List mode (input not focused): Esc closes the overlay.
	if msg.Type == tea.KeyEsc {
		if m.logFilterFocusInput {
			m.logFilterFocusInput = false
			m.logFilterInput.Clear()
			m.logFilterEditingIdx = -1
			return m, nil
		}
		m.overlay = overlayNone
		m.logFilterModalOpen = false
		m.logFilterEditingIdx = -1
		m.logFilterInput.Clear()
		return m, nil
	}
	// List mode also closes on Ctrl+C or `q` — the standard "quit overlay"
	// shortcuts used elsewhere in the app.
	if !m.logFilterFocusInput {
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			m.overlay = overlayNone
			m.logFilterModalOpen = false
			m.logFilterEditingIdx = -1
			m.logFilterInput.Clear()
			return m, nil
		}
	}
	return m, nil
}

// handleSavePresetPromptKey is the keystroke handler that is active while the
// save-preset prompt is open. It reads a name from the existing filter input
// and writes the preset to the sidecar on Enter. Saving (or cancelling)
// always returns to list (nav) mode, so the user's cursor is on the
// rules table after the prompt closes — matches the "S opens from list
// mode" expectation. Without this, if the filter input was focused
// before the prompt, the modal would revert to input mode on close.
func (m Model) handleSavePresetPromptKey(msg tea.KeyMsg) Model {
	switch msg.Type {
	case tea.KeyEsc:
		m.logSavePresetPrompt = false
		m.logFilterFocusInput = false
		m.logFilterInput.Set("")
		return m
	case tea.KeyEnter:
		name := strings.TrimSpace(m.logFilterInput.Value)
		if name == "" {
			m.logSavePresetPrompt = false
			m.logFilterFocusInput = false
			return m
		}
		path, err := logPresetsPath()
		if err != nil {
			m.logSavePresetPrompt = false
			m.logFilterFocusInput = false
			m.logFilterInput.Set("")
			return m
		}
		f, _ := readPresetFile(path)
		if f.Presets == nil {
			f.Presets = make(map[string][]LogPreset)
		}
		kind := m.actionCtx.kind
		preset := rulesToPreset(name, false, m.logIncludeMode, m.logRules)
		// Replace by name if exists.
		replaced := false
		for i, p := range f.Presets[kind] {
			if p.Name == name {
				f.Presets[kind][i] = preset
				replaced = true
				break
			}
		}
		if !replaced {
			f.Presets[kind] = append(f.Presets[kind], preset)
		}
		_ = writePresetFile(path, f)
		m.logSavePresetPrompt = false
		m.logFilterFocusInput = false
		m.logFilterInput.Set("")
		return m
	case tea.KeyBackspace:
		m.logFilterInput.Backspace()
		return m
	case tea.KeySpace:
		m.logFilterInput.Insert(" ")
		return m
	case tea.KeyRunes:
		m.logFilterInput.Insert(string(msg.Runes))
		return m
	}
	return m
}

// handleFilterInputKey handles keys when the filter modal input is focused.
// Returns (updatedModel, true) when the key was handled, (m, false) otherwise.
func (m Model) handleFilterInputKey(msg tea.KeyMsg) (Model, bool) {
	switch msg.Type {
	case tea.KeyEnter:
		return m.commitFilterInput(), true
	case tea.KeyBackspace:
		m.logFilterInput.Backspace()
		return m, true
	case tea.KeyCtrlW:
		// Unix convention: delete the previous word.
		m.logFilterInput.DeleteWord()
		return m, true
	case tea.KeyCtrlU:
		// Unix convention: delete everything before the cursor.
		m.logFilterInput.DeleteLine()
		return m, true
	case tea.KeySpace:
		// Bubbletea delivers space as its own Key type, not KeyRunes —
		// without this case, spaces would be silently dropped (needed
		// for group syntax like "foo AND bar" or multi-word patterns).
		m.logFilterInput.Insert(" ")
		return m, true
	case tea.KeyRunes:
		m.logFilterInput.Insert(string(msg.Runes))
		return m, true
	}
	return m, false
}

// cloneRules returns a fresh slice with the same elements so the value
// receiver can safely mutate without aliasing the caller's backing array.
// This is required on every handler that writes to logRules because Model
// is passed by value through Bubble Tea's Update loop — in-place slice
// mutation would otherwise corrupt any other code holding a pre-mutation
// reference (tab snapshots, tests, future undo history, etc.).
func cloneRules(rules []Rule) []Rule {
	out := make([]Rule, len(rules))
	copy(out, rules)
	return out
}

// firstEditableRuleIdx returns the smallest index in rules whose entry
// is NOT a SeverityRule — i.e. the first rule the user can delete or
// edit via d/e. Falls back to 0 when no editable rule exists so j/k
// still have a valid starting position.
func firstEditableRuleIdx(rules []Rule) int {
	for i, r := range rules {
		if _, sev := r.(SeverityRule); !sev {
			return i
		}
	}
	return 0
}

// minFilterCursor returns the smallest valid cursor position for the
// rules list: 1 when severity is pinned at index 0 (cursor skips past
// it — severity is read-only), 0 otherwise.
func minFilterCursor(rules []Rule) int {
	if len(rules) > 0 {
		if _, sev := rules[0].(SeverityRule); sev {
			return 1
		}
	}
	return 0
}

// clampFilterCursor pins m.logFilterListCursor into the legal range for
// the current rules slice, ensuring the cursor never lands on the
// read-only severity row. When there are no editable rules (e.g. only
// severity is present), parks the cursor at -1 so nothing is highlighted.
func (m *Model) clampFilterCursor() {
	lo := minFilterCursor(m.logRules)
	hi := len(m.logRules) - 1
	if lo > hi {
		// No editable rules — park cursor out of range so the render
		// layer can skip highlighting.
		m.logFilterListCursor = -1
		return
	}
	if m.logFilterListCursor < lo {
		m.logFilterListCursor = lo
	}
	if m.logFilterListCursor > hi {
		m.logFilterListCursor = hi
	}
}

// pinSeverityFirst rearranges rules so that any SeverityRule sits at
// index 0. Severity is pinned to the top of the rules list because it
// is always evaluated as a hard gate (AND with every other rule)
// regardless of the top-level IncludeMode — surfacing that at the top
// of the display makes the semantics obvious. At most one severity rule
// is expected; if duplicates somehow appear, only the first is kept.
func pinSeverityFirst(rules []Rule) []Rule {
	out := make([]Rule, 0, len(rules))
	var sev Rule
	for _, r := range rules {
		if _, ok := r.(SeverityRule); ok {
			if sev == nil {
				sev = r
			}
			continue
		}
		out = append(out, r)
	}
	if sev != nil {
		return append([]Rule{sev}, out...)
	}
	return out
}

// commitFilterInput parses the current input and adds or edits a rule.
func (m Model) commitFilterInput() Model {
	input := m.logFilterInput.Value
	rule, err := ParseRuleInput(input)
	if err != nil {
		// status: invalid input
		return m
	}
	// Clone before any mutation to break the value-receiver aliasing: the
	// caller's slice may share a backing array with m.logRules when cap
	// leaves headroom for append, and direct index writes would leak.
	newRules := cloneRules(m.logRules)
	if m.logFilterEditingIdx >= 0 && m.logFilterEditingIdx < len(newRules) {
		newRules[m.logFilterEditingIdx] = rule
		m.logFilterEditingIdx = -1
	} else {
		// Replace existing severity rule if adding another (only one allowed).
		replaced := false
		if rule.Kind() == RuleSeverity {
			for i, r := range newRules {
				if r.Kind() == RuleSeverity {
					newRules[i] = rule
					replaced = true
					break
				}
			}
		}
		if !replaced {
			newRules = append(newRules, rule)
		}
	}
	m.logRules = pinSeverityFirst(newRules)
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	m.logFilterInput.Set("")
	// Return to list (nav) mode after commit so the user can immediately
	// see / navigate the just-added rule. Adding another rule is one `a`
	// keystroke away; this matches the modal-dialog mental model "type,
	// commit, dismiss input".
	m.logFilterFocusInput = false
	return m
}

// listPageSize is the number of rows to move on Ctrl+D / Ctrl+U
// (half-page) in the filter modal's list mode. Chosen to roughly match
// the table-body slot so one press advances a visible page.
const listPageSize = 5

// handleFilterListKey handles keys in list mode (input not focused).
// Returns (updatedModel, true) when the key was handled, (m, false) otherwise.
func (m Model) handleFilterListKey(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case "j", "down":
		if m.logFilterListCursor < len(m.logRules)-1 {
			m.logFilterListCursor++
		}
		m.clampFilterCursor()
		return m, true
	case "k", "up":
		if m.logFilterListCursor > 0 {
			m.logFilterListCursor--
		}
		m.clampFilterCursor()
		return m, true
	case "ctrl+d":
		m.logFilterListCursor += listPageSize
		m.clampFilterCursor()
		return m, true
	case "ctrl+u":
		m.logFilterListCursor -= listPageSize
		m.clampFilterCursor()
		return m, true
	case "ctrl+f":
		m.logFilterListCursor += 2 * listPageSize
		m.clampFilterCursor()
		return m, true
	case "ctrl+b":
		m.logFilterListCursor -= 2 * listPageSize
		m.clampFilterCursor()
		return m, true
	case "g":
		// Jump to the first user-editable row — this already skips
		// the pinned severity slot when present.
		m.logFilterListCursor = firstEditableRuleIdx(m.logRules)
		m.clampFilterCursor()
		return m, true
	case "G":
		if len(m.logRules) > 0 {
			m.logFilterListCursor = len(m.logRules) - 1
		}
		m.clampFilterCursor()
		return m, true
	case "a":
		// Add a new rule: switch to input mode with a clean input.
		m.logFilterFocusInput = true
		m.logFilterEditingIdx = -1
		m.logFilterInput.Set("")
		return m, true
	case "d":
		return m.deleteSelectedRule(), true
	case "e":
		return m.editSelectedRule(), true
	case "m":
		return m.toggleIncludeMode(), true
	case "J":
		return m.moveSelectedRuleDown(), true
	case "K":
		return m.moveSelectedRuleUp(), true
	case ">":
		// Cycle severity floor up — same as in the log view. Useful so
		// the user doesn't have to close the overlay to change severity.
		return m.setSeverityFloor(nextSeverityFloor(m.currentSeverityFloor(), +1)), true
	case "<":
		return m.setSeverityFloor(nextSeverityFloor(m.currentSeverityFloor(), -1)), true
	case "S":
		if len(m.logRules) == 0 {
			return m, true // nothing to save
		}
		m.logSavePresetPrompt = true
		m.logFilterInput.Set("")
		return m, true
	case "L":
		m.logLoadPresetOpen = true
		m.logLoadPresetCursor = 0
		return m, true
	}
	return m, false
}

// handleLoadPresetPickerKey handles keystrokes while the preset picker is open.
// Keys: j/k navigate, Enter applies, d deletes, space toggles default, Esc closes.
func (m Model) handleLoadPresetPickerKey(msg tea.KeyMsg) Model {
	path, _ := logPresetsPath()
	f, _ := readPresetFile(path)
	kind := m.actionCtx.kind
	presets := f.Presets[kind]

	switch msg.Type {
	case tea.KeyEsc:
		m.logLoadPresetOpen = false
		return m
	case tea.KeyEnter:
		if m.logLoadPresetCursor >= 0 && m.logLoadPresetCursor < len(presets) {
			rules, mode, err := presetToRules(presets[m.logLoadPresetCursor])
			if err == nil {
				m.logRules = pinSeverityFirst(rules)
				m.logIncludeMode = mode
				m.logFilterChain = NewFilterChain(m.logRules, mode, m.logSeverityDetector)
				m.rebuildLogVisibleIndices()
			}
		}
		m.logLoadPresetOpen = false
		return m
	}

	switch msg.String() {
	case "j":
		if m.logLoadPresetCursor < len(presets)-1 {
			m.logLoadPresetCursor++
		}
	case "k":
		if m.logLoadPresetCursor > 0 {
			m.logLoadPresetCursor--
		}
	case "d":
		if m.logLoadPresetCursor >= 0 && m.logLoadPresetCursor < len(presets) {
			f.Presets[kind] = append(presets[:m.logLoadPresetCursor], presets[m.logLoadPresetCursor+1:]...)
			_ = writePresetFile(path, f)
			if m.logLoadPresetCursor >= len(f.Presets[kind]) && m.logLoadPresetCursor > 0 {
				m.logLoadPresetCursor--
			}
		}
	case " ":
		// Toggle default: clear all, set current if it wasn't already default.
		if m.logLoadPresetCursor >= 0 && m.logLoadPresetCursor < len(presets) {
			cur := presets[m.logLoadPresetCursor].Default
			for i := range f.Presets[kind] {
				f.Presets[kind][i].Default = false
			}
			if !cur {
				f.Presets[kind][m.logLoadPresetCursor].Default = true
			}
			_ = writePresetFile(path, f)
		}
	}
	return m
}

// deleteSelectedRule removes the rule at the list cursor and rebuilds the chain.
// Severity rows are read-only here — they can only be cycled via >/<
// (setSeverityFloor). Keeps the user from accidentally removing the
// severity gate when they meant to delete a pattern rule.
func (m Model) deleteSelectedRule() Model {
	if m.logFilterListCursor >= 0 && m.logFilterListCursor < len(m.logRules) {
		if _, sev := m.logRules[m.logFilterListCursor].(SeverityRule); sev {
			m.setStatusMessage("Severity is read-only — use >/< to change the floor", false)
			return m
		}
	}
	return m.deleteSelectedRuleInternal()
}

// deleteSelectedRuleInternal is the implementation sans severity guard,
// kept separate so we can keep the bulk of the logic intact.
func (m Model) deleteSelectedRuleInternal() Model {
	if m.logFilterListCursor < 0 || m.logFilterListCursor >= len(m.logRules) {
		return m
	}
	// Build a fresh slice that omits the deleted rule; the in-place
	// `append(slice[:i], slice[i+1:]...)` pattern would shift elements in
	// the caller's shared backing array.
	newRules := make([]Rule, 0, len(m.logRules)-1)
	newRules = append(newRules, m.logRules[:m.logFilterListCursor]...)
	newRules = append(newRules, m.logRules[m.logFilterListCursor+1:]...)
	m.logRules = pinSeverityFirst(newRules)
	if m.logFilterListCursor >= len(m.logRules) && m.logFilterListCursor > 0 {
		m.logFilterListCursor--
	}
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	return m
}

// editSelectedRule loads the selected rule's syntax into the input for editing.
// Severity rows are read-only — changing the floor is done via >/<.
func (m Model) editSelectedRule() Model {
	if m.logFilterListCursor < 0 || m.logFilterListCursor >= len(m.logRules) {
		return m
	}
	if _, sev := m.logRules[m.logFilterListCursor].(SeverityRule); sev {
		m.setStatusMessage("Severity is read-only — use >/< to change the floor", false)
		return m
	}
	m.logFilterInput.Set(RuleToInputString(m.logRules[m.logFilterListCursor]))
	m.logFilterEditingIdx = m.logFilterListCursor
	m.logFilterFocusInput = true
	return m
}

// toggleIncludeMode flips between IncludeAny and IncludeAll and rebuilds the chain.
func (m Model) toggleIncludeMode() Model {
	if m.logIncludeMode == IncludeAny {
		m.logIncludeMode = IncludeAll
	} else {
		m.logIncludeMode = IncludeAny
	}
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	return m
}

// moveSelectedRuleDown swaps the selected rule with the one below it.
// The pinned severity slot (always at index 0 when present) is skipped —
// swapping into position 0 would un-pin severity.
func (m Model) moveSelectedRuleDown() Model {
	i := m.logFilterListCursor
	if i < 0 || i >= len(m.logRules)-1 {
		return m
	}
	// Don't swap into the severity slot at 0, and don't swap the severity
	// slot away from 0.
	if _, cursorIsSev := m.logRules[i].(SeverityRule); cursorIsSev {
		return m
	}
	if _, belowIsSev := m.logRules[i+1].(SeverityRule); belowIsSev {
		return m
	}
	newRules := cloneRules(m.logRules)
	newRules[i], newRules[i+1] = newRules[i+1], newRules[i]
	m.logRules = newRules
	m.logFilterListCursor++
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	return m
}

// moveSelectedRuleUp swaps the selected rule with the one above it.
// Respects the pinned severity slot at index 0.
func (m Model) moveSelectedRuleUp() Model {
	i := m.logFilterListCursor
	if i <= 0 || i >= len(m.logRules) {
		return m
	}
	if _, cursorIsSev := m.logRules[i].(SeverityRule); cursorIsSev {
		return m
	}
	if _, aboveIsSev := m.logRules[i-1].(SeverityRule); aboveIsSev {
		return m
	}
	newRules := cloneRules(m.logRules)
	newRules[i], newRules[i-1] = newRules[i-1], newRules[i]
	m.logRules = newRules
	m.logFilterListCursor--
	m.logFilterChain = NewFilterChain(m.logRules, m.logIncludeMode, m.logSeverityDetector)
	m.rebuildLogVisibleIndices()
	return m
}
