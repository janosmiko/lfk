package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/k8s/localcluster"
	"github.com/janosmiko/lfk/internal/model"
)

func TestNewModel_LocalClusterStateInitialized(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	if m.localClusterState.screen != localClusterScreenList {
		t.Fatalf("expected default screen=list, got %v", m.localClusterState.screen)
	}
	if m.localClusterState.gen != 0 {
		t.Fatalf("expected gen=0, got %d", m.localClusterState.gen)
	}
}

func TestNewModel_LocalClusterCacheLoadsEmpty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	if m.localClusterCache == nil {
		t.Fatal("localClusterCache must be non-nil even when state file missing")
	}
}

func TestOverlayLocalClustersEnumExists(t *testing.T) {
	k := overlayLocalClusters
	_ = k
}

func TestLocalClusterScreenEnumValues_DistinctIotaValues(t *testing.T) {
	// Guard the iota mapping: each sub-screen must be its own value
	// (no shared / overlapping). A regression here would make the
	// dispatcher's switch route the wrong sub-screen for the same
	// keypress.
	seen := map[localClusterScreen]string{}
	for _, kv := range []struct {
		s    localClusterScreen
		name string
	}{
		{localClusterScreenList, "List"},
		{localClusterScreenWizardProvider, "WizardProvider"},
		{localClusterScreenWizardName, "WizardName"},
		{localClusterScreenWizardVersion, "WizardVersion"},
		{localClusterScreenWizardNodes, "WizardNodes"},
		{localClusterScreenWizardConfirm, "WizardConfirm"},
		{localClusterScreenDeleteConfirm, "DeleteConfirm"},
	} {
		if other, dup := seen[kv.s]; dup {
			t.Fatalf("screen value collision: %s and %s both = %d", kv.name, other, kv.s)
		}
		seen[kv.s] = kv.name
	}
}

func TestCtrlN_AtLevelClusters_OpensOverlay(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	m.nav.Level = model.LevelClusters
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	rm := updated.(Model)
	if rm.overlay != overlayLocalClusters {
		t.Fatalf("overlay = %v, want overlayLocalClusters", rm.overlay)
	}
	if rm.localClusterState.screen != localClusterScreenList {
		t.Fatalf("screen = %v, want list", rm.localClusterState.screen)
	}
	if rm.localClusterState.gen == 0 {
		t.Fatal("gen should have been bumped")
	}
}

func TestCtrlN_NotAtLevelClusters_Ignored(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	m.nav.Level = model.LevelResourceTypes
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	rm := updated.(Model)
	if rm.overlay == overlayLocalClusters {
		t.Fatal("Ctrl+N must be ignored away from LevelClusters")
	}
}

func TestLocalClustersDetectedMsg_StaleGenIgnored(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	m.overlay = overlayLocalClusters
	m.localClusterState.gen = 5
	stale := localClustersDetectedMsg{gen: 3, clusters: []localcluster.Cluster{{Name: "x"}}}
	updated, _ := m.Update(stale)
	rm := updated.(Model)
	if len(rm.localClusterState.clusters) != 0 {
		t.Fatal("stale gen msg must not populate state")
	}
}

func TestLocalClustersDetectedMsg_FreshGenApplied(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	m.overlay = overlayLocalClusters
	m.localClusterState.gen = 5
	fresh := localClustersDetectedMsg{
		gen: 5,
		clusters: []localcluster.Cluster{
			{Provider: "kind", Name: "dev", ContextName: "kind-dev", Status: localcluster.ClusterStatusRunning},
		},
	}
	updated, _ := m.Update(fresh)
	rm := updated.(Model)
	if len(rm.localClusterState.clusters) != 1 || rm.localClusterState.clusters[0].Name != "dev" {
		t.Fatalf("clusters = %+v", rm.localClusterState.clusters)
	}
}

func TestEsc_ClosesOverlay(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	m.overlay = overlayLocalClusters
	m.localClusterState.screen = localClusterScreenList
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm := updated.(Model)
	if rm.overlay == overlayLocalClusters {
		t.Fatal("Esc on list view should close overlay")
	}
}

