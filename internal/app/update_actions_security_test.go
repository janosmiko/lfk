package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func TestExecuteActionSecurityFindingsLoading(t *testing.T) {
	m := Model{}
	m.actionCtx = actionContext{kind: "Pod", name: "nginx", namespace: "default"}

	updated, _ := m.executeActionSecurityFindings()
	assert.Contains(t, updated.statusMessage, "still loading",
		"with no index built, the action surfaces a loading hint")
}

func TestExecuteActionSecurityFindingsNoFindings(t *testing.T) {
	m := Model{securityModelState: securityModelState{
		securityIndex: security.BuildFindingIndex(nil),
	}}
	m.actionCtx = actionContext{kind: "Pod", name: "nginx", namespace: "default"}

	updated, _ := m.executeActionSecurityFindings()
	assert.Contains(t, updated.statusMessage, "No security findings",
		"empty index for the resource -> 'no findings' message")
	assert.Contains(t, updated.statusMessage, "Pod/nginx",
		"status message names the resource so the user knows what was checked")
}

// securityActionModel builds a model positioned on a Pod row (with an owner
// Deployment) whose index holds findings for both, plus a warm security
// manager so the teleport's loadResources serves synchronously.
func securityActionModel(t *testing.T) Model {
	t.Helper()
	mgr := security.NewManager()
	mgr.SetRefreshTTL(time.Hour)
	mgr.Register(&security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{{
			Source: "heuristic", Title: "Privileged container", Severity: security.SeverityHigh,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api-xyz"},
			Labels:   map[string]string{"check": "privileged"},
		}},
	})
	mgr.Register(&security.FakeSource{
		NameStr: "trivy-operator", Available: true,
		Findings: []security.Finding{{
			Source: "trivy-operator", Title: "CVE-1", Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"},
			Labels:   map[string]string{"cve": "CVE-1"},
		}},
	})

	m := baseModelBoost2()
	m.securityManager = mgr
	m.client.SetSecurityManager(mgr)
	m.securityIndex = security.BuildFindingIndex([]security.Finding{
		{ID: "1", Severity: security.SeverityHigh, Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api-xyz"}},
		{ID: "2", Severity: security.SeverityCritical, Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"}},
	})
	m.leftItems = []model.Item{{Name: "Pods", Kind: "ResourceType"}}
	m.middleItems = []model.Item{{
		Kind: "Pod", Name: "api-xyz", Namespace: "default",
		Columns: []model.KeyValue{{Key: "owner:0", Value: "apps/v1||Deployment||api"}},
	}}
	m.setCursor(0)
	m.actionCtx = actionContext{kind: "Pod", name: "api-xyz", namespace: "default"}

	// Warm the shared scan so loadResources serves the list synchronously.
	_, err := mgr.FetchAll(m.reqCtx, m.nav.Context, m.effectiveNamespace())
	require.NoError(t, err)
	return m
}

// The action must open a real findings list filtered to the selected resource
// (plus its owners — the badge set), not just print a count.
func TestExecuteActionSecurityFindingsOpensFilteredList(t *testing.T) {
	m := securityActionModel(t)

	updated, cmd := m.executeActionSecurityFindings()

	assert.Equal(t, model.LevelResources, updated.nav.Level)
	assert.Equal(t, model.SecurityResourceFindingsKind, updated.nav.ResourceType.Kind,
		"navigates into the per-resource findings view")
	assert.Equal(t, model.SecurityVirtualAPIGroup, updated.nav.ResourceType.APIGroup)
	assert.Contains(t, updated.nav.ResourceType.DisplayName, "api-xyz",
		"breadcrumb names the resource the list is filtered to")
	assert.Equal(t, []security.ResourceRef{
		{Namespace: "default", Kind: "Pod", Name: "api-xyz"},
		{Namespace: "default", Kind: "Deployment", Name: "api"},
	}, updated.securityResourceFilter,
		"filter carries the pod's own ref plus its owner refs — the badge set")
	require.Len(t, updated.jumpBackStack, 1, "teleport records the origin for jump-back")

	// Warm cache: the load serves the cross-source list synchronously.
	require.NotNil(t, cmd)
	msg := cmd()
	rl, ok := msg.(resourcesLoadedMsg)
	require.True(t, ok)
	require.Len(t, rl.items, 2, "one group per source touching the pod or its owner")
	assert.Equal(t, "CVE-1", rl.items[0].Name, "severity-sorted: trivy CRIT first")
	assert.Equal(t, "trivy-operator", rl.items[0].ColumnValue("Source"))
	assert.Equal(t, "privileged", rl.items[1].Name)
}

// Jump-back after the teleport must restore the original pod list position.
func TestExecuteActionSecurityFindingsJumpBackReturnsToOrigin(t *testing.T) {
	m := securityActionModel(t)

	updated, _ := m.executeActionSecurityFindings()
	require.Len(t, updated.jumpBackStack, 1)

	back, _ := updated.jumpBack()
	bm := back.(Model)
	assert.Equal(t, model.LevelResources, bm.nav.Level)
	assert.Equal(t, "Pod", bm.nav.ResourceType.Kind, "back on the pod list")
	assert.Empty(t, bm.securityResourceFilter, "filter cleared on jump-back")
	require.NotEmpty(t, bm.middleItems)
	assert.Equal(t, "api-xyz", bm.middleItems[0].Name)
}

// From LevelOwned (pod under a workload) the teleport must unwind the
// left-pane stack back to the LevelResources depth so Esc behaves as if the
// user navigated to the findings view directly.
func TestExecuteActionSecurityFindingsFromOwnedLevelUnwinds(t *testing.T) {
	m := securityActionModel(t)
	// Simulate Deployment list -> drilled into deployment's pods.
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true}
	m.nav.ResourceName = "api"
	m.leftItemsHistory = [][]model.Item{{{Name: "Deployments", Kind: "ResourceType"}}}
	m.leftItems = []model.Item{{Name: "api", Kind: "Deployment"}}

	updated, _ := m.executeActionSecurityFindings()

	assert.Equal(t, model.LevelResources, updated.nav.Level)
	assert.Equal(t, model.SecurityResourceFindingsKind, updated.nav.ResourceType.Kind)
	assert.Empty(t, updated.nav.OwnedName)
	require.Len(t, updated.leftItems, 1, "left pane unwound to the sidebar level")
	assert.Equal(t, "Deployments", updated.leftItems[0].Name)
	assert.Empty(t, updated.leftItemsHistory)
}

