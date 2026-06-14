package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
)

// executeActionKnative dispatches Knative Serving action labels to
// their Tea commands. Returns ok=true when the label is handled here
// so executeActionExtended can early-return without growing its
// switch. Currently surfaces only the Activate verb on Revision; the
// traffic-split overlay on Service is deferred to a follow-up.
func (m Model) executeActionKnative(actionLabel string) (tea.Model, tea.Cmd, bool) {
	if actionLabel == "Activate" {
		mdl, cmd := m.executeActionSimpleLoading("Activating Revision", m.activateKnativeRevision)
		return mdl, cmd, true
	}
	return m, nil, false
}

// activateKnativeRevision patches the Revision's parent Knative
// Service so 100% of traffic routes to the selected Revision. Wrapped
// in trackBgTask so the title-bar indicator surfaces the in-flight
// patch and the :tasks overlay records it. The parent Service name is
// resolved by Client.ActivateKnativeRevision via the standard
// serving.knative.dev/service label on the Revision; an orphaned
// Revision returns a clear error naming the missing label.
func (m Model) activateKnativeRevision() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	logger.Info("Knative Revision activation requested", "context", ctx, "namespace", ns, "name", name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Activate Revision: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		parent, err := m.client.ActivateKnativeRevision(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Activated Revision %s on Service %s (100%% traffic)", name, parent)}
	})
}
