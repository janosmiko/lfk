package k8s

import (
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestPDBRows_UseSelectorMatchesAndStateDisruptionsAllowed(t *testing.T) {
	matching := &policyv1.PodDisruptionBudget{
		Name: "web-pdb", Namespace: "default",
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
		Status: policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 0},
	}
	other := &policyv1.PodDisruptionBudget{
		Name: "api-pdb", Namespace: "default",
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
		Status: policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 2},
	}
	client := newFakeClient(k8sfake.NewClientset(matching, other), nil)

	target := ConstraintTarget{Namespace: "default", PodLabels: map[string]string{"app": "web"}}

	rows, err := client.pdbConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("pdbConstraintRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %+v", len(rows), rows)
	}
	if rows[0].Name != "web-pdb" {
		t.Errorf("Name = %q, want web-pdb", rows[0].Name)
	}
	if !rows[0].Blocking {
		t.Error("expected row to be Blocking")
	}
	if rows[0].Detail != "0 disruptions allowed" {
		t.Errorf("Detail = %q, want %q", rows[0].Detail, "0 disruptions allowed")
	}
}