func TestLocalClustersDetectedMsg_PropagatesProviderErrors(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	m.overlay = overlayLocalClusters
	m.localClusterState.gen = 1
	msg := localClustersDetectedMsg{
		gen: 1,
		clusters: []localcluster.Cluster{
			{Provider: "kind", Name: "dev", ContextName: "kind-dev", Status: localcluster.ClusterStatusRunning},
		},
		providerErrors: map[string]string{
			"kind": "kind warning",    // kind has rows -> row-level ListError
			"k3d":  "k3d list failed", // k3d has no rows -> global err
		},
	}
	updated, _ := m.Update(msg)
	rm := updated.(Model)
	if rm.localClusterState.clusters[0].ListError != "kind warning" {
		t.Fatalf("kind row ListError = %q, want %q", rm.localClusterState.clusters[0].ListError, "kind warning")
	}
	if rm.localClusterState.err != "k3d: k3d list failed" {
		t.Fatalf("global err = %q, want 'k3d: k3d list failed'", rm.localClusterState.err)
	}
}

func TestCtrlN_TogglesOverlayClosed(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	m.nav.Level = model.LevelClusters

	// Open
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	rm := updated.(Model)
	if rm.overlay != overlayLocalClusters {
		t.Fatalf("first Ctrl+N must open overlay, got %v", rm.overlay)
	}

	// Press Ctrl+N again -- must close (not re-open)
	updated2, _ := rm.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	rm2 := updated2.(Model)
	if rm2.overlay == overlayLocalClusters {
		t.Fatal("second Ctrl+N must close overlay (toggle), got still-open")
	}
}

func TestList_SKey_DispatchesStart(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{
		{Provider: "k3d", Name: "staging", Status: "stopped"},
	}
	m.localClusterState.cursor = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	rm := updated.(Model)
	if cmd == nil {
		t.Fatal("s key must return a tea.Cmd for k3d")
	}
	if rm.localClusterState.clusters[0].Mutating != "starting..." {
		t.Fatalf("Mutating = %q, want starting...", rm.localClusterState.clusters[0].Mutating)
	}
}

func TestList_SKey_IgnoredForKind(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{
		{Provider: "kind", Name: "dev", Status: "running"},
	}
	m.localClusterState.cursor = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd != nil {
		t.Fatal("s key on kind row must be ignored (no cmd)")
	}
}

func TestList_CapitalSKey_DispatchesStop(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{
		{Provider: "minikube", Name: "exp", Status: "running"},
	}
	m.localClusterState.cursor = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd == nil {
		t.Fatal("S key must return a cmd for minikube")
	}
}

func TestList_DoubleStop_GuardedByMutatingFlag(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{
		{Provider: "k3d", Name: "staging", Status: "running", Mutating: "stopping..."},
	}
	// Capital S triggers stop now; the mutating flag should still suppress
	// a second dispatch.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if cmd != nil {
		t.Fatal("s on a row already mutating must be ignored")
	}
}

func TestUpdateMutatedMsg_ClearsPillAndRefreshes(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{
		{Provider: "k3d", Name: "staging", Status: "running", Mutating: "stopping..."},
	}
	updated, cmd := m.Update(localClusterMutatedMsg{provider: "k3d", name: "staging", verb: "stopped"})
	rm := updated.(Model)
	if rm.localClusterState.clusters[0].Mutating != "" {
		t.Fatalf("Mutating must be cleared, got %q", rm.localClusterState.clusters[0].Mutating)
	}
	if cmd == nil {
		t.Fatal("Mutated handler must return a Detect cmd")
	}
}

