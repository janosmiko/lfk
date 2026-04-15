package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func TestLoadSecurityAvailabilityNilManager(t *testing.T) {
	m := Model{}
	assert.Nil(t, m.loadSecurityAvailability())
}

func TestLoadSecurityAvailabilityDispatches(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "fake", Available: true})

	m := Model{securityManager: mgr}
	m.nav.Context = "kctx"

	cmd := m.loadSecurityAvailability()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityAvailabilityLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "kctx", loaded.context)
	assert.True(t, loaded.availability["fake"], "fake source should be available")
}

func TestSecurityAvailabilityLoadedMsgUpdatesModel(t *testing.T) {
	m := Model{securityAvailabilityByName: make(map[string]bool)}
	m.nav.Context = "kctx"
	msg := securityAvailabilityLoadedMsg{
		context: "kctx",
		availability: map[string]bool{
			"trivy-operator": true,
			"heuristic":      true,
		},
	}
	updated, _ := m.handleSecurityAvailabilityLoaded(msg)
	assert.True(t, updated.securityAvailabilityByName["trivy-operator"])
	assert.True(t, updated.securityAvailabilityByName["heuristic"])
}

func TestSecurityAvailabilityLoadedStaleContextDiscarded(t *testing.T) {
	m := Model{securityAvailabilityByName: make(map[string]bool)}
	m.nav.Context = "current"
	msg := securityAvailabilityLoadedMsg{
		context:      "stale",
		availability: map[string]bool{"trivy-operator": true},
	}
	updated, _ := m.handleSecurityAvailabilityLoaded(msg)
	assert.False(t, updated.securityAvailabilityByName["trivy-operator"])
}

// installTestSecurityHook wires SecuritySourcesFn so BuildSidebarItems sees
// the provided sources during the test. Returns a cleanup that restores the
// prior hook. Tests that rely on the handler rebuilding sidebar items via
// BuildSidebarItems must install this hook; otherwise the default nil hook
// makes injectSecuritySourceItems a no-op and the tests can't observe the
// Security category.
func installTestSecurityHook(t *testing.T, sources ...model.SecuritySourceEntry) {
	t.Helper()
	prev := model.SecuritySourcesFn
	t.Cleanup(func() { model.SecuritySourcesFn = prev })
	model.SecuritySourcesFn = func() []model.SecuritySourceEntry {
		return sources
	}
}

// TestSecurityAvailabilityLoadedRebuildsLeftItemsAtLevelResources verifies
// the fix for the cold-start bug: when the user restores a session and lands
// at LevelResources, the parent column (leftItems) holds the resource types
// list — which was built with an empty availability map and therefore has
// no Security entries. When the probe result arrives, the handler must
// rebuild leftItems (and clear any stale itemCache entry for the resource
// types level) so that navigating back with `h` immediately shows the
// Security category.
func TestSecurityAvailabilityLoadedRebuildsLeftItemsAtLevelResources(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: make(map[string]bool),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
	}
	m.nav.Level = model.LevelResources
	m.nav.Context = "kctx"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.discoveredResources["kctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	// leftItems starts with no Security entries (hook not installed yet).
	m.leftItems = model.BuildSidebarItems(m.discoveredResources["kctx"])
	for _, it := range m.leftItems {
		require.NotEqual(t, "Security", it.Category,
			"precondition: leftItems should have no Security entries yet")
	}
	// Install the hook AFTER the precondition so the rebuild picks up the
	// Trivy entry but the precondition check above still holds.
	installTestSecurityHook(t, model.SecuritySourceEntry{
		DisplayName: "Trivy",
		SourceName:  "trivy-operator",
		Icon:        model.Icon{Unicode: "◈"},
		Count:       -1,
	})

	// Availability probe completes and reports Trivy is available.
	updated, _ := m.handleSecurityAvailabilityLoaded(securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"trivy-operator": true},
	})

	// leftItems must now contain the Trivy Security entry.
	var gotTrivy bool
	for _, it := range updated.leftItems {
		if it.Category == "Security" && it.Kind == "__security_trivy-operator__" {
			gotTrivy = true
			break
		}
	}
	assert.True(t, gotTrivy,
		"handler must rebuild leftItems so Security category appears at LevelResources")
}

