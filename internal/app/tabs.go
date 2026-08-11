package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// pushLeft saves the current leftItems and promotes middleItems to become the new leftItems.
func (m *Model) pushLeft() {
	m.leftItemsHistory = append(m.leftItemsHistory, m.leftItems)
	m.leftItems = m.middleItems
}

// popLeft restores leftItems from the history stack.
func (m *Model) popLeft() {
	n := len(m.leftItemsHistory)
	if n > 0 {
		m.leftItems = m.leftItemsHistory[n-1]
		m.leftItemsHistory = m.leftItemsHistory[:n-1]
	} else {
		m.leftItems = nil
	}
}

// selectedResourceKind returns the Kind of the currently selected resource,
// which is context-dependent on the navigation level.
func (m *Model) selectedResourceKind() string {
	switch m.nav.Level {
	case model.LevelResources:
		return m.nav.ResourceType.Kind
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel != nil {
			return sel.Kind
		}
	case model.LevelContainers:
		return "Container"
	}
	return ""
}

// effectiveNamespace returns the namespace to use for API calls.
// Returns empty string when allNamespaces is true or multiple namespaces are
// selected (fetches all, filters client-side).
// isUnionSentinel reports whether the app is in union mode while nav.Context
// holds the internal sentinel value that must not be sent to the Kubernetes
// API. Keep this level-agnostic: union mode also uses the sentinel at
// LevelResourceTypes for discovery and metadata fallbacks. ValidateUnionOptions
// reserves the literal sentinel name, so it cannot collide with a configured
// union context.
func (m Model) isUnionSentinel() bool {
	return m.unionMode && m.nav.Context == UnionContextSentinel
}

// effectiveContext returns the Kubernetes context for API calls targeting the
// currently selected item. In union mode at LevelResources, nav.Context is
// the UnionContextSentinel, so we read the source cluster from the hovered
// item's ClusterName. At all other levels (post-drill-down), nav.Context is
// already the real cluster and is returned as-is.
//
// When the hovered item carries no ClusterName, fall back to unionContexts[0].
// Callers that are semantically per-context (dashboards, RBAC, bookmarks)
// should guard before calling this; the fallback is for discovery, namespace
// metadata, and other internals that need one representative real context.
func (m Model) effectiveContext() string {
	if m.isUnionSentinel() {
		if sel := m.selectedMiddleItem(); sel != nil && sel.ClusterName != "" {
			return sel.ClusterName
		}
		if len(m.unionContexts) > 0 {
			return m.unionContexts[0]
		}
	}
	return m.nav.Context
}

func (m *Model) effectiveNamespace() string {
	if m.allNamespaces || m.nsSelectionNegated || len(m.selectedNamespaces) > 1 {
		return "" // fetch all, filter client-side
	}
	if len(m.selectedNamespaces) == 1 {
		for ns := range m.selectedNamespaces {
			return ns
		}
	}
	return m.namespace
}

