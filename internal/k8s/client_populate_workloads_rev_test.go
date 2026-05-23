package k8s

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

func TestPopulateDeploymentDetails_REVColumn(t *testing.T) {
	tests := []struct {
		name            string
		resourceVersion string
		want            string // "" means no column emitted
	}{
		{name: "small value", resourceVersion: "12345", want: "12345"},
		{name: "zero", resourceVersion: "0", want: "0"},
		{name: "large value", resourceVersion: "1048576", want: "1048576"},
		{name: "empty", resourceVersion: "", want: ""},
		{name: "non-numeric", resourceVersion: "abc", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			obj := map[string]any{
				"metadata": map[string]any{"resourceVersion": tt.resourceVersion},
				"spec":     map[string]any{"replicas": float64(1)},
				"status":   map[string]any{"readyReplicas": float64(1)},
			}
			populateResourceDetails(ti, obj, "Deployment")
			got := ""
			for _, kv := range ti.Columns {
				if kv.Key == "REV" {
					got = kv.Value
					break
				}
			}
			if got != tt.want {
				t.Fatalf("REV column = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPopulateStatefulSetDetails_REVColumn(t *testing.T) {
	ti := &model.Item{}
	obj := map[string]any{
		"metadata": map[string]any{"resourceVersion": "65535"},
		"spec":     map[string]any{"replicas": float64(1)},
		"status":   map[string]any{"readyReplicas": float64(1)},
	}
	populateResourceDetails(ti, obj, "StatefulSet")
	for _, kv := range ti.Columns {
		if kv.Key == "REV" {
			if kv.Value != "65535" {
				t.Fatalf("REV = %q, want 65535", kv.Value)
			}
			return
		}
	}
	t.Fatal("REV column not found on StatefulSet")
}

func TestPopulateDeploymentDetails_FixedColumns(t *testing.T) {
	tests := []struct {
		name    string
		status  map[string]any
		wantCol string
		wantVal string
	}{
		{
			name:    "Up-to-date emits from int64",
			status:  map[string]any{"readyReplicas": int64(3), "updatedReplicas": int64(2)},
			wantCol: "Up-to-date",
			wantVal: "2",
		},
		{
			name:    "Available emits from int64",
			status:  map[string]any{"readyReplicas": int64(3), "availableReplicas": int64(3)},
			wantCol: "Available",
			wantVal: "3",
		},
		{
			name:    "Unavailable emits when present and > 0",
			status:  map[string]any{"readyReplicas": int64(1), "unavailableReplicas": int64(2)},
			wantCol: "Unavailable",
			wantVal: "2",
		},
		{
			name:    "Unavailable suppressed when zero",
			status:  map[string]any{"readyReplicas": int64(3), "unavailableReplicas": int64(0)},
			wantCol: "Unavailable",
			wantVal: "", // not emitted
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			obj := map[string]any{
				"metadata": map[string]any{},
				"spec":     map[string]any{"replicas": int64(3)},
				"status":   tt.status,
			}
			populateResourceDetails(ti, obj, "Deployment")
			got := ""
			for _, kv := range ti.Columns {
				if kv.Key == tt.wantCol {
					got = kv.Value
					break
				}
			}
			if got != tt.wantVal {
				t.Fatalf("%q column = %q, want %q", tt.wantCol, got, tt.wantVal)
			}
		})
	}
}

func TestPopulateStatefulSetDetails_NewColumns(t *testing.T) {
	ti := &model.Item{}
	obj := map[string]any{
		"metadata": map[string]any{},
		"spec": map[string]any{
			"replicas": int64(3),
			"updateStrategy": map[string]any{
				"type": "RollingUpdate",
			},
		},
		"status": map[string]any{
			"readyReplicas":   int64(3),
			"updatedReplicas": int64(2),
			"currentRevision": "web-abc123",
			"updateRevision":  "web-def456",
		},
	}
	populateResourceDetails(ti, obj, "StatefulSet")
	expect := map[string]string{
		"Up-to-date":       "2",
		"Update Strategy":  "RollingUpdate",
		"Current Revision": "web-abc123",
		"Update Revision":  "web-def456",
	}
	for key, want := range expect {
		got := ""
		for _, kv := range ti.Columns {
			if kv.Key == key {
				got = kv.Value
				break
			}
		}
		if got != want {
			t.Fatalf("%q column = %q, want %q", key, got, want)
		}
	}
}

func TestPopulateDaemonSetDetails_NewColumns(t *testing.T) {
	ti := &model.Item{}
	obj := map[string]any{
		"metadata": map[string]any{"resourceVersion": "777"},
		"spec":     map[string]any{},
		"status": map[string]any{
			"desiredNumberScheduled": int64(3),
			"numberReady":            int64(3),
			"currentNumberScheduled": int64(3),
			"updatedNumberScheduled": int64(2),
			"numberAvailable":        int64(3),
			"numberMisscheduled":     int64(1),
		},
	}
	populateResourceDetails(ti, obj, "DaemonSet")
	expect := map[string]string{
		"Current":      "3",
		"Up-to-date":   "2",
		"Available":    "3",
		"Misscheduled": "1",
		"REV":          "777",
	}
	for key, want := range expect {
		got := ""
		for _, kv := range ti.Columns {
			if kv.Key == key {
				got = kv.Value
				break
			}
		}
		if got != want {
			t.Fatalf("%q column = %q, want %q", key, got, want)
		}
	}
}

func TestPopulateDaemonSetDetails_MisscheduledSuppressedWhenZero(t *testing.T) {
	ti := &model.Item{}
	obj := map[string]any{
		"metadata": map[string]any{},
		"spec":     map[string]any{},
		"status": map[string]any{
			"desiredNumberScheduled": int64(3),
			"numberReady":            int64(3),
			"numberMisscheduled":     int64(0),
		},
	}
	populateResourceDetails(ti, obj, "DaemonSet")
	for _, kv := range ti.Columns {
		if kv.Key == "Misscheduled" {
			t.Fatalf("Misscheduled column should be suppressed when zero, got %q", kv.Value)
		}
	}
}

func TestPopulateReplicaSetDetails_NewColumns(t *testing.T) {
	ti := &model.Item{}
	obj := map[string]any{
		"metadata": map[string]any{"resourceVersion": "888"},
		"spec":     map[string]any{"replicas": int64(5)},
		"status":   map[string]any{"readyReplicas": int64(3), "replicas": int64(5)},
	}
	populateResourceDetails(ti, obj, "ReplicaSet")
	expect := map[string]string{
		"Desired": "5",
		"REV":     "888",
	}
	for key, want := range expect {
		got := ""
		for _, kv := range ti.Columns {
			if kv.Key == key {
				got = kv.Value
				break
			}
		}
		if got != want {
			t.Fatalf("%q column = %q, want %q", key, got, want)
		}
	}
}
