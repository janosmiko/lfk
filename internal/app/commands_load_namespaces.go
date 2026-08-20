package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
)

// loadNamespacesForContext tags silent loads as a background refresh in the
// resulting msg. The handler must not clear m.loading for a silent load:
// that flag belongs to the middle-column/resource-types load.
func (m Model) loadNamespacesForContext(kctx string, silent bool) tea.Cmd {
	if m.client == nil || kctx == "" {
		return nil
	}
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindNamespaceList,
		"List namespaces",
		kctx,
		func(ctx context.Context) tea.Msg {
			items, err := client.GetNamespaces(ctx, kctx)
			return namespacesLoadedMsg{context: kctx, items: items, err: err, silent: silent}
		},
	)
}
