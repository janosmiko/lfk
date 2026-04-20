package app

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- helpers ---

// hasSuggestion returns true if any suggestion has the given text.
func hasSuggestion(suggestions []ui.Suggestion, text string) bool {
	for _, s := range suggestions {
		if s.Text == text {
			return true
		}
	}
	return false
}

// hasSuggestionCategory returns true if any suggestion has the given text and category.
func hasSuggestionCategory(suggestions []ui.Suggestion, text, category string) bool {
	for _, s := range suggestions {
		if s.Text == text && s.Category == category {
			return true
		}
	}
	return false
}

// suggestionTexts extracts just the text field from each suggestion.
func suggestionTexts(suggestions []ui.Suggestion) []string {
	texts := make([]string, len(suggestions))
	for i, s := range suggestions {
		texts[i] = s.Text
	}
	return texts
}

// testLeftItems returns a representative set of left-column items for resource
// jump completion tests. The Extra field uses the group/version/resource format
// that the explorer's left column provides.
func testLeftItems() []model.Item {
	return []model.Item{
		{Name: "Overview", Extra: "__overview__"},
		{Name: "Nodes", Extra: "/v1/nodes"},
		{Name: "Namespaces", Extra: "/v1/namespaces"},
		{Name: "Pods", Extra: "/v1/pods"},
		{Name: "Services", Extra: "/v1/services"},
		{Name: "Deployments", Extra: "apps/v1/deployments"},
		{Name: "ReplicaSets", Extra: "apps/v1/replicasets"},
		{Name: "StatefulSets", Extra: "apps/v1/statefulsets"},
		{Name: "DaemonSets", Extra: "apps/v1/daemonsets"},
	}
}

// --- completeResourceJump ---

func TestCompleteResourceJump_PodPrefix(t *testing.T) {
	got := completeResourceJump("pod", testLeftItems())
	assert.True(t, hasSuggestion(got, "pod"), "should include 'pod' as a suggestion")
}

func TestCompleteResourceJump_AbbreviationPrefix(t *testing.T) {
	old := ui.SearchAbbreviations
	ui.SearchAbbreviations = map[string]string{
		"dep":    "deployments",
		"deploy": "deployments",
	}
	t.Cleanup(func() { ui.SearchAbbreviations = old })

	got := completeResourceJump("dep", testLeftItems())
	assert.True(t, hasSuggestion(got, "deployment"),
		"should suggest 'deployment' via abbreviation expansion")
	assert.True(t, hasSuggestion(got, "deploy"),
		"should suggest abbreviation key 'deploy' matching prefix 'dep'")
}

func TestCompleteResourceJump_EmptyPrefix(t *testing.T) {
	got := completeResourceJump("", testLeftItems())
	assert.NotEmpty(t, got, "empty prefix should return all resource types")
}

func TestCompleteResourceJump_Nonexistent(t *testing.T) {
	got := completeResourceJump("zzzznonexistent", testLeftItems())
	assert.Empty(t, got, "nonsense prefix should return no suggestions")
}

func TestCompleteResourceJump_WithCRDs(t *testing.T) {
	items := append(testLeftItems(), model.Item{
		Name:  "Certificates",
		Extra: "cert-manager.io/v1/certificates",
	})
	got := completeResourceJump("cert", items)
	assert.True(t, hasSuggestion(got, "certificate"),
		"should include CRD name matching prefix")
	// CRDs get their API group as category, built-in types get theirs.
	for _, s := range got {
		assert.NotEmpty(t, s.Category)
	}
}

func TestCompleteResourceJump_IncludesExactMatch(t *testing.T) {
	got := completeResourceJump("pod", testLeftItems())
	assert.True(t, hasSuggestion(got, "pod"),
		"exact match 'pod' should be included")
}

