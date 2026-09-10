package k8s

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestQuotaRows_ShowRequestsAgainstHeadroom(t *testing.T) {
	quota := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ResourceQuota",
			"metadata": map[string]any{
				"name":      "compute-quota",
				"namespace": "default",
			},
			"spec": map[string]any{
				"hard": map[string]any{"cpu": "4"},
			},
			"status": map[string]any{
				"used": map[string]any{"cpu": "3"},
			},
		},
	}
	dc := newFakeDynClient(quota)
	client := newFakeClient(nil, dc)

	target := ConstraintTarget{
		Namespace:  "default",
		Containers: []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "500m"}}},
	}

	rows, err := client.quotaConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("quotaConstraintRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Source != "Quota" {
		t.Errorf("Source = %q, want Quota", row.Source)
	}
	if !strings.Contains(row.Headroom, "1") {
		t.Errorf("Headroom = %q, want to contain 1", row.Headroom)
	}
	if !strings.Contains(row.Detail, "500m") {
		t.Errorf("Detail = %q, want to name the request", row.Detail)
	}
}

func TestLimitRangeRows_FlagRequestBelowMin(t *testing.T) {
	lr := &corev1.LimitRange{
		Name: "container-limits", Namespace: "default",
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Min:  corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
				},
			},
		},
	}
	client := newFakeClient(k8sfake.NewClientset(lr), nil)

	target := ConstraintTarget{
		Namespace:  "default",
		Containers: []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "100m"}}},
	}

	rows, err := client.limitRangeConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("limitRangeConstraintRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %+v", len(rows), rows)
	}
	if !rows[0].Blocking {
		t.Error("expected row to be Blocking")
	}
}
