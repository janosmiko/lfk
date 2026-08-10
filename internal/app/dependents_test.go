package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func TestDependentsNote(t *testing.T) {
	twoKinds := &k8s.DependentCount{
		Total:  5,
		ByKind: map[string]int{"Pod": 3, "ReplicaSet": 2},
	}

	tests := []struct {
		name     string
		state    dependentsState
		policy   model.DeletePropagation
		wantNote bool
		wantText string
		wantWarn bool
	}{
		{
			name:     "loading shows a placeholder",
			state:    dependentsState{loading: true},
			policy:   model.DeletePropagationBackground,
			wantNote: true,
			wantText: "counting...",
		},
		{
			name:     "unknown owner kind drops the line",
			state:    dependentsState{},
			policy:   model.DeletePropagationBackground,
			wantNote: false,
		},
		{
			name:     "background removes them",
			state:    dependentsState{count: twoKinds},
			policy:   model.DeletePropagationBackground,
			wantNote: true,
			wantText: "3 pods, 2 replicasets also removed",
		},
		{
			name:     "foreground removes them",
			state:    dependentsState{count: twoKinds},
			policy:   model.DeletePropagationForeground,
			wantNote: true,
			wantText: "3 pods, 2 replicasets also removed",
		},
		{
			name:     "orphan keeps them",
			state:    dependentsState{count: twoKinds},
			policy:   model.DeletePropagationOrphan,
			wantNote: true,
			wantText: "3 pods, 2 replicasets stay in the cluster",
			wantWarn: true,
		},
		{
			name:     "none leaves it to the server",
			state:    dependentsState{count: twoKinds},
			policy:   model.DeletePropagationNone,
			wantNote: true,
			wantText: "3 pods, 2 replicasets may stay (server decides)",
			wantWarn: true,
		},
		{
			name:     "no dependents",
			state:    dependentsState{count: &k8s.DependentCount{ByKind: map[string]int{}}},
			policy:   model.DeletePropagationBackground,
			wantNote: true,
			wantText: "none",
		},
		{
			name:     "no dependents under orphan says nothing is kept",
			state:    dependentsState{count: &k8s.DependentCount{ByKind: map[string]int{}}},
			policy:   model.DeletePropagationOrphan,
			wantNote: true,
			wantText: "none",
		},
		{
			name: "uncounted rows are declared",
			state: dependentsState{count: &k8s.DependentCount{
				Total:     5,
				ByKind:    map[string]int{"Pod": 3, "ReplicaSet": 2},
				Uncounted: 2,
			}},
			policy:   model.DeletePropagationBackground,
			wantNote: true,
			wantText: "3 pods, 2 replicasets also removed (2 rows not counted)",
		},
		{
			name: "only uncounted rows",
			state: dependentsState{count: &k8s.DependentCount{
				ByKind: map[string]int{}, Uncounted: 1,
			}},
			policy:   model.DeletePropagationBackground,
			wantNote: true,
			wantText: "none (1 row not counted)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notes := dependentsNotes(tt.state, tt.policy)
			if !tt.wantNote {
				if len(notes) != 0 {
					t.Fatalf("got %d notes, want none: %+v", len(notes), notes)
				}
				return
			}
			if len(notes) != 1 {
				t.Fatalf("got %d notes, want 1: %+v", len(notes), notes)
			}
			if notes[0].Label != "Dependents" {
				t.Errorf("Label = %q, want %q", notes[0].Label, "Dependents")
			}
			if notes[0].Text != tt.wantText {
				t.Errorf("Text = %q, want %q", notes[0].Text, tt.wantText)
			}
			if notes[0].Warn != tt.wantWarn {
				t.Errorf("Warn = %v, want %v", notes[0].Warn, tt.wantWarn)
			}
		})
	}
}

func TestDependentsStateReset(t *testing.T) {
	s := dependentsState{count: &k8s.DependentCount{Total: 3}, loading: true}
	s.reset()
	if s.count != nil || s.loading {
		t.Fatalf("reset left state populated: %+v", s)
	}
}
