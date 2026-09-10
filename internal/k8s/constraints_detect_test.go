package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestDetectConstraints_ForbiddenListOmitsRowsNotShowsAbsent(t *testing.T) {
	node := &corev1.Node{Name: "node-1"}
	cs := k8sfake.NewClientset(node)
	cs.PrependReactor("list", "poddisruptionbudgets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "policy", Resource: "poddisruptionbudgets"}, "", nil)
	})

	quota := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ResourceQuota",
			"metadata":   map[string]any{"name": "compute-quota", "namespace": "default"},
			"spec":       map[string]any{"hard": map[string]any{"cpu": "4"}},
			"status":     map[string]any{"used": map[string]any{"cpu": "3"}},
		},
	}
	dc := newFakeDynClient(quota)

	client := newFakeClient(cs, dc)

	target := ConstraintTarget{
		Namespace:  "default",
		PodLabels:  map[string]string{"app": "web"},
		Containers: []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "500m"}}},
	}

	report, _ := client.DetectConstraints(t.Context(), "", target)

	for _, row := range report.Rows {
		if row.Source == "PDB" {
			t.Errorf("expected no PDB rows, got %+v", row)
		}
	}
	foundSkipped := false
	for _, s := range report.Skipped {
		if s == "poddisruptionbudgets" {
			foundSkipped = true
		}
	}
	if !foundSkipped {
		t.Errorf("expected Skipped to contain poddisruptionbudgets, got %v", report.Skipped)
	}

	foundQuota := false
	for _, row := range report.Rows {
		if row.Source == "Quota" {
			foundQuota = true
		}
	}
	if !foundQuota {
		t.Errorf("expected the quota source to still populate rows, got %+v", report.Rows)
	}
}
