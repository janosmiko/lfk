package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// podPermModel builds a Model whose action target is a Pod in ns/kind-ctx,
// with the given verdicts already cached for that scope.
func podPermModel(t *testing.T, allowed map[string]bool) Model {
	t.Helper()
	m := Model{tabs: []TabState{{}}, width: 80, height: 40}
	m.perms = newPermissionState()
	m.itemCache = make(map[string][]model.Item)
	m.cursorMemory = make(map[string]int)
	m.actionCtx = actionContext{kind: "Pod", name: "p", namespace: "ns", context: "kind-ctx"}
	if allowed != nil {
		m.perms.record(podScope, allowed)
	}
	return m
}

// podScope is the cache key podPermModel files its verdicts under.
var podScope = permScopeKey{context: "kind-ctx", namespace: "ns", kind: "Pod"}

func TestDeniedByRBAC_HidesRefusedAction(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	assert.True(t, m.deniedByRBAC("Pod", "Delete"))
	assert.True(t, m.deniedByRBAC("Pod", "Force Delete"))
}

func TestDeniedByRBAC_KeepsAllowedAction(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": true})
	assert.False(t, m.deniedByRBAC("Pod", "Delete"))
}

func TestDeniedByRBAC_FailsOpen(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Model)
		kind    string
		label   string
		allowed map[string]bool
	}{
		{name: "no review yet", kind: "Pod", label: "Delete"},
		{
			name:    "verb not in the reviewed set",
			kind:    "Pod",
			label:   "Delete",
			allowed: map[string]bool{"get:pods/log": true},
		},
		{
			name:    "action has no mapped verb",
			kind:    "Pod",
			label:   "Describe",
			allowed: map[string]bool{"delete:pods": false},
		},
		{
			name:    "kind with no verb map",
			kind:    "ConfigMap",
			label:   "Delete",
			allowed: map[string]bool{"delete:pods": false},
		},
		{
			name:    "union sentinel spans clusters",
			kind:    "Pod",
			label:   "Delete",
			allowed: map[string]bool{"delete:pods": false},
			mutate:  func(m *Model) { m.actionCtx.context = UnionContextSentinel },
		},
		{
			name:    "no namespace to review",
			kind:    "Pod",
			label:   "Delete",
			allowed: map[string]bool{"delete:pods": false},
			mutate:  func(m *Model) { m.actionCtx.namespace = "" },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := podPermModel(t, tc.allowed)
			if tc.mutate != nil {
				tc.mutate(&m)
			}
			assert.False(t, m.deniedByRBAC(tc.kind, tc.label))
		})
	}
}

func TestDeniedByRBAC_DoesNotLeakAcrossContexts(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	require.True(t, m.deniedByRBAC("Pod", "Delete"))

	// A tab on another cluster asks the same question and gets no verdict.
	m.actionCtx.context = "other-ctx"
	assert.False(t, m.deniedByRBAC("Pod", "Delete"))
}

func TestDeniedByRBAC_DoesNotLeakAcrossNamespaces(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	m.actionCtx.namespace = "other-ns"
	assert.False(t, m.deniedByRBAC("Pod", "Delete"))
}

func TestActionBlockedReason_ReadOnlyWins(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	m.readOnly = true
	reason, blocked := m.actionBlockedReason("Pod", "Delete")
	assert.True(t, blocked)
	assert.Equal(t, readOnlyBlockedMessage("Delete"), reason)
}

func TestActionBlockedReason_RBACMessage(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	reason, blocked := m.actionBlockedReason("Pod", "Delete")
	assert.True(t, blocked)
	assert.Equal(t, rbacBlockedMessage("Delete"), reason)
}

func TestActionBlockedReason_AllowsWhatIsPermitted(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": true})
	_, blocked := m.actionBlockedReason("Pod", "Delete")
	assert.False(t, blocked)
}

func TestExecuteAction_RBACDenied_Blocks(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	ret, _ := m.executeAction("Delete")
	result := ret.(Model)
	assert.Equal(t, rbacBlockedMessage("Delete"), result.statusMessage)
	assert.True(t, result.statusMessageErr)
	assert.NotEqual(t, overlayConfirm, result.overlay)
}

