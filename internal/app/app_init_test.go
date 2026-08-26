package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestNewModel_StartsInLoadingState guards against the startup empty-state
// flash. Init() dispatches loadContexts() asynchronously, so Bubbletea renders
// the first frame before any contextsLoadedMsg arrives. If the model is
// constructed with loading=false, that frame falls through to the empty-state
// messages ("No items", "No resource types found") until the reply lands.
// Constructing with loading=true makes the first render show the spinner; the
// loaded-message handlers (updateContextsLoaded, updateResourceTypes) clear the
// flag once real data arrives.
func TestNewModel_StartsInLoadingState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	if !m.loading {
		t.Fatal("NewModel must start in the loading state so the first render " +
			"shows the spinner instead of flashing the empty-state messages")
	}
}

// show_rare_types: true makes the full resource-type list visible from launch,
// so the user does not have to press H every start (issue #321).
func TestNewModel_SeedsShowRareFromConfig(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	origCfg := ui.ConfigShowRareTypes
	origGlobal := model.ShowRareResources
	defer func() {
		ui.ConfigShowRareTypes = origCfg
		model.ShowRareResources = origGlobal
	}()

	ui.ConfigShowRareTypes = true
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	if !m.showRareResources {
		t.Fatal("show_rare_types: true must seed m.showRareResources")
	}
	if !model.ShowRareResources {
		t.Fatal("show_rare_types: true must seed the model.ShowRareResources global the sidebar reads")
	}

	ui.ConfigShowRareTypes = false
	m = NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	if m.showRareResources {
		t.Fatal("show_rare_types: false (default) must leave rare types hidden")
	}
}

// security.hide_badges seeds m.hideSecurityBadges so the per-row SEC badge is
// suppressed from launch (the user can still toggle it back on with B).
func TestNewModel_SeedsHideSecurityBadgesFromConfig(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	orig := ui.ConfigSecurityHideBadges
	defer func() { ui.ConfigSecurityHideBadges = orig }()

	ui.ConfigSecurityHideBadges = true
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	if !m.hideSecurityBadges {
		t.Fatal("security.hide_badges: true must seed m.hideSecurityBadges")
	}

	ui.ConfigSecurityHideBadges = false
	m = NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	if m.hideSecurityBadges {
		t.Fatal("security.hide_badges: false (default) must leave badges shown")
	}
}

// A nil map here makes allowSparklineFetch's pointer-receiver stamp land on a
// discarded copy through the value-receiver call chain, so the cluster range
// query fires unthrottled on every watch tick (metrics_throttle.go).
func TestNewModel_InitializesMetricsLastFetch(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	if m.metricsLastFetch == nil {
		t.Fatal("NewModel must initialize metricsLastFetch, or the metrics throttle stamps a discarded copy")
	}
}

// recomputeReadOnly runs on every context switch and must resolve the badge
// default per context: a per-context config override beats the global default,
// and a runtime toggle (recorded per context) beats config and never leaks to
// another context.
func TestRecomputeHideSecurityBadgesPerContext(t *testing.T) {
	origGlobal := ui.ConfigSecurityHideBadges
	origCluster := ui.ConfigClusterSecurityHideBadges
	defer func() {
		ui.ConfigSecurityHideBadges = origGlobal
		ui.ConfigClusterSecurityHideBadges = origCluster
	}()
	ui.ConfigSecurityHideBadges = false
	ui.ConfigClusterSecurityHideBadges = map[string]bool{"prod": true}

	m := baseModelWithFakeClient()
	m.contextBadgeOverrides = make(map[string]bool)

	m.recomputeReadOnly("prod")
	if !m.hideSecurityBadges {
		t.Error("prod (per-context config true) must hide badges on context entry")
	}
	m.recomputeReadOnly("dev")
	if m.hideSecurityBadges {
		t.Error("dev (no override, global false) must show badges")
	}

	// A runtime toggle in dev sticks for dev only.
	m.contextBadgeOverrides["dev"] = true
	m.recomputeReadOnly("dev")
	if !m.hideSecurityBadges {
		t.Error("dev session override (true) must win over global config")
	}
	m.recomputeReadOnly("prod")
	if !m.hideSecurityBadges {
		t.Error("prod config still applies; dev override must not leak")
	}
}
