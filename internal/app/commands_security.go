package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/logger"
)

// loadSecurityDashboard fetches findings from the security Manager and returns
// a securityFindingsLoadedMsg with the result. Per-source errors are logged
// via internal/logger and included in result.Errors.
func (m Model) loadSecurityDashboard() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := mgr.FetchAll(ctx, kctx, ns)
		if err != nil {
			return securityFetchErrorMsg{context: kctx, err: err}
		}
		for name, e := range res.Errors {
			logger.Logger.Error("security source fetch failed",
				"source", name, "context", kctx, "error", e.Error())
		}
		return securityFindingsLoadedMsg{context: kctx, namespace: ns, result: res}
	}
}

// loadSecurityAvailability probes Manager.AnyAvailable and returns a message
// so the Model can gate UI features (SEC column, H key).
func (m Model) loadSecurityAvailability() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ok, _ := mgr.AnyAvailable(ctx, kctx)
		return securityAvailabilityLoadedMsg{context: kctx, available: ok}
	}
}
