package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestNodeRows_NameTermNoNodeSatisfies(t *testing.T) {
	node1 := &corev1.Node{
		Name: "node-1", Labels: map[string]string{"disk": "hdd"},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}},
		},
	}
	node2 := &corev1.Node{
		Name: "node-2", Labels: map[string]string{"disk": "hdd"},
	}
	client := newFakeClient(k8sfake.NewClientset(node1, node2), nil)

	target := ConstraintTarget{NodeSelector: map[string]string{"disk": "nvme"}}

	rows, err := client.nodeConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("nodeConstraintRows: %v", err)
	}

	var sawSelector, sawTaint bool
	for _, r := range rows {
		if strings.Contains(r.Detail, "disk=nvme") {
			sawSelector = true
		}
		if r.Kind == "Node" && r.Name == "node-1" {
			sawTaint = true
		}
	}
	if !sawSelector {
		t.Errorf("expected a row naming disk=nvme, got %+v", rows)
	}
	if !sawTaint {
		t.Errorf("expected a Node row for node-1's untolerated taint, got %+v", rows)
	}
}

func TestSchedulingRows_SelectorAndAffinityMustMatchTheSameNode(t *testing.T) {
	selectorOnly := corev1.Node{Name: "node-1", Labels: map[string]string{"disk": "nvme"}}
	affinityOnly := corev1.Node{Name: "node-2", Labels: map[string]string{"zone": "eu-1"}}
	both := corev1.Node{Name: "node-3", Labels: map[string]string{"disk": "nvme", "zone": "eu-1"}}

	zoneTerm := NodeSelectorTerm{MatchExpressions: []NodeSelectorRequirement{
		{Key: "zone", Operator: "In", Values: []string{"eu-1"}},
	}}
	target := ConstraintTarget{
		NodeSelector: map[string]string{"disk": "nvme"},
		Affinity:     []NodeSelectorTerm{zoneTerm},
	}

	tests := []struct {
		name     string
		nodes    []corev1.Node
		wantRows bool
	}{
		{"split across two nodes", []corev1.Node{selectorOnly, affinityOnly}, true},
		{"one node satisfies both", []corev1.Node{selectorOnly, affinityOnly, both}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := schedulingRows(tt.nodes, target)
			if got := len(rows) > 0; got != tt.wantRows {
				t.Errorf("rows = %+v, want any = %v", rows, tt.wantRows)
			}
		})
	}
}

func TestNodeSelectorTermMatches_FieldsAndEmptyTerms(t *testing.T) {
	node := corev1.Node{Name: "node-1", Labels: map[string]string{"disk": "nvme"}}

	tests := []struct {
		name string
		term NodeSelectorTerm
		want bool
	}{
		{"empty term matches no node", NodeSelectorTerm{}, false},
		{
			"matchFields on the node name",
			NodeSelectorTerm{MatchFields: []NodeSelectorRequirement{
				{Key: "metadata.name", Operator: "In", Values: []string{"node-1"}},
			}},
			true,
		},
		{
			"matchFields naming another node",
			NodeSelectorTerm{MatchFields: []NodeSelectorRequirement{
				{Key: "metadata.name", Operator: "In", Values: []string{"node-9"}},
			}},
			false,
		},
		{
			"expressions and fields are ANDed",
			NodeSelectorTerm{
				MatchExpressions: []NodeSelectorRequirement{{Key: "disk", Operator: "In", Values: []string{"nvme"}}},
				MatchFields:      []NodeSelectorRequirement{{Key: "metadata.name", Operator: "In", Values: []string{"node-9"}}},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeSelectorTermMatches(tt.term, node); got != tt.want {
				t.Errorf("nodeSelectorTermMatches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnyNodeHasLabel_RequiresTheKeyToExist(t *testing.T) {
	nodes := []corev1.Node{{Name: "node-1", Labels: map[string]string{"disk": "nvme"}}}

	tests := []struct {
		name       string
		key, value string
		want       bool
	}{
		{"present key and value", "disk", "nvme", true},
		{"missing key with an empty value", "gpu", "", false},
		{"present key, other value", "disk", "hdd", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := anyNodeHasLabel(nodes, tt.key, tt.value); got != tt.want {
				t.Errorf("anyNodeHasLabel(%q, %q) = %v, want %v", tt.key, tt.value, got, tt.want)
			}
		})
	}
}

func TestNodeRows_SelectorSatisfiedByOneNodeYieldsNoRow(t *testing.T) {
	node1 := &corev1.Node{
		Name: "node-1", Labels: map[string]string{"disk": "nvme"},
	}
	client := newFakeClient(k8sfake.NewClientset(node1), nil)

	target := ConstraintTarget{NodeSelector: map[string]string{"disk": "nvme"}}

	rows, err := client.nodeConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("nodeConstraintRows: %v", err)
	}
	for _, r := range rows {
		if r.Kind == "NodeSelector" {
			t.Errorf("did not expect a NodeSelector row, got %+v", r)
		}
	}
}