func TestCompleteResourceJump_CapsMaxSuggestions(t *testing.T) {
	// Generate enough items to exceed maxSuggestions.
	items := make([]model.Item, 60)
	for i := range items {
		name := "aaa" + strings.Repeat("b", i)
		items[i] = model.Item{
			Name:  name,
			Extra: "example.io/v1/" + name,
		}
	}
	got := completeResourceJump("aaa", items)
	assert.LessOrEqual(t, len(got), maxSuggestions,
		"should not exceed maxSuggestions")
}

// --- completeBuiltin ---

func TestCompleteBuiltin_FirstTokenPartial(t *testing.T) {
	tokens := []token{{text: "n", start: 0, end: 1}}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "namespace", "command"),
		"should suggest 'namespace' for prefix 'n'")
	assert.True(t, hasSuggestionCategory(got, "ns", "command"),
		"should suggest 'ns' for prefix 'n'")
}

func TestCompleteBuiltin_NamespaceArg(t *testing.T) {
	tokens := []token{
		{text: "ns", start: 0, end: 2},
		{text: "kube", start: 3, end: 7},
	}
	m := baseModelCov()
	m.cachedNamespaces = map[string][]string{"": {"default", "kube-system", "production"}}
	got := completeBuiltin(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "kube-system", "namespace"),
		"should suggest 'kube-system' matching prefix 'kube'")
	assert.False(t, hasSuggestion(got, "default"),
		"'default' should not match prefix 'kube'")
}

func TestCompleteBuiltin_SetOption(t *testing.T) {
	tokens := []token{
		{text: "set", start: 0, end: 3},
		{text: "w", start: 4, end: 5},
	}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "wrap", "option"),
		"should suggest 'wrap' for set prefix 'w'")
}

func TestCompleteBuiltin_SortColumn(t *testing.T) {
	oldCols := ui.ActiveSortableColumns
	ui.ActiveSortableColumns = []string{"Name", "Namespace", "Status", "Age"}
	t.Cleanup(func() { ui.ActiveSortableColumns = oldCols })

	tokens := []token{
		{text: "sort", start: 0, end: 4},
		{text: "N", start: 5, end: 6},
	}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	texts := suggestionTexts(got)
	assert.Contains(t, texts, "Name", "should suggest 'Name' for prefix 'N'")
	assert.Contains(t, texts, "Namespace", "should suggest 'Namespace' for prefix 'N'")
	assert.NotContains(t, texts, "Status", "'Status' should not match prefix 'N'")
	for _, s := range got {
		assert.Equal(t, "column", s.Category)
	}
}

func TestCompleteBuiltin_ExportFormat(t *testing.T) {
	tokens := []token{
		{text: "export", start: 0, end: 6},
		{text: "", start: 7, end: 7},
	}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	texts := suggestionTexts(got)
	assert.Contains(t, texts, "yaml", "should suggest 'yaml' for export")
	assert.Contains(t, texts, "json", "should suggest 'json' for export")
	for _, s := range got {
		assert.Equal(t, "format", s.Category)
	}
}

func TestCompleteBuiltin_ContextArg(t *testing.T) {
	// client is nil in baseModelCov, so contextNames() should return nil safely.
	tokens := []token{
		{text: "ctx", start: 0, end: 3},
		{text: "", start: 4, end: 4},
	}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	// With nil client, should return empty (no panic).
	assert.Empty(t, got, "nil client should yield no context suggestions")
}

// --- completeKubectl ---

// TestCompleteKubectlBareKShowsSubcommands verifies that typing just
// ":k" (with no trailing space or subsequent tokens) surfaces the
// kubectl subcommand list in the autocomplete dropdown. Previously
// completeKubectl returned nil when effective tokens were empty, so
// users got zero suggestions for ":k" and had to know to type a space.
func TestCompleteKubectlBareKShowsSubcommands(t *testing.T) {
	tokens := []token{{text: "k", start: 0, end: 1}}
	m := baseModelCov()
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "get", "subcommand"),
		"':k' alone should offer kubectl subcommands")
	assert.True(t, hasSuggestionCategory(got, "describe", "subcommand"))
}

