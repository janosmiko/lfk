package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// deployRadius is the pod side of deleting a 3-replica Deployment: every pod
// goes, none is ready afterwards.
func deployRadius() *k8s.BlastRadius {
	return &k8s.BlastRadius{
		Evicting: 3, ReadyBefore: 3, ReadyAfter: 0,
		PDBs: []k8s.PDBImpact{{
			Name: "kyverno", AllowedBefore: 2, AllowedAfter: -1, Evicting: 3, Violated: true,
		}},
		Violation: true,
	}
}

// deployDeps is the owner side of the same delete: four ReplicaSets and the
// three pods under them.
func deployDeps() dependentsState {
	return dependentsState{count: &k8s.DependentCount{
		Total: 7, ByKind: map[string]int{"ReplicaSet": 4, "Pod": 3},
	}}
}

// rows flattens the notes to label/text pairs so a case reads as the box does.
func rows(notes []ui.ConfirmNote) map[string]string {
	out := make(map[string]string, len(notes))
	for _, n := range notes {
		out[n.Label] = n.Text
	}
	return out
}

func TestConfirmCostNotes(t *testing.T) {
	tests := []struct {
		name string
		cost confirmCost
		want map[string]string
	}{
		{
			name: "one placeholder covers both fetches",
			cost: confirmCost{loading: true, cascades: true},
			want: map[string]string{"Scope": "working out what this costs..."},
		},
		{
			name: "delete a deployment, background",
			cost: confirmCost{
				radius: deployRadius(), deps: deployDeps(), kind: "Deployment",
				cascades: true, policy: model.DeletePropagationBackground,
			},
			want: map[string]string{
				"Scope":        "4 replicasets, 3 pods",
				"Availability": "0 of 3 ready after",
				"Risk":         "kyverno allows 2 at once, this removes 3",
			},
		},
		{
			name: "foreground reads the same as background",
			cost: confirmCost{
				radius: deployRadius(), deps: deployDeps(), kind: "Deployment",
				cascades: true, policy: model.DeletePropagationForeground,
			},
			want: map[string]string{
				"Scope":        "4 replicasets, 3 pods",
				"Availability": "0 of 3 ready after",
				"Risk":         "kyverno allows 2 at once, this removes 3",
			},
		},
		{
			name: "orphan keeps the dependents and evicts nothing",
			cost: confirmCost{
				radius: deployRadius(), deps: deployDeps(), kind: "Deployment",
				cascades: true, policy: model.DeletePropagationOrphan,
			},
			want: map[string]string{
				"Scope":        "the deployment only",
				"Availability": "unchanged, the 3 pods keep running",
				"Risk":         "4 replicasets, 3 pods left with no owner",
			},
		},
		{
			name: "none defers every row to the server",
			cost: confirmCost{
				radius: deployRadius(), deps: deployDeps(), kind: "Deployment",
				cascades: true, policy: model.DeletePropagationNone,
			},
			want: map[string]string{
				"Scope":        "the deployment, plus 4 replicasets, 3 pods if the server cascades",
				"Availability": "depends on the server default",
				"Risk":         "the default is Background for most kinds, Orphan for Job and ReplicationController",
			},
		},
		{
			name: "a bare pod delete has no scope row",
			cost: confirmCost{
				radius:   &k8s.BlastRadius{Evicting: 1, ReadyBefore: 1, ReadyAfter: 0},
				kind:     "Pod",
				cascades: true, policy: model.DeletePropagationBackground,
			},
			want: map[string]string{"Availability": "0 of 1 ready after"},
		},
		{
			name: "a deployment that owns nothing has no scope row",
			cost: confirmCost{
				radius:   &k8s.BlastRadius{},
				deps:     dependentsState{count: &k8s.DependentCount{ByKind: map[string]int{}}},
				kind:     "Deployment",
				cascades: true, policy: model.DeletePropagationBackground,
			},
			want: map[string]string{"Availability": "no running pods"},
		},
		{
			name: "drain counts pods and keeps the blocking wording",
			cost: confirmCost{
				radius: &k8s.BlastRadius{
					Evicting: 12,
					PDBs: []k8s.PDBImpact{
						{Name: "a", Violated: true}, {Name: "b", Violated: true}, {Name: "c"},
					},
				},
				enforced: true,
			},
			want: map[string]string{
				"Scope": "12 pods",
				"Risk":  "3 budgets, 2 would block the drain",
			},
		},
		{
			name: "scale down reports the replicas left",
			cost: confirmCost{
				radius: &k8s.BlastRadius{Evicting: 2, ReadyBefore: 5, ReadyAfter: 3},
			},
			// No budget covers these pods, so there is no risk row to show.
			want: map[string]string{
				"Scope":        "2 pods",
				"Availability": "3 of 5 ready after",
			},
		},
		{
			name: "bulk delete declares the rows it could not walk",
			cost: confirmCost{
				radius: &k8s.BlastRadius{Uncounted: 2},
				deps: dependentsState{count: &k8s.DependentCount{
					Total: 6, ByKind: map[string]int{"Pod": 6}, Uncounted: 2,
				}},
				cascades: true, policy: model.DeletePropagationBackground,
			},
			want: map[string]string{
				"Scope":        "6 pods (2 rows not counted)",
				"Availability": "no running pods",
			},
		},
		{
			name: "a selection of rows that own nothing still declares them",
			cost: confirmCost{
				radius: &k8s.BlastRadius{Uncounted: 50},
				deps: dependentsState{count: &k8s.DependentCount{
					ByKind: map[string]int{}, Uncounted: 50,
				}},
				cascades: true, policy: model.DeletePropagationBackground,
			},
			want: map[string]string{
				"Scope":        "nothing countable (50 rows not counted)",
				"Availability": "no running pods",
			},
		},
		{
			name: "orphan on a bulk selection names the rows, not a kind",
			cost: confirmCost{
				radius: deployRadius(), deps: deployDeps(),
				cascades: true, policy: model.DeletePropagationOrphan,
			},
			want: map[string]string{
				"Scope":        "the selected rows only",
				"Availability": "unchanged, the 3 pods keep running",
				"Risk":         "4 replicasets, 3 pods left with no owner",
			},
		},
		{
			name: "nothing fetched at all leaves the block empty",
			cost: confirmCost{cascades: true, policy: model.DeletePropagationBackground},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rows(confirmCostNotes(tt.cost))
			for label, want := range tt.want {
				if got[label] != want {
					t.Errorf("%s = %q, want %q", label, got[label], want)
				}
			}
			for label, text := range got {
				if _, ok := tt.want[label]; !ok {
					t.Errorf("unexpected row %s = %q", label, text)
				}
			}
		})
	}
}

