package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// openExplainBrowser determines the resource type from the current navigation
// context and launches kubectl explain for it.
func (m Model) openExplainBrowser() (tea.Model, tea.Cmd) {
	var resource, apiVersion string

	switch m.nav.Level {
	case model.LevelResourceTypes:
		// At resource types level: use the selected middle item.
		sel := m.selectedMiddleItem()
		if sel == nil {
			m.setStatusMessage("No resource type selected", true)
			return m, scheduleStatusClear()
		}
		// Skip virtual items (overview, monitoring, collapsed groups, etc.).
		if sel.Kind == "__collapsed_group__" || sel.Kind == "__overview__" ||
			sel.Kind == "__monitoring__" || sel.Extra == "__overview__" ||
			sel.Extra == "__monitoring__" {
			m.setStatusMessage("Cannot explain this item", true)
			return m, scheduleStatusClear()
		}

		// At LevelResourceTypes, Item.Extra holds the resource ref in
		// format "group/version/resource" (from ResourceTypeEntry.ResourceRef()).
		// We need to find the actual ResourceTypeEntry to build the kubectl explain specifier.
		crds := m.discoveredResources[m.discoveryContext()]
		rt, ok := model.FindResourceTypeIn(sel.Extra, crds)
		if ok {
			resource, apiVersion = buildExplainResourceFromType(rt)
		} else {
			// Fallback: use the kind name lowercased.
			if sel.Kind != "" {
				resource = strings.ToLower(sel.Kind) + "s"
			}
		}
		if resource == "" {
			m.setStatusMessage("Cannot determine resource type", true)
			return m, scheduleStatusClear()
		}

	case model.LevelResources, model.LevelOwned, model.LevelContainers:
		// Use the current resource type from navigation state.
		rt := m.nav.ResourceType
		resource, apiVersion = buildExplainResourceFromType(rt)
		if resource == "" {
			m.setStatusMessage("Cannot determine resource type", true)
			return m, scheduleStatusClear()
		}

	default:
		m.setStatusMessage("Select a resource type first", true)
		return m, scheduleStatusClear()
	}

	m.loading = true
	m.explainReturnMode = modeExplorer
	m.explainPendingField = ""
	m.explainResource = resource
	m.explainAPIVersion = apiVersion
	m.setStatusMessage("Loading API Explorer...", false)
	return m, m.execKubectlExplain(resource, apiVersion, "")
}

// openExplainAtObjectPath opens the API Explorer for the current resource type
// at the schema level CONTAINING the item under the cursor, with the cursor
// placed on that field so it reads in the context of its siblings (e.g.
// spec.dnsPolicy opens "spec" with the cursor on "dnsPolicy", not the bare
// dnsPolicy leaf). Array indices ("[i]") are stripped since kubectl explain
// describes the element schema. returnMode is the view to return to on q/esc.
func (m Model) openExplainAtObjectPath(objPath []string, returnMode viewMode) (tea.Model, tea.Cmd) {
	rt := m.nav.ResourceType
	resource, apiVersion := buildExplainResourceFromType(rt)
	if resource == "" && rt.Kind != "" {
		resource = strings.ToLower(rt.Kind) + "s"
	}
	if resource == "" {
		m.setStatusMessage("Cannot determine resource type", true)
		return m, scheduleStatusClear()
	}
	parentPath, field := explainTarget(objPath)
	m.loading = true
	m.explainReturnMode = returnMode
	m.explainPendingField = field
	m.explainResource = resource
	m.explainAPIVersion = apiVersion
	m.setStatusMessage("Loading API Explorer...", false)
	return m, m.execKubectlExplain(resource, apiVersion, parentPath)
}

// explainTarget converts an object path into the kubectl-explain schema path of
// the item's PARENT plus the item's own field name. Array indices ("[i]") are
// dropped. For a root or empty path, both are empty.
func explainTarget(objPath []string) (parentPath, field string) {
	segs := make([]string, 0, len(objPath))
	for _, s := range objPath {
		if strings.HasPrefix(s, "[") {
			continue
		}
		segs = append(segs, s)
	}
	if len(segs) == 0 {
		return "", ""
	}
	return strings.Join(segs[:len(segs)-1], "."), segs[len(segs)-1]
}

