package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// allActionMenuItems collects every ActionMenuItem produced by every menu
// builder in internal/model, across a representative kind from each branch
// of ActionsForKind's internal routing (core, workload, gitops, cert-manager,
// karpenter, knative, and the unrecognized-kind fallback).
func allActionMenuItems() []model.ActionMenuItem {
	kinds := []string{
		"Pod", "Deployment", "Service", "Node", "PersistentVolumeClaim",
		"Job", "ConfigMap", "Application", "HelmRelease", "Certificate",
		"NodeClaim", "Revision", "NetworkPolicy", "HorizontalPodAutoscaler",
		"UnknownKind",
	}
	items := make([]model.ActionMenuItem, 0, len(kinds)*8)
	for _, kind := range kinds {
		items = append(items, model.ActionsForKind(kind)...)
	}
	items = append(items, model.ActionsForContainer()...)
	items = append(items, model.ActionsForBulk("")...)
	items = append(items, model.ActionsForBulk("Application")...)
	items = append(items, model.ActionsForLonghornNode()...)
	items = append(items, model.ActionsForClusterPicker(model.ClusterPickerKeys{SetColor: "L"})...)
	items = append(items, model.ActionsForPortForward()...)
	items = append(items, model.ActionsForCapture()...)
	return items
}

// TestActionMenuChipKeys_DoNotCollideWithHostToggleKey guards against
// TASK-891: a chip key equal to kb.ActionMenu is unreachable, because
// isOverlayToggleKey (update_overlays.go) intercepts that key and closes the
// overlay before handleActionOverlayKey's per-item matcher ever runs. The
// existing collision test only compared chips against each other, so it
// never caught a chip colliding with the key that opened the menu.
//
// Asserted against kb.ActionMenu directly (not a hardcoded "x") so rebinding
// either the toggle key or a chip re-triggers this guard.
func TestActionMenuChipKeys_DoNotCollideWithHostToggleKey(t *testing.T) {
	toggleKey := ui.ActiveKeybindings.ActionMenu
	for _, item := range allActionMenuItems() {
		if item.Key == "" {
			continue
		}
		assert.NotEqual(t, toggleKey, item.Key,
			"chip %q uses key %q, which is the action-menu toggle key: "+
				"isOverlayToggleKey closes the overlay before the chip matcher runs", item.Label, item.Key)
	}
}

// TestActionMenuChipKeys_DoNotCollideWithHandlerConsumedKeys guards against
// the same class of bug for the other keys handleActionOverlayKey consumes
// before reaching the per-item chip matcher (cursor movement, close, submit).
// A chip using one of these can never fire.
func TestActionMenuChipKeys_DoNotCollideWithHandlerConsumedKeys(t *testing.T) {
	consumed := []string{"esc", "q", "enter", "up", "k", "ctrl+p", "down", "j", "ctrl+n", "ctrl+c"}
	for _, item := range allActionMenuItems() {
		if item.Key == "" {
			continue
		}
		for _, key := range consumed {
			assert.NotEqual(t, key, item.Key,
				"chip %q uses key %q, which handleActionOverlayKey consumes before the chip matcher runs", item.Label, item.Key)
		}
	}
}
