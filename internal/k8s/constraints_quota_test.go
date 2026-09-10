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

func TestQuotaRows_LimitsResourceCountsContainerLimits(t *testing.T) {
	quota := scopedQuotaObject(map[string]any{
		"hard": map[string]any{"limits.cpu": "4", "requests.cpu": "4"},
	}, map[string]any{"used": map[string]any{"limits.cpu": "0", "requests.cpu": "0"}})
	client := newFakeClient(nil, newFakeDynClient(quota))

	target := ConstraintTarget{
		Namespace: "default",
		Containers: []ContainerRequest{{
			Name:     "app",
			Requests: map[string]string{"cpu": "500m"},
			Limits:   map[string]string{"cpu": "2"},
		}},
	}

	rows, err := client.quotaConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("quotaConstraintRows: %v", err)
	}

	byResource := map[string]string{}
	for _, r := range rows {
		switch {
		case strings.Contains(r.Detail, "limits.cpu"):
			byResource["limits.cpu"] = r.Detail
		case strings.Contains(r.Detail, "requests.cpu"):
			byResource["requests.cpu"] = r.Detail
		}
	}
	if got := byResource["limits.cpu"]; !strings.Contains(got, "limits 2 ") {
		t.Errorf("limits.cpu row = %q, want the container limit of 2", got)
	}
	if got := byResource["requests.cpu"]; !strings.Contains(got, "requests 500m ") {
		t.Errorf("requests.cpu row = %q, want the container request of 500m", got)
	}
}

func TestQuotaAppliesTo_Scopes(t *testing.T) {
	guaranteed := ConstraintTarget{
		PriorityClassName: "high",
		Containers:        []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "500m"}}},
	}
	bestEffort := ConstraintTarget{Containers: []ContainerRequest{{Name: "app"}}}

	tests := []struct {
		name   string
		quota  QuotaInfo
		target ConstraintTarget
		want   bool
	}{
		{"no scopes applies to everything", QuotaInfo{}, guaranteed, true},
		{
			"priorityClass In naming another class",
			QuotaInfo{ScopeSelector: []QuotaScopeRequirement{
				{ScopeName: "PriorityClass", Operator: "In", Values: []string{"low"}},
			}},
			guaranteed, false,
		},
		{
			"priorityClass In naming the target's class",
			QuotaInfo{ScopeSelector: []QuotaScopeRequirement{
				{ScopeName: "PriorityClass", Operator: "In", Values: []string{"high"}},
			}},
			guaranteed, true,
		},
		{
			"priorityClass NotIn naming the target's class",
			QuotaInfo{ScopeSelector: []QuotaScopeRequirement{
				{ScopeName: "PriorityClass", Operator: "NotIn", Values: []string{"high"}},
			}},
			guaranteed, false,
		},
		{"BestEffort scope against a pod with requests", QuotaInfo{Scopes: []string{"BestEffort"}}, guaranteed, false},
		{"BestEffort scope against a pod without any", QuotaInfo{Scopes: []string{"BestEffort"}}, bestEffort, true},
		{"NotBestEffort scope against a pod with requests", QuotaInfo{Scopes: []string{"NotBestEffort"}}, guaranteed, true},
		{"PriorityClass scope with no class set", QuotaInfo{Scopes: []string{"PriorityClass"}}, bestEffort, false},
		{"unevaluated scope stays applicable", QuotaInfo{Scopes: []string{"Terminating"}}, guaranteed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quotaAppliesTo(tt.quota, tt.target); got != tt.want {
				t.Errorf("quotaAppliesTo = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuotaRows_SkipQuotaScopedToAnotherPriorityClass(t *testing.T) {
	quota := scopedQuotaObject(map[string]any{
		"hard": map[string]any{"cpu": "4"},
		"scopeSelector": map[string]any{
			"matchExpressions": []any{
				map[string]any{"scopeName": "PriorityClass", "operator": "In", "values": []any{"low"}},
			},
		},
	}, map[string]any{"used": map[string]any{"cpu": "0"}})
	client := newFakeClient(nil, newFakeDynClient(quota))

	target := ConstraintTarget{
		Namespace:         "default",
		PriorityClassName: "high",
		Containers:        []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "500m"}}},
	}

	rows, err := client.quotaConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("quotaConstraintRows: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected no rows for a quota scoped to another PriorityClass, got %+v", rows)
	}
}

func scopedQuotaObject(spec, status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ResourceQuota",
			"metadata":   map[string]any{"name": "compute-quota", "namespace": "default"},
			"spec":       spec,
			"status":     status,
		},
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
