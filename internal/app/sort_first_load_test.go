package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// apiOrderWorkflowTemplates mimics a CRD list where every row carries the same
// name, so only the tiebreaker chain can put them in a repeatable order. The
// apiserver returns such a list in a different sequence on every call.
func apiOrderWorkflowTemplates() []model.Item {
	return []model.Item{
		{Name: "backup-db", Namespace: "mget-reports-dev", Kind: "WorkflowTemplate"},
		{Name: "backup-db", Namespace: "a2community-stage", Kind: "WorkflowTemplate"},
		{Name: "backup-db", Namespace: "emersys-demo", Kind: "WorkflowTemplate"},
		{Name: "backup-db", Namespace: "a2community-dev", Kind: "WorkflowTemplate"},
	}
}

func namespacesOf(items []model.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Namespace)
	}
	return out
}

// withoutRenderedColumns clears the render-time sortable-column global, which
// is how the process looks before View() has drawn a middle table even once.
func withoutRenderedColumns(t *testing.T) {
	t.Helper()
	saved := ui.ActiveSortableColumns
	ui.ActiveSortableColumns = nil
	t.Cleanup(func() { ui.ActiveSortableColumns = saved })
}

func TestSortMiddleItems_SortsBeforeTheFirstRender(t *testing.T) {
	withoutRenderedColumns(t)

	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = sortColDefault
	m.sortAscending = true
	m.middleItems = apiOrderWorkflowTemplates()

	m.sortMiddleItems()

	assert.Equal(t,
		[]string{"a2community-dev", "a2community-stage", "emersys-demo", "mget-reports-dev"},
		namespacesOf(m.middleItems),
		"equal names must fall through to the namespace tiebreaker on the very first load")
}

func TestSortMiddleItems_FirstAndSecondLoadAgree(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.sortColumnName = sortColDefault
	m.sortAscending = true

	withoutRenderedColumns(t)
	m.middleItems = apiOrderWorkflowTemplates()
	m.sortMiddleItems()
	first := namespacesOf(m.middleItems)

	// The render has happened by now, so the global is populated. A refresh
	// that returns the same rows in another order must not move any row.
	ui.ActiveSortableColumns = []string{"Name", "Namespace", "Age"}
	reshuffled := apiOrderWorkflowTemplates()
	reshuffled[0], reshuffled[3] = reshuffled[3], reshuffled[0]
	m.middleItems = reshuffled
	m.sortMiddleItems()

	assert.Equal(t, first, namespacesOf(m.middleItems), "the list must not reorder between loads")
}

func TestSortMiddleItems_StillSkipsTheResourceTypeBrowser(t *testing.T) {
	withoutRenderedColumns(t)

	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.sortColumnName = sortColDefault
	m.sortAscending = true
	m.middleItems = apiOrderWorkflowTemplates()

	m.sortMiddleItems()

	assert.Equal(t,
		namespacesOf(apiOrderWorkflowTemplates()),
		namespacesOf(m.middleItems),
		"the resource-type browser keeps its curated order")
}
