package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

func TestCovColumnToggleOpenClose(t *testing.T) {
	m := baseModelCov()
	m.cursors = [5]int{}
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
	}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	assert.Nil(t, r.(Model).columnToggleItems)
}

func TestCovColumnToggleCloseWithFilter(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{{key: "IP", visible: true}}
	m.columnToggleFilter = "IP"
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEscape})
	// First esc clears filter.
	assert.Empty(t, r.(Model).columnToggleFilter)
	assert.Equal(t, overlayColumnToggle, r.(Model).overlay)
}

func TestCovColumnToggleNav(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "a", visible: true},
		{key: "b", visible: true},
		{key: "c", visible: false},
	}
	m.columnToggleCursor = 0

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).columnToggleCursor)

	m2 := r.(Model)
	r, _ = m2.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, r.(Model).columnToggleCursor)
}

func TestCovColumnTogglePageScroll(t *testing.T) {
	items := make([]columnToggleEntry, 30)
	for i := range items {
		items[i] = columnToggleEntry{key: "col", visible: true}
	}
	m := baseModelCov()
	m.columnToggleItems = items

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Greater(t, r.(Model).columnToggleCursor, 0)

	m.columnToggleCursor = 20
	r, _ = m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Less(t, r.(Model).columnToggleCursor, 20)

	m.columnToggleCursor = 0
	r, _ = m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.Greater(t, r.(Model).columnToggleCursor, 0)

	m.columnToggleCursor = 25
	r, _ = m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.Less(t, r.(Model).columnToggleCursor, 25)
}

// TestCovColumnToggleClearWithCHotkey verifies that pressing `c` unchecks
// every entry in the overlay without dismissing it, so the user can then
// cherry-pick the columns they want from a clean slate.
func TestCovColumnToggleClearWithCHotkey(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: true, builtin: true},
		{key: "Ready", visible: true, builtin: true},
		{key: "IP", visible: true, builtin: false},
		{key: "CPU", visible: false, builtin: false},
	}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	rm := r.(Model)

	assert.Equal(t, overlayColumnToggle, rm.overlay, "overlay must stay open after clear")
	for _, e := range rm.columnToggleItems {
		assert.False(t, e.visible, "every entry must be unchecked after clear: %s", e.key)
	}
}

func TestCovColumnToggleSpace(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
	}
	m.columnToggleCursor = 0

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	// Toggle visibility of first item, cursor advances.
	assert.False(t, r.(Model).columnToggleItems[0].visible)
	assert.Equal(t, 1, r.(Model).columnToggleCursor)
}

func TestCovColumnToggleMoveUpDown(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "a", visible: true},
		{key: "b", visible: true},
		{key: "c", visible: true},
	}

	m.columnToggleCursor = 0
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	assert.Equal(t, "b", r.(Model).columnToggleItems[0].key)
	assert.Equal(t, "a", r.(Model).columnToggleItems[1].key)

	m2 := r.(Model)
	m2.columnToggleCursor = 2
	r, _ = m2.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	assert.Equal(t, 1, r.(Model).columnToggleCursor)
}

func TestCovColumnToggleMoveWithFilter(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{{key: "a"}, {key: "b"}}
	m.columnToggleFilter = "active"

	// Move operations are no-op when filtering.
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	assert.Equal(t, m.columnToggleItems, r.(Model).columnToggleItems)
}

func TestCovColumnToggleEnter(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	assert.Equal(t, []string{"IP"}, r.(Model).sessionColumns["pod"])
}

func TestCovColumnToggleEnterAllHidden(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.sessionColumns = map[string][]string{"pod": {"old"}}

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	_, exists := r.(Model).sessionColumns["pod"]
	assert.False(t, exists)
}

func TestCovColumnToggleSlash(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{{key: "a"}}
	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, r.(Model).columnToggleFilterActive)
}

func TestCovColumnToggleReset(t *testing.T) {
	m := baseModelCov()
	m.sessionColumns = map[string][]string{"pod": {"IP"}}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.columnToggleItems = []columnToggleEntry{{key: "IP"}}

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	assert.Equal(t, overlayNone, r.(Model).overlay)
	_, exists := r.(Model).sessionColumns["pod"]
	assert.False(t, exists)
}

