package app

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinFilterPresets_UniversalPresetsAlwaysPresent(t *testing.T) {
	kinds := []string{
		"Pod", "Deployment", "Node", "Job", "CronJob", "Service",
		"Certificate", "Application", "HelmRelease", "Kustomization",
		"PersistentVolumeClaim", "Event", "ConfigMap", "Secret", "UnknownKind", "",
	}

	for _, kind := range kinds {
		presets := builtinFilterPresets(kind)
		found := map[string]bool{"Old (>30d)": false, "Recent (<1h)": false}
		for _, p := range presets {
			if _, ok := found[p.Name]; ok {
				found[p.Name] = true
			}
		}
		for name, present := range found {
			if !present {
				t.Errorf("kind=%q: universal preset %q not found", kind, name)
			}
		}
	}
}

func TestBuiltinFilterPresets_PodSpecific(t *testing.T) {
	presets := builtinFilterPresets("Pod")
	names := presetNames(presets)

	expected := []string{"Failing", "Pending", "Not Ready", "Restarting", "High Restarts"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("Pod presets missing %q", name)
		}
	}
}

func TestBuiltinFilterPresets_DeploymentSpecific(t *testing.T) {
	presets := builtinFilterPresets("Deployment")
	names := presetNames(presets)

	expected := []string{"Not Ready", "Failing"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("Deployment presets missing %q", name)
		}
	}

	// Should NOT have Pod-specific presets.
	unexpected := []string{"Pending", "Restarting", "High Restarts"}
	for _, name := range unexpected {
		if names[name] {
			t.Errorf("Deployment presets should not have %q", name)
		}
	}
}

func TestBuiltinFilterPresets_NodeSpecific(t *testing.T) {
	presets := builtinFilterPresets("Node")
	names := presetNames(presets)

	if !names["Not Ready"] {
		t.Error("Node presets missing 'Not Ready'")
	}
	if !names["Cordoned"] {
		t.Error("Node presets missing 'Cordoned'")
	}
}

func TestBuiltinFilterPresets_EventSpecific(t *testing.T) {
	presets := builtinFilterPresets("Event")
	names := presetNames(presets)

	if !names["Warnings"] {
		t.Error("Event presets missing 'Warnings'")
	}
}

func TestBuiltinFilterPresets_UnknownKind(t *testing.T) {
	presets := builtinFilterPresets("SomeCustomCRD")
	// Should only have universal presets.
	if len(presets) != 2 {
		t.Errorf("unknown kind should have 2 universal presets, got %d", len(presets))
	}
}

func TestBuiltinFilterPresets_UniqueKeys(t *testing.T) {
	kinds := []string{
		"Pod", "Deployment", "Node", "Job", "CronJob", "Service",
		"Certificate", "Application", "HelmRelease", "Kustomization",
		"PersistentVolumeClaim", "Event", "StatefulSet", "DaemonSet",
	}

	for _, kind := range kinds {
		presets := builtinFilterPresets(kind)
		seen := make(map[string]string)
		for _, p := range presets {
			if prev, exists := seen[p.Key]; exists {
				t.Errorf("kind=%q: duplicate key %q used by both %q and %q", kind, p.Key, prev, p.Name)
			}
			seen[p.Key] = p.Name
		}
	}
}

// --- MatchFn tests ---