// TestCompleteKubectlBareKubectlShowsSubcommands is the same guard for
// the full-word ":kubectl" alias.
func TestCompleteKubectlBareKubectlShowsSubcommands(t *testing.T) {
	tokens := []token{{text: "kubectl", start: 0, end: 7}}
	m := baseModelCov()
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "get", "subcommand"),
		"':kubectl' alone should offer kubectl subcommands")
}

// TestDefaultSuggestionsIncludesKubectlPrefixes verifies that an empty
// command bar (immediately after pressing ':') lists both "k" and
// "kubectl" as discoverable entries, so users don't have to already
// know to type them.
func TestDefaultSuggestionsIncludesKubectlPrefixes(t *testing.T) {
	m := baseModelCov()
	got := m.defaultSuggestions()
	assert.True(t, hasSuggestionCategory(got, "k", "kubectl"),
		"empty ':' dropdown must include 'k' kubectl prefix")
	assert.True(t, hasSuggestionCategory(got, "kubectl", "kubectl"),
		"empty ':' dropdown must include 'kubectl' prefix")
}

// TestMixedSuggestionsIncludesKubectlPrefix is the same guard for
// partial-word input that falls through the cmdUnknown classifier
// branch. Typing ":kub" should surface "kubectl" even though "kub" is
// neither a builtin command nor a recognized kubectl prefix.
func TestMixedSuggestionsIncludesKubectlPrefix(t *testing.T) {
	m := baseModelCov()
	got := m.mixedSuggestions("kub")
	assert.True(t, hasSuggestionCategory(got, "kubectl", "kubectl"),
		"partial ':kub' must surface 'kubectl'")
}

func TestCompleteKubectl_SubcommandSuggestion(t *testing.T) {
	tokens := []token{
		{text: "kubectl", start: 0, end: 7},
		{text: "g", start: 8, end: 9},
	}
	m := baseModelCov()
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "get", "subcommand"),
		"should suggest 'get' for prefix 'g' after kubectl")
}

func TestCompleteKubectl_SubcommandWithKAlias(t *testing.T) {
	tokens := []token{
		{text: "k", start: 0, end: 1},
		{text: "g", start: 2, end: 3},
	}
	m := baseModelCov()
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "get", "subcommand"),
		"should suggest 'get' for prefix 'g' after 'k'")
}

func TestCompleteKubectl_ResourceType(t *testing.T) {
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "po", start: 4, end: 6},
	}
	m := baseModelCov()
	m.leftItems = testLeftItems()
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestion(got, "pod"),
		"should suggest 'pod' for prefix 'po' at resource position")
}

func TestCompleteKubectl_NamespaceFlag(t *testing.T) {
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "pods", start: 4, end: 8},
		{text: "-n", start: 9, end: 11},
		{text: "kube", start: 12, end: 16},
	}
	m := baseModelCov()
	m.cachedNamespaces = map[string][]string{"": {"default", "kube-system", "kube-public"}}
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "kube-system", "namespace"),
		"should suggest 'kube-system' after -n flag")
	assert.True(t, hasSuggestionCategory(got, "kube-public", "namespace"),
		"should suggest 'kube-public' after -n flag")
}

func TestCompleteKubectl_OutputFlag(t *testing.T) {
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "pods", start: 4, end: 8},
		{text: "-o", start: 9, end: 11},
		{text: "y", start: 12, end: 13},
	}
	m := baseModelCov()
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "yaml", "format"),
		"should suggest 'yaml' for prefix 'y' after -o")
}

func TestCompleteKubectl_FlagNameCompletion(t *testing.T) {
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "pods", start: 4, end: 8},
		{text: "-", start: 9, end: 10},
	}
	m := baseModelCov()
	got := completeKubectl(tokens, &m)
	assert.NotEmpty(t, got, "should suggest flag names for '-' prefix")
	for _, s := range got {
		assert.Equal(t, "flag", s.Category)
	}
}

