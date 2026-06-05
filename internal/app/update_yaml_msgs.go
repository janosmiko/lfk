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
		m.yamlView.content = ""
		m.yamlView.sections = nil
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		m.yamlView.content = "# Error loading resource\n# " + msg.err.Error()
		m.yamlView.sections = nil
		return m, scheduleStatusClear()
	}
	m.err = nil
	// Content and sections are pre-processed in the loading goroutine so
	// the main event loop stays responsive on very large CRD manifests.
	m.yamlView.content = msg.content
	m.yamlView.sections = msg.sections
	m.applyYAMLPendingCursor() // sync cursor when arriving from the Object Explorer
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
	// Pure-error path (content empty): surface the error and stop.
	// Partial-success path (content present + err set) falls through to
	// the copy + status branch so the user gets the partial payload on
	// the clipboard; buildBulkYAMLClipboardMsg's err already names the
	// failed items in the "copied K/N, M failed: ..." form.
	if msg.err != nil && msg.content == "" {
		m.setErrorFromErr("Error: ", msg.err)
		return m, scheduleStatusClear()
	}
	label, unit := copyFormatStatusParts(msg.format)
	switch {
	case msg.err != nil:
		m.setErrorFromErr("Warning: ", msg.err)
	case msg.count > 1:
		m.setStatusMessage(fmt.Sprintf("Copied %d %s as %s", msg.count, unit, label), false)
	default:
		m.setStatusMessage(fmt.Sprintf("%s copied to clipboard", label), false)
	}
	return m, tea.Batch(copyToSystemClipboard(msg.content), scheduleStatusClear())
}

// copyFormatStatusParts returns the (label, plural-unit) pair used in the
// clipboard status message. label is title-case for the status line ("YAML",
// "JSON", "Table"); unit is the plural noun for bulk copies. Empty format
// defaults to YAML so legacy callers stay correct.
func copyFormatStatusParts(format string) (label, unit string) {
	switch format {
	case "json":
		return "JSON", "manifests"
	case "table":
		return "Table", "rows"
	default:
		return "YAML", "manifests"
	}
}
