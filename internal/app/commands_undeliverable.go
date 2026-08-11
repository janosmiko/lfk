package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// cmdLoadUndeliverable runs DetectUndeliverable for the given context.
// Returns nil when a scan is already in flight, so opening the overlay twice
// in one tick does not issue two cluster-wide scans.
//
// The scan carries a generation number and a cancel func so a result that
// arrives after the user switched context is dropped rather than written back
// over their newer scope - the same guard the orphan scan uses.
func (m *Model) cmdLoadUndeliverable(kubeContext string) tea.Cmd {
	if m.undeliverable.inflight {
		return nil
	}
	m.undeliverable.gen++
	gen := m.undeliverable.gen
	ctx, cancel := context.WithCancel(context.Background())
	m.undeliverable.inflight = true
	m.undeliverable.cancel = cancel
	client := m.client
	return func() tea.Msg {
		report, err := client.DetectUndeliverable(ctx, kubeContext, "")
		return undeliverableLoadedMsg{
			kubeContext: kubeContext, gen: gen, report: report, err: err,
		}
	}
}

// handleUndeliverableLoaded pushes a completed scan into the overlay state.
//
// The bookkeeping is cleared before the context check, not after: a scan that
// finishes for a cluster the user has already left still has to release the
// inflight slot, or cmdLoadUndeliverable would refuse every later scan and the
// overlay would be stuck on its spinner forever.
//
// A result for another context is then discarded rather than written back -
// that guard, plus the loadedFor check on open, is what makes a context switch
// safe without cancelling the scan mid-flight. The scan is read-only, so
// letting it finish and dropping it costs one wasted list round.
//
// Dropping a foreign result can leave the active context with no scan in
// flight at all: the overlay may have been reopened for it while this one
// was still running, and cmdLoadUndeliverable refused that request because
// inflight was true. Restart the scan here so the overlay does not sit on a
// stale or empty report with no spinner and nothing to follow.
func (m Model) handleUndeliverableLoaded(msg undeliverableLoadedMsg) (Model, tea.Cmd) {
	if msg.gen != m.undeliverable.gen {
		return m, nil
	}
	if m.undeliverable.cancel != nil {
		m.undeliverable.cancel() // release the context now the scan is done
		m.undeliverable.cancel = nil
	}
	m.undeliverable.inflight = false
	if msg.kubeContext != m.nav.Context {
		m.undeliverable.loading = false
		if m.overlay == overlayUndeliverable && m.undeliverable.loadedFor == m.nav.Context {
			m.undeliverable.loading = true
			return m, (&m).cmdLoadUndeliverable(m.nav.Context)
		}
		return m, nil
	}
	m.undeliverable.report = msg.report
	m.undeliverable.partial = msg.err
	m.undeliverable.loading = false
	m = m.undeliverableClampCursor()

	if msg.err != nil && m.overlay != overlayUndeliverable {
		// The scan error quotes apiserver text, so it is as cluster-controlled
		// as anything on a row. Sanitize the interpolated half before it is
		// concatenated: the status bar's own sink only folds newlines and
		// truncates, so an escape reaching it is an escape on the screen.
		detail := ui.SanitizeTerminalText(msg.err.Error())
		m.setStatusMessage("Undeliverable scan: partial result ("+detail+")", true)
		return m, scheduleStatusClear()
	}
	return m, nil
}
