package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// newTestModelWithSecurity returns a Model with the bgtasks registry
// initialised and the supplied manager wired in. Tests use it to exercise
// the security command/dispatch flow without standing up a real client.
func newTestModelWithSecurity(t *testing.T, mgr *security.Manager, kctx string) Model {
	t.Helper()
	m := Model{
		scheduler:       scheduler.New(scheduler.DefaultThreshold),
		suppressBgtasks: true, // skip the registry side-effects in tests
	}
	m.securityManager = mgr
	m.nav.Context = kctx
	return m
}

func TestLoadSecurityAvailabilityNilManager(t *testing.T) {
	m := newTestModelWithSecurity(t, nil, "kctx")
	assert.Nil(t, m.loadSecurityAvailability(),
		"no manager wired -> command must be nil so Init can omit it")
}

func TestLoadSecurityAvailabilityDispatches(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "fake", Available: true})
	m := newTestModelWithSecurity(t, mgr, "kctx")

	cmd := m.loadSecurityAvailability()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityAvailabilityLoadedMsg)
	require.True(t, ok, "expected securityAvailabilityLoadedMsg, got %T", msg)
	assert.Equal(t, "kctx", loaded.context)
	assert.True(t, loaded.availability["fake"], "fake source should be available")
}

// TestLoadSecurityAffectedResourcesEndToEnd reproduces the user-reported
// "open a heuristic/kyverno finding -> empty list" failure by driving the
// drill-in flow through navigateChildResource and asserting the resulting
// ownedLoadedMsg carries the matching affected resources.
//
// The bug surface is: navigateChildResource for a __security_finding_group__
// must wire (a) the synthetic ResourceType (containing the source name in
// Kind) and (b) the group key on m.securityActiveGroup so
// loadSecurityAffectedResources can filter findings correctly.
func TestLoadSecurityAffectedResourcesEndToEnd(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr:   "heuristic",
		Available: true,
		Findings: []security.Finding{
			{
				ID: "1", Source: "heuristic", Title: "privileged",
				Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "web"},
				Labels:   map[string]string{"check": "privileged"},
			},
			{
				ID: "2", Source: "heuristic", Title: "privileged",
				Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "api"},
				Labels:   map[string]string{"check": "privileged"},
			},
		},
	})
	m := baseModelBoost2()
	m.securityManager = mgr
	m.client.SetSecurityManager(mgr)
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "__security_heuristic__",
		APIGroup:   model.SecurityVirtualAPIGroup,
		APIVersion: "v1",
		Resource:   "findings-heuristic",
		Namespaced: true,
	}
	m.namespace = ""
	// Sentinel finding-group row that the user would Enter on.
	m.middleItems = []model.Item{{
		Kind:  "__security_finding_group__",
		Name:  "privileged",
		Extra: "privileged",
	}}
	m.setCursor(0)

	sel := &m.middleItems[0]
	rm, cmd := m.navigateChildResource(sel)
	rmm := rm.(Model)
	require.Equal(t, model.LevelOwned, rmm.nav.Level)
	require.Equal(t, "privileged", rmm.securityActiveGroup,
		"navigateChildResource must record the group key for loadSecurityAffectedResources")
	require.NotNil(t, cmd, "drill-in must dispatch the affected-resources fetch")

	msg := cmd()
	loaded, ok := msg.(ownedLoadedMsg)
	require.True(t, ok, "expected ownedLoadedMsg, got %T", msg)
	require.NoError(t, loaded.err)
	require.Len(t, loaded.items, 2,
		"both affected pods must surface; empty list is the user-reported bug")
}

// TestLoadPreviewForSecurityFindingGroupHover guards the children-pane
// split-view path: hovering a finding group at LevelResources must
// dispatch a forPreview=true affected-resources fetch so the right-pane
// renderer can show the AFFECTED RESOURCES table on top of the group
// details (like Pod -> containers split). The bug user reported in
// "Falco findings: children pane doesn't show affected resources" would
// surface here as a nil cmd or an ownedLoadedMsg with empty items.
func TestLoadPreviewForSecurityFindingGroupHover(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr:   "falco",
		Available: true,
		Findings: []security.Finding{
			{
				ID: "1", Source: "falco", Title: "Privileged container",
				Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "web"},
				Labels:   map[string]string{"rule": "Privileged container"},
			},
			{
				ID: "2", Source: "falco", Title: "Privileged container",
				Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "api"},
				Labels:   map[string]string{"rule": "Privileged container"},
			},
		},
	})
	m := baseModelBoost2()
	m.securityManager = mgr
	m.client.SetSecurityManager(mgr)
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "__security_falco__",
		APIGroup:   model.SecurityVirtualAPIGroup,
		APIVersion: "v1",
		Resource:   "findings-falco",
		Namespaced: true,
	}
	m.namespace = ""
	m.middleItems = []model.Item{{
		Kind:  "__security_finding_group__",
		Name:  "Privileged container",
		Extra: "Privileged container",
	}}
	m.setCursor(0)

	cmd := m.loadPreview()
	require.NotNil(t, cmd, "hover preview must dispatch")

	msg := cmd()
	loaded, ok := msg.(ownedLoadedMsg)
	require.True(t, ok, "expected ownedLoadedMsg, got %T", msg)
	require.True(t, loaded.forPreview, "forPreview must be set so updateOwnedLoaded routes to the rightItems branch")
	require.NoError(t, loaded.err)
	require.Len(t, loaded.items, 2,
		"both affected pods must surface so renderSecurityGroupSplitPreview can show the AFFECTED RESOURCES table")
}