func TestOpenResourceActionMenu_DropsRefusedEntries(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false, "create:pods/exec": false})
	m.nav = model.NavigationState{
		Level:        model.LevelResources,
		Context:      "kind-ctx",
		Namespace:    "ns",
		ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"},
	}
	m.setMiddleItems([]model.Item{{Name: "p", Namespace: "ns", Kind: "Pod"}})

	out := m.openResourceActionMenu()

	labels := make(map[string]bool, len(out.overlayItems))
	for _, item := range out.overlayItems {
		labels[item.Name] = true
	}
	assert.False(t, labels["Delete"], "a refused delete must not be offered")
	assert.False(t, labels["Exec"], "a refused exec must not be offered")
	assert.False(t, labels["Force Delete"], "force delete needs the same delete verb")
	assert.True(t, labels["Describe"], "read-only actions stay")
	assert.True(t, labels["Events"], "an action with no mapped verb stays")
}

func TestPermissionState_BeginRunsOnePassPerScope(t *testing.T) {
	s := newPermissionState()
	key := podScope

	assert.True(t, s.begin(key), "first entry starts the pass")
	assert.False(t, s.begin(key), "a pass already in flight is not repeated")

	s.record(key, map[string]bool{"delete:pods": true})
	assert.False(t, s.begin(key), "a cached answer needs no second pass")

	other := permScopeKey{context: "kind-ctx", namespace: "other"}
	assert.True(t, s.begin(other), "another namespace is its own pass")
}

func TestPermissionState_RetriesAPassTheSchedulerDropped(t *testing.T) {
	// A low-priority pass is superseded when the user leaves the namespace,
	// and the scheduler drops it without a reply. The scope must not stay
	// marked as running, or the menu would never hide anything again.
	now := time.Now()
	permNow = func() time.Time { return now }
	t.Cleanup(func() { permNow = time.Now })

	s := newPermissionState()
	key := podScope
	require.True(t, s.begin(key))
	require.False(t, s.begin(key), "a pass started a moment ago is not repeated")

	now = now.Add(permissionRetryAfter + time.Second)
	assert.True(t, s.begin(key), "a pass with no reply is retried")
}

func TestPermissionState_FailReleasesTheScope(t *testing.T) {
	s := newPermissionState()
	key := podScope
	require.True(t, s.begin(key))

	s.fail(key)
	assert.Empty(t, s.allowed, "a failed review stores no verdict")
	assert.True(t, s.begin(key), "the next visit may retry")
}

func TestLoadActionPermissions_SkipsListsWithNoSingleScope(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Model)
	}{
		{name: "all namespaces", mutate: func(m *Model) { m.allNamespaces = true }},
		{
			name: "several namespaces selected",
			mutate: func(m *Model) {
				m.selectedNamespaces = map[string]bool{"ns": true, "other": true}
			},
		},
		{
			name: "union list spans clusters",
			mutate: func(m *Model) {
				m.unionMode = true
				m.nav.Context = UnionContextSentinel
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := podPermModel(t, nil)
			m.namespace = "ns"
			m.nav.Context = "kind-ctx"
			tc.mutate(&m)

			assert.Nil(t, m.loadActionPermissions("Pod"), "no pass for a list with no single scope")
			assert.Empty(t, m.perms.inflight, "and no scope is marked as under review")
		})
	}
}

func TestUpdateContextsLoaded_ReloadDropsCachedVerdicts(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	require.True(t, m.deniedByRBAC("Pod", "Delete"))

	ret, _ := m.updateContextsLoaded(contextsLoadedMsg{reloaded: true})
	result := ret.(Model)
	assert.False(t, result.deniedByRBAC("Pod", "Delete"),
		"a re-read kubeconfig may point the context at another identity")
}

func TestUpdateContextsLoaded_PlainListKeepsVerdicts(t *testing.T) {
	m := podPermModel(t, map[string]bool{"delete:pods": false})

	ret, _ := m.updateContextsLoaded(contextsLoadedMsg{})
	result := ret.(Model)
	assert.True(t, result.deniedByRBAC("Pod", "Delete"))
}