func TestCovColumnToggleFilterKey(t *testing.T) {
	m := baseModelCov()
	m.columnToggleFilterActive = true
	m.columnToggleFilter = ""

	// Type a character.
	r, _ := m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "a", r.(Model).columnToggleFilter)

	// Backspace.
	m2 := r.(Model)
	r, _ = m2.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Empty(t, r.(Model).columnToggleFilter)

	// Enter.
	m.columnToggleFilterActive = true
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).columnToggleFilterActive)

	// Esc with filter text: clears.
	m.columnToggleFilter = "text"
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).columnToggleFilter)

	// Esc without filter text: exits filter mode.
	m.columnToggleFilter = ""
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).columnToggleFilterActive)

	// Ctrl+W.
	m.columnToggleFilter = "hello world"
	r, _ = m.handleColumnToggleFilterKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Equal(t, "hello ", r.(Model).columnToggleFilter)
}

func TestCovFilteredColumnToggleItems(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "IP", visible: true},
		{key: "Port", visible: false},
		{key: "Image", visible: true},
	}

	// No filter: all items.
	assert.Len(t, m.filteredColumnToggleItems(), 3)

	// With filter.
	m.columnToggleFilter = "I"
	filtered := m.filteredColumnToggleItems()
	assert.GreaterOrEqual(t, len(filtered), 1)
}

func TestCovOpenColumnToggle(t *testing.T) {
	m := baseModelNav()
	m.middleItems = []model.Item{
		{Name: "pod-1", Columns: []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}, {Key: "Node", Value: "node-1"}}},
	}
	m.openColumnToggle()
	assert.Equal(t, overlayColumnToggle, m.overlay)
}

func TestCovOpenColumnToggleEmpty(t *testing.T) {
	m := baseModelNav()
	m.middleItems = []model.Item{{
		Name: "item",
		Columns: []model.KeyValue{
			{Key: "IP", Value: "10.0.0.1"},
			{Key: "Node", Value: "node-1"},
		},
	}}
	m.openColumnToggle()
	assert.Equal(t, overlayColumnToggle, m.overlay)
}

// --- built-in column support ---

// findColumnToggleEntry searches for an entry by key and builtin flag.
// Returns nil if not found.
func findColumnToggleEntry(entries []columnToggleEntry, key string, builtin bool) *columnToggleEntry {
	for i := range entries {
		if entries[i].key == key && entries[i].builtin == builtin {
			return &entries[i]
		}
	}
	return nil
}

// TestCovOpenColumnToggleIncludesBuiltins verifies that opening the column
// toggle overlay for a resource with built-in field data (Ready/Restarts/
// Status/Age/Namespace) produces toggle entries for each built-in and that
// Name is NOT toggleable.
func TestCovOpenColumnToggleIncludesBuiltins(t *testing.T) {
	m := baseModelNav()
	m.middleItems = []model.Item{
		{
			Name:      "pod-1",
			Namespace: "default",
			Kind:      "Pod",
			Ready:     "1/1",
			Restarts:  "0",
			Status:    "Running",
			Age:       "5m",
			Columns:   []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}},
		},
	}

	m.openColumnToggle()

	entries := m.columnToggleItems
	assert.NotNil(t, findColumnToggleEntry(entries, "Namespace", true), "Namespace built-in entry must exist")
	assert.NotNil(t, findColumnToggleEntry(entries, "Ready", true), "Ready built-in entry must exist")
	assert.NotNil(t, findColumnToggleEntry(entries, "Restarts", true), "Restarts built-in entry must exist")
	assert.NotNil(t, findColumnToggleEntry(entries, "Status", true), "Status built-in entry must exist")
	assert.NotNil(t, findColumnToggleEntry(entries, "Age", true), "Age built-in entry must exist")
	assert.Nil(t, findColumnToggleEntry(entries, "Name", true), "Name must NOT be toggleable")

	// All built-ins should be visible by default when nothing is hidden.
	for _, key := range []string{"Namespace", "Ready", "Restarts", "Status", "Age"} {
		e := findColumnToggleEntry(entries, key, true)
		if assert.NotNil(t, e, "entry for %s", key) {
			assert.True(t, e.visible, "%s must be visible when no hidden set", key)
		}
	}

	// The extra column should also be present, but not builtin.
	ipExtra := findColumnToggleEntry(entries, "IP", false)
	assert.NotNil(t, ipExtra, "IP extra entry must exist")
}

