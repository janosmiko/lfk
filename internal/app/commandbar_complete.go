package app

import (
	"context"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// maxSuggestions limits results for broad categories (resource types, mixed suggestions).
// Namespace and context lists are uncapped since users need to see all of them.
const maxSuggestions = 50

// completeResourceJump returns resource type suggestions matching the given prefix.
// It pulls candidates from the built-in resource registry, search abbreviations
// (both keys and values), and any discovered CRD names. Results are filtered by
// case-insensitive prefix, exact matches are excluded, and the list is capped at
// maxSuggestions. Every suggestion carries category "resource".
func completeResourceJump(prefix string, leftItems []model.Item) []ui.Suggestion {
	lower := strings.ToLower(prefix)
	seen := make(map[string]bool)
	var results []ui.Suggestion

	add := func(name, category string) {
		nl := strings.ToLower(name)
		if seen[nl] {
			return
		}
		if lower != "" && !strings.HasPrefix(nl, lower) {
			return
		}
		seen[nl] = true
		results = append(results, ui.Suggestion{Text: name, Category: category})
	}

	// Build a lookup from leftItems Extra field (group/version/resource) to determine
	// which types actually exist in the cluster and their API group.
	type itemInfo struct {
		resource string
		group    string
	}
	clusterTypes := make(map[string]itemInfo)
	for _, item := range leftItems {
		if item.Extra == "" || item.Extra == "__overview__" || item.Extra == "__monitoring__" {
			continue
		}
		parts := strings.Split(item.Extra, "/")
		if len(parts) >= 3 {
			resource := strings.ToLower(parts[len(parts)-1])
			group := parts[0]
			if group == "" {
				group = "core"
			}
			clusterTypes[resource] = itemInfo{resource: resource, group: group}
		} else if len(parts) == 2 {
			// "v1/resource" format (core types).
			resource := strings.ToLower(parts[1])
			clusterTypes[resource] = itemInfo{resource: resource, group: "core"}
		}
	}

	// Build a set of all cluster type names (plural and singular forms).
	clusterTypeNames := make(map[string]bool)
	for _, info := range clusterTypes {
		// Only show singular form (matches common kubectl usage).
		singular := toSingular(info.resource)
		add(singular, info.group)
		clusterTypeNames[info.resource] = true
		clusterTypeNames[singular] = true
	}
	// Also include display names from leftItems (e.g., "Pods", "Deployments").
	for _, item := range leftItems {
		if item.Extra != "" && item.Extra != "__overview__" && item.Extra != "__monitoring__" {
			clusterTypeNames[strings.ToLower(item.Name)] = true
		}
	}

	// Add search abbreviations (from defaults + user config) for types that exist in the cluster.
	if ui.SearchAbbreviations != nil {
		for abbr, expansion := range ui.SearchAbbreviations {
			exp := strings.ToLower(expansion)
			if clusterTypeNames[exp] {
				add(abbr, "alias")
			}
		}
	}

	if len(results) > maxSuggestions {
		results = results[:maxSuggestions]
	}

	// Sort: built-in resources first, then CRDs, then aliases. Within each group, alphabetical.
	sort.SliceStable(results, func(i, j int) bool {
		pi := categoryPriority(results[i].Category)
		pj := categoryPriority(results[j].Category)
		if pi != pj {
			return pi < pj
		}
		return results[i].Text < results[j].Text
	})

	return results
}

// categoryPriority returns a sort priority for suggestion categories.
// Lower values appear first in the dropdown.
func categoryPriority(category string) int {
	switch category {
	case "alias":
		return 0
	case "core":
		return 1
	case "apps", "batch", "autoscaling", "policy", "networking.k8s.io",
		"rbac.authorization.k8s.io", "storage.k8s.io", "scheduling.k8s.io",
		"coordination.k8s.io", "discovery.k8s.io", "node.k8s.io":
		return 2
	case "crd":
		return 4
	default:
		// Other API groups.
		return 3
	}
}

// completeBuiltin returns suggestions for builtin commands (:ns, :ctx, :set, etc.).
// For the first token it suggests matching command names. For subsequent tokens it
// suggests context-appropriate values based on the canonical command name.
func completeBuiltin(tokens []token, m *Model) []ui.Suggestion {
	if len(tokens) == 0 {
		return nil
	}

	// First token: suggest matching command names.
	// But if it's already a recognized command, don't suggest alternatives.
	if len(tokens) == 1 {
		prefix := strings.ToLower(tokens[0].text)
		if _, ok := builtinCommands[prefix]; ok {
			return nil
		}
		return filterSuggestionsTyped(builtinCommandNames(), prefix, "command")
	}

	// Second+ token: based on the canonical command resolved from the first token.
	firstWord := strings.ToLower(tokens[0].text)
	canonical, ok := builtinCommands[firstWord]
	if !ok {
		return nil
	}

	// Single-argument commands: stop suggesting once the argument is filled.
	// Namespace is multi-argument (can select multiple namespaces).
	singleArg := canonical == "context" ||
		canonical == "set" || canonical == "sort" || canonical == "export" ||
		canonical == "orphans"
	if singleArg && len(tokens) > 2 {
		return nil
	}

	lastToken := tokens[len(tokens)-1]
	prefix := strings.ToLower(lastToken.text)

	switch canonical {
	case "namespace":
		// Exclude already-selected namespaces from suggestions.
		already := make(map[string]bool)
		for _, tok := range tokens[1 : len(tokens)-1] {
			already[strings.ToLower(tok.text)] = true
		}
		var candidates []string
		for _, ns := range m.namespaceNames() {
			if !already[strings.ToLower(ns)] {
				candidates = append(candidates, ns)
			}
		}
		return filterSuggestionsFuzzy(candidates, prefix, "namespace")
	case "context":
		return filterSuggestionsFuzzy(m.contextNames(), prefix, "context")
	case "set":
		return filterSuggestionsFuzzy(setOptions(), prefix, "option")
	case "sort":
		return filterSuggestionsFuzzy(ui.ActiveSortableColumns, prefix, "column")
	case "export":
		return filterSuggestionsFuzzy([]string{"yaml", "json"}, prefix, "format")
	case "orphans":
		return filterSuggestionsFuzzy(orphanKindCompletions(), prefix, "kind")
	default:
		return nil
	}
}

// generateCommandBarSuggestions is the main dispatcher that produces suggestions
// for the current command bar input. It classifies the input and delegates to
// the appropriate completer function.
func (m *Model) generateCommandBarSuggestions() []ui.Suggestion {
	input := m.commandBarInput.Value
	if input == "" {
		return m.defaultSuggestions()
	}

	crdNames := extractCRDNames(m)
	ct := classifyInputWithCRDs(input, crdNames)

	switch ct {
	case cmdShell:
		return nil
	case cmdBuiltin:
		tokens := parseTokens(input, len(input))
		return completeBuiltin(tokens, m)
	case cmdKubectl:
		tokens := parseTokens(input, len(input))
		return completeKubectl(tokens, m)
	case cmdResourceJump:
		tokens := parseTokens(input, len(input))
		if len(tokens) >= 2 {
			// Only suggest namespaces if the resource type is namespaced.
			resourceName := m.resolveResourceType(strings.ToLower(tokens[0].text))
			if !m.isNamespacedResource(resourceName) {
				return nil
			}
			// Second+ tokens: suggest namespace names, excluding already selected.
			prefix := strings.ToLower(tokens[len(tokens)-1].text)
			already := make(map[string]bool)
			for _, tok := range tokens[1 : len(tokens)-1] {
				already[strings.ToLower(tok.text)] = true
			}
			var candidates []string
			for _, ns := range m.namespaceNames() {
				if !already[strings.ToLower(ns)] {
					candidates = append(candidates, ns)
				}
			}
			return filterSuggestionsFuzzy(candidates, prefix, "namespace")
		}
		// First token: still show matching resource types.
		return completeResourceJump(input, m.resourceTypeItems())
	case cmdUnknown:
		return m.mixedSuggestions(input)
	default:
		return nil
	}
}

// --- Helper functions ---

// builtinCommandNames returns all unique keys from builtinCommands, sorted.
func builtinCommandNames() []string {
	// Exclude short quit aliases -- :q and :q! are typed deliberately.
	// Keep "quit" so :qu shows it as a suggestion.
	exclude := map[string]bool{"q": true, "q!": true}
	seen := make(map[string]bool)
	var names []string
	for k := range builtinCommands {
		if !seen[k] && !exclude[k] {
			seen[k] = true
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}

// orphanKindCompletions returns the kind tokens accepted by :orphans.
func orphanKindCompletions() []string {
	return []string{"pods", "secrets", "configmaps", "services"}
}

// setOptions returns the available options for the :set command.
func setOptions() []string {
	return []string{
		"wrap", "nowrap",
		"linenumbers", "nolinenumbers",
		"timestamps", "notimestamps",
		"follow", "nofollow",
		"ansi", "noansi",
	}
}

// filterSuggestionsTyped filters candidates by a case-insensitive prefix and excludes
// exact matches. No hard cap -- the dropdown renderer handles scrolling.
func filterSuggestionsTyped(candidates []string, prefix, category string) []ui.Suggestion {
	lower := strings.ToLower(prefix)
	var result []ui.Suggestion

	for _, c := range candidates {
		cl := strings.ToLower(c)
		if lower != "" && !strings.HasPrefix(cl, lower) {
			continue
		}
		result = append(result, ui.Suggestion{Text: c, Category: category})
	}

	return result
}

// extractCRDNames collects CRD display names from the left column items.
// It skips special entries like __overview__ and __monitoring__.
func extractCRDNames(m *Model) []string {
	var names []string
	seen := make(map[string]bool)
	for _, item := range m.resourceTypeItems() {
		if item.Extra == "" || item.Extra == "__overview__" || item.Extra == "__monitoring__" {
			continue
		}
		// Extract resource name from Extra field (group/version/resource).
		res := strings.ToLower(resourceFromExtra(item.Extra))
		if res != "" && !seen[res] {
			seen[res] = true
			names = append(names, res)
		}
		// Also add display name lowercased for matching.
		nameLower := strings.ToLower(item.Name)
		if !seen[nameLower] {
			seen[nameLower] = true
			names = append(names, nameLower)
		}
	}
	return names
}

// resourceTypeItems returns the list of resource type items regardless of the
// current navigation level. The resource types are at different positions in
// the left-column stack depending on how deep the user has navigated.
func (m *Model) resourceTypeItems() []model.Item {
	switch m.nav.Level {
	case model.LevelResourceTypes:
		// At resource types level, middleItems ARE the resource types.
		return m.middleItems
	case model.LevelResources:
		// At resources level, leftItems are the resource types.
		return m.leftItems
	default:
		// Deeper levels: resource types are in the history stack.
		// leftItemsHistory[0] = clusters, [1] = resource types (if exists).
		if len(m.leftItemsHistory) >= 1 {
			return m.leftItemsHistory[len(m.leftItemsHistory)-1]
		}
		return m.leftItems
	}
}

// cachedResourceNames returns resource names from the async cache.
// If not cached and not currently loading, triggers an async fetch
// and returns nil (the caller will see a loading indicator).
func (m *Model) cachedResourceNames(resourceType, namespace string) []string {
	cacheKey := m.nav.Context + "/" + namespace + "/" + resourceType
	if m.commandBarNameCache != nil {
		if names, ok := m.commandBarNameCache[cacheKey]; ok {
			return names
		}
	}
	// Not cached and not already loading -- trigger async fetch.
	if m.commandBarNameLoading != cacheKey {
		m.commandBarNameLoading = cacheKey
		// The fetch is triggered by returning a special suggestion.
	}
	return nil
}

// fetchCommandBarResourceNames creates a tea.Cmd that fetches resource names
// for the given resource type and namespace, returning them as a message.
func (m Model) fetchCommandBarResourceNames(resourceType, namespace string) tea.Cmd {
	cacheKey := m.nav.Context + "/" + namespace + "/" + resourceType
	kctx := m.nav.Context
	client := m.client
	if client == nil {
		return nil
	}
	// Find the ResourceTypeEntry for this resource type.
	var rt model.ResourceTypeEntry
	found := false
	for _, t := range m.discoveredResources[kctx] {
		if strings.ToLower(t.Resource) == resourceType {
			rt = t
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	return func() tea.Msg {
		items, err := client.GetResources(context.Background(), kctx, namespace, rt)
		if err != nil {
			return commandBarNamesFetchedMsg{cacheKey: cacheKey, names: nil}
		}
		names := make([]string, 0, len(items))
		for _, item := range items {
			names = append(names, item.Name)
		}
		return commandBarNamesFetchedMsg{cacheKey: cacheKey, names: names}
	}
}

// resourceFromExtra is defined in commandbar_execute.go.

// contextNames returns context names from the k8s client, nil-safe.
func (m *Model) contextNames() []string {
	if m.client == nil {
		return nil
	}
	items, err := m.client.GetContexts()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// namespaceNames returns cached namespace names for completion in the
// current nav context. Each tab has its own nav.Context, so keying by
// context keeps completions correct across tab switches and `:ctx`
// changes within a tab. The cache is populated asynchronously when the
// command bar opens for a context that isn't cached yet.
//
// A stale entry (older than namespaceCacheTTL) is still returned here
// so completions remain visible while a background refresh runs; the
// refresh is scheduled from handleKeyCommandBar.
func (m *Model) namespaceNames() []string {
	return m.cachedNamespaces[m.activeContext()].names
}

// isNamespacedResource checks if a resource type (plural name) is namespaced.
// Returns true if not found (safer default for unknown resources).
func (m *Model) isNamespacedResource(resourceName string) bool {
	lower := strings.ToLower(resourceName)
	for _, rt := range m.discoveredResources[m.nav.Context] {
		if strings.ToLower(rt.Resource) == lower {
			return rt.Namespaced
		}
	}
	// Unknown resource (e.g., CRD not yet discovered): assume namespaced.
	return true
}

// resolveResourceType maps a resource name (singular or plural, or abbreviation)
// to the plural resource type name used in the API. Consults abbreviations
// first, then the discovered resource set for the current context.
func (m *Model) resolveResourceType(name string) string {
	lower := strings.ToLower(name)
	if expanded, ok := ui.SearchAbbreviations[lower]; ok {
		lower = strings.ToLower(expanded)
	}
	for _, rt := range m.discoveredResources[m.nav.Context] {
		res := strings.ToLower(rt.Resource)
		kind := strings.ToLower(rt.Kind)
		if res == lower || kind == lower {
			return res
		}
	}
	if !strings.HasSuffix(lower, "s") {
		return lower + "s"
	}
	return lower
}

// resourceNames returns unique resource names from the middle column.
func resourceNames(m *Model) []string {
	if m.nav.Level < model.LevelResources {
		return nil
	}
	seen := make(map[string]bool)
	var names []string
	for _, item := range m.middleItems {
		if item.Name != "" && !seen[item.Name] {
			seen[item.Name] = true
			names = append(names, item.Name)
		}
	}
	return names
}

// effectivePosition returns the positional index of the current (last) token,
// counting only non-flag tokens. Flags (tokens starting with "-") and their
// values are skipped.
func effectivePosition(tokens []token) int {
	if len(tokens) == 0 {
		return 0
	}

	pos := 0
	// Walk all tokens except the last one (which is being typed).
	for i := 0; i < len(tokens)-1; i++ {
		t := tokens[i].text
		if strings.HasPrefix(t, "-") {
			// Skip the flag itself. If it's a short flag expecting a value
			// (e.g., -n, -o), also skip the next token.
			lower := strings.ToLower(t)
			if lower == "-n" || lower == "-o" || lower == "-l" || lower == "-f" ||
				lower == "--namespace" || lower == "--output" || lower == "--selector" || lower == "--filename" {
				i++ // skip the flag value
			}
			continue
		}
		pos++
	}

	return pos
}

// defaultSuggestions returns a mix of builtin command names and resource types
// for when the command bar is empty.
func (m *Model) defaultSuggestions() []ui.Suggestion {
	var result []ui.Suggestion

	// Add builtin commands.
	for _, name := range builtinCommandNames() {
		result = append(result, ui.Suggestion{Text: name, Category: "command"})
	}

	// Add kubectl prefixes so users discover :k / :kubectl without
	// already knowing to type them.
	result = append(result, kubectlPrefixSuggestions()...)

	// Add some common resource types.
	count := 0
	for _, rt := range m.discoveredResources[m.nav.Context] {
		result = append(result, ui.Suggestion{
			Text:     strings.ToLower(rt.Resource),
			Category: "resource",
		})
		count++
		if count+len(builtinCommandNames())+len(kubectlPrefixSuggestions()) >= maxSuggestions {
			return result
		}
	}

	if len(result) > maxSuggestions {
		result = result[:maxSuggestions]
	}

	return result
}

// mixedSuggestions returns suggestions from all categories for a partially typed
// unknown input. This handles cases where the user has typed something that
// doesn't yet match any specific command type.
func (m *Model) mixedSuggestions(input string) []ui.Suggestion {
	prefix := strings.ToLower(firstWordOf(input))
	var result []ui.Suggestion

	// Builtin commands.
	for _, name := range builtinCommandNames() {
		if strings.HasPrefix(strings.ToLower(name), prefix) && strings.ToLower(name) != prefix {
			result = append(result, ui.Suggestion{Text: name, Category: "command"})
		}
	}

	// Kubectl prefixes — matched by prefix so "ku", "kub", "kube" all
	// surface "kubectl". Skip exact matches since classifyInput would
	// route those directly to completeKubectl.
	for _, s := range kubectlPrefixSuggestions() {
		if strings.HasPrefix(s.Text, prefix) && s.Text != prefix {
			result = append(result, s)
		}
	}

	// Resource types.
	resourceSuggestions := completeResourceJump(prefix, m.resourceTypeItems())
	result = append(result, resourceSuggestions...)

	// Deduplicate by text.
	seen := make(map[string]bool)
	var deduped []ui.Suggestion
	for _, s := range result {
		if !seen[s.Text] {
			seen[s.Text] = true
			deduped = append(deduped, s)
		}
	}

	if len(deduped) > maxSuggestions {
		deduped = deduped[:maxSuggestions]
	}

	return deduped
}