func TestPodFailingPreset(t *testing.T) {
	presets := builtinFilterPresets("Pod")
	failing := findPreset(presets, "Failing")
	if failing == nil {
		t.Fatal("Failing preset not found")
	}

	tests := []struct {
		status string
		want   bool
	}{
		{"CrashLoopBackOff", true},
		{"Error", true},
		{"Running", false},
		{"ImagePullBackOff", true},
		{"OOMKilled", true},
		{"Evicted", true},
		{"Completed", false},
	}
	for _, tt := range tests {
		item := model.Item{Status: tt.status}
		if got := failing.MatchFn(item); got != tt.want {
			t.Errorf("Failing(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestReadyMismatch(t *testing.T) {
	tests := []struct {
		ready string
		want  bool
	}{
		{"2/3", true},
		{"3/3", false},
		{"0/1", true},
		{"", false},
		{"1", false},
	}
	for _, tt := range tests {
		item := model.Item{Ready: tt.ready}
		if got := matchReadyMismatch(item); got != tt.want {
			t.Errorf("matchReadyMismatch(%q) = %v, want %v", tt.ready, got, tt.want)
		}
	}
}

func TestHighRestarts(t *testing.T) {
	fn := matchRestartsGt(10)
	tests := []struct {
		restarts string
		want     bool
	}{
		{"", false},
		{"0", false},
		{"10", false},
		{"11", true},
		{"100", true},
		{"abc", false},
	}
	for _, tt := range tests {
		item := model.Item{Restarts: tt.restarts}
		if got := fn(item); got != tt.want {
			t.Errorf("matchRestartsGt(10)(%q) = %v, want %v", tt.restarts, got, tt.want)
		}
	}
}

func TestNodeCordonedPreset(t *testing.T) {
	presets := builtinFilterPresets("Node")
	cordoned := findPreset(presets, "Cordoned")
	if cordoned == nil {
		t.Fatal("Cordoned preset not found")
	}

	tests := []struct {
		status string
		want   bool
	}{
		{"Ready,SchedulingDisabled", true},
		{"Ready", false},
		{"NotReady,SchedulingDisabled", true},
	}
	for _, tt := range tests {
		item := model.Item{Status: tt.status}
		if got := cordoned.MatchFn(item); got != tt.want {
			t.Errorf("Cordoned(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestEventWarningsPreset(t *testing.T) {
	presets := builtinFilterPresets("Event")
	warnings := findPreset(presets, "Warnings")
	if warnings == nil {
		t.Fatal("Warnings preset not found")
	}

	item := model.Item{Status: "Warning"}
	if !warnings.MatchFn(item) {
		t.Error("expected Warning event to match")
	}
	item.Status = "Normal"
	if warnings.MatchFn(item) {
		t.Error("expected Normal event not to match")
	}
}

func TestUniversalOldPreset(t *testing.T) {
	presets := builtinFilterPresets("Pod")
	old := findPreset(presets, "Old (>30d)")
	if old == nil {
		t.Fatal("Old preset not found")
	}

	recent := model.Item{CreatedAt: time.Now().Add(-1 * time.Hour)}
	if old.MatchFn(recent) {
		t.Error("1-hour-old item should not match Old (>30d)")
	}

	ancient := model.Item{CreatedAt: time.Now().Add(-60 * 24 * time.Hour)}
	if !old.MatchFn(ancient) {
		t.Error("60-day-old item should match Old (>30d)")
	}

	zero := model.Item{}
	if old.MatchFn(zero) {
		t.Error("item with zero timestamp should not match Old (>30d)")
	}
}

func TestColumnValue(t *testing.T) {
	item := model.Item{
		Columns: []model.KeyValue{
			{Key: "Node", Value: "worker-1"},
			{Key: "Type", Value: "LoadBalancer"},
		},
	}
	if v := columnValue(item, "node"); v != "worker-1" {
		t.Errorf("columnValue(node) = %q, want %q", v, "worker-1")
	}
	if v := columnValue(item, "Type"); v != "LoadBalancer" {
		t.Errorf("columnValue(Type) = %q, want %q", v, "LoadBalancer")
	}
	if v := columnValue(item, "missing"); v != "" {
		t.Errorf("columnValue(missing) = %q, want empty", v)
	}
}

func TestServiceLBNoIPPreset(t *testing.T) {
	presets := builtinFilterPresets("Service")
	lb := findPreset(presets, "LB No IP")
	if lb == nil {
		t.Fatal("LB No IP preset not found")
	}

	tests := []struct {
		name    string
		columns []model.KeyValue
		want    bool
	}{
		{"LB with pending IP", []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "<pending>"}}, true},
		{"LB with no IP", []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "<none>"}}, true},
		{"LB with IP", []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "1.2.3.4"}}, false},
		{"ClusterIP", []model.KeyValue{{Key: "Type", Value: "ClusterIP"}}, false},
	}
	for _, tt := range tests {
		item := model.Item{Columns: tt.columns}
		if got := lb.MatchFn(item); got != tt.want {
			t.Errorf("LB No IP(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestConfigPresets(t *testing.T) {
	// Save and restore global state.
	origPresets := ui.ConfigFilterPresets
	defer func() { ui.ConfigFilterPresets = origPresets }()

	ui.ConfigFilterPresets = map[string][]ui.ConfigFilterPreset{
		"pod": {
			{Name: "GPU Pods", Key: "g", Match: ui.ConfigFilterMatch{
				Column: "Node", ColumnValue: "gpu",
			}},
		},
	}

	presets := builtinFilterPresets("Pod")
	found := false
	for _, p := range presets {
		if p.Name == "GPU Pods" {
			found = true
			// Test the match function.
			item := model.Item{Columns: []model.KeyValue{{Key: "Node", Value: "gpu-worker-1"}}}
			if !p.MatchFn(item) {
				t.Error("expected GPU Pods to match item on gpu node")
			}
			noMatch := model.Item{Columns: []model.KeyValue{{Key: "Node", Value: "cpu-worker-1"}}}
			if p.MatchFn(noMatch) {
				t.Error("expected GPU Pods not to match item on cpu node")
			}
		}
	}
	if !found {
		t.Error("config preset 'GPU Pods' not found in presets")
	}
}

func TestBuildConfigMatchFn(t *testing.T) {
	t.Run("status match", func(t *testing.T) {
		fn := buildConfigMatchFn(ui.ConfigFilterMatch{Status: "error"})
		if !fn(model.Item{Status: "Error"}) {
			t.Error("expected status 'Error' to match 'error'")
		}
		if fn(model.Item{Status: "Running"}) {
			t.Error("expected status 'Running' not to match 'error'")
		}
	})

	t.Run("ready_not match", func(t *testing.T) {
		fn := buildConfigMatchFn(ui.ConfigFilterMatch{ReadyNot: true})
		if !fn(model.Item{Ready: "1/3"}) {
			t.Error("expected 1/3 to match ready_not")
		}
		if fn(model.Item{Ready: "3/3"}) {
			t.Error("expected 3/3 not to match ready_not")
		}
	})

	t.Run("restarts_gt match", func(t *testing.T) {
		fn := buildConfigMatchFn(ui.ConfigFilterMatch{RestartsGt: 5})
		if !fn(model.Item{Restarts: "10"}) {
			t.Error("expected restarts 10 to match > 5")
		}
		if fn(model.Item{Restarts: "3"}) {
			t.Error("expected restarts 3 not to match > 5")
		}
	})

	t.Run("AND logic", func(t *testing.T) {
		fn := buildConfigMatchFn(ui.ConfigFilterMatch{
			Status:   "error",
			ReadyNot: true,
		})
		// Both conditions must hold.
		if !fn(model.Item{Status: "Error", Ready: "0/1"}) {
			t.Error("expected both conditions to match")
		}
		if fn(model.Item{Status: "Error", Ready: "1/1"}) {
			t.Error("expected ready_not to fail")
		}
		if fn(model.Item{Status: "Running", Ready: "0/1"}) {
			t.Error("expected status to fail")
		}
	})
}

// --- helpers ---

func presetNames(presets []FilterPreset) map[string]bool {
	m := make(map[string]bool, len(presets))
	for _, p := range presets {
		m[p.Name] = true
	}
	return m
}

func findPreset(presets []FilterPreset, name string) *FilterPreset {
	for i := range presets {
		if presets[i].Name == name {
			return &presets[i]
		}
	}
	return nil
}

func TestCovFilterPresetsServiceLBNoIP(t *testing.T) {
	presets := builtinFilterPresets("Service")
	lbNoIP := findPreset(presets, "LB No IP")
	require.NotNil(t, lbNoIP)

	assert.True(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "<pending>"}}}))
	assert.True(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "<none>"}}}))
	assert.True(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: ""}}}))
	assert.False(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "LoadBalancer"}, {Key: "External-IP", Value: "1.2.3.4"}}}))
	assert.False(t, lbNoIP.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Type", Value: "ClusterIP"}}}))
}

