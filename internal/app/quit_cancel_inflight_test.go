package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// armReqCtx wires a cancelable request context onto the model so the
// quit handlers' m.cancelInFlightRequests() call can actually do its
// job. baseModelWithFakeClient leaves reqCancel nil, which would make
// the cancel a silent no-op and miss the regression.
func armReqCtx(m *Model) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	m.reqCtx = ctx
	m.reqCancel = cancel
	return ctx
}

// All three quit paths (overlay confirm, ctrl+c last-tab, :quit
// command) must cancel m.reqCtx before returning tea.Quit so
// in-flight goroutines (discovery, list, owned, containers) abort
// with context.Canceled rather than riding out kernel TCP timeouts.
//
// Without this, on an unreachable cluster the quit can stretch to
// minutes while main's deferred cleanups (informer wg.Wait,
// stderr-pipe reader, etc.) wait for those goroutines to finish
// their HTTP calls naturally.

func TestHandleQuitConfirmOverlayKey_CancelsInFlightRequests(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}}
	reqCtx := armReqCtx(&m)
	require.NoError(t, reqCtx.Err(),
		"precondition: reqCtx must not be canceled before quit")

	result, cmd := m.handleQuitConfirmOverlayKey(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'y'},
	})

	require.NotNil(t, cmd, "quit handler must dispatch tea.Quit")
	_ = result.(Model)

	assert.ErrorIs(t, reqCtx.Err(), context.Canceled,
		"quit confirm must cancel m.reqCtx so in-flight API requests "+
			"abort instead of riding out TCP timeouts during shutdown")
}

func TestCloseTabOrQuit_LastTab_CancelsInFlightRequests(t *testing.T) {
	// Disable the confirm overlay so closeTabOrQuit goes straight to
	// the quit branch instead of bouncing through handleQuitConfirmOverlayKey.
	prev := ui.ConfigConfirmOnExit
	ui.ConfigConfirmOnExit = false
	defer func() { ui.ConfigConfirmOnExit = prev }()

	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}} // single tab → quit path
	reqCtx := armReqCtx(&m)

	result, cmd := m.closeTabOrQuit()
	require.NotNil(t, cmd, "last-tab close with confirm-off must dispatch tea.Quit")
	_ = result.(Model)

	assert.ErrorIs(t, reqCtx.Err(), context.Canceled,
		"closeTabOrQuit's last-tab branch must cancel m.reqCtx before tea.Quit")
}

// Multi-tab close (not a quit) must NOT cancel reqCtx — surviving
// tabs need the request context to stay live for their loads.
func TestCloseTabOrQuit_MultiTab_PreservesReqCtx(t *testing.T) {
	m := baseModelWithFakeClient()
	m.tabs = []TabState{{}, {}}
	m.activeTab = 1
	reqCtx := armReqCtx(&m)

	result, _ := m.closeTabOrQuit()
	_ = result.(Model)

	assert.NoError(t, reqCtx.Err(),
		"closing one tab when others remain must not cancel reqCtx — "+
			"surviving tabs still need it for their fetches")
}
