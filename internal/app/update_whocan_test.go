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
	m := Model{canIMode: canIModeWhoCan}
	mdl, _ := m.handleWhoCanKey(keyMsg("tab"))
	result := mdl.(Model)
	assert.Equal(t, canIModeForward, result.canIMode,
		"Tab from Who-Can goes back to forward Can-I — the toggle is bidirectional")
}

func TestHandleWhoCanKey_LeftRightCyclesVerb(t *testing.T) {
	m := Model{canIMode: canIModeWhoCan}
	require.Equal(t, "get", ui.WhoCanVerbs[0])
	mdl, _ := m.handleWhoCanKey(keyMsg("right"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.verbCursor, "right key advances the verb cursor")

	mdl, _ = result.handleWhoCanKey(keyMsg("left"))
	result = mdl.(Model)
	assert.Equal(t, 0, result.whoCan.verbCursor, "left wraps back")
}

func TestHandleWhoCanKey_LeftAtZeroDoesNotUnderflow(t *testing.T) {
	m := Model{canIMode: canIModeWhoCan, whoCan: whoCanState{verbCursor: 0}}
	mdl, _ := m.handleWhoCanKey(keyMsg("left"))
	result := mdl.(Model)
	assert.Equal(t, 0, result.whoCan.verbCursor,
		"left at the first verb stays at 0 — guards against underflow")
}

func TestHandleWhoCanKey_SlashEntersFilterMode(t *testing.T) {
	m := Model{canIMode: canIModeWhoCan}
	mdl, _ := m.handleWhoCanKey(keyMsg("/"))
	result := mdl.(Model)
	assert.True(t, result.whoCan.resourceFilterActive,
		"/ flips the resource filter input into edit mode")
}

func TestUpdateWhoCanLoaded_StoresSubjects(t *testing.T) {
	m := Model{requestGen: 4, whoCan: whoCanState{loading: true}}
	subs := []k8s.WhoCanSubject{
		{Kind: "User", Name: "alice", Via: "ClusterRoleBinding/admins → ClusterRole/cluster-admin"},
	}
	result := m.updateWhoCanLoaded(whoCanLoadedMsg{gen: 4, subjects: subs})
	assert.False(t, result.whoCan.loading, "spinner clears once the fetch lands")
	assert.Len(t, result.whoCan.subjects, 1, "fetched subjects copied into Model state")
}

func TestUpdateWhoCanLoaded_StaleGenIgnored(t *testing.T) {
	m := Model{requestGen: 10, whoCan: whoCanState{loading: true}}
	result := m.updateWhoCanLoaded(whoCanLoadedMsg{gen: 1, subjects: []k8s.WhoCanSubject{{Kind: "User", Name: "x"}}})
	assert.True(t, result.whoCan.loading,
		"stale response leaves the loading flag armed so the next fresh response can clear it")
	assert.Empty(t, result.whoCan.subjects, "stale subjects must not overwrite Model state")
}
