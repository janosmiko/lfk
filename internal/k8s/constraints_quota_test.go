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

func TestQuotaRows_EffectiveRequestIsMaxOfContainerSumAndInitContainerMax(t *testing.T) {
	quota := scopedQuotaObject(map[string]any{
		"hard": map[string]any{"cpu": "10"},
	}, map[string]any{"used": map[string]any{"cpu": "0"}})

	tests := []struct {
		name           string
		containers     []ContainerRequest
		initContainers []ContainerRequest
		wantAsked      string
	}{
		{
			name:           "container sum exceeds any single init container",
			containers:     []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "500m"}}},
			initContainers: []ContainerRequest{{Name: "init", Requests: map[string]string{"cpu": "200m"}}},
			wantAsked:      "500m",
		},
		{
			name:           "a single init container exceeds the container sum",
			containers:     []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "500m"}}},
			initContainers: []ContainerRequest{{Name: "init", Requests: map[string]string{"cpu": "2"}}},
			wantAsked:      "2",
		},
		{
			name:       "init containers use max, not sum, across each other",
			containers: []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "100m"}}},
			initContainers: []ContainerRequest{
				{Name: "init-a", Requests: map[string]string{"cpu": "300m"}},
				{Name: "init-b", Requests: map[string]string{"cpu": "1"}},
			},
			wantAsked: "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(nil, newFakeDynClient(quota))
			target := ConstraintTarget{
				Namespace:      "default",
				Containers:     tt.containers,
				InitContainers: tt.initContainers,
			}

			rows, err := client.quotaConstraintRows(t.Context(), "", target)
			if err != nil {
				t.Fatalf("quotaConstraintRows: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d: %+v", len(rows), rows)
			}
			if !strings.Contains(rows[0].Detail, tt.wantAsked+" cpu") {
				t.Errorf("Detail = %q, want the effective request %s", rows[0].Detail, tt.wantAsked)
			}
		})
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

func TestLimitRangeRows_ChecksInitContainers(t *testing.T) {
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
		Namespace:      "default",
		Containers:     []ContainerRequest{{Name: "app", Requests: map[string]string{"cpu": "500m"}}},
		InitContainers: []ContainerRequest{{Name: "init", Requests: map[string]string{"cpu": "100m"}}},
	}

	rows, err := client.limitRangeConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("limitRangeConstraintRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row for the violating init container, got %d: %+v", len(rows), rows)
	}
	if !strings.Contains(rows[0].Detail, "init") {
		t.Errorf("Detail = %q, want it to name the init container", rows[0].Detail)
	}
}

func TestLimitRangeRows_ChecksRequestsAndLimits(t *testing.T) {
	lr := corev1.LimitRange{
		Name: "container-limits", Namespace: "default",
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Min:  corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
					Max:  corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
				},
			},
		},
	}

	tests := []struct {
		name          string
		container     ContainerRequest
		wantFields    []string
		wantRowsCount int
	}{
		{
			name:          "request below min only",
			container:     ContainerRequest{Name: "app", Requests: map[string]string{"cpu": "100m"}},
			wantFields:    []string{"requests"},
			wantRowsCount: 1,
		},
		{
			name:          "limit above max only",
			container:     ContainerRequest{Name: "app", Limits: map[string]string{"cpu": "2"}},
			wantFields:    []string{"limits"},
			wantRowsCount: 1,
		},
		{
			name: "request below min and limit above max both reported",
			container: ContainerRequest{
				Name:     "app",
				Requests: map[string]string{"cpu": "100m"},
				Limits:   map[string]string{"cpu": "2"},
			},
			wantFields:    []string{"requests", "limits"},
			wantRowsCount: 2,
		},
		{
			name:          "requests and limits within bounds produce no rows",
			container:     ContainerRequest{Name: "app", Requests: map[string]string{"cpu": "500m"}, Limits: map[string]string{"cpu": "500m"}},
			wantRowsCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(k8sfake.NewClientset(&lr), nil)
			target := ConstraintTarget{Namespace: "default", Containers: []ContainerRequest{tt.container}}

			rows, err := client.limitRangeConstraintRows(t.Context(), "", target)
			if err != nil {
				t.Fatalf("limitRangeConstraintRows: %v", err)
			}
			if len(rows) != tt.wantRowsCount {
				t.Fatalf("expected %d rows, got %d: %+v", tt.wantRowsCount, len(rows), rows)
			}
			for _, field := range tt.wantFields {
				var found bool
				for _, r := range rows {
					if strings.Contains(r.Detail, field) {
						found = true
					}
				}
				if !found {
					t.Errorf("expected a row mentioning %q, got %+v", field, rows)
				}
			}
		})
	}
}

func TestLimitRangeRows_FlagsMaxLimitRequestRatio(t *testing.T) {
	lr := &corev1.LimitRange{
		Name: "container-limits", Namespace: "default",
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type:                 corev1.LimitTypeContainer,
					MaxLimitRequestRatio: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
				},
			},
		},
	}

	tests := []struct {
		name          string
		container     ContainerRequest
		wantRowsCount int
	}{
		{
			name: "ratio within bound produces no row",
			container: ContainerRequest{
				Name: "app", Requests: map[string]string{"cpu": "500m"}, Limits: map[string]string{"cpu": "1"},
			},
			wantRowsCount: 0,
		},
		{
			name: "ratio exceeding bound is flagged",
			container: ContainerRequest{
				Name: "app", Requests: map[string]string{"cpu": "100m"}, Limits: map[string]string{"cpu": "1"},
			},
			wantRowsCount: 1,
		},
		{
			name: "missing request skips the ratio check",
			container: ContainerRequest{
				Name: "app", Limits: map[string]string{"cpu": "1"},
			},
			wantRowsCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(k8sfake.NewClientset(lr), nil)
			target := ConstraintTarget{Namespace: "default", Containers: []ContainerRequest{tt.container}}

			rows, err := client.limitRangeConstraintRows(t.Context(), "", target)
			if err != nil {
				t.Fatalf("limitRangeConstraintRows: %v", err)
			}
			if len(rows) != tt.wantRowsCount {
				t.Fatalf("expected %d rows, got %d: %+v", tt.wantRowsCount, len(rows), rows)
			}
			if tt.wantRowsCount > 0 && !rows[0].Blocking {
				t.Error("expected row to be Blocking")
			}
		})
	}
}
