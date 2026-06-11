package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// actionKeysSorted reports whether menu items are ordered by hotkey,
// case-insensitively, with lowercase before uppercase on ties.
func actionKeysSorted(t *testing.T, items []ActionMenuItem) {
	t.Helper()
	for i := 1; i < len(items); i++ {
		a, b := items[i-1].Key, items[i].Key
		la, lb := strings.ToLower(a), strings.ToLower(b)
		ordered := la < lb || (la == lb && a >= b)
		assert.True(t, ordered, "items out of hotkey order: %q (%s) before %q (%s)",
			a, items[i-1].Label, b, items[i].Label)
	}
}

func TestActionMenusSortedByHotkey(t *testing.T) {
	kinds := []string{
		"Pod", "Service", "Deployment", "StatefulSet", "DaemonSet", "Node",
		"Job", "CronJob", "Secret", "ConfigMap", "NetworkPolicy", "Ingress",
		"PersistentVolumeClaim", "Application", "HelmRelease", "Workflow",
		"SomeUnknownKind",
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			actionKeysSorted(t, ActionsForKind(kind))
		})
	}
}

func TestAuxiliaryActionMenusSortedByHotkey(t *testing.T) {
	t.Run("container", func(t *testing.T) {
		actionKeysSorted(t, ActionsForContainer())
	})
	t.Run("bulk generic", func(t *testing.T) {
		actionKeysSorted(t, ActionsForBulk(""))
	})
	t.Run("bulk application", func(t *testing.T) {
		actionKeysSorted(t, ActionsForBulk("Application"))
	})
	t.Run("port forward", func(t *testing.T) {
		actionKeysSorted(t, ActionsForPortForward())
	})
	t.Run("capture", func(t *testing.T) {
		actionKeysSorted(t, ActionsForCapture())
	})
}

func TestSortActionsByKey_TieBreak(t *testing.T) {
	items := sortActionsByKey([]ActionMenuItem{
		{Label: "Upper", Key: "B"},
		{Label: "Lower", Key: "b"},
		{Label: "First", Key: "a"},
	})
	assert.Equal(t, []string{"a", "b", "B"}, []string{items[0].Key, items[1].Key, items[2].Key},
		"lowercase must sort before uppercase on the same letter")
}