func TestUpdateActionPermissions_StoresVerdicts(t *testing.T) {
	m := podPermModel(t, nil)
	key := podScope
	require.True(t, m.perms.begin(key))

	ret := m.updateActionPermissions(actionPermissionsMsg{
		scope:   key,
		allowed: map[string]bool{"delete:pods": false},
	})
	result := ret.(Model)
	assert.True(t, result.deniedByRBAC("Pod", "Delete"))
}

func TestUpdateActionPermissions_ErrorFailsOpenSilently(t *testing.T) {
	m := podPermModel(t, nil)
	key := podScope
	require.True(t, m.perms.begin(key))

	ret := m.updateActionPermissions(actionPermissionsMsg{scope: key, err: assert.AnError})
	result := ret.(Model)
	assert.False(t, result.deniedByRBAC("Pod", "Delete"))
	assert.Empty(t, result.statusMessage, "a review the user did not ask for stays quiet")
}

func TestActionQueries_LabelsExistInTheirMenu(t *testing.T) {
	for kind, byLabel := range actionQueries {
		t.Run(kind, func(t *testing.T) {
			menu := make(map[string]bool)
			for _, a := range model.ActionsForKind(kind) {
				menu[a.Label] = true
			}
			for label := range byLabel {
				assert.True(t, menu[label], "mapped label %q is not in the %s action menu", label, kind)
			}
		})
	}
}

func TestPermissionQueriesFor_CoverEveryMappedAction(t *testing.T) {
	for kind, byLabel := range actionQueries {
		t.Run(kind, func(t *testing.T) {
			keys := make(map[string]bool)
			for _, q := range permissionQueriesFor(kind) {
				keys[q.Key()] = true
			}
			for label, q := range byLabel {
				assert.True(t, keys[q.Key()], "no review is asked for %q", label)
			}
		})
	}
}

func TestPermissionQueriesFor_UnmappedKindAsksNothing(t *testing.T) {
	assert.Nil(t, permissionQueriesFor("ConfigMap"))
}

func TestDeniedByRBAC_WorkloadVerbs(t *testing.T) {
	tests := []struct {
		kind    string
		label   string
		allowed map[string]bool
	}{
		{kind: "Deployment", label: "Scale", allowed: map[string]bool{"update:deployments/scale": false}},
		{kind: "Deployment", label: "Restart", allowed: map[string]bool{"patch:deployments": false}},
		{kind: "Deployment", label: "Rollback", allowed: map[string]bool{"patch:deployments": false}},
		{kind: "Deployment", label: "Delete", allowed: map[string]bool{"delete:deployments": false}},
		{kind: "StatefulSet", label: "Scale", allowed: map[string]bool{"update:statefulsets/scale": false}},
		{kind: "DaemonSet", label: "Restart", allowed: map[string]bool{"patch:daemonsets": false}},
		{kind: "ReplicaSet", label: "Delete", allowed: map[string]bool{"delete:replicasets": false}},
		{kind: "Deployment", label: "Exec", allowed: map[string]bool{"create:pods/exec": false}},
	}
	for _, tc := range tests {
		t.Run(tc.kind+"/"+tc.label, func(t *testing.T) {
			m := podPermModel(t, nil)
			m.actionCtx.kind = tc.kind
			m.perms.record(permScopeKey{context: "kind-ctx", namespace: "ns", kind: tc.kind}, tc.allowed)
			assert.True(t, m.deniedByRBAC(tc.kind, tc.label))
		})
	}
}

func TestDeniedByRBAC_VerdictsDoNotCrossKinds(t *testing.T) {
	// The Pod pass and the Deployment pass ask different questions, so a
	// Pod verdict must not answer for a Deployment menu.
	m := podPermModel(t, map[string]bool{"delete:pods": false})
	m.actionCtx.kind = "Deployment"
	assert.False(t, m.deniedByRBAC("Deployment", "Delete"))
}

func TestLoadActionPermissions_UnmappedKindMakesNoCall(t *testing.T) {
	m := podPermModel(t, nil)
	m.namespace = "ns"
	m.nav.Context = "kind-ctx"

	assert.Nil(t, m.loadActionPermissions("ConfigMap"))
	assert.Empty(t, m.perms.inflight)
}