// buildExplainResourceFromType returns the resource name and api-version flag value
// for kubectl explain. The resource is just the plural name (e.g., "deployments").
// The apiVersion is "group/version" (e.g., "apps/v1") for non-core resources, empty for core.
func buildExplainResourceFromType(rt model.ResourceTypeEntry) (resource, apiVersion string) {
	if rt.Resource == "" {
		return "", ""
	}
	if rt.APIGroup != "" && rt.APIVersion != "" {
		return rt.Resource, rt.APIGroup + "/" + rt.APIVersion
	}
	return rt.Resource, ""
}

// exitExplainView resets all explain state and returns to the mode the API
// Explorer was opened from (the explorer by default; the YAML viewer or
// resource tree when opened from there via the I key).
func (m *Model) exitExplainView() {
	m.mode = m.explainReturnMode
	m.explainReturnMode = modeExplorer
	m.explainFields = nil
	m.explainDesc = ""
	m.explainPath = ""
	m.explainResource = ""
	m.explainAPIVersion = ""
	m.explainTitle = ""
	m.explainCursor = 0
	m.explainScroll = 0
	m.explainSearchQuery = ""
	m.explainSearchActive = false
	m.explainRecursiveFilter.Clear()
	m.explainRecursiveFilterActive = false
}

// handleExplainKey handles keyboard input in the explain view mode.
func (m Model) handleExplainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fieldCount := len(m.explainFields)
	visibleLines := max(m.height-6, 3)

	switch msg.String() {
	case "?", "f1":
		return m.handleExplainKeyQuestion()
	case "q":
		return m.handleExplainKeyQ()
	case "esc":
		return m.handleExplainKeyEsc()
	case "/":
		return m.handleExplainKeySlash()
	case "n":
		return m.handleExplainKeyN()
	case "N":
		return m.handleExplainKeyN2()
	case "r":
		return m.handleExplainKeyR()
	case "j", "down":
		return m.handleExplainKeyJ(fieldCount, visibleLines)
	case "k", "up":
		return m.handleExplainKeyK()
	case "g":
		return m.handleExplainKeyG()
	case "G":
		return m.handleExplainKeyG2(fieldCount, visibleLines)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.explainLineInput += msg.String()
		return m, nil
	case "0":
		return m.handleExplainKeyZero()
	case "ctrl+d":
		return m.handleExplainPageMove(visibleLines/2, fieldCount, visibleLines)
	case "ctrl+u":
		return m.handleExplainPageMove(-visibleLines/2, fieldCount, visibleLines)
	case "ctrl+f", "pgdown":
		return m.handleExplainPageMove(visibleLines, fieldCount, visibleLines)
	case "ctrl+b", "pgup":
		return m.handleExplainPageMove(-visibleLines, fieldCount, visibleLines)
	case "home":
		m.pendingG = false
		m.explainLineInput = ""
		m.explainCursor = 0
		m.explainScroll = 0
		return m, nil
	case "end":
		m.explainLineInput = ""
		if fieldCount > 0 {
			m.explainCursor = fieldCount - 1
			m.explainScroll = max(fieldCount-visibleLines, 0)
		}
		return m, nil
	case "l", "right", "enter":
		return m.handleExplainKeyDrill(fieldCount)
	case "h", "left", "backspace":
		return m.handleExplainKeyH()
	case "ctrl+c":
		m.explainLineInput = ""
		return m.closeTabOrQuit()
	default:
		m.explainLineInput = ""
	}

	return m, nil
}

// handleExplainSearchKey handles keyboard input when search is active in the explain view.
func (m Model) handleExplainSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.explainSearchActive = false
		m.explainSearchQuery = m.explainSearchInput.Value
		return m, nil
	case "esc":
		m.explainSearchActive = false
		m.explainSearchInput.Clear()
		m.explainCursor = m.explainSearchPrevCursor
		// Adjust scroll to keep cursor visible.
		visibleLines := max(m.height-6, 3)
		if m.explainCursor < m.explainScroll {
			m.explainScroll = m.explainCursor
		} else if m.explainCursor >= m.explainScroll+visibleLines {
			m.explainScroll = m.explainCursor - visibleLines + 1
		}
		return m, nil
	case "backspace":
		if len(m.explainSearchInput.Value) > 0 {
			m.explainSearchInput.Backspace()
			m.explainJumpToMatch(m.explainSearchInput.Value, 0, true)
		}
		return m, nil
	case "ctrl+w":
		m.explainSearchInput.DeleteWord()
		m.explainJumpToMatch(m.explainSearchInput.Value, 0, true)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.explainSearchInput.Insert(key)
			m.explainJumpToMatch(m.explainSearchInput.Value, m.explainCursor, true)
		}
		return m, nil
	}
}

