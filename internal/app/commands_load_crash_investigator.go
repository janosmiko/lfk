package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
)

// crashInvestigationTimeout bounds the background fetch for a single
// crash investigation. An unresponsive apiserver should not leave the
// bg-task tracker holding a slot indefinitely.
const crashInvestigationTimeout = 60 * time.Second

// loadCrashInvestigation kicks off a CrashLoopBackOff investigation
// against the action-context pod and emits crashInvestigationMsg with
// the result.
func (m Model) loadCrashInvestigation() tea.Cmd {
	client := m.client
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return m.trackBgTask(scheduler.KindResourceList, "Crash investigator: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		fetchCtx, cancel := context.WithTimeout(context.Background(), crashInvestigationTimeout)
		defer cancel()
		info, err := client.GetCrashInvestigation(fetchCtx, ctx, ns, name)
		return crashInvestigationMsg{info: info, err: err}
	})
}