// TestSecurityAvailabilityLoadedStillRebuildsMiddleAtLevelResourceTypes
// guards the existing behavior: at LevelResourceTypes, middleItems (not
// leftItems) holds the sidebar, so the original rebuild path must still fire.
func TestSecurityAvailabilityLoadedStillRebuildsMiddleAtLevelResourceTypes(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: make(map[string]bool),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
	}
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "kctx"
	m.discoveredResources["kctx"] = []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	m.middleItems = model.BuildSidebarItems(m.discoveredResources["kctx"])
	installTestSecurityHook(t, model.SecuritySourceEntry{
		DisplayName: "Heuristic",
		SourceName:  "heuristic",
		Icon:        model.Icon{Unicode: "◉"},
		Count:       -1,
	})

	updated, _ := m.handleSecurityAvailabilityLoaded(securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"heuristic": true},
	})

	var gotHeuristic bool
	for _, it := range updated.middleItems {
		if it.Category == "Security" && it.Kind == "__security_heuristic__" {
			gotHeuristic = true
			break
		}
	}
	assert.True(t, gotHeuristic,
		"middleItems must be rebuilt at LevelResourceTypes")
}

func TestRenderRightResourcesShowsFindingDetails(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{
		{
			Name: "CVE-2024-1234",
			Kind: "__security_finding__",
			Columns: []model.KeyValue{
				{Key: "Severity", Value: "CRIT"},
				{Key: "Title", Value: "CVE-2024-1234"},
				{Key: "Resource", Value: "deploy/api"},
			},
		},
	}
	out := m.renderRightResources(80, 20)
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "deploy/api")
}

// ---------------------------------------------------------------------------
// Tests for per-source bgtask tracking (loadSecurityFindings)
// ---------------------------------------------------------------------------

func TestLoadSecurityFindingsNilManager(t *testing.T) {
	m := Model{}
	assert.Nil(t, m.loadSecurityFindings(), "nil securityManager must return nil cmd")
}

func TestLoadSecurityFindingsNoAvailableSources(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: map[string]bool{"trivy-operator": false, "heuristic": false},
		bgtasks:                    bgtasks.New(0),
	}
	m.nav.Context = "kctx"
	assert.Nil(t, m.loadSecurityFindings(),
		"no available sources must return nil cmd")
}

func TestLoadSecurityFindingsDispatchesPerSource(t *testing.T) {
	trivySrc := &security.FakeSource{
		NameStr:   "trivy-operator",
		Available: true,
		Findings: []security.Finding{
			{
				ID: "CVE-1", Source: "trivy-operator", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "web"},
			},
		},
	}
	heuristicSrc := &security.FakeSource{
		NameStr:   "heuristic",
		Available: true,
		Findings: []security.Finding{
			{
				ID: "H-1", Source: "heuristic", Severity: security.SeverityMedium,
				Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"},
			},
		},
	}
	mgr := security.NewManager()
	mgr.Register(trivySrc)
	mgr.Register(heuristicSrc)

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: map[string]bool{"trivy-operator": true, "heuristic": true},
		bgtasks:                    bgtasks.New(0),
		namespace:                  "default",
		selectedNamespaces:         make(map[string]bool),
	}
	m.nav.Context = "kctx"

	cmd := m.loadSecurityFindings()
	require.NotNil(t, cmd, "two available sources must produce a non-nil cmd")

	// tea.Batch returns a func that yields a tea.BatchMsg ([]tea.Cmd).
	batchResult := cmd()
	batch, ok := batchResult.(tea.BatchMsg)
	require.True(t, ok, "top-level cmd must produce a tea.BatchMsg")
	assert.Len(t, batch, 2, "one cmd per available source")

	// Execute each inner cmd and collect the messages.
	sources := make(map[string]securityFindingsScannedMsg)
	for _, innerCmd := range batch {
		msg := innerCmd()
		scanned, ok := msg.(securityFindingsScannedMsg)
		require.True(t, ok, "inner cmd must produce securityFindingsScannedMsg")
		sources[scanned.source] = scanned
	}
	assert.Contains(t, sources, "trivy-operator")
	assert.Contains(t, sources, "heuristic")
	assert.Len(t, sources["trivy-operator"].findings, 1)
	assert.Equal(t, "CVE-1", sources["trivy-operator"].findings[0].ID)
	assert.Equal(t, "kctx", sources["trivy-operator"].context)
	assert.Equal(t, "default", sources["trivy-operator"].namespace)
}

// ---------------------------------------------------------------------------
// Tests for handleSecurityFindingsScanned
// ---------------------------------------------------------------------------