// TestCovOpenColumnTogglePreSelectsFromHiddenBuiltins verifies that when
// hiddenBuiltinColumns lists a built-in as hidden for the current kind,
// that entry opens with visible=false.
func TestCovOpenColumnTogglePreSelectsFromHiddenBuiltins(t *testing.T) {
	m := baseModelNav()
	m.middleItems = []model.Item{
		{
			Name:      "pod-1",
			Namespace: "default",
			Kind:      "Pod",
			Ready:     "1/1",
			Restarts:  "0",
			Status:    "Running",
			Age:       "5m",
		},
	}
	m.hiddenBuiltinColumns = map[string][]string{"pod": {"Ready", "Status"}}

	m.openColumnToggle()

	ready := findColumnToggleEntry(m.columnToggleItems, "Ready", true)
	if assert.NotNil(t, ready) {
		assert.False(t, ready.visible, "Ready must be hidden per hiddenBuiltinColumns")
	}
	status := findColumnToggleEntry(m.columnToggleItems, "Status", true)
	if assert.NotNil(t, status) {
		assert.False(t, status.visible, "Status must be hidden per hiddenBuiltinColumns")
	}
	namespace := findColumnToggleEntry(m.columnToggleItems, "Namespace", true)
	if assert.NotNil(t, namespace) {
		assert.True(t, namespace.visible, "Namespace must remain visible")
	}
}

// TestCovOpenColumnTogglePreSelectsFromActiveExtras verifies Bug 2: the
// overlay always reflects currently rendered columns (ActiveExtraColumnKeys),
// never a stale sessionColumns override. When the window is too narrow to fit
// the full saved preference, the overlay must reflect what actually fits.
func TestCovOpenColumnTogglePreSelectsFromActiveExtras(t *testing.T) {
	defer func(orig []string) { ui.ActiveExtraColumnKeys = orig }(ui.ActiveExtraColumnKeys)

	m := baseModelNav()
	m.middleItems = []model.Item{{
		Name: "pod-1",
		Columns: []model.KeyValue{
			{Key: "IP", Value: "10.0.0.1"},
			{Key: "Node", Value: "node-1"},
			{Key: "Image", Value: "nginx:latest"},
		},
	}}
	// Saved preference is larger than what currently fits.
	m.sessionColumns = map[string][]string{"pod": {"IP", "Node", "Image"}}
	// Only IP actually fits on screen right now.
	ui.ActiveExtraColumnKeys = []string{"IP"}

	m.openColumnToggle()

	ipEntry := findColumnToggleEntry(m.columnToggleItems, "IP", false)
	if assert.NotNil(t, ipEntry) {
		assert.True(t, ipEntry.visible, "IP must be visible (it is currently rendered)")
	}
	nodeEntry := findColumnToggleEntry(m.columnToggleItems, "Node", false)
	if assert.NotNil(t, nodeEntry) {
		assert.False(t, nodeEntry.visible, "Node must NOT be visible (not in ActiveExtraColumnKeys despite sessionColumns)")
	}
	imageEntry := findColumnToggleEntry(m.columnToggleItems, "Image", false)
	if assert.NotNil(t, imageEntry) {
		assert.False(t, imageEntry.visible, "Image must NOT be visible (not in ActiveExtraColumnKeys despite sessionColumns)")
	}
}

// TestCovColumnToggleEnterSavesBuiltinsAndExtras verifies the Enter handler
// splits entries between sessionColumns (extras) and hiddenBuiltinColumns
// (built-ins) correctly.
func TestCovColumnToggleEnterSavesBuiltinsAndExtras(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: true, builtin: true},
		{key: "Ready", visible: false, builtin: true},
		{key: "Status", visible: false, builtin: true},
		{key: "Restarts", visible: true, builtin: true},
		{key: "Age", visible: true, builtin: true},
		{key: "IP", visible: true, builtin: false},
		{key: "Node", visible: false, builtin: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)

	assert.Equal(t, overlayNone, rm.overlay)
	assert.Equal(t, []string{"IP"}, rm.sessionColumns["pod"], "sessionColumns holds visible extras")
	hidden := rm.hiddenBuiltinColumns["pod"]
	assert.ElementsMatch(t, []string{"Ready", "Status"}, hidden, "hiddenBuiltinColumns holds invisible built-ins")
}

// TestCovMiddleColumnKindAtLevelContainers verifies that at LevelContainers,
// the middle column kind is derived from the rendered items (containers),
// NOT the parent ResourceType.Kind ("Pod"). This prevents column
// visibility changes on pods from leaking into containers.
func TestCovMiddleColumnKindAtLevelContainers(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelContainers
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.middleItems = []model.Item{
		{Name: "nginx", Kind: "Container"},
		{Name: "sidecar", Kind: "Container"},
	}

	assert.Equal(t, "container", m.middleColumnKind(),
		"at LevelContainers, kind must come from middleItems (Container), not the parent Pod")
}

