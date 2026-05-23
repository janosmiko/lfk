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

func TestPopulateJobDetails_Columns(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]any
		wantReady string
		wantStat  string
		wantCols  map[string]string
		notCols   []string
	}{
		{
			name: "completed job",
			obj: map[string]any{
				"metadata": map[string]any{"resourceVersion": "1234"},
				"spec":     map[string]any{"completions": int64(3)},
				"status": map[string]any{
					"succeeded":      int64(3),
					"failed":         int64(0),
					"startTime":      "2026-01-01T00:00:00Z",
					"completionTime": "2026-01-01T00:05:00Z",
					"conditions": []any{
						map[string]any{"type": "Complete", "status": "True"},
					},
				},
			},
			wantReady: "3/3",
			wantStat:  "Complete",
			wantCols: map[string]string{
				"Completions": "3",
				"Succeeded":   "3",
				"Duration":    "5m0s",
				"REV":         "1234",
			},
			notCols: []string{"Failed", "Active"},
		},
		{
			name: "failed job with active",
			obj: map[string]any{
				"metadata": map[string]any{},
				"spec":     map[string]any{"completions": float64(5)},
				"status": map[string]any{
					"succeeded": float64(1),
					"failed":    float64(2),
					"active":    float64(1),
					"conditions": []any{
						map[string]any{"type": "Failed", "status": "True"},
					},
				},
			},
			wantReady: "1/5",
			wantStat:  "Failed",
			wantCols: map[string]string{
				"Completions": "5",
				"Succeeded":   "1",
				"Failed":      "2",
				"Active":      "1",
			},
		},
		{
			name: "suspended job",
			obj: map[string]any{
				"metadata": map[string]any{},
				"spec":     map[string]any{"completions": int64(1), "suspend": true},
				"status":   map[string]any{},
			},
			wantStat: "Suspended",
			wantCols: map[string]string{
				"Suspend":     "true",
				"Completions": "1",
			},
		},
		{
			name: "running job",
			obj: map[string]any{
				"metadata": map[string]any{},
				"spec":     map[string]any{"completions": int64(3)},
				"status":   map[string]any{"active": int64(2), "succeeded": int64(1)},
			},
			wantReady: "1/3",
			wantStat:  "Running",
			wantCols: map[string]string{
				"Active":      "2",
				"Succeeded":   "1",
				"Completions": "3",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetails(ti, tt.obj, "Job")
			if tt.wantReady != "" && ti.Ready != tt.wantReady {
				t.Errorf("Ready = %q, want %q", ti.Ready, tt.wantReady)
			}
			if ti.Status != tt.wantStat {
				t.Errorf("Status = %q, want %q", ti.Status, tt.wantStat)
			}
			for key, want := range tt.wantCols {
				got := ""
				for _, kv := range ti.Columns {
					if kv.Key == key {
						got = kv.Value
						break
					}
				}
				if got != want {
					t.Errorf("%q column = %q, want %q", key, got, want)
				}
			}
			for _, notKey := range tt.notCols {
				for _, kv := range ti.Columns {
					if kv.Key == notKey {
						t.Errorf("%q column should be absent, got %q", notKey, kv.Value)
					}
				}
			}
		})
	}
}

func TestPopulateCronJobDetails_NewColumns(t *testing.T) {
	t.Run("active count emitted", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]any{
			"metadata": map[string]any{"resourceVersion": "555"},
			"spec":     map[string]any{"schedule": "*/5 * * * *"},
			"status": map[string]any{
				"active": []any{
					map[string]any{"name": "job-1"},
					map[string]any{"name": "job-2"},
				},
			},
		}
		populateResourceDetails(ti, obj, "CronJob")
		got := ""
		for _, kv := range ti.Columns {
			if kv.Key == "Active" {
				got = kv.Value
				break
			}
		}
		if got != "2" {
			t.Fatalf("Active column = %q, want 2", got)
		}
		gotREV := ""
		for _, kv := range ti.Columns {
			if kv.Key == "REV" {
				gotREV = kv.Value
				break
			}
		}
		if gotREV != "555" {
			t.Fatalf("REV = %q, want 555", gotREV)
		}
	})

	t.Run("active suppressed when empty", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]any{
			"metadata": map[string]any{},
			"spec":     map[string]any{"schedule": "*/5 * * * *"},
			"status":   map[string]any{"active": []any{}},
		}
		populateResourceDetails(ti, obj, "CronJob")
		for _, kv := range ti.Columns {
			if kv.Key == "Active" {
				t.Fatalf("Active column should be suppressed when empty, got %q", kv.Value)
			}
		}
	})
}

func TestProgressingColumn_Workloads(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		metadata  map[string]any
		status    map[string]any
		spec      map[string]any
		wantValue string // "" means column should be absent
	}{
		{
			name:      "Deployment in sync — no Progressing column",
			kind:      "Deployment",
			metadata:  map[string]any{"generation": int64(3)},
			status:    map[string]any{"observedGeneration": int64(3), "readyReplicas": int64(2)},
			spec:      map[string]any{"replicas": int64(2)},
			wantValue: "",
		},
		{
			name:      "Deployment with newer generation — column shows desired gen",
			kind:      "Deployment",
			metadata:  map[string]any{"generation": int64(5)},
			status:    map[string]any{"observedGeneration": int64(3), "readyReplicas": int64(2)},
			spec:      map[string]any{"replicas": int64(2)},
			wantValue: "5",
		},
		{
			name:      "StatefulSet without observedGeneration — column shows desired gen",
			kind:      "StatefulSet",
			metadata:  map[string]any{"generation": int64(1)},
			status:    map[string]any{"readyReplicas": int64(0)},
			spec:      map[string]any{"replicas": int64(2)},
			wantValue: "1",
		},
		{
			name:      "DaemonSet in sync",
			kind:      "DaemonSet",
			metadata:  map[string]any{"generation": int64(2)},
			status:    map[string]any{"observedGeneration": int64(2), "desiredNumberScheduled": int64(1), "numberReady": int64(1)},
			spec:      map[string]any{},
			wantValue: "",
		},
		{
			name:      "ReplicaSet rolling — Progressing shown",
			kind:      "ReplicaSet",
			metadata:  map[string]any{"generation": int64(4)},
			status:    map[string]any{"observedGeneration": int64(3), "readyReplicas": int64(0)},
			spec:      map[string]any{"replicas": int64(3)},
			wantValue: "4",
		},
		{
			name:      "Deployment with no generation field — column absent",
			kind:      "Deployment",
			metadata:  map[string]any{},
			status:    map[string]any{"readyReplicas": int64(1)},
			spec:      map[string]any{"replicas": int64(1)},
			wantValue: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			obj := map[string]any{
				"metadata": tt.metadata,
				"spec":     tt.spec,
				"status":   tt.status,
			}
			populateResourceDetails(ti, obj, tt.kind)
			got := ""
			for _, kv := range ti.Columns {
				if kv.Key == "Progressing" {
					got = kv.Value
					break
				}
			}
			if got != tt.wantValue {
				t.Fatalf("Progressing = %q, want %q", got, tt.wantValue)
			}
		})
	}
}