func TestSecurityFindingsScannedUpdatesIndex(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: map[string]bool{"trivy-operator": true},
		securityFindingsBySource:   make(map[string][]security.Finding),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
		selectedNamespaces:         make(map[string]bool),
	}
	m.nav.Context = "kctx"
	m.nav.Level = model.LevelResourceTypes

	findings := []security.Finding{
		{
			ID: "CVE-1", Title: "CVE-2024-1001", Source: "trivy-operator", Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "web"},
		},
		{
			ID: "CVE-2", Title: "CVE-2024-1002", Source: "trivy-operator", Severity: security.SeverityHigh,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "web"},
		},
	}
	msg := securityFindingsScannedMsg{
		context:   "kctx",
		namespace: "default",
		source:    "trivy-operator",
		findings:  findings,
	}

	updated := m.handleSecurityFindingsScanned(msg)
	assert.Equal(t, findings, updated.securityFindingsBySource["trivy-operator"])
	// The manager's index must now reflect the findings.
	idx := updated.securityManager.Index()
	assert.Equal(t, 2, idx.CountBySource("trivy-operator"))
}

func TestSecurityFindingsScannedStaleContextDiscarded(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: map[string]bool{"heuristic": true},
		securityFindingsBySource:   make(map[string][]security.Finding),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
		selectedNamespaces:         make(map[string]bool),
	}
	m.nav.Context = "current-ctx"

	msg := securityFindingsScannedMsg{
		context:   "stale-ctx",
		namespace: "default",
		source:    "heuristic",
		findings: []security.Finding{
			{
				ID: "H-1", Source: "heuristic", Severity: security.SeverityLow,
				Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "x"},
			},
		},
	}

	updated := m.handleSecurityFindingsScanned(msg)
	assert.Empty(t, updated.securityFindingsBySource["heuristic"],
		"stale context message must be discarded")
}

func TestSecurityFindingsScannedAggregatesMultipleSources(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: map[string]bool{"trivy-operator": true, "heuristic": true},
		securityFindingsBySource:   make(map[string][]security.Finding),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
		selectedNamespaces:         make(map[string]bool),
	}
	m.nav.Context = "kctx"
	m.nav.Level = model.LevelResourceTypes

	// First source arrives.
	m = m.handleSecurityFindingsScanned(securityFindingsScannedMsg{
		context:   "kctx",
		namespace: "default",
		source:    "trivy-operator",
		findings: []security.Finding{
			{
				ID: "CVE-1", Source: "trivy-operator", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "web"},
			},
		},
	})

	// Second source arrives.
	m = m.handleSecurityFindingsScanned(securityFindingsScannedMsg{
		context:   "kctx",
		namespace: "default",
		source:    "heuristic",
		findings: []security.Finding{
			{
				ID: "H-1", Title: "Privileged container", Source: "heuristic", Severity: security.SeverityMedium,
				Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"},
			},
			{
				ID: "H-2", Title: "No resource limits", Source: "heuristic", Severity: security.SeverityLow,
				Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"},
			},
		},
	})

	// Both sources must be present in the per-source map.
	assert.Len(t, m.securityFindingsBySource["trivy-operator"], 1)
	assert.Len(t, m.securityFindingsBySource["heuristic"], 2)

	// The manager's index must aggregate all 3 findings.
	idx := m.securityManager.Index()
	assert.Equal(t, 1, idx.CountBySource("trivy-operator"))
	assert.Equal(t, 2, idx.CountBySource("heuristic"))
}

// ---------------------------------------------------------------------------
// Tests for securitySourceDisplayName helper
// ---------------------------------------------------------------------------

func TestSecuritySourceDisplayName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"heuristic", "Heuristic"},
		{"trivy-operator", "Trivy"},
		{"policy-report", "Kyverno"},
		{"kube-bench", "CIS"},
		{"unknown-source", "unknown-source"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, securitySourceDisplayName(tt.input))
		})
	}
}

// ---------------------------------------------------------------------------
// Test for handleSecurityAvailabilityLoaded triggering scan
// ---------------------------------------------------------------------------

func TestHandleSecurityAvailabilityLoadedTriggersScan(t *testing.T) {
	src := &security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{
			{
				ID: "H-1", Source: "heuristic", Severity: security.SeverityLow,
				Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "x"},
			},
		},
	}
	mgr := security.NewManager()
	mgr.Register(src)

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: make(map[string]bool),
		securityFindingsBySource:   make(map[string][]security.Finding),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
		selectedNamespaces:         make(map[string]bool),
		bgtasks:                    bgtasks.New(0),
		namespace:                  "default",
	}
	m.nav.Context = "kctx"
	m.nav.Level = model.LevelResourceTypes

	_, cmd := m.handleSecurityAvailabilityLoaded(securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"heuristic": true},
	})
	assert.NotNil(t, cmd, "handler must return scan cmd when sources are available")
}

// ---------------------------------------------------------------------------
// Test for navKey including namespace for namespaced RTs
// ---------------------------------------------------------------------------

