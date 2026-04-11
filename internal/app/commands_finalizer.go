package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// searchFinalizers returns a command that scans resource types for resources
// with finalizers matching the given pattern. The scope depends on the
// navigation level:
//   - LevelResources/LevelOwned: only the current resource type
//   - LevelResourceTypes/LevelClusters: all discovered resource types
func (m Model) searchFinalizers(pattern string) tea.Cmd {
	client := m.client
	kctx := m.nav.Context
	ns := m.effectiveNamespace()

	var resourceTypes []model.ResourceTypeEntry

	if m.nav.Level >= model.LevelResources && m.nav.ResourceType.Kind != "" {
		// Scoped to the current resource type.
		resourceTypes = []model.ResourceTypeEntry{m.nav.ResourceType}
	} else {
		// All known resource types from the discovered API resource set.
		resourceTypes = append(resourceTypes, m.discoveredResources[kctx]...)
	}

	return func() tea.Msg {
		results, err := client.FindResourcesWithFinalizer(
			context.Background(), kctx, ns, pattern, resourceTypes,
		)
		return finalizerSearchResultMsg{results: results, err: err}
	}
}

// bulkRemoveFinalizer returns a command that removes the matched finalizer
// from all selected resources in the finalizer search results.
func (m Model) bulkRemoveFinalizer() tea.Cmd {
	client := m.client
	kctx := m.nav.Context
	selected := make(map[string]bool, len(m.finalizerSearchSelected))
	for k, v := range m.finalizerSearchSelected {
		selected[k] = v
	}

	// Collect the matching results for selected items.
	var targets []finalizerTarget
	for _, result := range m.finalizerSearchResults {
		key := finalizerMatchKey(result)
		if selected[key] {
			targets = append(targets, finalizerTarget{match: result})
		}
	}

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string

		for _, t := range targets {
			err := client.RemoveFinalizerFromResource(context.Background(), kctx, t.match)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s/%s: %s", t.match.Kind, t.match.Name, err.Error()))
			} else {
				succeeded++
			}
		}

		return finalizerRemoveResultMsg{
			succeeded: succeeded,
			failed:    failed,
			errors:    errors,
		}
	}
}

// finalizerTarget holds a match targeted for finalizer removal.
type finalizerTarget struct {
	match k8s.FinalizerMatch
}

// finalizerMatchKey returns a unique key for a FinalizerMatch suitable for
// use in the selection map.
func finalizerMatchKey(m k8s.FinalizerMatch) string {
	return m.Namespace + "/" + m.Kind + "/" + m.Name
}
