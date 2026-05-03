package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
)

func (m Model) updateYamlLoaded(msg yamlLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	// enterFullView sets yamlContent="Loading..." as a placeholder; we
	// must replace it on every reply path (success, cancel, error) so the
	// viewer never renders the loader indefinitely. The canceled case can
	// fire when a mid-load navigation tears down reqCtx — show an empty
	// body so the user understands the fetch did not complete rather than
	// being stuck on the spinner.
	if isContextCanceled(msg.err) {
		m.yamlContent = ""
		m.yamlSections = nil
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		m.yamlContent = "# Error loading resource\n# " + msg.err.Error()
		m.yamlSections = nil
		return m, scheduleStatusClear()
	}
	m.err = nil
	// Content and sections are pre-processed in the loading goroutine so
	// the main event loop stays responsive on very large CRD manifests.
	m.yamlContent = msg.content
	m.yamlSections = msg.sections
	return m, nil
}

func (m Model) updatePreviewYAMLLoaded(msg previewYAMLLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response, discard
	}
	if msg.err != nil {
		m.previewYAML = ""
		return m
	}
	// Pre-indented in the loading goroutine — no heavy work on main thread.
	m.previewYAML = msg.content
	return m
}

func (m Model) updateActionResult(msg actionResultMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.bulkMode = false
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
	} else {
		if msg.message != "" {
			logger.Info("Action completed", "message", msg.message)
			m.setStatusMessage(msg.message, false)
		}
		// Only invalidate when the action succeeded; a failed `create
		// ns` or template apply did not actually mutate the cluster.
		if msg.invalidateNamespaceCache {
			m.invalidateNamespaceCache()
		}
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateYamlClipboard(msg yamlClipboardMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		return m, scheduleStatusClear()
	}
	if msg.count > 1 {
		m.setStatusMessage(fmt.Sprintf("Copied %d manifests to clipboard", msg.count), false)
	} else {
		m.setStatusMessage("YAML copied to clipboard", false)
	}
	return m, tea.Batch(copyToSystemClipboard(msg.content), scheduleStatusClear())
}
