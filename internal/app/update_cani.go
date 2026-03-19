package app

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// openCanIBrowser starts loading the can-i browser data.
func (m Model) openCanIBrowser() (tea.Model, tea.Cmd) {
	m.loading = true
	m.canISubject = ""
	m.canISubjectName = "Current User"
	m.setStatusMessage("Loading RBAC permissions...", false)
	return m, m.loadCanIRules()
}

// processCanIRules converts raw access rules into grouped CanIResource entries,
// cross-referencing with discovered CRDs for kind names.
func (m *Model) processCanIRules(rules []k8s.AccessRule) {
	allVerbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}

	// Build permission lookup: "group/resource" -> set of allowed verbs.
	perms := make(map[string]map[string]bool)
	for _, rule := range rules {
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				key := group + "/" + resource
				if perms[key] == nil {
					perms[key] = make(map[string]bool)
				}
				for _, verb := range rule.Verbs {
					if verb == "*" {
						for _, v := range allVerbs {
							perms[key][v] = true
						}
					} else {
						perms[key][verb] = true
					}
				}
			}
		}
	}

	// Handle wildcard resources: if a rule has "*" as resource, it applies to
	// all resources in that group.
	for _, rule := range rules {
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				if resource == "*" {
					key := group + "/*"
					if perms[key] == nil {
						perms[key] = make(map[string]bool)
					}
					for _, verb := range rule.Verbs {
						if verb == "*" {
							for _, v := range allVerbs {
								perms[key][v] = true
							}
						} else {
							perms[key][verb] = true
						}
					}
				}
			}
		}
	}

	// Build resource list from discovered CRDs.
	groupMap := make(map[string][]model.CanIResource)

	crds := m.discoveredCRDs[m.nav.Context]
	for _, rt := range crds {
		group := rt.APIGroup
		resource := rt.Resource
		kind := rt.Kind

		key := group + "/" + resource
		verbs := make(map[string]bool)
		for _, v := range allVerbs {
			verbs[v] = false
		}

		// Check specific resource permissions.
		if p, ok := perms[key]; ok {
			for v, allowed := range p {
				if allowed {
					verbs[v] = true
				}
			}
		}
		// Check wildcard resource permissions for this group.
		wildcardKey := group + "/*"
		if p, ok := perms[wildcardKey]; ok {
			for v, allowed := range p {
				if allowed {
					verbs[v] = true
				}
			}
		}
		// Check wildcard group with specific resource.
		wildcardGroupKey := "*/" + resource
		if p, ok := perms[wildcardGroupKey]; ok {
			for v, allowed := range p {
				if allowed {
					verbs[v] = true
				}
			}
		}
		// Check wildcard group + wildcard resource.
		if p, ok := perms["*/*"]; ok {
			for v, allowed := range p {
				if allowed {
					verbs[v] = true
				}
			}
		}

		groupMap[group] = append(groupMap[group], model.CanIResource{
			APIGroup: group,
			Resource: resource,
			Kind:     kind,
			Verbs:    verbs,
		})
	}

	// Also include built-in resource types from TopLevelResourceTypes.
	for _, cat := range model.TopLevelResourceTypes() {
		for _, rt := range cat.Types {
			// Skip internal/synthetic types (e.g., _helm, _portforward).
			if strings.HasPrefix(rt.APIGroup, "_") {
				continue
			}

			group := rt.APIGroup
			resource := rt.Resource
			kind := rt.Kind

			// Check if already in the list (from CRDs).
			found := false
			for _, r := range groupMap[group] {
				if r.Resource == resource {
					found = true
					break
				}
			}
			if found {
				continue
			}

			key := group + "/" + resource
			verbs := make(map[string]bool)
			for _, v := range allVerbs {
				verbs[v] = false
			}

			// Check specific resource permissions.
			if p, ok := perms[key]; ok {
				for v, allowed := range p {
					if allowed {
						verbs[v] = true
					}
				}
			}
			// Check wildcard resource permissions for this group.
			wildcardKey := group + "/*"
			if p, ok := perms[wildcardKey]; ok {
				for v, allowed := range p {
					if allowed {
						verbs[v] = true
					}
				}
			}
			// Check wildcard group with specific resource.
			wildcardGroupKey := "*/" + resource
			if p, ok := perms[wildcardGroupKey]; ok {
				for v, allowed := range p {
					if allowed {
						verbs[v] = true
					}
				}
			}
			// Check wildcard group + wildcard resource.
			if p, ok := perms["*/*"]; ok {
				for v, allowed := range p {
					if allowed {
						verbs[v] = true
					}
				}
			}

			groupMap[group] = append(groupMap[group], model.CanIResource{
				APIGroup: group,
				Resource: resource,
				Kind:     kind,
				Verbs:    verbs,
			})
		}
	}

	// Also add resources from rules that aren't in discovered CRDs.
	for key, verbSet := range perms {
		if strings.HasSuffix(key, "/*") {
			continue // Skip wildcard entries.
		}
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		group, resource := parts[0], parts[1]
		if resource == "*" {
			continue
		}

		// Check if already in the list.
		found := false
		for _, r := range groupMap[group] {
			if r.Resource == resource {
				found = true
				break
			}
		}
		if found {
			continue
		}

		verbs := make(map[string]bool)
		for _, v := range allVerbs {
			verbs[v] = verbSet[v]
		}
		groupMap[group] = append(groupMap[group], model.CanIResource{
			APIGroup: group,
			Resource: resource,
			Kind:     resource, // fallback: use resource name as kind
			Verbs:    verbs,
		})
	}

	// Sort resources within each group.
	for group := range groupMap {
		sort.Slice(groupMap[group], func(i, j int) bool {
			return groupMap[group][i].Resource < groupMap[group][j].Resource
		})
	}

	// Build sorted group list.
	groupNames := make([]string, 0, len(groupMap))
	for g := range groupMap {
		groupNames = append(groupNames, g)
	}
	sort.Slice(groupNames, func(i, j int) bool {
		// Put core ("") first, then alphabetical.
		if groupNames[i] == "" {
			return true
		}
		if groupNames[j] == "" {
			return false
		}
		return groupNames[i] < groupNames[j]
	})

	m.canIGroups = make([]model.CanIGroup, len(groupNames))
	for i, name := range groupNames {
		m.canIGroups[i] = model.CanIGroup{
			Name:      name,
			Resources: groupMap[name],
		}
	}
}

