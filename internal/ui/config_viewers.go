package ui

// Runtime globals for fullscreen-viewer startup-toggle defaults. These are set
// from the log_viewer / yaml_viewer / diff_viewer / describe_viewer config
// groups (see config_apply.go) and read by the app package when it constructs
// or re-opens each viewer. Kept in their own file to keep config.go under the
// 800-line limit.

// ConfigLogShowPreview is the startup default for the log viewer's structured
// preview side panel (runtime toggle: P). Default true (panel shown).
var ConfigLogShowPreview = true

// ConfigLogShowPrefixes is the startup default for the [pod/name/container]
// line prefixes in the log viewer (runtime toggle: p). Default true (shown).
var ConfigLogShowPrefixes = true

// ConfigLogShowTimestamps is the startup default for log line timestamps
// (runtime toggle: s). Default false (timestamps hidden).
var ConfigLogShowTimestamps = false

// ConfigYAMLViewerWrap is the startup default for YAML viewer line wrapping
// (runtime toggle: z). Default false.
var ConfigYAMLViewerWrap = false

// ConfigDiffViewerWrap is the startup default for diff viewer line wrapping
// (runtime toggle: Ctrl+W / >). Default false.
var ConfigDiffViewerWrap = false

// ConfigDiffViewerLineNumbers is the startup default for diff viewer gutter
// line numbers (runtime toggle: #). Default true.
var ConfigDiffViewerLineNumbers = true

// ConfigDiffViewerUnified is the startup default for the diff viewer's unified
// (vs side-by-side) layout (runtime toggle: u). Default false.
var ConfigDiffViewerUnified = false

// ConfigDescribeViewerWrap is the startup default for describe viewer line
// wrapping (runtime toggle: z). Default false.
var ConfigDescribeViewerWrap = false

// ConfigObjectExplorerLive is the startup default for live-refreshing the
// Object Explorer as the browsed resource changes under watch mode (runtime
// toggle: w). Default true.
var ConfigObjectExplorerLive = true
