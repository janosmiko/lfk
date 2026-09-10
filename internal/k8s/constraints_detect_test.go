package k8s

import (
	"errors"
	"slices"
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

func TestDetectConstraints_PartialFailureReturnsNilTopLevelError(t *testing.T) {
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

	report, err := client.DetectConstraints(t.Context(), "", target)
	if err != nil {
		t.Errorf("expected a partial failure to return a nil top-level error, got %v", err)
	}
	if len(report.Rows) == 0 {
		t.Errorf("expected surviving sources to still populate rows, got %+v", report)
	}
}

func TestDetectConstraints_AllSourcesFailReturnsJoinedError(t *testing.T) {
	failErr := apierrors.NewForbidden(schema.GroupResource{Resource: "*"}, "", nil)
	failAll := func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, failErr }

	cs := k8sfake.NewClientset()
	cs.PrependReactor("list", "*", failAll)
	cs.PrependReactor("get", "*", failAll)
	dc := newFakeDynClient()
	dc.PrependReactor("list", "*", failAll)
	client := newFakeClient(cs, dc)

	target := ConstraintTarget{
		Namespace:         "default",
		PriorityClassName: "high",
	}

	report, err := client.DetectConstraints(t.Context(), "", target)

	if err == nil {
		t.Error("expected a total failure (every source erroring) to return a non-nil error")
	}
	if len(report.Rows) != 0 {
		t.Errorf("expected no rows when every source fails, got %+v", report.Rows)
	}
}

func TestDetectConstraints_NonRBACErrorGoesToFailedNotSkipped(t *testing.T) {
	node := &corev1.Node{Name: "node-1"}
	cs := k8sfake.NewClientset(node)
	cs.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("connection reset by peer")
	})
	dc := newFakeDynClient()
	client := newFakeClient(cs, dc)

	report, err := client.DetectConstraints(t.Context(), "", ConstraintTarget{Namespace: "default"})
	if err != nil {
		t.Errorf("expected a partial failure to return a nil top-level error, got %v", err)
	}
	if slices.Contains(report.Skipped, "nodes") {
		t.Errorf("a non-RBAC error must not land in Skipped, got %v", report.Skipped)
	}
	if !slices.Contains(report.Failed, "nodes") {
		t.Errorf("expected Failed to contain nodes, got %v", report.Failed)
	}
}