func TestCompleteKubectl_ResourceNames(t *testing.T) {
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "pods", start: 4, end: 8},
		{text: "ngi", start: 9, end: 12},
	}
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Resource: "pods"}
	m.middleItems = []model.Item{
		{Name: "nginx-pod"},
		{Name: "redis-pod"},
	}
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "nginx-pod", "name"),
		"should suggest 'nginx-pod' for prefix 'ngi'")
	assert.False(t, hasSuggestion(got, "redis-pod"),
		"'redis-pod' should not match prefix 'ngi'")
}

func TestCompleteKubectl_NoPrefix_SubcommandPosition(t *testing.T) {
	// Direct kubectl subcommand (no kubectl/k prefix), empty second token.
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "", start: 4, end: 4},
	}
	m := baseModelCov()
	m.leftItems = testLeftItems()
	got := completeKubectl(tokens, &m)
	// Position 1 (after subcommand) should return resource types.
	assert.NotEmpty(t, got, "should return resource type suggestions at position 1")
}

// --- generateCommandBarSuggestions (dispatcher) ---

func TestGenerateSuggestions_ShellCommand(t *testing.T) {
	m := baseModelCov()
	m.commandBarInput.Value = "! grep error"
	got := m.generateCommandBarSuggestions()
	assert.Nil(t, got, "shell commands should return nil suggestions")
}

func TestGenerateSuggestions_BuiltinNamespace(t *testing.T) {
	m := baseModelCov()
	m.commandBarInput.Value = "ns kube"
	m.cachedNamespaces = map[string][]string{"": {"default", "kube-system", "production"}}
	got := m.generateCommandBarSuggestions()
	assert.True(t, hasSuggestionCategory(got, "kube-system", "namespace"),
		"should suggest 'kube-system' for 'ns kube'")
}

func TestGenerateSuggestions_Kubectl(t *testing.T) {
	m := baseModelCov()
	m.commandBarInput.Value = "kubectl g"
	got := m.generateCommandBarSuggestions()
	assert.True(t, hasSuggestion(got, "get"),
		"should suggest 'get' for 'kubectl g'")
}

func TestGenerateSuggestions_ResourceJumpExactMatch(t *testing.T) {
	m := baseModelCov()
	// "pods" is an exact known resource type, so it should be cmdResourceJump -> nil.
	m.commandBarInput.Value = "pods"
	got := m.generateCommandBarSuggestions()
	assert.Nil(t, got, "exact resource match should return nil suggestions")
}

func TestGenerateSuggestions_UnknownPartial(t *testing.T) {
	m := baseModelCov()
	m.commandBarInput.Value = "s"
	got := m.generateCommandBarSuggestions()
	assert.NotEmpty(t, got, "partial unknown input should return mixed suggestions")
}

func TestGenerateSuggestions_Empty(t *testing.T) {
	m := baseModelCov()
	m.commandBarInput.Value = ""
	got := m.generateCommandBarSuggestions()
	assert.NotEmpty(t, got, "empty input should return default suggestions")
}

// --- helper functions ---

func TestBuiltinCommandNames(t *testing.T) {
	names := builtinCommandNames()
	assert.NotEmpty(t, names)
	// Should be deduplicated (canonical + alias both appear as keys).
	seen := make(map[string]bool)
	for _, n := range names {
		assert.False(t, seen[n], "duplicate name: %s", n)
		seen[n] = true
	}
}

func TestKubectlSubcommandList(t *testing.T) {
	list := kubectlSubcommandList()
	assert.NotEmpty(t, list)
	assert.Contains(t, list, "get")
	assert.Contains(t, list, "describe")
}