func TestCovFilterPresetsNodeCordoned(t *testing.T) {
	presets := builtinFilterPresets("Node")
	cordoned := findPreset(presets, "Cordoned")
	require.NotNil(t, cordoned)
	assert.True(t, cordoned.MatchFn(model.Item{Status: "Ready,SchedulingDisabled"}))
	assert.False(t, cordoned.MatchFn(model.Item{Status: "Ready"}))
}

func TestCovFilterPresetsEventWarnings(t *testing.T) {
	presets := builtinFilterPresets("Event")
	warnings := findPreset(presets, "Warnings")
	require.NotNil(t, warnings)
	assert.True(t, warnings.MatchFn(model.Item{Status: "Warning"}))
	assert.False(t, warnings.MatchFn(model.Item{Status: "Normal"}))
}

func TestCovFilterPresetsCertExpiring(t *testing.T) {
	presets := builtinFilterPresets("Certificate")
	expiring := findPreset(presets, "Expiring Soon")
	require.NotNil(t, expiring)

	in15d := time.Now().Add(15 * 24 * time.Hour).Format(time.RFC3339)
	assert.True(t, expiring.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Expires", Value: in15d}}}))

	in60d := time.Now().Add(60 * 24 * time.Hour).Format(time.RFC3339)
	assert.False(t, expiring.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Expires", Value: in60d}}}))
	assert.False(t, expiring.MatchFn(model.Item{}))

	in10d := time.Now().Add(10 * 24 * time.Hour).Format("2006-01-02")
	assert.True(t, expiring.MatchFn(model.Item{Columns: []model.KeyValue{{Key: "Not After", Value: in10d}}}))
}

func TestCovBuildConfigMatchFnCombined(t *testing.T) {
	fn := buildConfigMatchFn(ui.ConfigFilterMatch{Status: "error", ReadyNot: true})
	assert.True(t, fn(model.Item{Status: "Error", Ready: "0/1"}))
	assert.False(t, fn(model.Item{Status: "Error", Ready: "1/1"}))
	assert.False(t, fn(model.Item{Status: "Running", Ready: "0/1"}))
}

func TestCovBuildConfigMatchFnColumnNoValue(t *testing.T) {
	fn := buildConfigMatchFn(ui.ConfigFilterMatch{Column: "IP"})
	assert.True(t, fn(model.Item{Columns: []model.KeyValue{{Key: "IP", Value: "10.0.0.1"}}}))
	assert.False(t, fn(model.Item{Columns: []model.KeyValue{{Key: "IP", Value: ""}}}))
}

func TestCovBuildConfigMatchFnRestartsGt(t *testing.T) {
	fn := buildConfigMatchFn(ui.ConfigFilterMatch{RestartsGt: 5})
	assert.True(t, fn(model.Item{Restarts: "10"}))
	assert.False(t, fn(model.Item{Restarts: "3"}))
	assert.False(t, fn(model.Item{Restarts: ""}))
}
