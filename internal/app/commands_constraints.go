package app

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
)

var errConstraintsNoClient = errors.New("no cluster client available")

// constraintsLoadedMsg carries a DetectConstraints result. gen guards
// against a reply landing on a view reopened for another object, tabUID
// against one landing on another tab, as gen is only unique per tab.
type constraintsLoadedMsg struct {
	gen    uint64
	tabUID uint64
	report k8s.ConstraintReport
	err    error
}

// loadConstraints fetches every constraint source for m.constraints.target.
func (m Model) loadConstraints(name string) tea.Cmd {
	client := m.client
	kubeCtx := m.effectiveContext()
	target := m.constraints.target
	gen := m.constraints.gen
	tabUID := m.currentTabUID()
	if client == nil {
		return func() tea.Msg { return constraintsLoadedMsg{gen: gen, tabUID: tabUID, err: errConstraintsNoClient} }
	}
	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceTree,
		"Constraints: "+name,
		bgtaskTarget(kubeCtx, target.Namespace),
		func(ctx context.Context) tea.Msg {
			report, err := client.DetectConstraints(ctx, kubeCtx, target)
			return constraintsLoadedMsg{gen: gen, tabUID: tabUID, report: report, err: err}
		},
	)
}

//nolint:unparam // consistent message handler signature, the caller passes the Cmd on
func (m Model) updateConstraintsLoaded(msg constraintsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.constraints.gen || msg.tabUID != m.currentTabUID() {
		return m, nil
	}
	m.constraints.loading = false
	m.constraints.err = msg.err
	m.constraints.report = msg.report
	m.constraints.clampCursor()
	m.constraints.clampScroll(m.constraintsViewportHeight())
	return m, nil
}
