package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func annotatedQuarantinedPodItem() model.Item {
	return model.Item{
		Name:      "pod-1",
		Namespace: "default",
		Kind:      "Pod",
		Raw: map[string]any{
			"metadata": map[string]any{
				"name":      "pod-1",
				"namespace": "default",
				"annotations": map[string]any{
					"lfk.janosmiko.dev/quarantined-labels": `{"app":"web"}`,
				},
			},
		},
	}
}

func TestOpenResourceActionMenu_AnnotatedPodShowsRestore(t *testing.T) {
	m := withMiddleItem(basePush80Model(), annotatedQuarantinedPodItem())

	out := m.openResourceActionMenu()

	names := make(map[string]bool, len(out.overlayItems))
	for _, item := range out.overlayItems {
		names[item.Name] = true
	}
	assert.True(t, names[model.ActionLabelRestore], "an annotated pod should offer Restore")
	assert.False(t, names[model.ActionLabelQuarantine], "an annotated pod should not still offer Quarantine")
}

func TestOpenResourceActionMenu_PlainPodShowsQuarantine(t *testing.T) {
	m := withMiddleItem(basePush80Model(), model.Item{Name: "pod-1", Namespace: "default", Kind: "Pod"})

	out := m.openResourceActionMenu()

	names := make(map[string]bool, len(out.overlayItems))
	for _, item := range out.overlayItems {
		names[item.Name] = true
	}
	assert.True(t, names[model.ActionLabelQuarantine], "a plain pod should offer Quarantine")
	assert.False(t, names[model.ActionLabelRestore], "a plain pod should not offer Restore")
}