// explainJumpToMatch jumps the cursor to the next (or previous) field matching the search query.
// Returns true if a match was found, false otherwise.
func (m *Model) explainJumpToMatch(searchQuery string, startIdx int, forward bool) bool {
	query := strings.ToLower(searchQuery)
	if query == "" {
		return false
	}
	fieldCount := len(m.explainFields)
	if fieldCount == 0 {
		return false
	}

	visibleLines := max(m.height-6, 3)

	for i := range fieldCount {
		var idx int
		if forward {
			idx = (startIdx + i) % fieldCount
			if idx < 0 {
				idx += fieldCount
			}
		} else {
			idx = (startIdx - i + fieldCount) % fieldCount
		}
		if ui.MatchLine(m.explainFields[idx].Name, query) {
			m.explainCursor = idx
			// Adjust scroll to keep cursor visible.
			if m.explainCursor < m.explainScroll {
				m.explainScroll = m.explainCursor
			} else if m.explainCursor >= m.explainScroll+visibleLines {
				m.explainScroll = m.explainCursor - visibleLines + 1
			}
			return true
		}
	}
	return false
}

// handleExplainSearchOverlayKey handles keyboard input for the recursive search results overlay.
func (m Model) handleExplainSearchOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.explainRecursiveFilterActive {
		return m.handleExplainSearchOverlayFilterKey(msg)
	}
	return m.handleExplainSearchOverlayNormalKey(msg)
}

// renderOverlayExplainSearch builds the centered recursive field browser
// overlay via the shared OverlayList renderer.
func (m Model) renderOverlayExplainSearch() (string, int, int) {
	w, h, maxVisible := m.explainRecursiveOverlayDims()
	filtered := m.filteredExplainRecursiveResults()
	content := ui.RenderExplainSearchOverlay(
		filtered, m.explainRecursiveCursor, m.explainRecursiveScroll, maxVisible,
		m.explainRecursiveFilter.Value, m.explainRecursiveFilterActive, w-4,
	)
	return content, w, h
}

// explainRecursiveOverlayDims returns the recursive field browser overlay box
// width, height, and the number of result rows visible inside it. Shared by the
// renderer and the scroll math so the scrollbar and cursor stay in sync.
func (m Model) explainRecursiveOverlayDims() (w, h, maxVisible int) {
	w = min(m.width-6, max(m.width*70/100, 64))
	h = min(m.height-4, max(m.height*70/100, 12))
	maxVisible = max(h-8, 1) // title + subtitle + filter(2) + footer(2) + borders
	return w, h, maxVisible
}

// handleExplainSearchOverlayNormalKey handles navigation keys in the recursive field browser.
func (m Model) handleExplainSearchOverlayNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredExplainRecursiveResults()
	resultCount := len(filtered)
	_, _, visibleLines := m.explainRecursiveOverlayDims()

	switch msg.String() {
	case "/":
		m.explainRecursiveFilterActive = true
		return m, nil

	case "j", "down":
		if m.explainRecursiveCursor < resultCount-1 {
			m.explainRecursiveCursor++
			if m.explainRecursiveCursor >= m.explainRecursiveScroll+visibleLines {
				m.explainRecursiveScroll = m.explainRecursiveCursor - visibleLines + 1
			}
		}
		return m, nil

	case "k", "up":
		if m.explainRecursiveCursor > 0 {
			m.explainRecursiveCursor--
			if m.explainRecursiveCursor < m.explainRecursiveScroll {
				m.explainRecursiveScroll = m.explainRecursiveCursor
			}
		}
		return m, nil

	case "g":
		if m.pendingG {
			m.pendingG = false
			m.explainRecursiveCursor = 0
			m.explainRecursiveScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil

	case "G":
		if resultCount > 0 {
			m.explainRecursiveCursor = resultCount - 1
			maxScroll := max(resultCount-visibleLines, 0)
			m.explainRecursiveScroll = maxScroll
		}
		return m, nil

	case "ctrl+d":
		halfPage := visibleLines / 2
		m.explainRecursiveCursor = min(m.explainRecursiveCursor+halfPage, max(resultCount-1, 0))
		m.explainRecursiveScroll = min(m.explainRecursiveScroll+halfPage, max(resultCount-visibleLines, 0))
		return m, nil

	case "ctrl+u":
		halfPage := visibleLines / 2
		m.explainRecursiveCursor = max(m.explainRecursiveCursor-halfPage, 0)
		m.explainRecursiveScroll = max(m.explainRecursiveScroll-halfPage, 0)
		return m, nil

	case "ctrl+f":
		m.explainRecursiveCursor = min(m.explainRecursiveCursor+visibleLines, max(resultCount-1, 0))
		m.explainRecursiveScroll = min(m.explainRecursiveScroll+visibleLines, max(resultCount-visibleLines, 0))
		return m, nil

	case "ctrl+b":
		m.explainRecursiveCursor = max(m.explainRecursiveCursor-visibleLines, 0)
		m.explainRecursiveScroll = max(m.explainRecursiveScroll-visibleLines, 0)
		return m, nil

	case "enter", "l", "right":
		// Navigate to the parent path of the selected result.
		if m.explainRecursiveCursor >= 0 && m.explainRecursiveCursor < resultCount {
			field := filtered[m.explainRecursiveCursor]
			parentPath := field.Path
			if idx := strings.LastIndex(parentPath, "."); idx >= 0 {
				parentPath = parentPath[:idx]
			} else {
				parentPath = ""
			}
			m.overlay = overlayNone
			m.explainRecursiveFilter.Clear()
			m.explainRecursiveFilterActive = false
			m.loading = true
			return m, m.execKubectlExplain(m.explainResource, m.explainAPIVersion, parentPath)
		}
		return m, nil

	case "esc", "q":
		m.overlay = overlayNone
		m.explainRecursiveFilter.Clear()
		m.explainRecursiveFilterActive = false
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
	}

	return m, nil
}