// canIVisibleGroups returns the indices into m.canIGroups that match the current
// search query. When there is no search query, all groups are visible.
func (m *Model) canIVisibleGroups() []int {
	query := m.canISearchQuery
	if m.canISearchActive {
		query = m.canISearchInput.Value
	}
	indices := make([]int, 0, len(m.canIGroups))
	for i, g := range m.canIGroups {
		if query == "" {
			indices = append(indices, i)
			continue
		}
		name := g.Name
		if name == "" {
			name = "core"
		}
		if strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			indices = append(indices, i)
		}
	}
	return indices
}

// handleCanIKey handles keyboard input in the can-i browser mode.
// The left column (API groups) is the only interactive column.
func (m Model) handleCanIKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If search is active, delegate to the search key handler.
	if m.canISearchActive {
		return m.handleCanISearchKey(msg)
	}

	groupCount := len(m.canIVisibleGroups())
	visibleLines := m.canIVisibleLines()

	switch msg.String() {
	case "?":
		m.helpPreviousMode = modeExplorer
		m.overlay = overlayNone
		m.mode = modeHelp
		m.helpScroll = 0
		m.helpFilter.Clear()
		m.helpSearchActive = false
		m.helpContextMode = "Can-I Browser"
		return m, nil

	case "/":
		// Start search mode.
		m.canISearchActive = true
		m.canISearchInput.Clear()
		return m, nil

	case "s":
		// Open subject selector.
		m.loading = true
		m.setStatusMessage("Loading service accounts...", false)
		return m, m.loadCanISAList()

	case "q", "esc":
		// If a search query is active, clear it first.
		if m.canISearchQuery != "" {
			m.canISearchQuery = ""
			m.canIGroupCursor = 0
			m.canIGroupScroll = 0
			return m, nil
		}
		m.exitCanIView()
		return m, nil

	case "j", "down":
		if m.canIGroupCursor < groupCount-1 {
			m.canIGroupCursor++
			if m.canIGroupCursor >= m.canIGroupScroll+visibleLines {
				m.canIGroupScroll = m.canIGroupCursor - visibleLines + 1
			}
		}
		m.canIResourceScroll = 0
		return m, nil

	case "k", "up":
		if m.canIGroupCursor > 0 {
			m.canIGroupCursor--
			if m.canIGroupCursor < m.canIGroupScroll {
				m.canIGroupScroll = m.canIGroupCursor
			}
		}
		m.canIResourceScroll = 0
		return m, nil

	case "g":
		if m.pendingG {
			m.pendingG = false
			m.canIGroupCursor = 0
			m.canIGroupScroll = 0
			m.canIResourceScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil

	case "G":
		if groupCount > 0 {
			m.canIGroupCursor = groupCount - 1
			maxScroll := max(groupCount-visibleLines, 0)
			m.canIGroupScroll = maxScroll
		}
		m.canIResourceScroll = 0
		return m, nil

	case "ctrl+d":
		halfPage := visibleLines / 2
		m.canIGroupCursor = min(m.canIGroupCursor+halfPage, max(groupCount-1, 0))
		maxScroll := max(groupCount-visibleLines, 0)
		m.canIGroupScroll = min(m.canIGroupScroll+halfPage, maxScroll)
		m.canIResourceScroll = 0
		return m, nil

	case "ctrl+u":
		halfPage := visibleLines / 2
		m.canIGroupCursor = max(m.canIGroupCursor-halfPage, 0)
		m.canIGroupScroll = max(m.canIGroupScroll-halfPage, 0)
		m.canIResourceScroll = 0
		return m, nil

	case "ctrl+f":
		m.canIGroupCursor = min(m.canIGroupCursor+visibleLines, max(groupCount-1, 0))
		maxScroll := max(groupCount-visibleLines, 0)
		m.canIGroupScroll = min(m.canIGroupScroll+visibleLines, maxScroll)
		m.canIResourceScroll = 0
		return m, nil

	case "ctrl+b":
		m.canIGroupCursor = max(m.canIGroupCursor-visibleLines, 0)
		m.canIGroupScroll = max(m.canIGroupScroll-visibleLines, 0)
		m.canIResourceScroll = 0
		return m, nil

	case "J":
		// Scroll resource column down.
		visibleGroupIdxs := m.canIVisibleGroups()
		if m.canIGroupCursor >= 0 && m.canIGroupCursor < len(visibleGroupIdxs) {
			resources := m.canIGroups[visibleGroupIdxs[m.canIGroupCursor]].Resources
			maxScroll := max(len(resources)-visibleLines, 0)
			if m.canIResourceScroll < maxScroll {
				m.canIResourceScroll++
			}
		}
		return m, nil

	case "K":
		// Scroll resource column up.
		if m.canIResourceScroll > 0 {
			m.canIResourceScroll--
		}
		return m, nil
	}

	return m, nil
}

