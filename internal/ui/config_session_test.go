package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// snapshotSessionDefaultGlobals saves and restores the session-default globals.
func snapshotSessionDefaultGlobals(t *testing.T) {
	t.Helper()
	prevSplit := ConfigSplitPreview
	prevWatch := ConfigWatchMode
	prevAllNs := ConfigAllNamespaces
	prevAllNsSet := ConfigAllNamespacesSet
	prevWarn := ConfigEventsWarningsOnly
	prevGroup := ConfigEventsGrouping
	t.Cleanup(func() {
		ConfigSplitPreview = prevSplit
		ConfigWatchMode = prevWatch
		ConfigAllNamespaces = prevAllNs
		ConfigAllNamespacesSet = prevAllNsSet
		ConfigEventsWarningsOnly = prevWarn
		ConfigEventsGrouping = prevGroup
	})
	ConfigSplitPreview = true
	ConfigWatchMode = true
	ConfigAllNamespaces = true
	ConfigAllNamespacesSet = false
	ConfigEventsWarningsOnly = true
	ConfigEventsGrouping = true
}

// TestSessionDefaults_AllApply verifies the flat session switches and the events
// group wire into their runtime globals.
func TestSessionDefaults_AllApply(t *testing.T) {
	snapshotSessionDefaultGlobals(t)

	path := writeConfigFile(t, `split_preview: false
watch_mode: false
all_namespaces: false
events:
  warnings_only: false
  grouping: false
`)
	LoadConfig(path)

	assert.False(t, ConfigSplitPreview, "split_preview")
	assert.False(t, ConfigWatchMode, "watch_mode")
	assert.False(t, ConfigAllNamespaces, "all_namespaces")
	assert.True(t, ConfigAllNamespacesSet, "all_namespaces (set flag)")
	assert.False(t, ConfigEventsWarningsOnly, "events.warnings_only")
	assert.False(t, ConfigEventsGrouping, "events.grouping")
}

// TestSessionDefaults_OmittedPreserveDefaults verifies omitted keys keep the
// compiled defaults (pointer fields distinguish unset from false).
func TestSessionDefaults_OmittedPreserveDefaults(t *testing.T) {
	snapshotSessionDefaultGlobals(t)

	path := writeConfigFile(t, `watch_mode: false
events:
  grouping: false
`)
	LoadConfig(path)

	assert.False(t, ConfigWatchMode, "watch_mode applied")
	assert.True(t, ConfigSplitPreview, "split_preview default preserved")
	assert.True(t, ConfigAllNamespaces, "all_namespaces default preserved")
	assert.False(t, ConfigAllNamespacesSet, "all_namespaces (set flag) stays unset when the key is absent")
	assert.False(t, ConfigEventsGrouping, "events.grouping applied")
	assert.True(t, ConfigEventsWarningsOnly, "events.warnings_only default preserved")
}
