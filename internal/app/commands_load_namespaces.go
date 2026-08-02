package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
)

func (m Model) loadNamespaces() tea.Cmd {
	return m.loadNamespacesSilent(false)
}

// loadNamespacesSilent issues the same namespace fetch as loadNamespaces
// but tags the resulting msg as a background refresh. Silent loads must
// not clear m.loading in the handler: that flag belongs to the
// middle-column/resource-types load.
func (m Model) loadNamespacesSilent(silent bool) tea.Cmd {
	return m.loadNamespacesForContext(m.activeContext(), silent)
}

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