func TestKubectlFlagsForSubcommand(t *testing.T) {
	// Common flags always present.
	flags := kubectlFlagsForSubcommand("get")
	assert.Contains(t, flags, "-n")
	assert.Contains(t, flags, "--namespace")

	// Subcommand-specific flags.
	deleteFlags := kubectlFlagsForSubcommand("delete")
	assert.Contains(t, deleteFlags, "--force")
	assert.Contains(t, deleteFlags, "--grace-period=0")

	logsFlags := kubectlFlagsForSubcommand("logs")
	assert.Contains(t, logsFlags, "--previous")
	assert.Contains(t, logsFlags, "--tail=")
}

func TestOutputFormatsComplete(t *testing.T) {
	fmts := outputFormatsComplete()
	assert.Contains(t, fmts, "json")
	assert.Contains(t, fmts, "yaml")
}

func TestFilterSuggestionsTyped(t *testing.T) {
	got := filterSuggestionsTyped([]string{"apple", "apricot", "banana"}, "ap", "fruit")
	assert.Len(t, got, 2)
	assert.Equal(t, "apple", got[0].Text)
	assert.Equal(t, "fruit", got[0].Category)
}

func TestFilterSuggestionsTyped_ExactMatch(t *testing.T) {
	got := filterSuggestionsTyped([]string{"apple"}, "apple", "fruit")
	assert.NotEmpty(t, got, "exact match should be included")
	assert.Equal(t, "apple", got[0].Text)
}

func TestExtractCRDNames(t *testing.T) {
	m := baseModelCov()
	m.leftItems = []model.Item{
		{Name: "Pods", Extra: "v1/pods"},
		{Name: "Overview", Extra: "__overview__"},
		{Name: "Monitoring", Extra: "__monitoring__"},
		{Name: "Certificates", Extra: "cert-manager.io/v1/certificates"},
	}
	got := extractCRDNames(&m)
	assert.Contains(t, got, "certificates")
	assert.NotContains(t, got, "overview")
	assert.NotContains(t, got, "monitoring")
}

func TestContextNames_NilClient(t *testing.T) {
	m := baseModelCov()
	got := m.contextNames()
	assert.Nil(t, got, "nil client should return nil")
}

func TestCompleteBuiltin_QuitNoSuggestions(t *testing.T) {
	// :q is a recognized command -- no suggestions shown.
	tokens := []token{{text: "q", start: 0, end: 1}}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	assert.Empty(t, got)
}

func TestCompleteBuiltin_QuPartialShowsQuit(t *testing.T) {
	// :qu is NOT a recognized command -- shows "quit" as suggestion.
	tokens := []token{{text: "qu", start: 0, end: 2}}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	texts := suggestionTexts(got)
	assert.Contains(t, texts, "quit")
}

func TestCompleteKubectl_LongNamespaceFlag(t *testing.T) {
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "pods", start: 4, end: 8},
		{text: "--namespace", start: 9, end: 20},
		{text: "def", start: 21, end: 24},
	}
	m := baseModelCov()
	m.cachedNamespaces = map[string][]string{"": {"default", "kube-system"}}
	got := completeKubectl(tokens, &m)
	assert.True(t, hasSuggestionCategory(got, "default", "namespace"),
		"should suggest 'default' after --namespace flag")
}

func TestCompleteKubectl_OutputFormatEmptyPrefix(t *testing.T) {
	tokens := []token{
		{text: "get", start: 0, end: 3},
		{text: "pods", start: 4, end: 8},
		{text: "--output", start: 9, end: 17},
		{text: "", start: 18, end: 18},
	}
	m := baseModelCov()
	got := completeKubectl(tokens, &m)
	texts := suggestionTexts(got)
	assert.Contains(t, texts, "json")
	assert.Contains(t, texts, "yaml")
	for _, s := range got {
		assert.Equal(t, "format", s.Category)
	}
}

func TestCompleteResourceJump_HasCategory(t *testing.T) {
	got := completeResourceJump("nod", testLeftItems())
	for _, s := range got {
		// Categories are API group names (e.g., "core", "apps") or "crd"/"alias".
		assert.NotEmpty(t, s.Category)
	}
}

