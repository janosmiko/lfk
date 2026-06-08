package ui

// Runtime globals for session-level startup defaults. Set from the
// split_preview / watch_mode / all_namespaces top-level keys and the events
// group (see config_apply.go); read by the app package when it constructs the
// initial model and fresh session tabs.

// ConfigSplitPreview is the startup default for the split preview pane.
// Default true (pane shown).
var ConfigSplitPreview = true

// ConfigWatchMode is the startup default for live watch/polling. Default true.
var ConfigWatchMode = true

// ConfigAllNamespaces is the startup default for the namespace scope: true
// shows all namespaces, false scopes to the context's default namespace.
// Default true.
var ConfigAllNamespaces = true

// ConfigEventsWarningsOnly is the startup default for the events view's
// warning-only filter. Default true.
var ConfigEventsWarningsOnly = true

// ConfigEventsGrouping is the startup default for grouping related events.
// Default true.
var ConfigEventsGrouping = true