// fetchFingerprint returns a stable digest of what a resource list fetch
// returns: effective namespace, the allNamespaces toggle, and the
// selectedNamespaces multi-select filter (with its negation flag). Used by
// the preview-cache shortcut; context/resource live in the paired navKey.
func (m *Model) fetchFingerprint() string {
	var b strings.Builder
	if m.allNamespaces {
		b.WriteString("A|")
	} else {
		b.WriteString("ns=")
		b.WriteString(m.namespace)
		b.WriteString("|")
	}
	if len(m.selectedNamespaces) > 0 {
		keys := make([]string, 0, len(m.selectedNamespaces))
		for k := range m.selectedNamespaces {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if m.nsSelectionNegated {
			b.WriteString("!")
		}
		fmt.Fprintf(&b, "sel=%s", strings.Join(keys, ","))
	}
	return b.String()
}

// sortApplies reports whether sort keybindings (>, <, =, -) have any
// effect at the current navigation level. False at the cluster picker
// and resource type browser, where items keep their original ordering.
// Callers in the key-handler layer must short-circuit before mutating
// sort state so the bar doesn't lie that sort changed when items stay
// put.
func (m *Model) sortApplies() bool {
	return m.nav.Level != model.LevelClusters && m.nav.Level != model.LevelResourceTypes
}

// sortModeName returns a display name for the current sort column with direction indicator.
func (m *Model) sortModeName() string {
	col := m.sortColumnName
	asc := m.sortAscending
	// Mirror the Event override from sortMiddleItems so the display
	// matches the actual sort order.
	if col == sortColDefault && m.nav.ResourceType.Kind == "Event" {
		col = "Last Seen"
	}
	if col != "" {
		dir := "\u2191" // ↑
		if !asc {
			dir = "\u2193" // ↓
		}
		return col + " " + dir
	}
	return "Name \u2191"
}

// sanitizeError strips newlines and truncates an error message for status bar display.
func (m *Model) sanitizeError(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Collapse multiple spaces.
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	maxLen := max(m.width-20, 40)
	// Rune-slice, not byte-slice: error text can contain arbitrary UTF-8 and a
	// mid-rune cut emits a replacement char. Matches sanitizeMessage below.
	runes := []rune(s)
	if len(runes) > maxLen {
		s = string(runes[:maxLen-3]) + "..."
	}
	return s
}

// fullErrorMessage returns the full error message with newlines collapsed, for logging.
func fullErrorMessage(err error) string {
	s := strings.ReplaceAll(err.Error(), "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

// sanitizeMessage strips newlines and truncates a string for status bar display.
func (m *Model) sanitizeMessage(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	maxLen := max(
		// account for status bar padding
		m.width-6, 40)
	runes := []rune(s)
	if len(runes) > maxLen {
		s = string(runes[:maxLen-3]) + "..."
	}
	return s
}

// setStatusMessageText is the only place that writes m.statusMessage. Both
// setStatusMessage and setErrorFromErr route through it so a hostile string
// - a cluster-controlled resource name, apiserver error text - cannot reach
// the status bar via a second, forgotten direct write. Sanitizing here,
// before the caller-specific log entry is built below, keeps that order
// (sanitize before truncate/measure) automatic for both callers.
func (m *Model) setStatusMessageText(msg string, isErr bool) string {
	msg = ui.SanitizeTerminalText(msg)
	m.statusMessage = msg
	m.statusMessageErr = isErr
	m.statusMessageExp = time.Now().Add(5 * time.Second)
	return msg
}

// setStatusMessage sets a temporary status bar message.
// All messages are appended to the application log buffer with appropriate level.
func (m *Model) setStatusMessage(msg string, isErr bool) {
	msg = m.setStatusMessageText(msg, isErr)

	level := "INF"
	if isErr {
		level = "ERR"
		logger.Error("Application error", "message", msg)
	} else {
		logger.Info("Status message", "message", msg)
	}
	m.appendErrorLogEntry(ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: msg,
		Level:   level,
	}, 200)
}

// setErrorFromErr shows a sanitized error in the status bar and logs the
// full untruncated error to the error log overlay.
func (m *Model) setErrorFromErr(prefix string, err error) {
	// Show truncated version in status bar.
	m.setStatusMessageText(prefix+m.sanitizeError(err), true)

	// Log the full untruncated error to the error log.
	full := fullErrorMessage(err)
	logger.Error("Application error", "message", full)
	m.appendErrorLogEntry(ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: prefix + full,
		Level:   "ERR",
	}, 200)
}

// hasStatusMessage checks whether there's a non-expired status message.
func (m *Model) hasStatusMessage() bool {
	return m.statusMessage != "" && time.Now().Before(m.statusMessageExp)
}

// appendErrorLogEntry is the single choke point for every writer of
// m.errorLog (addLogEntry, setStatusMessage, setErrorFromErr,
// appendLoggerUIEntry in log_ui.go). It sanitizes entry.Message before
// appending so a future writer can't add a new raw-append path by mistake -
// this is the third time a sibling writer to an already-sanitized field
// leaked unsanitized content, so the fix goes at the one place all of them
// share rather than patching each call site again. maxEntries is the cap to
// retain (oldest dropped first); callers pass their existing cap so
// log_ui.go/addLogEntry's larger buffer isn't shrunk to match
// setStatusMessage/setErrorFromErr's smaller one.
func (m *Model) appendErrorLogEntry(entry ui.ErrorLogEntry, maxEntries int) {
	entry.Message = ui.SanitizeTerminalText(entry.Message)
	m.errorLog = append(m.errorLog, entry)
	if len(m.errorLog) > maxEntries {
		m.errorLog = m.errorLog[len(m.errorLog)-maxEntries:]
	}
}

// addLogEntry appends an entry to the in-app error log at the given level.
//
// msg is sanitized here rather than at each of its ~80 call sites. Most
// callers pass lfk-constructed strings (echoed kubectl commands), but a few
// - command-bar output, err.Error() on a port-forward failure - carry
// attacker-influenced content, and every entry renders through WrapEventLine
// in the error-log overlay with no sanitizing of its own. SanitizeTerminalText
// because an ErrorLogEntry.Message is a single log line, matching how
// buildEventTimelineLines treats other single-line table/list values.
func (m *Model) addLogEntry(level, msg string) {
	m.appendErrorLogEntry(ui.ErrorLogEntry{
		Time:    time.Now(),
		Message: msg,
		Level:   level,
	}, 500)
}

// actionNamespace returns the namespace to use for action commands.
// It prefers the namespace captured when the action menu was opened.
func (m Model) actionNamespace() string {
	if m.actionCtx.namespace != "" {
		return m.actionCtx.namespace
	}
	return m.resolveNamespace()
}
