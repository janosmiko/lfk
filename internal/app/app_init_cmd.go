package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// Init loads the initial context list.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.loadContexts(),
		m.spinner.Tick,
		// Security source availability is probed lazily on first focus of the
		// Security category (maybeProbeSecurityOnFocus), not at startup, so a
		// cluster the user never inspects for security never pays the probe's
		// API calls (which on EKS surface aws SSO-expired noise).
		// Run the discovery-cache preload off the main goroutine. Blocking
		// startup on it serialises a clientcmd.ClientConfig() call per
		// kubeconfig context; see the comment on discoveryCachePreloadCmd.
		discoveryCachePreloadCmd(m.reqCtx, m.client),
	}
	if m.stderrChan != nil {
		cmds = append(cmds, m.waitForStderr())
	}
	// Subscribe to deduplicated log events so background failures
	// (metrics-server unreachable, RBAC denied, ...) reach the in-app
	// log overlay instead of only the on-disk file.
	cmds = append(cmds, waitForLoggerUI())
	// Dispatch the SEC-badge findings-cache seed produced during NewModel. The
	// disk read runs off the Update goroutine; refreshSecuritySources returns
	// nil when security is disabled or no findings cache applies.
	if m.initialSecuritySeedCmd != nil {
		cmds = append(cmds, m.initialSecuritySeedCmd)
	}
	if m.watchMode {
		cmds = append(cmds, scheduleWatchTick(m.activeWatchInterval(), m.watchTickGen))
	}
	if ui.ConfigTipsEnabled {
		cmds = append(cmds, scheduleStartupTip())
	}
	if ui.ColorModeEnabled() {
		cmds = append(cmds, ui.EnableColorModeCmd())
	}
	return tea.Batch(cmds...)
}
