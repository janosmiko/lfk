package k8s

import (
	"errors"
	"slices"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

func TestDetectConstraints_MutatingFailureKeepsValidatingRows(t *testing.T) {
	sideEffects := admissionregistrationv1.SideEffectClassNone
	cfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
		Name: "guard",
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:        "guard.example.com",
			SideEffects: &sideEffects,
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				APIGroups:  []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"*"},
			}},
		}},
	}
	ns := &corev1.Namespace{Name: "default"}
	cs := k8sfake.NewClientset(ns, cfg)
	cs.PrependReactor("list", "mutatingwebhookconfigurations", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Group: "admissionregistration.k8s.io", Resource: "mutatingwebhookconfigurations"}, "", nil)
	})
	client := newFakeClient(cs, newFakeDynClient())

	target := ConstraintTarget{
		Namespace: "default",
		GVR:       schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	report, err := client.DetectConstraints(t.Context(), "", target)
	if err != nil {
		t.Fatalf("expected a partial failure to return a nil top-level error, got %v", err)
	}

	sawWebhook := false
	for _, row := range report.Rows {
		if row.Source == "Webhook" && row.Name == "guard" {
			sawWebhook = true
		}
	}
	if !sawWebhook {
		t.Errorf("expected the validating webhook row to survive, got %+v", report.Rows)
	}
	if !slices.Contains(report.Skipped, "mutatingwebhookconfigurations") {
		t.Errorf("expected the denied mutating source in Skipped, got %v", report.Skipped)
	}
	if slices.Contains(report.Skipped, "validatingwebhookconfigurations") {
		t.Errorf("the validating source read fine and must not be reported denied, got %v", report.Skipped)
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
