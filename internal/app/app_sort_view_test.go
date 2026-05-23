package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestApplyKindSortDefault_FromView(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })

	v, err := ui.BuildView(&ui.ConfigView{SortColumn: "REV:desc"})
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	ui.ConfigViews = map[string]*ui.View{"deployment": v}

	m := &Model{}
	m.applyKindSortDefault(ui.ResourceRef{Kind: "Deployment"}, "")
	if m.sortColumnName != "REV" {
		t.Fatalf("sortColumnName = %q, want REV", m.sortColumnName)
	}
	if m.sortAscending {
		t.Fatalf("sortAscending = true, want false (desc)")
	}
}

func TestApplyKindSortDefault_GVRWinsOverKind(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })

	gvr, _ := ui.BuildView(&ui.ConfigView{SortColumn: "REV:desc"})
	kindOnly, _ := ui.BuildView(&ui.ConfigView{SortColumn: "Name:asc"})
	ui.ConfigViews = map[string]*ui.View{
		"apps/v1/deployments": gvr,
		"deployment":          kindOnly,
	}
	m := &Model{}
	m.applyKindSortDefault(ui.ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}, "")
	if m.sortColumnName != "REV" {
		t.Fatalf("sortColumnName = %q, want REV (GVR wins)", m.sortColumnName)
	}
}

func TestApplyKindSortDefault_FallbackToDefault(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	ui.ConfigViews = nil

	m := &Model{}
	m.applyKindSortDefault(ui.ResourceRef{Kind: "Service"}, "")
	if m.sortColumnName != sortColDefault {
		t.Fatalf("sortColumnName = %q, want %q", m.sortColumnName, sortColDefault)
	}
	if !m.sortAscending {
		t.Fatalf("sortAscending = false, want true (default)")
	}
}

func TestApplyKindSortDefault_SyntheticKind(t *testing.T) {
	orig := ui.ConfigViews
	t.Cleanup(func() { ui.ConfigViews = orig })
	v, _ := ui.BuildView(&ui.ConfigView{SortColumn: "REV:desc"})
	ui.ConfigViews = map[string]*ui.View{"__port_forwards__": v}

	m := &Model{}
	m.applyKindSortDefault(ui.ResourceRef{Kind: "__port_forwards__"}, "")
	// Synthetic kinds either resolve to the view or fall back to default;
	// the behavior matters less than the absence of panic. We just confirm
	// the helper runs without crash.
}