// In union-sentinel mode the per-resource list cannot resolve a single
// cluster's scan; the action keeps the count-summary fallback.
func TestExecuteActionSecurityFindingsUnionFallsBackToSummary(t *testing.T) {
	m := securityActionModel(t)
	m.unionMode = true
	m.nav.Context = UnionContextSentinel

	updated, _ := m.executeActionSecurityFindings()
	assert.Equal(t, model.LevelResources, updated.nav.Level)
	assert.Equal(t, "Pod", updated.nav.ResourceType.Kind, "no navigation in union mode")
	assert.Contains(t, updated.statusMessage, "security findings")
	assert.Contains(t, updated.statusMessage, "Pod/api-xyz")
}

// Drilling into a finding group inside the per-resource view must record the
// row's source so the affected-resources fetch can scope to it — the view's
// sentinel Kind carries no source.
func TestNavigateChildResourceFindingGroupRecordsSource(t *testing.T) {
	m := securityActionModel(t)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:     model.SecurityResourceFindingsKind,
		APIGroup: model.SecurityVirtualAPIGroup,
		Resource: "findings-resource/default/Pod/api-xyz",
	}
	sel := &model.Item{
		Kind: "__security_finding_group__", Name: "CVE-1", Extra: "CVE-1",
		Columns: []model.KeyValue{{Key: "__source__", Value: "trivy-operator"}},
	}
	m.middleItems = []model.Item{*sel}
	m.setCursor(0)

	rm, cmd := m.navigateChildResource(sel)
	rmm := rm.(Model)
	assert.Equal(t, "CVE-1", rmm.securityActiveGroup)
	assert.Equal(t, "trivy-operator", rmm.securityActiveSource,
		"drill-in records the row's source for the affected-resources fetch")
	assert.Equal(t, model.LevelOwned, rmm.nav.Level)

	// Warm cache: the affected-resources load serves synchronously and is
	// scoped to the recorded source.
	require.NotNil(t, cmd)
	msg := cmd()
	ol, ok := msg.(ownedLoadedMsg)
	require.True(t, ok, "warm cache serves ownedLoadedMsg synchronously")
	require.Len(t, ol.items, 1)
	assert.Equal(t, "deploy/api", ol.items[0].Name)
}

// The hover preview inside the per-resource view resolves the source from
// the hovered row, not from nav.ResourceType.
func TestLoadSecurityAffectedResourcesPreviewUsesRowSource(t *testing.T) {
	m := securityActionModel(t)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:     model.SecurityResourceFindingsKind,
		APIGroup: model.SecurityVirtualAPIGroup,
		Resource: "findings-resource/default/Pod/api-xyz",
	}
	m.middleItems = []model.Item{{
		Kind: "__security_finding_group__", Name: "privileged", Extra: "privileged",
		Columns: []model.KeyValue{{Key: "__source__", Value: "heuristic"}},
	}}
	m.setCursor(0)

	cmd := m.loadSecurityAffectedResources(true)
	require.NotNil(t, cmd)
	msg := cmd()
	ol, ok := msg.(ownedLoadedMsg)
	require.True(t, ok)
	require.Len(t, ol.items, 1)
	assert.Equal(t, "pod/api-xyz", ol.items[0].Name)
}

// The badge-parity guarantee from L959 carries over: the refs the list is
// filtered by include owner-attributed findings, so a pod whose only finding
// sits on its Deployment still opens a non-empty list.
func TestExecuteActionSecurityFindingsOwnerOnlyFindingsStillNavigate(t *testing.T) {
	m := securityActionModel(t)
	// Index where ONLY the owner Deployment has a finding.
	m.securityIndex = security.BuildFindingIndex([]security.Finding{
		{ID: "2", Severity: security.SeverityCritical, Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"}},
	})

	updated, _ := m.executeActionSecurityFindings()
	assert.Equal(t, model.SecurityResourceFindingsKind, updated.nav.ResourceType.Kind,
		"owner-attributed finding must open the list, matching the badge")
}
