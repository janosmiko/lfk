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