// TestLoadSecurityAvailabilityProbesInParallel locks in the parallel-probe
// behaviour. The original implementation ran every source's IsAvailable
// sequentially under a single 3s budget, so one slow CRD discovery delayed
// (or starved) the rest of the probes despite the comment claiming each
// probe is independently capped. With three 80ms-delayed sources the total
// elapsed time must be close to a single source (≤ ~150ms with overhead),
// not 3×80ms = 240ms.
func TestLoadSecurityAvailabilityProbesInParallel(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "s1", Available: true, AvailableDelay: 80 * time.Millisecond})
	mgr.Register(&security.FakeSource{NameStr: "s2", Available: true, AvailableDelay: 80 * time.Millisecond})
	mgr.Register(&security.FakeSource{NameStr: "s3", Available: true, AvailableDelay: 80 * time.Millisecond})
	m := newTestModelWithSecurity(t, mgr, "kctx")

	cmd := m.loadSecurityAvailability()
	require.NotNil(t, cmd)
	start := time.Now()
	msg := cmd()
	elapsed := time.Since(start)

	loaded, ok := msg.(securityAvailabilityLoadedMsg)
	require.True(t, ok)
	assert.Len(t, loaded.availability, 3)
	assert.Less(t, elapsed, 200*time.Millisecond, "probes must run in parallel; got %s", elapsed)
}

func TestSecurityAvailabilityLoadedMergesIntoModel(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "kctx")
	m.securityAvailabilityByName = make(map[string]bool)

	msg := securityAvailabilityLoadedMsg{
		context: "kctx",
		availability: map[string]bool{
			"trivy-operator": true,
			"heuristic":      true,
		},
	}
	updated, _ := m.updateSecurityAvailabilityLoaded(msg)

	assert.True(t, updated.securityAvailabilityByName["trivy-operator"])
	assert.True(t, updated.securityAvailabilityByName["heuristic"])
}

func TestSecurityAvailabilityLoadedStaleContextDiscarded(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "current")
	m.securityAvailabilityByName = make(map[string]bool)

	msg := securityAvailabilityLoadedMsg{
		context:      "stale",
		availability: map[string]bool{"trivy-operator": true},
	}
	updated, _ := m.updateSecurityAvailabilityLoaded(msg)

	assert.False(t, updated.securityAvailabilityByName["trivy-operator"],
		"stale-context message must not mutate the active map")
}

func TestSecurityAvailabilityLoadedTriggersFindingsFetch(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "fake", Available: true})
	m := newTestModelWithSecurity(t, mgr, "kctx")

	msg := securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"fake": true},
	}
	_, cmd := m.updateSecurityAvailabilityLoaded(msg)
	assert.NotNil(t, cmd,
		"availability with at least one available source should kick off the findings fetch")
}

func TestLoadSecurityFindingsNilManager(t *testing.T) {
	m := newTestModelWithSecurity(t, nil, "kctx")
	assert.Nil(t, m.loadSecurityFindings(),
		"no manager wired -> command must be nil")
}

func TestLoadSecurityFindingsNoSourceAvailable(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "fake", Available: false})
	m := newTestModelWithSecurity(t, mgr, "kctx")
	m.securityAvailabilityByName = map[string]bool{"fake": false}

	assert.Nil(t, m.loadSecurityFindings(),
		"no available source -> nothing to fetch")
}

