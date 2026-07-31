package model

import "testing"

func TestDeletePropagationCycle(t *testing.T) {
	tests := []struct {
		name string
		in   DeletePropagation
		want DeletePropagation
	}{
		{"background to foreground", DeletePropagationBackground, DeletePropagationForeground},
		{"foreground to orphan", DeletePropagationForeground, DeletePropagationOrphan},
		{"orphan to none", DeletePropagationOrphan, DeletePropagationNone},
		{"none wraps to background", DeletePropagationNone, DeletePropagationBackground},
		{"unknown falls back to background", DeletePropagation("bogus"), DeletePropagationBackground},
		{"empty falls back to background", DeletePropagation(""), DeletePropagationBackground},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Cycle(); got != tt.want {
				t.Errorf("Cycle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeletePropagationCycleVisitsEveryPolicyOnce(t *testing.T) {
	seen := map[DeletePropagation]bool{}
	p := DeletePropagationBackground
	for range deletePropagationOrder {
		if seen[p] {
			t.Fatalf("Cycle() repeated %q before visiting all policies", p)
		}
		seen[p] = true
		p = p.Cycle()
	}
	if p != DeletePropagationBackground {
		t.Errorf("Cycle() did not return to start after a full cycle, got %q", p)
	}
	if len(seen) != len(deletePropagationOrder) {
		t.Errorf("visited %d policies, want %d", len(seen), len(deletePropagationOrder))
	}
}

// Paths that shell out to kubectl cannot express "send no policy": omitting
// --cascade makes kubectl send background anyway, and --cascade=none is not a
// valid value. So None is absent from this cycle and clamps to Background.
func TestDeletePropagationCycleCascading(t *testing.T) {
	tests := []struct {
		in   DeletePropagation
		want DeletePropagation
	}{
		{DeletePropagationBackground, DeletePropagationForeground},
		{DeletePropagationForeground, DeletePropagationOrphan},
		{DeletePropagationOrphan, DeletePropagationBackground},
		{DeletePropagationNone, DeletePropagationBackground},
		{DeletePropagation("bogus"), DeletePropagationBackground},
	}
	for _, tt := range tests {
		if got := tt.in.CycleCascading(); got != tt.want {
			t.Errorf("DeletePropagation(%q).CycleCascading() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDeletePropagationCycleCascadingNeverYieldsNone(t *testing.T) {
	p := DeletePropagationBackground
	for range 10 {
		p = p.CycleCascading()
		if p == DeletePropagationNone {
			t.Fatal("CycleCascading() must never reach None")
		}
	}
}

func TestDeletePropagationCascading(t *testing.T) {
	tests := []struct {
		in   DeletePropagation
		want DeletePropagation
	}{
		{DeletePropagationBackground, DeletePropagationBackground},
		{DeletePropagationForeground, DeletePropagationForeground},
		{DeletePropagationOrphan, DeletePropagationOrphan},
		{DeletePropagationNone, DeletePropagationBackground},
		{DeletePropagation(""), DeletePropagationBackground},
		{DeletePropagation("bogus"), DeletePropagationBackground},
	}
	for _, tt := range tests {
		if got := tt.in.Cascading(); got != tt.want {
			t.Errorf("DeletePropagation(%q).Cascading() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// KubectlCascade feeds `kubectl delete --cascade=<v>`; an invalid value there
// makes kubectl reject the command outright.
func TestDeletePropagationKubectlCascade(t *testing.T) {
	valid := map[string]bool{"background": true, "foreground": true, "orphan": true}
	for _, p := range []DeletePropagation{
		DeletePropagationBackground,
		DeletePropagationForeground,
		DeletePropagationOrphan,
		DeletePropagationNone,
		DeletePropagation(""),
		DeletePropagation("bogus"),
	} {
		got := p.KubectlCascade()
		if !valid[got] {
			t.Errorf("DeletePropagation(%q).KubectlCascade() = %q, not a value kubectl accepts", p, got)
		}
	}
	if got := DeletePropagationOrphan.KubectlCascade(); got != "orphan" {
		t.Errorf("Orphan.KubectlCascade() = %q, want %q", got, "orphan")
	}
}

func TestDeletePropagationLabel(t *testing.T) {
	tests := []struct {
		in   DeletePropagation
		want string
	}{
		{DeletePropagationBackground, "Background"},
		{DeletePropagationForeground, "Foreground"},
		{DeletePropagationOrphan, "Orphan"},
		{DeletePropagationNone, "None"},
		{DeletePropagation("bogus"), "Background"},
	}
	for _, tt := range tests {
		if got := tt.in.Label(); got != tt.want {
			t.Errorf("DeletePropagation(%q).Label() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseDeletePropagation(t *testing.T) {
	tests := []struct {
		in   string
		want DeletePropagation
		ok   bool
	}{
		{"background", DeletePropagationBackground, true},
		{"foreground", DeletePropagationForeground, true},
		{"orphan", DeletePropagationOrphan, true},
		{"none", DeletePropagationNone, true},
		{"BACKGROUND", DeletePropagationBackground, true},
		{" Orphan ", DeletePropagationOrphan, true},
		{"bogus", DeletePropagationBackground, false},
		{"", DeletePropagationBackground, false},
	}
	for _, tt := range tests {
		got, ok := ParseDeletePropagation(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseDeletePropagation(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

// Callers gate warning copy on these predicates, so they must not drift from
// the constants.
func TestDeletePropagationPredicates(t *testing.T) {
	tests := []struct {
		policy      DeletePropagation
		orphans     bool
		defers      bool
		needsWarned bool
	}{
		{DeletePropagationBackground, false, false, false},
		{DeletePropagationForeground, false, false, false},
		{DeletePropagationOrphan, true, false, true},
		{DeletePropagationNone, false, true, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			if got := tt.policy.OrphansDependents(); got != tt.orphans {
				t.Errorf("OrphansDependents() = %v, want %v", got, tt.orphans)
			}
			if got := tt.policy.DefersToServer(); got != tt.defers {
				t.Errorf("DefersToServer() = %v, want %v", got, tt.defers)
			}
			if got := tt.policy.NeedsWarning(); got != tt.needsWarned {
				t.Errorf("NeedsWarning() = %v, want %v", got, tt.needsWarned)
			}
		})
	}
}
