package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// canIGroupOf builds a CanIGroup for tests with the given group name
// and resource names. Keeps the test bodies focused on the assertion
// rather than the boilerplate of nesting Resources slices.
func canIGroupOf(name string, resources ...string) model.CanIGroup {
	rs := make([]model.CanIResource, len(resources))
	for i, r := range resources {
		rs[i] = model.CanIResource{Resource: r}
	}
	return model.CanIGroup{Name: name, Resources: rs}
}

func TestEnterWhoCanMode_FlipsModeAndResetsState(t *testing.T) {
	m := Model{}
	m.canIMode = canIModeForward
	m.whoCan.subjectsScroll = 7 // stale value from prior session
	mdl, _ := m.enterWhoCanMode()
	result := mdl.(Model)
	assert.Equal(t, canIModeWhoCan, result.canIMode,
		"Tab from Can-I must flip the overlay into Who-Can mode")
	assert.Equal(t, 0, result.whoCan.subjectsScroll,
		"scroll resets so the user lands at the top of the new subjects list")
}

func TestEnterWhoCanMode_BuildsResourceListFromCanIGroups(t *testing.T) {
	m := Model{}
	m.canIGroups = []model.CanIGroup{
		canIGroupOf("apps", "deployments", "statefulsets"),
		canIGroupOf("", "pods", "secrets"),
	}
	mdl, _ := m.enterWhoCanMode()
	result := mdl.(Model)
	assert.Equal(t, []string{"deployments", "pods", "secrets", "statefulsets"},
		result.whoCan.resourceList,
		"resource picker is the deduped sorted union of Can-I groups")
}

func TestEnterWhoCanMode_PositionsCursorOnPreSelectedResource(t *testing.T) {
	m := Model{}
	// Can-I cursor is on the first group; canIResourceUnderCursor returns its first
	// resource name. Expect the picker to land on that name in the deduped list.
	m.canIGroups = []model.CanIGroup{
		canIGroupOf("", "pods", "secrets"),
	}
	m.canIGroupCursor = 0
	mdl, _ := m.enterWhoCanMode()
	result := mdl.(Model)
	assert.Equal(t, "pods", result.whoCan.resource,
		"pre-positioned resource is the one highlighted in the Can-I view")
}

