// Package app — commands_security.go
package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// loadSecurityAvailability probes each registered source's IsAvailable
// and returns a securityAvailabilityLoadedMsg with a per-source map.
// Used by the SEC column and Security category to decide what to show.
func (m Model) loadSecurityAvailability() tea.Cmd {
	if m.securityManager == nil {
		return nil
	}
	mgr := m.securityManager
	kctx := m.nav.Context
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		byName := make(map[string]bool)
		for _, s := range mgr.Sources() {
			ok, _ := s.IsAvailable(ctx, kctx)
			byName[s.Name()] = ok
		}
		return securityAvailabilityLoadedMsg{context: kctx, availability: byName}
	}
}