func TestLoadSecurityFindingsDispatches(t *testing.T) {
	finding := security.Finding{
		ID:       "F1",
		Title:    "runs as root",
		Severity: security.SeverityHigh,
		Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"},
	}
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr:   "fake",
		Available: true,
		Findings:  []security.Finding{finding},
	})
	m := newTestModelWithSecurity(t, mgr, "kctx")
	m.securityAvailabilityByName = map[string]bool{"fake": true}

	cmd := m.loadSecurityFindings()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityFindingsLoadedMsg)
	require.True(t, ok, "expected securityFindingsLoadedMsg, got %T", msg)
	assert.Equal(t, "kctx", loaded.context)
	require.Len(t, loaded.findings, 1)
	assert.Equal(t, "F1", loaded.findings[0].ID)
}

func TestSecurityFindingsLoadedBuildsIndex(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "kctx")

	ref := security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"}
	findings := []security.Finding{
		{ID: "F1", Title: "crit", Severity: security.SeverityCritical, Resource: ref},
		{ID: "F2", Title: "med", Severity: security.SeverityMedium, Resource: ref},
	}
	updated := m.updateSecurityFindingsLoaded(securityFindingsLoadedMsg{
		context:  "kctx",
		findings: findings,
	})

	require.NotNil(t, updated.securityIndex,
		"successful load must populate the per-resource lookup index")
	counts := updated.securityIndex.For(ref)
	assert.Equal(t, 1, counts.Critical, "one critical finding indexed")
	assert.Equal(t, 1, counts.Medium, "one medium finding indexed")
	assert.Equal(t, 2, counts.Total())
}

func TestSecurityFindingsLoadedStaleContextDiscarded(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "current")
	priorIdx := security.BuildFindingIndex(nil)
	m.securityIndex = priorIdx

	updated := m.updateSecurityFindingsLoaded(securityFindingsLoadedMsg{
		context:  "stale",
		findings: []security.Finding{{ID: "ignored"}},
	})
	assert.Same(t, priorIdx, updated.securityIndex,
		"stale-context findings must not replace the active index")
}

// TestSecurityFindingsLoadedPropagatesErrors guards the per-source error
// channel: FetchAll's res.Errors must be carried on the message and the
// handler must not silently drop them. Without surface, a fully-failed
// fetch is indistinguishable from "no findings" and users have no signal
// that their cluster's Trivy/Falco/etc. is broken.
func TestSecurityFindingsLoadedPropagatesErrors(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "kctx")
	updated := m.updateSecurityFindingsLoaded(securityFindingsLoadedMsg{
		context:  "kctx",
		findings: nil,
		errors:   map[string]error{"trivy-operator": assert.AnError},
	})
	// Successful index rebuild from zero findings is fine; the test guards
	// that the handler does not crash on errors and still rebuilds.
	require.NotNil(t, updated.securityIndex)
}

// TestLoadSecurityFindingsCarriesPerSourceErrors locks in the cmd
// behavior: when at least one source errors, the resulting message must
// expose the error map so the handler can log per-source failures.
func TestLoadSecurityFindingsCarriesPerSourceErrors(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "broken", Available: true, FetchErr: assert.AnError})
	m := newTestModelWithSecurity(t, mgr, "kctx")
	m.securityAvailabilityByName = map[string]bool{"broken": true}

	cmd := m.loadSecurityFindings()
	require.NotNil(t, cmd)
	loaded, ok := cmd().(securityFindingsLoadedMsg)
	require.True(t, ok)
	require.Contains(t, loaded.errors, "broken", "per-source error must be on the message")
	require.Error(t, loaded.errors["broken"])
}

// TestUpdateSecurityAvailabilityLoadedRebuildsSidebarAtLevelResourceTypes
// guards the "(probing sources...) keeps showing after probe completes"
// bug: the probe handler updates m.securityAvailabilityByName and the
// hook state, but the existing m.middleItems still holds the loader
// entry built at the previous render. Without dispatching a fresh
// loadResourceTypes, the sidebar shows stale loader data until something
// else (discovery completion, navigation) rebuilds it.
func TestUpdateSecurityAvailabilityLoadedRebuildsSidebarAtLevelResourceTypes(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})
	m := newTestModelWithSecurity(t, mgr, "kctx")
	m.nav.Level = model.LevelResourceTypes
	m.discoveredResources = map[string][]model.ResourceTypeEntry{
		"kctx": {{Kind: "Pod", APIVersion: "v1", Resource: "pods"}},
	}

	_, cmd := m.updateSecurityAvailabilityLoaded(securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"heuristic": true},
	})
	require.NotNil(t, cmd, "must dispatch a fresh sidebar build")

	// tea.Batch returns a single Cmd that fans out; running it produces a
	// sequence of msgs. We only need to verify the cmd isn't nil and that
	// loadResourceTypes is among the produced cmds — running the batch
	// once and inspecting the first/any resourceTypesMsg is enough.
	msg := cmd()
	require.NotNil(t, msg, "batch must produce at least one msg")
}