func TestList_DKey_OpensDeleteConfirm(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{{Provider: "kind", Name: "dev"}}
	m.localClusterState.cursor = 0

	// Lowercase d is the global Diff key — it must NOT open delete-confirm.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	rm := updated.(Model)
	if rm.localClusterState.screen == localClusterScreenDeleteConfirm {
		t.Fatal("lowercase d must not open delete-confirm; it's reserved for global Diff")
	}

	// Shift+D opens the delete-confirm sub-screen.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	rm = updated.(Model)
	if rm.localClusterState.screen != localClusterScreenDeleteConfirm {
		t.Fatalf("screen = %v, want DeleteConfirm", rm.localClusterState.screen)
	}
	if rm.localClusterState.deleteRow != 0 {
		t.Fatal("deleteRow must point at cursor")
	}
}

func TestList_QKey_ClosesOverlay(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{{Provider: "kind", Name: "dev"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	rm := updated.(Model)
	if rm.overlay == overlayLocalClusters {
		t.Fatal("q must close the local-cluster manager overlay")
	}
}

func TestDeleteConfirm_TypingBuildsBuffer(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenDeleteConfirm

	for _, r := range "DEL" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.localClusterState.deleteBuf != "DEL" {
		t.Fatalf("deleteBuf = %q, want DEL", m.localClusterState.deleteBuf)
	}
}

func TestDeleteConfirm_EnterBlocksUntilFullDelete(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenDeleteConfirm
	m.localClusterState.deleteBuf = "DEL"
	m.localClusterState.clusters = []localClusterRow{{Provider: "kind", Name: "dev"}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if cmd != nil {
		t.Fatal("partial DELETE must not dispatch")
	}
	if rm.localClusterState.screen != localClusterScreenDeleteConfirm {
		t.Fatal("partial DELETE must stay on confirm screen")
	}
}

func TestDeleteConfirm_FullDeleteDispatches(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenDeleteConfirm
	m.localClusterState.deleteBuf = "DELETE"
	m.localClusterState.clusters = []localClusterRow{{Provider: "kind", Name: "dev"}}
	m.localClusterState.deleteRow = 0

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if cmd == nil {
		t.Fatal("full DELETE must dispatch a cmd")
	}
	if rm.localClusterState.screen != localClusterScreenList {
		t.Fatal("full DELETE must return to list view")
	}
	if rm.localClusterState.clusters[0].Mutating != "deleting..." {
		t.Fatalf("Mutating = %q, want deleting...", rm.localClusterState.clusters[0].Mutating)
	}
}

func TestDeleteConfirm_EscReturnsToList(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenDeleteConfirm
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenList {
		t.Fatal("Esc on confirm must return to list")
	}
}

func TestList_EnterSwitches(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{
		{Provider: "kind", Name: "dev", ContextName: "kind-dev", Status: "running"},
	}
	m.localClusterState.cursor = 0
	m.middleItems = []model.Item{
		{Name: "kind-dev"},
		{Name: "gke-prod"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.overlay != overlayNone {
		t.Fatal("Enter on a row must close the overlay")
	}
	if rm.cursors[model.LevelClusters] != 0 {
		t.Fatalf("cluster picker cursor = %d, want 0 (kind-dev)", rm.cursors[model.LevelClusters])
	}
}

func TestList_EnterSwitches_PositionsToMatchingContext(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{
		{Provider: "kind", Name: "dev", ContextName: "kind-dev"},
		{Provider: "k3d", Name: "staging", ContextName: "k3d-staging"},
	}
	m.localClusterState.cursor = 1
	m.middleItems = []model.Item{
		{Name: "kind-dev"},
		{Name: "gke-prod"},
		{Name: "k3d-staging"},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.overlay != overlayNone {
		t.Fatal("Enter must close overlay")
	}
	if rm.cursors[model.LevelClusters] != 2 {
		t.Fatalf("cursor = %d, want 2 (k3d-staging at index 2)", rm.cursors[model.LevelClusters])
	}
}

func TestList_EnterOnEmpty_NoOp(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = nil
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("Enter on empty list must be a no-op")
	}
	if updated.(Model).overlay != overlayLocalClusters {
		t.Fatal("Enter on empty list must keep overlay open")
	}
}