func TestHandleWhoCanKey_JKMovesCursor(t *testing.T) {
	m := Model{}
	m.canIMode = canIModeWhoCan
	m.whoCan.resourceList = []string{"pods", "secrets", "services"}
	m.whoCan.resourceCursor = 0

	mdl, _ := m.handleWhoCanKey(keyMsg("j"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.resourceCursor,
		"j moves the resource cursor down one")

	mdl, _ = result.handleWhoCanKey(keyMsg("k"))
	result = mdl.(Model)
	assert.Equal(t, 0, result.whoCan.resourceCursor,
		"k moves the resource cursor up one")
}

func TestHandleWhoCanKey_JAtBottomDoesNotOverflow(t *testing.T) {
	m := Model{}
	m.canIMode = canIModeWhoCan
	m.whoCan.resourceList = []string{"pods", "secrets"}
	m.whoCan.resourceCursor = 1 // already at the bottom
	mdl, _ := m.handleWhoCanKey(keyMsg("j"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.resourceCursor,
		"j at the last row stays put — guards against overflow")
}

func TestHandleWhoCanKey_GJumpsToTopAndShiftGToBottom(t *testing.T) {
	m := Model{}
	m.canIMode = canIModeWhoCan
	m.whoCan.resourceList = []string{"a", "b", "c", "d", "e"}
	m.whoCan.resourceCursor = 2

	mdl, _ := m.handleWhoCanKey(keyMsg("G"))
	result := mdl.(Model)
	assert.Equal(t, 4, result.whoCan.resourceCursor, "G jumps to the last entry")

	mdl, _ = result.handleWhoCanKey(keyMsg("g"))
	result = mdl.(Model)
	assert.Equal(t, 0, result.whoCan.resourceCursor, "g jumps to the first entry")
}

func TestHandleWhoCanKey_HomeJumpsToTopAndEndToBottom(t *testing.T) {
	m := Model{}
	m.canIMode = canIModeWhoCan
	m.whoCan.resourceList = []string{"a", "b", "c", "d"}
	m.whoCan.resourceCursor = 2

	mdl, _ := m.handleWhoCanKey(keyMsg("end"))
	result := mdl.(Model)
	assert.Equal(t, 3, result.whoCan.resourceCursor, "end aliases G")

	mdl, _ = result.handleWhoCanKey(keyMsg("home"))
	result = mdl.(Model)
	assert.Equal(t, 0, result.whoCan.resourceCursor, "home aliases g")
}

func TestHandleWhoCanKey_JJKKScrollsSubjectsNotResources(t *testing.T) {
	m := Model{}
	m.width = 200
	m.height = 60
	m.canIMode = canIModeWhoCan
	m.whoCan.resourceList = []string{"pods"}
	m.whoCan.resourceCursor = 0
	// Many subjects so subjectsScroll has room to move.
	subjects := make([]k8s.WhoCanSubject, 50)
	for i := range subjects {
		subjects[i].Name = "user"
	}
	m.whoCan.subjects = subjects

	mdl, _ := m.handleWhoCanKey(keyMsg("J"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.subjectsScroll, "J scrolls the subjects column down by one")
	assert.Equal(t, 0, result.whoCan.resourceCursor, "J must not move the resource cursor")

	mdl, _ = result.handleWhoCanKey(keyMsg("K"))
	result = mdl.(Model)
	assert.Equal(t, 0, result.whoCan.subjectsScroll, "K scrolls the subjects column up by one")
}

func TestHandleWhoCanKey_JKAtBottomDoesNotOverflowSubjects(t *testing.T) {
	m := Model{}
	m.width = 200
	m.height = 60
	m.canIMode = canIModeWhoCan
	m.whoCan.subjects = []k8s.WhoCanSubject{{Name: "x"}} // exactly one row, fits in any panel
	m.whoCan.subjectsScroll = 0

	mdl, _ := m.handleWhoCanKey(keyMsg("J"))
	result := mdl.(Model)
	assert.Equal(t, 0, result.whoCan.subjectsScroll,
		"J at the last row stays clamped to 0 — there's nothing to scroll past")
}

func TestNamespaceToggleKey_RestoresParentAndClearsPreviousOverlay(t *testing.T) {
	// Regression: pressing the namespace selector hotkey while it was
	// nested on top of RBAC used to drop both overlays AND leave
	// previousOverlay stale, so the next open of the namespace
	// selector would incorrectly re-render RBAC behind it.
	m := Model{}
	m.overlay = overlayNamespace
	m.previousOverlay = overlayCanI

	mdl, _ := m.handleOverlayKey(keyMsg("\\"))
	result := mdl.(Model)
	assert.Equal(t, overlayCanI, result.overlay,
		"toggle key on a layered ns selector restores the parent overlay")
	assert.Equal(t, overlayNone, result.previousOverlay,
		"previousOverlay must clear so a re-open of the ns selector starts fresh — otherwise RBAC re-appears as a stale background")
}

func TestNamespaceSelectorEsc_RestoresCanIOverlayWhenOpenedFromIt(t *testing.T) {
	// Regression: opening the namespace selector from inside Can-I
	// used to set m.overlay = overlayNamespace and on close drop back
	// to overlayNone, so the user lost their RBAC overlay.
	m := Model{}
	m.overlay = overlayNamespace
	m.previousOverlay = overlayCanI

	mdl, _ := m.handleNamespaceOverlayKey(keyMsg("esc"))
	result := mdl.(Model)
	assert.Equal(t, overlayCanI, result.overlay,
		"esc must restore the parent overlay when the namespace selector was nested inside it")
	assert.Equal(t, overlayNone, result.previousOverlay,
		"previousOverlay must clear after restore so the next esc cleanly closes")
}

func TestSyncCanINamespacesFromSelection_AllNamespacesProducesEmptyString(t *testing.T) {
	m := Model{}
	m.allNamespaces = true
	m.syncCanINamespacesFromSelection()
	assert.Equal(t, []string{""}, m.canINamespaces,
		"all-ns scope must collapse to the empty-string sentinel that loadWhoCan / loadCanIRules treat as cluster-wide")
}

func TestSyncCanINamespacesFromSelection_SingleNamespacePassesThrough(t *testing.T) {
	m := Model{}
	m.namespace = "kube-system"
	m.syncCanINamespacesFromSelection()
	assert.Equal(t, []string{"kube-system"}, m.canINamespaces)
}

func TestWhoCanScroll_ScrollingUpReleasesCursorFromBottomEdge(t *testing.T) {
	// Regression: stateless scroll re-pinned the cursor to the last
	// visible row whenever it sat past the first viewport. After
	// scrolling down then up, vim semantics should leave the viewport
	// alone until the cursor crosses an edge — the cursor should be
	// able to walk up *inside* the viewport, not stay glued to its bottom.
	m := Model{}
	m.width = 200
	m.height = 60
	m.canIMode = canIModeWhoCan
	// Build a big enough list that scrolling matters.
	resources := make([]string, 80)
	for i := range resources {
		resources[i] = "r" + itoa(i)
	}
	m.whoCan.resourceList = resources

	// Jump to the bottom — cursor + scroll both at the end.
	mdl, _ := m.handleWhoCanKey(keyMsg("G"))
	result := mdl.(Model)
	require.Equal(t, len(resources)-1, result.whoCan.resourceCursor)

	scrollAfterG := result.whoCan.resourceScroll
	require.Greater(t, scrollAfterG, 0, "G must have scrolled down — list is bigger than the viewport")

	// Step up by one. Cursor decrements; viewport must NOT shift.
	mdl, _ = result.handleWhoCanKey(keyMsg("k"))
	result = mdl.(Model)
	assert.Equal(t, len(resources)-2, result.whoCan.resourceCursor,
		"k must move the cursor up by one")
	assert.Equal(t, scrollAfterG, result.whoCan.resourceScroll,
		"scroll must stay put while the cursor is still inside the viewport — vim semantics")
}

// itoa is a tiny helper to avoid importing strconv just for this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func TestWhoCanFilter_NarrowsListAndResetsCursor(t *testing.T) {
	m := Model{}
	m.canIMode = canIModeWhoCan
	m.whoCan.resourceList = []string{"configmaps", "pods", "pods/exec", "secrets"}
	m.whoCan.resourceCursor = 3 // user was on "secrets"
	m.whoCan.resourceFilterActive = true
	m.whoCan.resourceFilter.Insert("po")

	visible := m.whoCanVisibleResources()
	assert.Equal(t, []string{"pods", "pods/exec"}, visible,
		"filter narrows to substring matches across the deduped list")
}

func TestHandleWhoCanKey_TabReturnsToForward(t *testing.T) {
	m := Model{canIState: canIState{canIMode: canIModeWhoCan}}
	mdl, _ := m.handleWhoCanKey(keyMsg("tab"))
	result := mdl.(Model)
	assert.Equal(t, canIModeForward, result.canIMode,
		"Tab from Who-Can goes back to forward Can-I — the toggle is bidirectional")
}

func TestHandleWhoCanKey_LeftRightCyclesVerb(t *testing.T) {
	m := Model{canIState: canIState{canIMode: canIModeWhoCan}}
	require.Equal(t, "get", ui.WhoCanVerbs[0])
	mdl, _ := m.handleWhoCanKey(keyMsg("right"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.verbCursor, "right key advances the verb cursor")

	mdl, _ = result.handleWhoCanKey(keyMsg("left"))
	result = mdl.(Model)
	assert.Equal(t, 0, result.whoCan.verbCursor, "left wraps back")
}

func TestHandleWhoCanKey_LeftAtZeroDoesNotUnderflow(t *testing.T) {
	m := Model{canIState: canIState{canIMode: canIModeWhoCan, whoCan: whoCanState{verbCursor: 0}}}
	mdl, _ := m.handleWhoCanKey(keyMsg("left"))
	result := mdl.(Model)
	assert.Equal(t, 0, result.whoCan.verbCursor,
		"left at the first verb stays at 0 — guards against underflow")
}

func TestHandleWhoCanKey_SlashEntersFilterMode(t *testing.T) {
	m := Model{canIState: canIState{canIMode: canIModeWhoCan}}
	mdl, _ := m.handleWhoCanKey(keyMsg("/"))
	result := mdl.(Model)
	assert.True(t, result.whoCan.resourceFilterActive,
		"/ flips the resource filter input into edit mode")
}

func TestUpdateWhoCanLoaded_StoresSubjects(t *testing.T) {
	m := Model{requestGen: 4, canIState: canIState{whoCan: whoCanState{loading: true}}}
	subs := []k8s.WhoCanSubject{
		{Kind: "User", Name: "alice", Via: "ClusterRoleBinding/admins → ClusterRole/cluster-admin"},
	}
	result := m.updateWhoCanLoaded(whoCanLoadedMsg{gen: 4, subjects: subs})
	assert.False(t, result.whoCan.loading, "spinner clears once the fetch lands")
	assert.Len(t, result.whoCan.subjects, 1, "fetched subjects copied into Model state")
}

func TestUpdateWhoCanLoaded_StaleGenIgnored(t *testing.T) {
	m := Model{requestGen: 10, canIState: canIState{whoCan: whoCanState{loading: true}}}
	result := m.updateWhoCanLoaded(whoCanLoadedMsg{gen: 1, subjects: []k8s.WhoCanSubject{{Kind: "User", Name: "x"}}})
	assert.True(t, result.whoCan.loading,
		"stale response leaves the loading flag armed so the next fresh response can clear it")
	assert.Empty(t, result.whoCan.subjects, "stale subjects must not overwrite Model state")
}

func TestHandleCanIKey_TabDuringSearchDoesNotEnterWhoCan(t *testing.T) {
	// Regression: an earlier dispatcher checked the WhoCan-pivot Tab
	// branch BEFORE the search-active guard, so pressing Tab while
	// `/`-search was active hijacked the key into enterWhoCanMode and
	// left the model with the search input mid-edit. Search must own
	// Tab when active — only after it falls through can the WhoCan
	// pivot consume the key.
	m := Model{}
	m.canIMode = canIModeForward
	m.canISearchActive = true

	mdl, _ := m.handleCanIKey(keyMsg("tab"))
	result := mdl.(Model)

	assert.Equal(t, canIModeForward, result.canIMode,
		"Tab while search is active must NOT enter Who-Can mode — search owns Tab")
	assert.True(t, result.canISearchActive,
		"search must remain active — the key was delegated to handleCanISearchKey, not the WhoCan pivot")
}

func TestEnterWhoCanMode_SetsLoadingFlagOnReturnedModel(t *testing.T) {
	// Regression: loading was set inside loadWhoCan (a value-receiver
	// method), so the mutation lived on a discarded copy. The renderer
	// then never showed the spinner during the in-flight fetch on entry.
	// Loading must be set on the Model the caller returns to Update.
	m := Model{}
	m.canIGroups = []model.CanIGroup{
		canIGroupOf("", "pods"),
	}
	m.canIGroupCursor = 0
	mdl, cmd := m.enterWhoCanMode()
	result := mdl.(Model)
	assert.True(t, result.whoCan.loading,
		"entering Who-Can with a resource must arm the spinner on the persisted Model")
	assert.NotNil(t, cmd, "loadWhoCan must dispatch a fetch when there is a resource")
}

func TestEnterWhoCanMode_NoResourceLeavesLoadingClear(t *testing.T) {
	// When there's nothing to fetch (no Can-I groups), entry must NOT
	// arm the spinner — otherwise the overlay would show a permanent
	// "loading…" with no fetch ever landing to clear it.
	m := Model{}
	mdl, cmd := m.enterWhoCanMode()
	result := mdl.(Model)
	assert.False(t, result.whoCan.loading,
		"empty resource list must leave loading=false — no fetch is dispatched")
	assert.Nil(t, cmd, "no fetch dispatched when there is no resource to query")
}

func TestWhoCanCycleVerb_ArmsSpinnerWhenResourceSet(t *testing.T) {
	// Regression: cycling the verb dispatches a new fetch but used to
	// rely on loadWhoCan's value-receiver mutation — so the spinner
	// never showed during verb cycling either.
	m := Model{canIState: canIState{canIMode: canIModeWhoCan, whoCan: whoCanState{resource: "pods", verbCursor: 0}}}
	mdl, cmd := m.handleWhoCanKey(keyMsg("right"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.verbCursor, "right advances the verb cursor")
	assert.True(t, result.whoCan.loading, "verb cycle must arm the spinner on the persisted Model")
	assert.NotNil(t, cmd, "verb cycle dispatches a fetch when a resource is selected")
}

func TestWhoCanCycleVerb_NoResourceLeavesLoadingClear(t *testing.T) {
	m := Model{canIState: canIState{canIMode: canIModeWhoCan, whoCan: whoCanState{verbCursor: 0}}}
	mdl, cmd := m.handleWhoCanKey(keyMsg("right"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.verbCursor)
	assert.False(t, result.whoCan.loading, "no resource = no fetch = no spinner")
	assert.Nil(t, cmd)
}

func TestWhoCanNamespaceToggle_ArmsSpinnerWhenResourceSet(t *testing.T) {
	// 'A' toggles namespace scope and re-fires the query when a resource
	// is selected. The spinner must reflect the in-flight refetch.
	m := Model{canIState: canIState{canIMode: canIModeWhoCan, whoCan: whoCanState{resource: "pods"}, canINamespaces: []string{""}}}
	m.namespace = "default"
	mdl, cmd := m.handleWhoCanKey(keyMsg("A"))
	result := mdl.(Model)
	assert.True(t, result.whoCan.loading,
		"'A' must arm the spinner when toggling scope re-fires the query")
	assert.NotNil(t, cmd)
}

func TestRefreshWhoCanForCursor_ArmsSpinnerOnResourceChange(t *testing.T) {
	// Cursor navigation funnels through refreshWhoCanForCursor; the
	// spinner must light up on the persisted Model when the cursor
	// lands on a different resource.
	m := Model{canIState: canIState{canIMode: canIModeWhoCan, whoCan: whoCanState{resource: "pods", resourceList: []string{"pods", "secrets"}, resourceCursor: 0}}}
	m.width = 200
	m.height = 60
	mdl, cmd := m.handleWhoCanKey(keyMsg("j"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.resourceCursor, "j moved the cursor")
	assert.Equal(t, "secrets", result.whoCan.resource, "resource updated to the cursor's row")
	assert.True(t, result.whoCan.loading, "cursor moving onto a new resource arms the spinner")
	assert.NotNil(t, cmd)
}

func TestRefreshWhoCanForCursor_SameResourceDoesNotArmSpinner(t *testing.T) {
	// j on the last row clamps without changing resource — no fetch,
	// no spinner. (Otherwise the user could spam keys and pin the
	// spinner on indefinitely with no actual work happening.)
	m := Model{canIState: canIState{canIMode: canIModeWhoCan, whoCan: whoCanState{resource: "secrets", resourceList: []string{"pods", "secrets"}, resourceCursor: 1}}}
	m.width = 200
	m.height = 60
	mdl, cmd := m.handleWhoCanKey(keyMsg("j"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.resourceCursor)
	assert.False(t, result.whoCan.loading, "no resource change = no fetch = no spinner")
	assert.Nil(t, cmd)
}

func TestWhoCanFilterEscape_RefreshesSubjectsForCursor(t *testing.T) {
	// Regression: live-narrowing then Esc cleared the filter and reset
	// scroll, but did not refresh the subjects pane against the cursor's
	// new (un-narrowed) row. The picker would highlight one resource
	// while the right pane still showed another resource's subjects.
	m := Model{}
	m.canIMode = canIModeWhoCan
	m.whoCan.resourceList = []string{"configmaps", "deployments", "pods", "secrets"}
	// Simulate state after live-narrow on "po": cursor at 0 of narrowed
	// list, m.whoCan.resource set to "pods" (the previously-narrowed row).
	m.whoCan.resource = "pods"
	m.whoCan.resourceFilter.Insert("po")
	m.whoCan.resourceFilterActive = true
	m.whoCan.resourceCursor = 0
	m.whoCan.resourceScroll = 5 // pretend the user had scrolled inside the narrowed list

	mdl, cmd := m.handleWhoCanKey(keyMsg("esc"))
	result := mdl.(Model)
	assert.False(t, result.whoCan.resourceFilterActive, "esc exits filter mode")
	assert.Equal(t, "", result.whoCan.resourceFilter.Value, "filter cleared")
	assert.Equal(t, 0, result.whoCan.resourceScroll, "scroll reset to top of un-narrowed list")
	// Cursor 0 in the un-narrowed list points to "configmaps", not the
	// previously-loaded "pods", so the resource must update and a
	// refetch must dispatch — otherwise the highlight and the right
	// pane stay desynced.
	assert.Equal(t, "configmaps", result.whoCan.resource,
		"esc must refresh the resource to the un-narrowed cursor row, not preserve the narrowed selection")
	assert.True(t, result.whoCan.loading, "spinner armed for the refetch dispatched after esc")
	assert.NotNil(t, cmd, "a refetch is dispatched when the resource changes")
}

func TestWhoCanFilterEscape_EmptyListClearsResource(t *testing.T) {
	// Esc on an empty resource list (e.g. canIGroups never loaded) must
	// not crash and must clear the resource so a stale "loading…" doesn't
	// linger.
	m := Model{}
	m.canIMode = canIModeWhoCan
	m.whoCan.resource = "pods"
	m.whoCan.resourceFilter.Insert("zzz")
	m.whoCan.resourceFilterActive = true

	mdl, cmd := m.handleWhoCanKey(keyMsg("esc"))
	result := mdl.(Model)
	assert.Equal(t, "", result.whoCan.resource, "empty list clears the resource so the right pane goes idle")
	assert.Nil(t, cmd, "no fetch dispatched when the un-narrowed list is empty")
}

func TestSyncCanINamespacesFromSelection_MultiSelectSortsForCanI(t *testing.T) {
	m := Model{}
	m.selectedNamespaces = map[string]bool{
		"monitoring":  true,
		"default":     true,
		"kube-system": true,
	}
	m.canIMode = canIModeForward
	m.syncCanINamespacesFromSelection()
	assert.Equal(t, []string{"default", "kube-system", "monitoring"}, m.canINamespaces,
		"forward Can-I keeps multi-select; sorted so the title bar renders deterministically across frames")
}

func TestSyncCanINamespacesFromSelection_MultiSelectCollapsesInWhoCan(t *testing.T) {
	// Regression: Who-Can's loadWhoCan only recognises canINamespaces of
	// length 0 or 1 — multi-select would render a "ns: a,b,c" title but
	// query cluster-wide, so the displayed scope didn't match the data.
	// In Who-Can mode we collapse to the first sorted namespace so the
	// scope label and the query agree.
	m := Model{}
	m.selectedNamespaces = map[string]bool{
		"monitoring":  true,
		"default":     true,
		"kube-system": true,
	}
	m.canIMode = canIModeWhoCan
	m.syncCanINamespacesFromSelection()
	assert.Equal(t, []string{"default"}, m.canINamespaces,
		"Who-Can collapses multi-select to the first sorted namespace so scope label and query match")
}

func TestEnterWhoCanMode_RefreshesResourceOnEachEntry(t *testing.T) {
	// Regression: a previous version preserved m.whoCan.resource across
	// Tab pivots — so after tabbing back to forward, moving the Can-I
	// cursor to a different group, and tabbing again, the user saw
	// stale Who-Can results from the FIRST entry instead of subjects
	// for the row they were now hovering. enterWhoCanMode must re-read
	// the cursor unconditionally on every entry.
	m := Model{}
	m.canIGroups = []model.CanIGroup{
		canIGroupOf("", "pods"),
		canIGroupOf("apps", "deployments"),
	}

	// First entry: cursor on the core group, expect "pods".
	m.canIGroupCursor = 0
	mdl, _ := m.enterWhoCanMode()
	first := mdl.(Model)
	require.Equal(t, "pods", first.whoCan.resource,
		"first entry pre-positions on the resource under the Can-I cursor")

	// Tab back to forward, then move the Can-I cursor to a different group.
	mdl, _ = first.handleWhoCanKey(keyMsg("tab"))
	back := mdl.(Model)
	require.Equal(t, canIModeForward, back.canIMode)
	back.canIGroupCursor = 1

	// Re-enter Who-Can — must reflect the NEW cursor, not the stale resource.
	mdl, _ = back.enterWhoCanMode()
	second := mdl.(Model)
	assert.Equal(t, "deployments", second.whoCan.resource,
		"re-entry must refresh the resource from the current Can-I cursor — preserving the previous Who-Can target leaks stale results across pivots")
}