// TestCovMiddleColumnKindAtLevelResources verifies that at LevelResources,
// the kind still comes from nav.ResourceType.Kind so the helper does not
// regress the existing behavior.
func TestCovMiddleColumnKindAtLevelResources(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod"}}

	assert.Equal(t, "pod", m.middleColumnKind())
}

// TestCovMiddleColumnKindAtLevelOwned verifies that at LevelOwned the
// middle column kind comes from the owned items (e.g. ReplicaSet under a
// Deployment parent).
func TestCovMiddleColumnKindAtLevelOwned(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	m.middleItems = []model.Item{
		{Name: "nginx-abc", Kind: "ReplicaSet"},
	}

	assert.Equal(t, "replicaset", m.middleColumnKind(),
		"at LevelOwned, kind must come from middleItems (ReplicaSet), not the parent Deployment")
}

// TestCovRightColumnKindFromRightItems verifies that the right-column kind
// is derived from rightItems, not the middle column's nav.ResourceType.
func TestCovRightColumnKindFromRightItems(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.rightItems = []model.Item{
		{Name: "nginx", Kind: "Container"},
	}

	assert.Equal(t, "container", m.rightColumnKind(),
		"rightColumnKind must come from rightItems[0].Kind, not the parent Pod")
}

// TestCovRightColumnKindEmptyItems verifies that an empty right column
// returns an empty kind (the helper then clears overrides).
func TestCovRightColumnKindEmptyItems(t *testing.T) {
	m := baseModelCov()
	m.rightItems = nil
	assert.Equal(t, "", m.rightColumnKind())
}

// TestCovWithSessionColumnsForKindRestoresState verifies that
// withSessionColumnsForKind applies the scoped config during fn and
// restores the caller's state when fn returns.
func TestCovWithSessionColumnsForKindRestoresState(t *testing.T) {
	defer func(orig []string) { ui.ActiveSessionColumns = orig }(ui.ActiveSessionColumns)
	defer func(orig map[string]bool) { ui.ActiveHiddenBuiltinColumns = orig }(ui.ActiveHiddenBuiltinColumns)
	defer func(orig []string) { ui.ActiveColumnOrder = orig }(ui.ActiveColumnOrder)

	m := baseModelCov()
	m.sessionColumns = map[string][]string{
		"pod":       {"IP"},
		"container": {"Image"},
	}
	m.hiddenBuiltinColumns = map[string][]string{
		"pod":       {"Ready"},
		"container": {"Status"},
	}
	m.columnOrder = map[string][]string{
		"pod":       {"IP", "Namespace"},
		"container": {"Image", "Namespace"},
	}

	// Start with the pod config applied, as viewExplorer would.
	m.applySessionColumnsForKind("pod")
	assert.Equal(t, []string{"IP"}, ui.ActiveSessionColumns)
	assert.True(t, ui.ActiveHiddenBuiltinColumns["Ready"])
	assert.Equal(t, []string{"IP", "Namespace"}, ui.ActiveColumnOrder)

	// Scope to container inside fn.
	var seenSession []string
	var seenHidden map[string]bool
	var seenOrder []string
	_ = m.withSessionColumnsForKind("container", func() string {
		seenSession = ui.ActiveSessionColumns
		seenHidden = ui.ActiveHiddenBuiltinColumns
		seenOrder = ui.ActiveColumnOrder
		return ""
	})

	assert.Equal(t, []string{"Image"}, seenSession,
		"inside fn the container's session extras must be active")
	assert.True(t, seenHidden["Status"],
		"inside fn the container's hidden built-ins must be active")
	assert.Equal(t, []string{"Image", "Namespace"}, seenOrder,
		"inside fn the container's column order must be active")

	// After fn returns, the pod config must be restored.
	assert.Equal(t, []string{"IP"}, ui.ActiveSessionColumns,
		"pod session columns must be restored")
	assert.True(t, ui.ActiveHiddenBuiltinColumns["Ready"],
		"pod hidden built-ins must be restored")
	assert.Equal(t, []string{"IP", "Namespace"}, ui.ActiveColumnOrder,
		"pod column order must be restored")
}