// canISubjectFilteredItems returns the overlay items filtered by the current subject filter query.
func (m Model) canISubjectFilteredItems() []model.Item {
	query := m.canISubjectFilterQuery
	if m.canISubjectFilterActive {
		query = m.canISubjectFilterInput.Value
	}
	if query == "" {
		return m.overlayItems
	}
	lq := strings.ToLower(query)
	filtered := make([]model.Item, 0)
	for _, item := range m.overlayItems {
		if strings.Contains(strings.ToLower(item.Name), lq) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// handleCanISubjectOverlayKey handles the subject selector overlay in the can-i browser.
func (m Model) handleCanISubjectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If filter input is active, delegate to the filter key handler.
	if m.canISubjectFilterActive {
		return m.handleCanISubjectFilterKey(msg)
	}

	items := m.canISubjectFilteredItems()
	overlayH := min(m.height-4, m.height*70/100)
	maxVisible := max(overlayH-7, 1)

	switch msg.String() {
	case "q", "esc":
		if m.canISubjectFilterQuery != "" {
			m.canISubjectFilterQuery = ""
			m.overlayCursor = 0
			m.canISubjectScroll = 0
			return m, nil
		}
		m.overlay = overlayCanI
		return m, nil
	case "/":
		m.canISubjectFilterActive = true
		m.canISubjectFilterInput.Clear()
		return m, nil
	case "j", "down":
		if m.overlayCursor < len(items)-1 {
			m.overlayCursor++
			if m.overlayCursor >= m.canISubjectScroll+maxVisible {
				m.canISubjectScroll = m.overlayCursor - maxVisible + 1
			}
		}
		return m, nil
	case "k", "up":
		if m.overlayCursor > 0 {
			m.overlayCursor--
			if m.overlayCursor < m.canISubjectScroll {
				m.canISubjectScroll = m.overlayCursor
			}
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.overlayCursor = 0
			m.canISubjectScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		if len(items) > 0 {
			m.overlayCursor = len(items) - 1
			maxScroll := max(len(items)-maxVisible, 0)
			m.canISubjectScroll = maxScroll
		}
		return m, nil
	case "ctrl+d":
		halfPage := maxVisible / 2
		m.overlayCursor = min(m.overlayCursor+halfPage, max(len(items)-1, 0))
		maxScroll := max(len(items)-maxVisible, 0)
		m.canISubjectScroll = min(m.canISubjectScroll+halfPage, maxScroll)
		return m, nil
	case "ctrl+u":
		halfPage := maxVisible / 2
		m.overlayCursor = max(m.overlayCursor-halfPage, 0)
		m.canISubjectScroll = max(m.canISubjectScroll-halfPage, 0)
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			sel := items[m.overlayCursor]
			m.canISubject = sel.Extra
			if sel.Extra == "" {
				m.canISubjectName = "Current User"
			} else {
				m.canISubjectName = sel.Name
			}
			m.overlay = overlayNone
			m.canISubjectFilterActive = false
			m.canISubjectFilterQuery = ""
			m.loading = true
			m.setStatusMessage("Loading RBAC permissions...", false)
			return m, m.loadCanIRules()
		}
		return m, nil
	}
	return m, nil
}

// handleCanISubjectFilterKey handles keyboard input when filter is active in the subject selector.
func (m Model) handleCanISubjectFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.canISubjectFilterActive = false
		m.canISubjectFilterQuery = m.canISubjectFilterInput.Value
		m.overlayCursor = 0
		m.canISubjectScroll = 0
		return m, nil
	case "esc":
		m.canISubjectFilterActive = false
		m.canISubjectFilterInput.Clear()
		m.canISubjectFilterQuery = ""
		m.overlayCursor = 0
		m.canISubjectScroll = 0
		return m, nil
	case "backspace":
		if len(m.canISubjectFilterInput.Value) > 0 {
			m.canISubjectFilterInput.Backspace()
			m.overlayCursor = 0
			m.canISubjectScroll = 0
		}
		return m, nil
	case "ctrl+w":
		m.canISubjectFilterInput.DeleteWord()
		m.overlayCursor = 0
		m.canISubjectScroll = 0
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.canISubjectFilterInput.Insert(key)
			m.overlayCursor = 0
			m.canISubjectScroll = 0
		}
		return m, nil
	}
}