func TestConfirmCostNotesRowOrder(t *testing.T) {
	notes := confirmCostNotes(confirmCost{
		radius: deployRadius(), deps: deployDeps(), kind: "Deployment",
		cascades: true, policy: model.DeletePropagationBackground,
	})

	want := []string{"Scope", "Availability", "Risk"}
	if len(notes) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(notes), len(want), notes)
	}
	for i, label := range want {
		if notes[i].Label != label {
			t.Errorf("row %d = %q, want %q", i, notes[i].Label, label)
		}
	}
}

func TestConfirmCostNotesWarns(t *testing.T) {
	tests := []struct {
		name   string
		cost   confirmCost
		warned []string
	}{
		{
			name: "a breached budget warns",
			cost: confirmCost{
				radius: deployRadius(), deps: deployDeps(), kind: "Deployment",
				cascades: true, policy: model.DeletePropagationBackground,
			},
			warned: []string{"Risk"},
		},
		{
			name: "orphaning warns on both the scope and the risk",
			cost: confirmCost{
				radius: deployRadius(), deps: deployDeps(), kind: "Deployment",
				cascades: true, policy: model.DeletePropagationOrphan,
			},
			warned: []string{"Scope", "Risk"},
		},
		{
			name: "a healthy scale-down warns on nothing",
			cost: confirmCost{
				radius: &k8s.BlastRadius{Evicting: 1, ReadyBefore: 5, ReadyAfter: 4},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warned := map[string]bool{}
			for _, n := range confirmCostNotes(tt.cost) {
				if n.Warn {
					warned[n.Label] = true
				}
			}
			for _, label := range tt.warned {
				if !warned[label] {
					t.Errorf("%s should warn", label)
				}
				delete(warned, label)
			}
			for label := range warned {
				t.Errorf("%s should not warn", label)
			}
		})
	}
}
