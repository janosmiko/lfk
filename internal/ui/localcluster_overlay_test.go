package ui

import (
	"strings"
	"testing"
)

func TestRenderLocalClusterOverlay_EmptyNoProviders(t *testing.T) {
	t.Parallel()
	out := RenderLocalClusterOverlay(LocalClusterOverlayState{
		ProvidersInstalled: nil,
		Width:              80, Height: 24,
	})
	if !strings.Contains(out, "No local-cluster provider installed") {
		t.Fatalf("missing install hint:\n%s", out)
	}
	if !strings.Contains(out, "kind") || !strings.Contains(out, "k3d") || !strings.Contains(out, "minikube") {
		t.Fatal("install hint must mention all three providers")
	}
}

func TestRenderLocalClusterOverlay_EmptyWithProvidersInstalled(t *testing.T) {
	t.Parallel()
	out := RenderLocalClusterOverlay(LocalClusterOverlayState{
		ProvidersInstalled: []string{"kind"},
		Clusters:           nil,
		Width:              80, Height: 24,
	})
	if !strings.Contains(out, "No local clusters yet") {
		t.Fatalf("missing empty hint:\n%s", out)
	}
	// Hints live in the bottom hint bar (overlay_hintbar.go), not in
	// the panel — the panel must not duplicate the keymap.
	if strings.Contains(out, "c create") || strings.Contains(out, "esc close") {
		t.Fatalf("empty panel must not duplicate hint-bar text:\n%s", out)
	}
}

func TestRenderLocalClusterOverlay_ListWithRows(t *testing.T) {
	t.Parallel()
	out := RenderLocalClusterOverlay(LocalClusterOverlayState{
		ProvidersInstalled: []string{"kind", "k3d"},
		Clusters: []LocalClusterRowView{
			{Provider: "kind", Name: "dev", Status: "running", K8sVersion: "v1.30.0", Nodes: 1, Age: "2d"},
			{Provider: "k3d", Name: "staging", Status: "stopped", K8sVersion: "v1.29.5", Nodes: 2, Age: "5h"},
		},
		Cursor: 1,
		Width:  100, Height: 24,
	})
	if !strings.Contains(out, "Local Clusters") {
		t.Fatal("missing title")
	}
	if !strings.Contains(out, "dev") || !strings.Contains(out, "staging") {
		t.Fatal("missing cluster names")
	}
	if !strings.Contains(out, "running") || !strings.Contains(out, "stopped") {
		t.Fatal("missing status text")
	}
}

func TestRenderLocalClusterOverlay_ListOmitsInlineHintBar(t *testing.T) {
	t.Parallel()
	// The keymap lives in the project-wide bottom status bar
	// (see overlay_hintbar.go::overlayHintBarOverlayLocalClusters),
	// not in the panel. Guard against regressing to an inline hint row.
	out := RenderLocalClusterOverlay(LocalClusterOverlayState{
		ProvidersInstalled: []string{"kind"},
		Clusters: []LocalClusterRowView{
			{Provider: "kind", Name: "dev", Status: "running"},
		},
		Width: 100, Height: 24,
	})
	if strings.Contains(out, "c create") || strings.Contains(out, "enter switch") {
		t.Fatalf("panel must not duplicate hint-bar text:\n%s", out)
	}
}

func TestRenderLocalClusterOverlay_TransitioningPill(t *testing.T) {
	t.Parallel()
	out := RenderLocalClusterOverlay(LocalClusterOverlayState{
		ProvidersInstalled: []string{"k3d"},
		Clusters: []LocalClusterRowView{
			{Provider: "k3d", Name: "staging", Status: "running", Mutating: "stopping..."},
		},
		Width: 100, Height: 24,
	})
	if !strings.Contains(out, "stopping") {
		t.Fatalf("transitioning pill missing:\n%s", out)
	}
}

func TestRenderLocalClusterOverlay_GlobalErrShown(t *testing.T) {
	t.Parallel()
	out := RenderLocalClusterOverlay(LocalClusterOverlayState{
		ProvidersInstalled: []string{"kind", "k3d"},
		Clusters: []LocalClusterRowView{
			{Provider: "kind", Name: "dev", Status: "running"},
		},
		GlobalErr: "k3d: list failed",
		Width:     100, Height: 24,
	})
	if !strings.Contains(out, "k3d: list failed") {
		t.Fatalf("global err missing:\n%s", out)
	}
}
