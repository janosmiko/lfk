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
	case "?", "f1":
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

	case "a":
		// Toggle showing all permissions vs allowed only.
		m.canIAllowedOnly = !m.canIAllowedOnly
		m.canIResourceScroll = 0
		return m, nil

	case "s":
		// Open subject selector.
		m.loading = true
		m.setStatusMessage("Loading RBAC subjects...", false)
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

// handleCanISubjectOverlayKey handles the subject selector overlay in the can-i browser.
func (m Model) handleCanISubjectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.canISubjectFilterMode {
		return m.handleCanISubjectFilterMode(msg)
	}
	return m.handleCanISubjectNormalMode(msg)
}

// handleCanISubjectNormalMode handles normal-mode keys in the subject selector overlay.
func (m Model) handleCanISubjectNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.filteredOverlayItems()

	switch msg.String() {
	case "esc", "q":
		if m.overlayFilter.Value != "" {
			m.overlayFilter.Clear()
			m.overlayCursor = 0
			return m, nil
		}
		m.overlay = overlayCanI
		m.overlayFilter.Clear()
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
			m.overlayFilter.Clear()
			m.canISubjectFilterMode = false
			m.loading = true
			m.setStatusMessage("Loading RBAC permissions...", false)
			return m, m.loadCanIRules()
		}
		return m, nil

	case "/":
		m.canISubjectFilterMode = true
		m.overlayFilter.Clear()
		return m, nil

	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil

	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil

	case "ctrl+d":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil

	case "ctrl+u":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, len(items)-1)
		return m, nil

	case "ctrl+f":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 20, len(items)-1)
		return m, nil

	case "ctrl+b":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -20, len(items)-1)
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleCanISubjectFilterMode handles filter-mode keys in the subject selector overlay.
func (m Model) handleCanISubjectFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch handleFilterKey(&m.overlayFilter, msg.String()) {
	case filterEscape:
		m.canISubjectFilterMode = false
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		return m, nil
	case filterAccept:
		m.canISubjectFilterMode = false
		m.overlayCursor = 0
		return m, nil
	case filterClose:
		return m.closeTabOrQuit()
	case filterContinue:
		m.overlayCursor = 0
		return m, nil
	}
	return m, nil
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

// filterAllowedResources returns only resources that have at least one allowed verb.
func filterAllowedResources(resources []model.CanIResource) []model.CanIResource {
	filtered := make([]model.CanIResource, 0, len(resources))
	for _, r := range resources {
		for _, allowed := range r.Verbs {
			if allowed {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}

// countAllowedResources returns the number of resources with at least one allowed verb.
func countAllowedResources(resources []model.CanIResource) int {
	count := 0
	for _, r := range resources {
		for _, allowed := range r.Verbs {
			if allowed {
				count++
				break
			}
		}
	}
	return count
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
	m.canISubjectFilterMode = false
	m.canIAllowedOnly = false
	m.canINamespaces = nil
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