func TestCompleteBuiltin_NamespaceEmptyPrefix(t *testing.T) {
	tokens := []token{
		{text: "namespace", start: 0, end: 9},
		{text: "", start: 10, end: 10},
	}
	m := baseModelCov()
	m.cachedNamespaces = map[string][]string{"": {"default", "kube-system"}}
	got := completeBuiltin(tokens, &m)
	assert.Len(t, got, 2)
	for _, s := range got {
		assert.Equal(t, "namespace", s.Category)
	}
}

func TestNamespaceNames_ScopedByContext(t *testing.T) {
	m := baseModelCov()
	m.cachedNamespaces = map[string][]string{
		"ctx-a": {"a-ns-1", "a-ns-2"},
		"ctx-b": {"b-ns-1"},
	}

	m.nav.Context = "ctx-a"
	assert.ElementsMatch(t, []string{"a-ns-1", "a-ns-2"}, m.namespaceNames(),
		"nav.Context=ctx-a should read the ctx-a entry")

	m.nav.Context = "ctx-b"
	assert.ElementsMatch(t, []string{"b-ns-1"}, m.namespaceNames(),
		"nav.Context=ctx-b should read the ctx-b entry, not leak from ctx-a")

	m.nav.Context = "ctx-missing"
	assert.Empty(t, m.namespaceNames(),
		"unknown context should return nil so the command bar triggers a fresh load")
}

func TestCompleteBuiltin_NamespaceArg_PerContext(t *testing.T) {
	// Cache holds entries for two contexts. Completing `:ns kube` under
	// nav.Context=ctx-b must only see ctx-b's namespaces, even though
	// ctx-a's cache contains a matching "kube-system".
	tokens := []token{
		{text: "ns", start: 0, end: 2},
		{text: "kube", start: 3, end: 7},
	}
	m := baseModelCov()
	m.nav.Context = "ctx-b"
	m.cachedNamespaces = map[string][]string{
		"ctx-a": {"kube-system", "kube-public"},
		"ctx-b": {"default", "production"},
	}

	got := completeBuiltin(tokens, &m)
	assert.False(t, hasSuggestion(got, "kube-system"),
		"ctx-a's kube-system must not leak into ctx-b's completions")
	assert.False(t, hasSuggestion(got, "kube-public"),
		"ctx-a's kube-public must not leak into ctx-b's completions")
}

func TestUpdateNamespacesLoaded_StoresUnderMessageContext(t *testing.T) {
	m := baseModelCov()
	m.nav.Context = "ctx-current"
	items := []model.Item{{Name: "ns-x"}, {Name: "ns-y"}}

	// A fetch issued for ctx-other completes; its result must be stored
	// under ctx-other even though the tab has since moved to ctx-current.
	result, _ := m.Update(namespacesLoadedMsg{context: "ctx-other", items: items})
	rm := result.(Model)

	assert.ElementsMatch(t, []string{"ns-x", "ns-y"}, rm.cachedNamespaces["ctx-other"],
		"stored under the message's context, not the model's current nav.Context")
	assert.Empty(t, rm.cachedNamespaces["ctx-current"],
		"current context must not pick up values fetched for another context")
}

func TestCompleteBuiltin_SortEmptyPrefix(t *testing.T) {
	oldCols := ui.ActiveSortableColumns
	ui.ActiveSortableColumns = []string{"Name", "Age"}
	t.Cleanup(func() { ui.ActiveSortableColumns = oldCols })

	tokens := []token{
		{text: "sort", start: 0, end: 4},
		{text: "", start: 5, end: 5},
	}
	m := baseModelCov()
	got := completeBuiltin(tokens, &m)
	texts := suggestionTexts(got)
	sort.Strings(texts)
	assert.Equal(t, []string{"Age", "Name"}, texts)
}