// TestCovColumnToggleDoesNotLeakBetweenPodAndContainer verifies the end-to-end
// scenario from the bug report: configuring pod columns at LevelResources
// must not affect container columns at LevelContainers (and vice versa).
func TestCovColumnToggleDoesNotLeakBetweenPodAndContainer(t *testing.T) {
	m := baseModelCov()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.middleItems = []model.Item{{Name: "pod-1", Kind: "Pod"}}

	// User at LevelResources (pods): save a session that hides everything
	// except Ready.
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: false, builtin: true},
		{key: "Ready", visible: true, builtin: true},
		{key: "Restarts", visible: false, builtin: true},
		{key: "Status", visible: false, builtin: true},
		{key: "Age", visible: false, builtin: true},
	}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)

	// The pod configuration should be saved under "pod".
	_, podSessionExists := rm.sessionColumns["pod"]
	assert.True(t, podSessionExists, "sessionColumns[pod] must be set")

	// The container configuration should NOT have been touched.
	_, containerSessionExists := rm.sessionColumns["container"]
	assert.False(t, containerSessionExists, "sessionColumns[container] must be untouched")
	_, containerHiddenExists := rm.hiddenBuiltinColumns["container"]
	assert.False(t, containerHiddenExists, "hiddenBuiltinColumns[container] must be untouched")
}

// TestCovColumnToggleEnterBuiltinsOnlyPersistsEmptyExtras verifies that
// saving a selection containing only built-in columns (no extras) records
// an EXPLICIT empty extras list rather than deleting the session entry.
// Deleting would cause selectColumnCandidates to fall back to auto-detect
// and re-add columns like CPU/MEM that the user just unselected.
func TestCovColumnToggleEnterBuiltinsOnlyPersistsEmptyExtras(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: true, builtin: true},
		{key: "Ready", visible: false, builtin: true},
		{key: "Restarts", visible: true, builtin: true},
		{key: "Status", visible: false, builtin: true},
		{key: "Age", visible: false, builtin: true},
		{key: "IP", visible: false, builtin: false},
		{key: "CPU", visible: false, builtin: false},
		{key: "MEM", visible: false, builtin: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)

	sess, exists := rm.sessionColumns["pod"]
	assert.True(t, exists, "sessionColumns[pod] must be set to record user's choice")
	assert.Empty(t, sess, "sessionColumns[pod] must be empty (no extras selected)")
	assert.NotNil(t, sess, "sessionColumns[pod] must be non-nil to distinguish from auto-detect")

	assert.ElementsMatch(t, []string{"Ready", "Status", "Age"}, rm.hiddenBuiltinColumns["pod"])
}

// TestCovColumnToggleEnterAllUnselectedResetsToDefault verifies that
// pressing Enter when no entries are visible (user unselected everything)
// resets both sessionColumns and hiddenBuiltinColumns for the kind, so the
// table reverts to its default column set instead of rendering nothing.
func TestCovColumnToggleEnterAllUnselectedResetsToDefault(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: false, builtin: true},
		{key: "Ready", visible: false, builtin: true},
		{key: "Status", visible: false, builtin: true},
		{key: "IP", visible: false, builtin: false},
		{key: "Node", visible: false, builtin: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.sessionColumns = map[string][]string{"pod": {"IP"}}
	m.hiddenBuiltinColumns = map[string][]string{"pod": {"Ready"}}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)

	assert.Equal(t, overlayNone, rm.overlay)
	_, sessionExists := rm.sessionColumns["pod"]
	assert.False(t, sessionExists, "sessionColumns[pod] must be cleared on full unselect")
	_, hiddenExists := rm.hiddenBuiltinColumns["pod"]
	assert.False(t, hiddenExists, "hiddenBuiltinColumns[pod] must be cleared on full unselect")
}

// TestCovColumnToggleResetClearsBothMaps verifies the R key clears both
// sessionColumns and hiddenBuiltinColumns for the current kind.
func TestCovColumnToggleResetClearsBothMaps(t *testing.T) {
	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.sessionColumns = map[string][]string{"pod": {"IP"}}
	m.hiddenBuiltinColumns = map[string][]string{"pod": {"Ready"}}
	m.columnToggleItems = []columnToggleEntry{{key: "IP"}}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	rm := r.(Model)

	assert.Equal(t, overlayNone, rm.overlay)
	_, sessionExists := rm.sessionColumns["pod"]
	assert.False(t, sessionExists, "sessionColumns[pod] must be cleared")
	_, hiddenExists := rm.hiddenBuiltinColumns["pod"]
	assert.False(t, hiddenExists, "hiddenBuiltinColumns[pod] must be cleared")
}

