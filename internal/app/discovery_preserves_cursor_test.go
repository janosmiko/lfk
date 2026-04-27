package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Periodic discovery must not reset the cursor at LevelResourceTypes.
// Repro of the user-reported "press j, cursor jumps back to top": with
// watch mode on, every 2s the watch tick triggers a fresh discovery
// (intentional — needed for new CRDs). When the
// apiResourceDiscoveryMsg arrives, the handler used to call
// restoreCursor() unconditionally, which falls back to 0 whenever
// cursorMemory has no entry for the current navKey — and at
// LevelResourceTypes the cursor is rarely "saved" until the user
// drills in.
//
// Fix contract: the cursor must be preserved across periodic discovery
// completions. clampCursor (preserves any in-bounds value) replaces
// restoreCursor for the success path.
func TestDiscoveryCompletionPreservesCursorPosition(t *testing.T) {
	t.Parallel()
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.allGroupsExpanded = true // bypass accordion side-effects for this test
	// Subsequent discovery (loading already false) — the periodic refresh
	// path that the bug touches.
	m.loading = false

	// Use real built-in kinds so BuildSidebarItems keeps them. Pods,
	// Deployments, Services, ConfigMaps, Secrets, Jobs (>= 6 items so
	// cursor=5 is valid in the merged list).
	entries := []model.ResourceTypeEntry{
		{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Deployment", Resource: "deployments", APIGroup: "apps", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Service", Resource: "services", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "ConfigMap", Resource: "configmaps", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Secret", Resource: "secrets", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Job", Resource: "jobs", APIGroup: "batch", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "CronJob", Resource: "cronjobs", APIGroup: "batch", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
		{Kind: "Ingress", Resource: "ingresses", APIGroup: "networking.k8s.io", APIVersion: "v1", Namespaced: true, Verbs: []string{"list"}},
	}
	// Seed middleItems from the same entries so the "subsequent refresh"
	// branch (clampCursor) sees a populated list to clamp against.
	m.middleItems = model.BuildSidebarItems(entries)
	m.setCursor(5)
	// Note: cursorMemory has NO entry for navKey — the user moved the
	// cursor interactively. restoreCursor would fall back to 0.

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		entries: entries,
	})

	assert.NotZero(t, updated.cursor(),
		"cursor must NOT reset to 0 after periodic discovery — it should be preserved")
}
