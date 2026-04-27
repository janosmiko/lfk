package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Reproducer for the user-reported bug: at LevelResourceTypes, pressing
// j (or any movement) sets m.loading=true via invalidatePreviewForCursorChange.
// When the next watch-tick discovery completes, the previous fix used
// m.loading to distinguish "initial vs. subsequent" — and saw the
// post-cursor-move loading=true as "initial", calling restoreCursor,
// which falls back to cursorMemory entry (the position the user was on
// when they last drilled in: Pods=0). Cursor jumped back to Pods.
//
// The real distinguisher is "was middleItems already populated before
// this discovery?" — if yes, the user has been here, preserve their
// cursor; if no, this is initial population, restore from memory.
func TestDiscoveryAfterCursorMovePreservesCursor(t *testing.T) {
	t.Parallel()
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.allGroupsExpanded = true

	entries := []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Deployment", Resource: "deployments", APIGroup: "apps", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Service", Resource: "services", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "ConfigMap", Resource: "configmaps", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Secret", Resource: "secrets", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Job", Resource: "jobs", APIGroup: "batch", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Ingress", Resource: "ingresses", APIGroup: "networking.k8s.io", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
	}
	m.middleItems = model.BuildSidebarItems(entries)
	m.setCursor(3)

	// Simulate the state right after a cursor move: m.loading was set
	// to true by invalidatePreviewForCursorChange, even though the
	// middle list is actually populated.
	m.loading = true

	// Cursor memory points to Pods (0) from the user's previous drill-in
	// — restoreCursor would fall back to it and reset to 0.
	m.cursorMemory[m.navKey()] = 0

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		entries: entries,
	})

	assert.Equal(t, 3, updated.cursor(),
		"cursor must stay at the user's position even when m.loading=true from a recent cursor move")
}
