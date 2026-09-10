package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestPriorityRows_ReportPreemptionFromEventAndNominatedNode(t *testing.T) {
	pc := &schedulingv1.PriorityClass{
		Name:  "high",
		Value: 1000000,
	}
	pod := &corev1.Pod{
		Name: "web-1", Namespace: "default",
		Status: corev1.PodStatus{NominatedNodeName: "node-2"},
	}
	event := &corev1.Event{
		Name: "web-1.preempt", Namespace: "default",
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod", Name: "web-1", Namespace: "default",
		},
		Reason:  "Preempted",
		Message: "preempted by a higher priority pod",
	}
	client := newFakeClient(k8sfake.NewClientset(pc, pod, event), nil)

	target := ConstraintTarget{
		Namespace:         "default",
		PodName:           "web-1",
		PriorityClassName: "high",
	}

	rows, err := client.priorityConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("priorityConstraintRows: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("expected at least 3 rows (class, nominated, event), got %d: %+v", len(rows), rows)
	}
	if !strings.Contains(rows[0].Detail, "1000000") {
		t.Errorf("first row Detail = %q, want the priority value", rows[0].Detail)
	}
	var sawNominated, sawEvent bool
	for _, r := range rows[1:] {
		if strings.Contains(r.Detail, "node-2") {
			sawNominated = true
		}
		if strings.Contains(r.Detail, "preempted by a higher priority pod") {
			sawEvent = true
		}
		if !r.Blocking {
			t.Errorf("row %+v should be Blocking", r)
		}
	}
	if !sawNominated {
		t.Error("expected a row naming the nominated node")
	}
	if !sawEvent {
		t.Error("expected a row from the Preempted event")
	}
}

func TestPriorityRows_NoPriorityClassNameYieldsNoRows(t *testing.T) {
	client := newFakeClient(k8sfake.NewClientset(), nil)
	rows, err := client.priorityConstraintRows(t.Context(), "", ConstraintTarget{Namespace: "default"})
	if err != nil {
		t.Fatalf("priorityConstraintRows: %v", err)
	}
	if rows != nil {
		t.Errorf("expected no rows, got %+v", rows)
	}
}
