package k8s

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCountDependents(t *testing.T) {
	tests := []struct {
		name    string
		objects []DependentRef
		roots   []string
		want    DependentCount
	}{
		{
			name:  "no objects",
			roots: []string{"root"},
			want:  DependentCount{ByKind: map[string]int{}},
		},
		{
			name: "direct children only",
			objects: []DependentRef{
				{Kind: "Pod", UID: "p1", OwnerUIDs: []string{"root"}},
				{Kind: "Pod", UID: "p2", OwnerUIDs: []string{"root"}},
				{Kind: "Pod", UID: "p3", OwnerUIDs: []string{"other"}},
			},
			roots: []string{"root"},
			want:  DependentCount{Total: 2, ByKind: map[string]int{"Pod": 2}},
		},
		{
			name: "deep chain deployment to replicaset to pod",
			objects: []DependentRef{
				{Kind: "ReplicaSet", UID: "rs1", OwnerUIDs: []string{"deploy"}},
				{Kind: "ReplicaSet", UID: "rs2", OwnerUIDs: []string{"deploy"}},
				{Kind: "Pod", UID: "p1", OwnerUIDs: []string{"rs1"}},
				{Kind: "Pod", UID: "p2", OwnerUIDs: []string{"rs1"}},
				{Kind: "Pod", UID: "p3", OwnerUIDs: []string{"rs2"}},
			},
			roots: []string{"deploy"},
			want: DependentCount{
				Total:  5,
				ByKind: map[string]int{"ReplicaSet": 2, "Pod": 3},
			},
		},
		{
			name: "object owned by two owners counts once",
			objects: []DependentRef{
				{Kind: "Pod", UID: "p1", OwnerUIDs: []string{"root", "rs1"}},
				{Kind: "ReplicaSet", UID: "rs1", OwnerUIDs: []string{"root"}},
			},
			roots: []string{"root"},
			want: DependentCount{
				Total:  2,
				ByKind: map[string]int{"Pod": 1, "ReplicaSet": 1},
			},
		},
		{
			name: "owner cycle terminates",
			objects: []DependentRef{
				{Kind: "A", UID: "a", OwnerUIDs: []string{"root", "b"}},
				{Kind: "B", UID: "b", OwnerUIDs: []string{"a"}},
			},
			roots: []string{"root"},
			want:  DependentCount{Total: 2, ByKind: map[string]int{"A": 1, "B": 1}},
		},
		{
			name: "root listed among the objects is not counted as its own dependent",
			objects: []DependentRef{
				{Kind: "Deployment", UID: "root"},
				{Kind: "Pod", UID: "p1", OwnerUIDs: []string{"root"}},
			},
			roots: []string{"root"},
			want:  DependentCount{Total: 1, ByKind: map[string]int{"Pod": 1}},
		},
		{
			name: "several roots share one total",
			objects: []DependentRef{
				{Kind: "Pod", UID: "p1", OwnerUIDs: []string{"r1"}},
				{Kind: "Pod", UID: "p2", OwnerUIDs: []string{"r2"}},
			},
			roots: []string{"r1", "r2"},
			want:  DependentCount{Total: 2, ByKind: map[string]int{"Pod": 2}},
		},
		{
			name: "object without a uid is skipped",
			objects: []DependentRef{
				{Kind: "Pod", OwnerUIDs: []string{"root"}},
			},
			roots: []string{"root"},
			want:  DependentCount{ByKind: map[string]int{}},
		},
		{
			name:    "no roots counts nothing",
			objects: []DependentRef{{Kind: "Pod", UID: "p1", OwnerUIDs: []string{"root"}}},
			want:    DependentCount{ByKind: map[string]int{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountDependents(tt.objects, tt.roots)
			if got.Total != tt.want.Total {
				t.Errorf("Total = %d, want %d", got.Total, tt.want.Total)
			}
			if !reflect.DeepEqual(got.ByKind, tt.want.ByKind) {
				t.Errorf("ByKind = %v, want %v", got.ByKind, tt.want.ByKind)
			}
		})
	}
}

func TestDependentCountSummary(t *testing.T) {
	tests := []struct {
		name  string
		count DependentCount
		want  string
	}{
		{
			name:  "empty",
			count: DependentCount{},
			want:  "",
		},
		{
			name:  "one pod",
			count: DependentCount{Total: 1, ByKind: map[string]int{"Pod": 1}},
			want:  "1 pod",
		},
		{
			name:  "several kinds, biggest first",
			count: DependentCount{Total: 5, ByKind: map[string]int{"Pod": 3, "ReplicaSet": 2}},
			want:  "3 pods, 2 replicasets",
		},
		{
			name: "equal counts sort by kind",
			count: DependentCount{
				Total:  4,
				ByKind: map[string]int{"ReplicaSet": 2, "ControllerRevision": 2},
			},
			want: "2 controllerrevisions, 2 replicasets",
		},
		{
			name: "long tail collapses to a total",
			count: DependentCount{
				Total: 10,
				ByKind: map[string]int{
					"Pod": 4, "ReplicaSet": 3, "Job": 2, "EndpointSlice": 1,
				},
			},
			want: "10 objects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.count.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

// BenchmarkCountDependents guards the cost model: the walk is one pass over
// the objects and their owner edges, so a namespace-sized input must stay
// linear rather than degrading into a scan per node.
func BenchmarkCountDependents(b *testing.B) {
	const replicaSets, podsPer = 100, 100
	objects := make([]DependentRef, 0, replicaSets*(podsPer+1))
	for rs := range replicaSets {
		rsUID := fmt.Sprintf("rs-%d", rs)
		objects = append(objects, DependentRef{
			Kind: "ReplicaSet", UID: rsUID, OwnerUIDs: []string{"root"},
		})
		for p := range podsPer {
			objects = append(objects, DependentRef{
				Kind: "Pod", UID: fmt.Sprintf("p-%d-%d", rs, p), OwnerUIDs: []string{rsUID},
			})
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		if got := CountDependents(objects, []string{"root"}); got.Total != len(objects) {
			b.Fatalf("Total = %d, want %d", got.Total, len(objects))
		}
	}
}

func TestPluralKind(t *testing.T) {
	tests := []struct {
		kind string
		n    int
		want string
	}{
		{"Pod", 1, "pod"},
		{"Pod", 2, "pods"},
		{"ReplicaSet", 2, "replicasets"},
		{"Ingress", 2, "ingresses"},
		{"NetworkPolicy", 2, "networkpolicies"},
		{"Gateway", 2, "gateways"},
		{"y", 2, "ys"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			if got := pluralKind(tt.kind, tt.n); got != tt.want {
				t.Errorf("pluralKind(%q, %d) = %q, want %q", tt.kind, tt.n, got, tt.want)
			}
		})
	}
}