// handleExplainSearchOverlayFilterKey handles typing keys when the filter bar is active.
func (m Model) handleExplainSearchOverlayFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.explainRecursiveFilterActive = false
		m.explainRecursiveCursor = 0
		m.explainRecursiveScroll = 0
		return m, nil
	case "esc":
		m.explainRecursiveFilterActive = false
		m.explainRecursiveFilter.Clear()
		m.explainRecursiveCursor = 0
		m.explainRecursiveScroll = 0
		return m, nil
	case "backspace":
		if len(m.explainRecursiveFilter.Value) > 0 {
			m.explainRecursiveFilter.Backspace()
			m.explainRecursiveCursor = 0
			m.explainRecursiveScroll = 0
		}
		return m, nil
	case "ctrl+w":
		m.explainRecursiveFilter.DeleteWord()
		m.explainRecursiveCursor = 0
		m.explainRecursiveScroll = 0
		return m, nil
	case "ctrl+a":
		m.explainRecursiveFilter.Home()
		return m, nil
	case "ctrl+e":
		m.explainRecursiveFilter.End()
		return m, nil
	case "left":
		m.explainRecursiveFilter.Left()
		return m, nil
	case "right":
		m.explainRecursiveFilter.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.explainRecursiveFilter.Insert(key)
			m.explainRecursiveCursor = 0
			m.explainRecursiveScroll = 0
		}
		return m, nil
	}
}

func (m Model) handleExplainKeyJ(fieldCount, visibleLines int) (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	if m.explainCursor < fieldCount-1 {
		m.explainCursor++
		if m.explainCursor >= m.explainScroll+visibleLines {
			m.explainScroll = m.explainCursor - visibleLines + 1
		}
	}
	return m, nil
}

func (m Model) handleExplainKeyG2(fieldCount, visibleLines int) (tea.Model, tea.Cmd) {
	if m.explainLineInput != "" {
		lineNum, _ := strconv.Atoi(m.explainLineInput)
		m.explainLineInput = ""
		if lineNum > 0 {
			lineNum--
		}
		if fieldCount > 0 {
			m.explainCursor = min(lineNum, fieldCount-1)
		}
		if m.explainCursor < m.explainScroll {
			m.explainScroll = m.explainCursor
		} else if m.explainCursor >= m.explainScroll+visibleLines {
			m.explainScroll = m.explainCursor - visibleLines + 1
		}
		return m, nil
	}
	if fieldCount > 0 {
		m.explainCursor = fieldCount - 1
		m.explainScroll = max(fieldCount-visibleLines, 0)
	}
	return m, nil
}

func (m Model) handleExplainPageMove(delta, fieldCount, visibleLines int) (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	m.explainCursor += delta
	m.explainCursor = max(min(m.explainCursor, fieldCount-1), 0)
	m.explainScroll += delta
	m.explainScroll = max(min(m.explainScroll, max(fieldCount-visibleLines, 0)), 0)
	return m, nil
}