// canIVisibleLines returns the number of visible content lines in the Can-I
// overlay, matching the height calculation used by the overlay renderer in
// app.go and RenderCanIView. This ensures that scroll limits in key handlers
// stay in sync with what is actually rendered on screen.
func (m *Model) canIVisibleLines() int {
	overlayH := min(m.height-4, m.height*80/100)
	innerH := overlayH - 2 // OverlayStyle vertical padding
	contentHeight := max(innerH-4, 3)
	return contentHeight - 1 // subtract 1 for the header line
}

// exitCanIView resets all can-i state and closes the overlay.
func (m *Model) exitCanIView() {
	m.overlay = overlayNone
	m.canIGroups = nil
	m.canIGroupCursor = 0
	m.canIGroupScroll = 0
	m.canIResourceScroll = 0
	m.canISubject = ""
	m.canISubjectName = ""
	m.canIServiceAccounts = nil
	m.canISearchActive = false
	m.canISearchQuery = ""
	m.canISubjectScroll = 0
	m.canISubjectFilterActive = false
	m.canISubjectFilterQuery = ""
}

// handleCanISearchKey handles keyboard input when search is active in the can-i browser.
func (m Model) handleCanISearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.canISearchActive = false
		m.canISearchQuery = m.canISearchInput.Value
		// Reset cursor to beginning of filtered results.
		m.canIGroupCursor = 0
		m.canIGroupScroll = 0
		m.canIResourceScroll = 0
		return m, nil
	case "esc":
		m.canISearchActive = false
		m.canISearchInput.Clear()
		m.canIGroupCursor = 0
		m.canIGroupScroll = 0
		m.canIResourceScroll = 0
		return m, nil
	case "backspace":
		if len(m.canISearchInput.Value) > 0 {
			m.canISearchInput.Backspace()
			m.canIGroupCursor = 0
			m.canIGroupScroll = 0
			m.canIResourceScroll = 0
		}
		return m, nil
	case "ctrl+w":
		m.canISearchInput.DeleteWord()
		m.canIGroupCursor = 0
		m.canIGroupScroll = 0
		m.canIResourceScroll = 0
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.canISearchInput.Insert(key)
			m.canIGroupCursor = 0
			m.canIGroupScroll = 0
			m.canIResourceScroll = 0
		}
		return m, nil
	}
}
