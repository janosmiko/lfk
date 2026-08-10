package app

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// assertDependentsOnlyCmd checks that a confirm-opening handler returned the
// dependent-count fetch and nothing that acts on the cluster.
func assertDependentsOnlyCmd(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	if _, ok := cmd().(dependentsLoadedMsg); !ok {
		t.Fatalf("expected a dependents fetch, got %T", cmd())
	}
}

func rawWithUID(uid string) map[string]any {
	return map[string]any{"metadata": map[string]any{"uid": uid}}
}

func TestUIDFrom(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{"present", rawWithUID("abc"), "abc"},
		{"no metadata", map[string]any{}, ""},
		{"metadata is not a map", map[string]any{"metadata": "nope"}, ""},
		{"uid is not a string", map[string]any{"metadata": map[string]any{"uid": 7}}, ""},
		{"nil raw", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uidFrom(tt.raw); got != tt.want {
				t.Errorf("uidFrom() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBulkDependentTargets(t *testing.T) {
	items := []model.Item{
		{Kind: "Deployment", Namespace: "a", Raw: rawWithUID("d1")},
		{Kind: "Deployment", Namespace: "a", Raw: rawWithUID("d2")},
		{Kind: "CronJob", Namespace: "b", Raw: rawWithUID("c1")},
		// Unwalkable rows, each for a different reason.
		{Kind: "ConfigMap", Namespace: "a", Raw: rawWithUID("cm1")},
		{Kind: "Deployment", Namespace: "a"},
		{Kind: "Deployment", Raw: rawWithUID("d3")},
	}

	byNS, uncounted := bulkDependentTargets(items)

	if uncounted != 3 {
		t.Errorf("uncounted = %d, want 3", uncounted)
	}
	if len(byNS) != 2 {
		t.Fatalf("namespaces = %d, want 2", len(byNS))
	}
	if got := byNS["a"].uids; len(got) != 2 {
		t.Errorf("namespace a uids = %v, want 2", got)
	}
	if got := byNS["b"].kinds; len(got) != 1 || got[0] != "CronJob" {
		t.Errorf("namespace b kinds = %v, want [CronJob]", got)
	}
}

func TestBulkDependentCount(t *testing.T) {
	byNS := map[string]*dependentTargets{
		"a": {uids: []string{"d1"}, kinds: []string{"Deployment"}},
		"b": {uids: []string{"j1"}, kinds: []string{"Job"}},
	}
	refs := map[string][]k8s.DependentRef{
		"a": {
			{Kind: "ReplicaSet", UID: "rs1", OwnerUIDs: []string{"d1"}},
			{Kind: "Pod", UID: "p1", OwnerUIDs: []string{"rs1"}},
		},
		"b": {{Kind: "Pod", UID: "p2", OwnerUIDs: []string{"j1"}}},
	}

	got, err := bulkDependentCount(byNS, 1, func(ns string, _ []k8s.DependentKind) ([]k8s.DependentRef, error) {
		return refs[ns], nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Total != 3 {
		t.Errorf("Total = %d, want 3", got.Total)
	}
	if got.ByKind["Pod"] != 2 || got.ByKind["ReplicaSet"] != 1 {
		t.Errorf("ByKind = %v, want 2 pods and 1 replicaset", got.ByKind)
	}
	if got.Uncounted != 1 {
		t.Errorf("Uncounted = %d, want 1", got.Uncounted)
	}
}

func TestBulkDependentCountPropagatesError(t *testing.T) {
	byNS := map[string]*dependentTargets{
		"a": {uids: []string{"d1"}, kinds: []string{"Deployment"}},
	}
	want := errors.New("listing failed")
	if _, err := bulkDependentCount(byNS, 0, func(string, []k8s.DependentKind) ([]k8s.DependentRef, error) {
		return nil, want
	}); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestLoadDependentsAnswersWithoutATarget(t *testing.T) {
	// A kind with no known children must still answer, or the dialog reads
	// "counting..." forever.
	m := Model{}
	m.actionCtx.kind = "ConfigMap"
	m.actionCtx.namespace = "default"
	m.actionCtx.raw = rawWithUID("cm1")

	msg, ok := m.loadDependents()().(dependentsLoadedMsg)
	if !ok {
		t.Fatalf("expected dependentsLoadedMsg")
	}
	if !errors.Is(msg.err, errNoDependentTarget) {
		t.Fatalf("err = %v, want errNoDependentTarget", msg.err)
	}
}

func TestLoadBulkDependentsSkipsTheClusterWhenNothingIsWalkable(t *testing.T) {
	m := Model{}
	m.bulkItems = []model.Item{{Kind: "ConfigMap", Namespace: "a", Raw: rawWithUID("cm1")}}

	msg, ok := m.loadBulkDependents()().(dependentsLoadedMsg)
	if !ok {
		t.Fatalf("expected dependentsLoadedMsg")
	}
	if msg.err != nil {
		t.Fatalf("unexpected error: %v", msg.err)
	}
	if msg.count.Total != 0 || msg.count.Uncounted != 1 {
		t.Fatalf("count = %+v, want 0 total and 1 uncounted", msg.count)
	}
}

func TestUpdateDependentsLoadedIgnoresAStaleReply(t *testing.T) {
	m := Model{}
	m.beginDependents() // req 1
	m.beginDependents() // req 2

	out, _ := m.updateDependentsLoaded(dependentsLoadedMsg{
		req:   1,
		count: &k8s.DependentCount{Total: 9},
	})
	got := out.(Model)
	if got.deps.count != nil {
		t.Fatalf("a stale reply landed on the open dialog")
	}
	if !got.deps.loading {
		t.Fatalf("a stale reply cleared the placeholder")
	}
}
