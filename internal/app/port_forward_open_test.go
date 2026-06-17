package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestConsumePortForwardBrowserOpen covers the helper that decides whether to
// open the resolved localhost URL and clear the pending intent.
func TestConsumePortForwardBrowserOpen(t *testing.T) {
	t.Run("no pending intent is a no-op", func(t *testing.T) {
		m := baseModelUpdate()
		m.pfOpenInBrowserAfterStart = false
		cmd := m.consumePortForwardBrowserOpen("8080")
		assert.Nil(t, cmd)
		assert.False(t, m.pfOpenInBrowserAfterStart)
	})

	t.Run("pending intent with unresolved port keeps the intent", func(t *testing.T) {
		m := baseModelUpdate()
		m.pfOpenInBrowserAfterStart = true
		for _, port := range []string{"0", ""} {
			cmd := m.consumePortForwardBrowserOpen(port)
			assert.Nil(t, cmd)
			assert.True(t, m.pfOpenInBrowserAfterStart, "intent must survive port %q", port)
		}
	})

	t.Run("pending intent with resolved port opens and clears", func(t *testing.T) {
		m := baseModelUpdate()
		m.pfOpenInBrowserAfterStart = true
		cmd := m.consumePortForwardBrowserOpen("8080")
		assert.NotNil(t, cmd)
		assert.False(t, m.pfOpenInBrowserAfterStart)
		assert.Equal(t, "Opening http://localhost:8080", m.statusMessage)
		assert.False(t, m.statusMessageErr)
	})
}

// TestExecuteActionPortForwardAndOpen asserts the action sets the pending
// browser-open intent before loading ports.
func TestExecuteActionPortForwardAndOpen(t *testing.T) {
	m := baseModelUpdate()
	m.actionCtx = actionContext{kind: "Service", name: "web", namespace: "default", context: "test-ctx"}

	ret, cmd := m.executeAction("Port Forward & Open")
	result := ret.(Model)

	assert.True(t, result.pfOpenInBrowserAfterStart)
	assert.NotNil(t, cmd) // loadContainerPorts
}

// TestExecuteActionPortForwardAndOpen_ReadOnlyBlocked asserts the chained
// action is gated by read-only mode and does not set the intent.
func TestExecuteActionPortForwardAndOpen_ReadOnlyBlocked(t *testing.T) {
	m := baseModelUpdate()
	m.readOnly = true
	m.actionCtx = actionContext{kind: "Service", name: "web", namespace: "default", context: "test-ctx"}

	ret, _ := m.executeAction("Port Forward & Open")
	result := ret.(Model)

	assert.Equal(t, readOnlyBlockedMessage("Port Forward & Open"), result.statusMessage)
	assert.True(t, result.statusMessageErr)
	assert.False(t, result.pfOpenInBrowserAfterStart)
}

// TestPortForwardStarted_OpensWhenPortKnown covers the explicit-local-port path
// where the browser opens immediately on start.
func TestPortForwardStarted_OpensWhenPortKnown(t *testing.T) {
	m := baseModelUpdate()
	m.pfOpenInBrowserAfterStart = true

	ret, cmd := m.updatePortForwardStarted(portForwardStartedMsg{id: 1, localPort: "8080", remotePort: "80"})
	result := ret.(Model)

	assert.False(t, result.pfOpenInBrowserAfterStart, "intent consumed once port is known")
	assert.Equal(t, "Opening http://localhost:8080", result.statusMessage)
	assert.NotNil(t, cmd)
}

// TestPortForwardStarted_DefersWhenPortRandom keeps the intent until the random
// port resolves later.
func TestPortForwardStarted_DefersWhenPortRandom(t *testing.T) {
	m := baseModelUpdate()
	m.pfOpenInBrowserAfterStart = true

	ret, _ := m.updatePortForwardStarted(portForwardStartedMsg{id: 1, localPort: "0", remotePort: "80"})
	result := ret.(Model)

	assert.True(t, result.pfOpenInBrowserAfterStart, "intent survives until the port resolves")
}

// TestPortForwardStarted_ErrorClearsIntent ensures a failed start does not leak
// the intent into an unrelated future forward.
func TestPortForwardStarted_ErrorClearsIntent(t *testing.T) {
	m := baseModelUpdate()
	m.pfOpenInBrowserAfterStart = true

	ret, _ := m.updatePortForwardStarted(portForwardStartedMsg{err: assertErr{}})
	result := ret.(Model)

	assert.False(t, result.pfOpenInBrowserAfterStart)
}

// TestHandleOpenBrowser_ServiceTriggersPortForwardOpen asserts Ctrl+O on a
// Service routes through the chained action and sets the intent.
func TestHandleOpenBrowser_ServiceTriggersPortForwardOpen(t *testing.T) {
	m := baseModelUpdate()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Service", Resource: "services", Namespaced: true}
	m.middleItems = []model.Item{{Name: "web", Namespace: "default", Kind: "Service"}}

	ret, cmd, handled := m.handleExplorerActionKeyOpenBrowser()
	result := ret.(Model)

	assert.True(t, handled)
	assert.NotNil(t, cmd)
	assert.True(t, result.pfOpenInBrowserAfterStart)
}

// TestHandleOpenBrowser_ServiceReadOnlyBlocked asserts read-only mode blocks the
// Service Ctrl+O path without setting the intent.
func TestHandleOpenBrowser_ServiceReadOnlyBlocked(t *testing.T) {
	m := baseModelUpdate()
	m.readOnly = true
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Service", Resource: "services", Namespaced: true}
	m.middleItems = []model.Item{{Name: "web", Namespace: "default", Kind: "Service"}}

	ret, _, handled := m.handleExplorerActionKeyOpenBrowser()
	result := ret.(Model)

	assert.True(t, handled)
	assert.False(t, result.pfOpenInBrowserAfterStart)
	assert.Equal(t, readOnlyBlockedMessage("Port Forward & Open"), result.statusMessage)
}

// TestPortForwardOverlayCancelClearsIntent ensures escaping the overlay drops a
// pending browser-open intent.
func TestPortForwardOverlayCancelClearsIntent(t *testing.T) {
	m := baseModelUpdate()
	m.pfOpenInBrowserAfterStart = true
	m.overlay = overlayPortForward

	ret, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	result := ret.(Model)

	assert.Equal(t, overlayNone, result.overlay)
	assert.False(t, result.pfOpenInBrowserAfterStart)
}

// assertErr is a trivial error used to exercise the start-error path.
type assertErr struct{}

func (assertErr) Error() string { return "boom" }