// TestCovColumnToggleEnterSavesColumnOrder verifies that pressing Enter
// records the full intermixed column order (built-ins + extras) in
// m.columnOrder[kind], so the next render honors the user's reordering.
func TestCovColumnToggleEnterSavesColumnOrder(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "Status", visible: true, builtin: true},
		{key: "Namespace", visible: true, builtin: true},
		{key: "IP", visible: true, builtin: false},
		{key: "Age", visible: true, builtin: true},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)

	assert.Equal(t, overlayNone, rm.overlay)
	assert.Equal(t, []string{"Status", "Namespace", "IP", "Age"}, rm.columnOrder["pod"],
		"columnOrder[pod] must preserve the overlay order")
}

// TestCovOpenColumnToggleRespectsSavedOrder verifies that opening the overlay
// after a user-saved column order reorders entries according to that saved
// order. Entries not in the saved order are appended in default position
// (built-ins first, then extras).
func TestCovOpenColumnToggleRespectsSavedOrder(t *testing.T) {
	m := baseModelNav()
	m.middleItems = []model.Item{
		{
			Name:      "pod-1",
			Namespace: "default",
			Kind:      "Pod",
			Ready:     "1/1",
			Restarts:  "0",
			Status:    "Running",
			Age:       "5m",
			Columns: []model.KeyValue{
				{Key: "IP", Value: "10.0.0.1"},
			},
		},
	}
	// Saved order puts Age first, then IP, then Namespace.
	m.columnOrder = map[string][]string{"pod": {"Age", "IP", "Namespace"}}
	// ActiveExtraColumnKeys controls which extras are "currently visible";
	// must include IP so it ends up visible in the overlay.
	defer func(orig []string) { ui.ActiveExtraColumnKeys = orig }(ui.ActiveExtraColumnKeys)
	ui.ActiveExtraColumnKeys = []string{"IP"}

	m.openColumnToggle()

	if !assert.GreaterOrEqual(t, len(m.columnToggleItems), 3) {
		return
	}
	// The first three entries must match the saved order.
	assert.Equal(t, "Age", m.columnToggleItems[0].key)
	assert.Equal(t, "IP", m.columnToggleItems[1].key)
	assert.Equal(t, "Namespace", m.columnToggleItems[2].key)

	// The remaining built-ins appear after, in default position.
	remainingKeys := make([]string, 0, len(m.columnToggleItems)-3)
	for _, e := range m.columnToggleItems[3:] {
		remainingKeys = append(remainingKeys, e.key)
	}
	assert.Contains(t, remainingKeys, "Ready")
	assert.Contains(t, remainingKeys, "Restarts")
	assert.Contains(t, remainingKeys, "Status")
}

// TestCovColumnToggleResetClearsColumnOrder verifies the R key also clears
// the per-kind columnOrder, so the next render reverts to default layout.
func TestCovColumnToggleResetClearsColumnOrder(t *testing.T) {
	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.sessionColumns = map[string][]string{"pod": {"IP"}}
	m.hiddenBuiltinColumns = map[string][]string{"pod": {"Ready"}}
	m.columnOrder = map[string][]string{"pod": {"Age", "Namespace"}}
	m.columnToggleItems = []columnToggleEntry{{key: "IP"}}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	rm := r.(Model)

	assert.Equal(t, overlayNone, rm.overlay)
	_, sessionExists := rm.sessionColumns["pod"]
	assert.False(t, sessionExists, "sessionColumns[pod] must be cleared")
	_, hiddenExists := rm.hiddenBuiltinColumns["pod"]
	assert.False(t, hiddenExists, "hiddenBuiltinColumns[pod] must be cleared")
	_, orderExists := rm.columnOrder["pod"]
	assert.False(t, orderExists, "columnOrder[pod] must be cleared")
}

// TestCovColumnToggleEnterAllUnselectedClearsColumnOrder verifies that
// pressing Enter with no visible entries also deletes m.columnOrder[kind]
// so the next render reverts to the default layout.
func TestCovColumnToggleEnterAllUnselectedClearsColumnOrder(t *testing.T) {
	m := baseModelCov()
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: false, builtin: true},
		{key: "IP", visible: false, builtin: false},
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.columnOrder = map[string][]string{"pod": {"IP", "Namespace"}}
	m.overlay = overlayColumnToggle

	r, _ := m.handleColumnToggleKey(tea.KeyMsg{Type: tea.KeyEnter})
	rm := r.(Model)

	_, exists := rm.columnOrder["pod"]
	assert.False(t, exists, "columnOrder[pod] must be cleared on full unselect")
}