func TestNavKeyIncludesNamespaceForNamespacedRT(t *testing.T) {
	m := Model{
		namespace:          "prod",
		selectedNamespaces: make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
	}
	m.nav.Context = "cluster-a"

	// Namespaced resource type: key must include ns: component.
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", Resource: "pods", Namespaced: true,
	}
	key := m.navKey()
	assert.Contains(t, key, "ns:prod",
		"namespaced RT must include ns: component in navKey")
	assert.Contains(t, key, "cluster-a")
	assert.Contains(t, key, "pods")

	// Cluster-scoped resource type: key must NOT include ns: component.
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Node", Resource: "nodes", Namespaced: false,
	}
	key = m.navKey()
	assert.NotContains(t, key, "ns:",
		"cluster-scoped RT must not include ns: component in navKey")
	assert.Contains(t, key, "cluster-a")
	assert.Contains(t, key, "nodes")
}

// ---------------------------------------------------------------------------
// Tests for security finding group navigation
// ---------------------------------------------------------------------------

func TestNavigateChildResourceSecurityGroup(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: map[string]bool{"heuristic": true},
		bgtasks:                    bgtasks.New(0),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		selectedNamespaces:         make(map[string]bool),
		selectedItems:              make(map[string]bool),
		selectionAnchor:            -1,
		tabs:                       []TabState{{}},
	}
	m.nav.Level = model.LevelResources
	m.nav.Context = "kctx"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "__security_heuristic__",
		APIGroup:   model.SecurityVirtualAPIGroup,
		APIVersion: "v1",
		Resource:   "findings-heuristic",
		Namespaced: true,
	}

	sel := &model.Item{
		Name:  "privileged",
		Kind:  "__security_finding_group__",
		Extra: "privileged",
	}

	ret, cmd := m.navigateChildResource(sel)
	updated, ok := ret.(Model)
	require.True(t, ok, "return value must be a Model")

	assert.Equal(t, model.LevelOwned, updated.nav.Level,
		"group navigation must advance to LevelOwned")
	assert.Equal(t, "privileged", updated.nav.ResourceName,
		"ResourceName must be set to the group name")
	assert.NotNil(t, cmd,
		"navigateChildResource must return a load command for affected resources")
}

func TestEnterFullViewSecurityGroup(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})

	m := Model{
		securityManager:            mgr,
		securityAvailabilityByName: map[string]bool{"heuristic": true},
		bgtasks:                    bgtasks.New(0),
		itemCache:                  make(map[string][]model.Item),
		cursorMemory:               make(map[string]int),
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		selectedNamespaces:         make(map[string]bool),
		selectedItems:              make(map[string]bool),
		selectionAnchor:            -1,
		tabs:                       []TabState{{}},
	}
	m.nav.Level = model.LevelResources
	m.nav.Context = "kctx"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "__security_heuristic__",
		APIGroup:   model.SecurityVirtualAPIGroup,
		APIVersion: "v1",
		Resource:   "findings-heuristic",
		Namespaced: true,
	}
	m.middleItems = []model.Item{
		{
			Name:  "privileged",
			Kind:  "__security_finding_group__",
			Extra: "privileged",
		},
	}
	m.setCursor(0)

	ret, _ := m.enterFullView()
	updated, ok := ret.(Model)
	require.True(t, ok, "return value must be a Model")

	assert.Equal(t, model.LevelOwned, updated.nav.Level,
		"Enter on a group item must navigate to LevelOwned")
}

func TestEnterFullViewAffectedResource(t *testing.T) {
	m := Model{
		itemCache:           make(map[string][]model.Item),
		cursorMemory:        make(map[string]int),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		selectedNamespaces:  make(map[string]bool),
		selectedItems:       make(map[string]bool),
		selectionAnchor:     -1,
		tabs:                []TabState{{}},
	}
	m.nav.Level = model.LevelOwned
	m.nav.Context = "kctx"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "__security_heuristic__",
		APIGroup:   model.SecurityVirtualAPIGroup,
		APIVersion: "v1",
		Resource:   "findings-heuristic",
		Namespaced: true,
	}
	m.middleItems = []model.Item{
		{
			Name: "pod/web",
			Kind: "__security_affected_resource__",
			Columns: []model.KeyValue{
				{Key: "ResourceKind", Value: "Pod"},
				{Key: "Resource", Value: "pod/web"},
			},
		},
	}
	m.setCursor(0)

	// enterFullView on an affected resource calls jumpToFindingResource,
	// which requires discoveredResources to resolve the kind. Without
	// that wiring, it returns a status message. Verify it does not panic
	// and returns a valid model.
	ret, _ := m.enterFullView()
	_, ok := ret.(Model)
	assert.True(t, ok, "enterFullView on affected resource must return a Model without panicking")
}
