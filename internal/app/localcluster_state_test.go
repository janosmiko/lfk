package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalClusterState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	now := time.Date(2026, 5, 7, 8, 30, 0, 0, time.UTC)
	in := []localClusterCacheEntry{
		{
			Provider: "kind", Name: "dev", ContextName: "kind-dev",
			Status: "running", K8sVersion: "v1.30.0", Nodes: 1, LastSeen: now,
		},
	}
	if err := saveLocalClusterState(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out := loadLocalClusterState()
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(out))
	}
	got, ok := out["kind-dev"]
	if !ok {
		t.Fatal("missing kind-dev entry")
	}
	if got.Status != "running" || got.K8sVersion != "v1.30.0" || got.Nodes != 1 {
		t.Fatalf("round-trip lost data: %+v", got)
	}
}

func TestLocalClusterState_MissingFileReturnsEmpty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out := loadLocalClusterState()
	if out == nil {
		t.Fatal("loadLocalClusterState must return non-nil empty map")
	}
	if len(out) != 0 {
		t.Fatalf("expected empty map, got %v", out)
	}
}

func TestLocalClusterState_CorruptFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "lfk"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lfk", "local-clusters.yaml"), []byte("not yaml{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := loadLocalClusterState()
	if len(out) != 0 {
		t.Fatalf("corrupt should return empty, got %v", out)
	}
}

func TestLocalClusterState_FutureSchemaReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "lfk"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "schema_version: 999\nclusters: []\n"
	if err := os.WriteFile(filepath.Join(dir, "lfk", "local-clusters.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := loadLocalClusterState()
	if len(out) != 0 {
		t.Fatalf("future schema should return empty, got %v", out)
	}
}

func TestLocalClusterState_AtomicWriteNoTmpLeftover(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if err := saveLocalClusterState(nil); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "lfk", "*.tmp"))
	if len(matches) != 0 {
		t.Fatalf("expected no .tmp leftovers, got %v", matches)
	}
}

func TestLocalClusterState_DropsEntriesMissingKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "lfk"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `schema_version: 1
clusters:
  - provider: kind
    name: dev
    context_name: kind-dev
    status: running
  - provider: ""
    name: ""
    context_name: ""
    status: stopped
`
	if err := os.WriteFile(filepath.Join(dir, "lfk", "local-clusters.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out := loadLocalClusterState()
	if len(out) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(out))
	}
	if _, ok := out["kind-dev"]; !ok {
		t.Fatal("expected kind-dev present")
	}
}