func (m Model) handleExplainKeyDrill(fieldCount int) (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	if m.explainCursor >= 0 && m.explainCursor < fieldCount {
		f := m.explainFields[m.explainCursor]
		if ui.IsDrillableType(f.Type) {
			m.loading = true
			m.setStatusMessage("Loading field...", false)
			return m, m.execKubectlExplain(m.explainResource, m.explainAPIVersion, f.Path)
		}
		m.setStatusMessage("This field is a primitive type and cannot be drilled into", true)
		return m, scheduleStatusClear()
	}
	return m, nil
}

func (m Model) handleExplainKeyQuestion() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	m.helpPreviousMode = modeExplain
	m.mode = modeHelp
	m.helpScroll = 0
	m.helpFilter.Clear()
	m.helpSearchActive = false
	m.helpContextMode = "API Explorer"
	return m, nil
}

func (m Model) handleExplainKeyQ() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	// Quit explain view immediately.
	m.exitExplainView()
	return m, nil
}

func (m Model) handleExplainKeyEsc() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	// When opened from the YAML viewer / resource tree, Esc returns straight to
	// that opener (use h/Backspace to walk schema levels instead).
	if m.explainReturnMode != modeExplorer {
		m.exitExplainView()
		return m, nil
	}
	// Step back one level; exit only at root.
	if m.explainPath == "" {
		m.exitExplainView()
		return m, nil
	}
	// Go back one level (same as h/left/backspace).
	newPath := m.explainPath
	if idx := strings.LastIndex(newPath, "."); idx >= 0 {
		newPath = newPath[:idx]
	} else {
		newPath = ""
	}
	m.loading = true
	m.setStatusMessage("Loading parent...", false)
	return m, m.execKubectlExplain(m.explainResource, m.explainAPIVersion, newPath)
}

func (m Model) handleExplainKeySlash() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	// Start search mode.
	m.explainSearchActive = true
	m.explainSearchInput.Clear()
	m.explainSearchPrevCursor = m.explainCursor
	return m, nil
}

func (m Model) handleExplainKeyN() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	// Jump to next search match; wrap around if no match found.
	if m.explainSearchQuery != "" {
		found := m.explainJumpToMatch(m.explainSearchQuery, m.explainCursor+1, true)
		if !found {
			// Wrap around from beginning.
			found = m.explainJumpToMatch(m.explainSearchQuery, 0, true)
			if !found {
				m.setStatusMessage("No matches at this level - press r to search recursively", true)
				return m, scheduleStatusClear()
			}
		}
	}
	return m, nil
}

func (m Model) handleExplainKeyN2() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	// Jump to previous search match; wrap around if no match found.
	if m.explainSearchQuery != "" {
		found := m.explainJumpToMatch(m.explainSearchQuery, m.explainCursor-1, false)
		if !found {
			// Wrap around from end.
			found = m.explainJumpToMatch(m.explainSearchQuery, len(m.explainFields)-1, false)
			if !found {
				m.setStatusMessage("No matches at this level - press r to search recursively", true)
				return m, scheduleStatusClear()
			}
		}
	}
	return m, nil
}

func (m Model) handleExplainKeyR() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	// Launch recursive field browser: load all fields and show filter overlay.
	m.loading = true
	m.setStatusMessage("Loading recursive fields...", false)
	return m, m.execKubectlExplainRecursive(m.explainResource, m.explainAPIVersion, "")
}

func (m Model) handleExplainKeyK() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	if m.explainCursor > 0 {
		m.explainCursor--
		// Scroll up if cursor goes above visible area.
		if m.explainCursor < m.explainScroll {
			m.explainScroll = m.explainCursor
		}
	}
	return m, nil
}

func (m Model) handleExplainKeyG() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	if m.pendingG {
		m.pendingG = false
		m.explainCursor = 0
		m.explainScroll = 0
		return m, nil
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleExplainKeyZero() (tea.Model, tea.Cmd) {
	if m.explainLineInput != "" {
		m.explainLineInput += "0"
		return m, nil
	}
	return m, nil
}

func (m Model) handleExplainKeyH() (tea.Model, tea.Cmd) {
	m.explainLineInput = ""
	// Go back one level in the path.
	if m.explainPath == "" {
		// Already at root: exit explain view.
		m.exitExplainView()
		return m, nil
	}
	// Trim the last path segment.
	newPath := m.explainPath
	if idx := strings.LastIndex(newPath, "."); idx >= 0 {
		newPath = newPath[:idx]
	} else {
		newPath = ""
	}
	m.loading = true
	m.setStatusMessage("Loading parent...", false)
	return m, m.execKubectlExplain(m.explainResource, m.explainAPIVersion, newPath)
}