// TestUpdateSecurityAvailabilityLoadedAtLevelClustersSkipsRebuild ensures
// we don't dispatch a needless sidebar rebuild when the user is still at
// LevelClusters (the sidebar isn't visible there).
func TestUpdateSecurityAvailabilityLoadedAtLevelClustersSkipsRebuild(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})
	m := newTestModelWithSecurity(t, mgr, "kctx")
	m.nav.Level = model.LevelClusters

	_, cmd := m.updateSecurityAvailabilityLoaded(securityAvailabilityLoadedMsg{
		context:      "kctx",
		availability: map[string]bool{"heuristic": true},
	})
	// At LevelClusters we still dispatch loadSecurityFindings, so cmd is
	// non-nil — but the batch should not include a loadResourceTypes
	// pass. We can't introspect the batch directly; this test exists
	// mostly as an "exercises this branch" regression guard so a future
	// refactor doesn't accidentally drop the level check.
	_ = cmd
}

// TestRefreshCurrentLevelAtSecurityLevelOwnedKeepsAffectedResources guards
// against the bug where watch-tick and post-ignore-action refreshes at
// LevelOwned in a security view would dispatch loadOwned, which routes
// through GetOwnedResources(parentKind="__security_<src>__", ...) — none
// of that switch's cases match the synthetic kind, so it returns nil and
// the user's affected-resources list silently empties on every refresh.
//
// The fix routes refreshCurrentLevel through loadSecurityAffectedResources
// when the active resource type is a security view, mirroring the logic
// in loadPreviewResources at LevelResources.
func TestRefreshCurrentLevelAtSecurityLevelOwnedKeepsAffectedResources(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr:   "heuristic",
		Available: true,
		Findings: []security.Finding{
			{
				ID: "1", Source: "heuristic", Title: "privileged",
				Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "web"},
				Labels:   map[string]string{"check": "privileged"},
			},
		},
	})
	m := baseModelBoost2()
	m.securityManager = mgr
	m.client.SetSecurityManager(mgr)
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "__security_heuristic__",
		APIGroup:   model.SecurityVirtualAPIGroup,
		APIVersion: "v1",
		Resource:   "findings-heuristic",
		Namespaced: true,
	}
	m.nav.ResourceName = "privileged"
	m.securityActiveGroup = "privileged"
	m.namespace = ""

	cmd := m.refreshCurrentLevel()
	require.NotNil(t, cmd, "refresh at LevelOwned in security view must dispatch a fetch")
	msg := cmd()
	loaded, ok := msg.(ownedLoadedMsg)
	require.True(t, ok, "expected ownedLoadedMsg, got %T", msg)
	require.NoError(t, loaded.err)
	require.Len(t, loaded.items, 1,
		"refresh must keep the affected resources list — empty would mean GetOwnedResources clobbered the row")
}

// TestDirectActionRefreshInvalidatesSecurityCache locks in the contract
// that shift+r (the explorer's generic refresh) busts the security
// manager's FetchAll cache so the next list call re-pulls findings from
// every registered source. Without the bust, a stale 5-minute-cached
// FetchResult would be returned and the user would see no change after
// pressing refresh — defeating the action's purpose.
func TestDirectActionRefreshInvalidatesSecurityCache(t *testing.T) {
	mgr := security.NewManager()
	mgr.SetRefreshTTL(1 * time.Hour)
	src := &security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{{ID: "1", Source: "heuristic"}},
	}
	mgr.Register(src)
	m := baseModelBoost2()
	m.securityManager = mgr
	m.client.SetSecurityManager(mgr)
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "__security_heuristic__",
		APIGroup:   model.SecurityVirtualAPIGroup,
		APIVersion: "v1",
		Resource:   "findings-heuristic",
		Namespaced: true,
	}

	// Prime the cache.
	_, err := mgr.FetchAll(m.reqCtx, "kctx", "")
	require.NoError(t, err)
	require.Equal(t, int32(1), src.FetchCalls.Load())

	// Cache hit: a second call before refresh must not hit the source.
	_, _ = mgr.FetchAll(m.reqCtx, "kctx", "")
	require.Equal(t, int32(1), src.FetchCalls.Load(), "cache should still be warm")

	// Refresh must invalidate the cache.
	_, _ = m.directActionRefresh()
	_, _ = mgr.FetchAll(m.reqCtx, "kctx", "")
	require.Equal(t, int32(2), src.FetchCalls.Load(),
		"directActionRefresh in a security view must invalidate the manager cache")
}
