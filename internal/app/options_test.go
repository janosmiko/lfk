package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- HasCLIOverrides ---

func TestHasCLIOverrides(t *testing.T) {
	tests := []struct {
		name string
		opts StartupOptions
		want bool
	}{
		{
			name: "empty options returns false",
			opts: StartupOptions{},
			want: false,
		},
		{
			name: "only Context set returns true",
			opts: StartupOptions{Context: "my-ctx"},
			want: true,
		},
		{
			name: "only Namespaces set returns true",
			opts: StartupOptions{Namespaces: []string{"ns1"}},
			want: true,
		},
		{
			name: "both Context and Namespaces set returns true",
			opts: StartupOptions{Context: "my-ctx", Namespaces: []string{"ns1", "ns2"}},
			want: true,
		},
		{
			name: "only Kubeconfig set returns false",
			opts: StartupOptions{Kubeconfig: "/some/path"},
			want: false,
		},
		{
			name: "Kubeconfig with Context returns true",
			opts: StartupOptions{Kubeconfig: "/some/path", Context: "ctx"},
			want: true,
		},
		{
			name: "Kubeconfig with Namespaces returns true",
			opts: StartupOptions{Kubeconfig: "/some/path", Namespaces: []string{"ns"}},
			want: true,
		},
		{
			name: "empty Namespaces slice returns false",
			opts: StartupOptions{Namespaces: []string{}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opts.HasCLIOverrides()
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- NewModel CLI override tests ---

// newTestClientForOptions creates a *k8s.Client with an in-memory kubeconfig
// containing a single "test-ctx" context. It does not require a real cluster.
func newTestClientForOptions(t *testing.T) *k8s.Client {
	t.Helper()
	return k8s.NewTestClient(nil, nil)
}

func TestNewModel_CLIOverrideContextOnly(t *testing.T) {
	client := newTestClientForOptions(t)

	opts := StartupOptions{Context: "test-ctx"}
	m := NewModel(client, opts)

	require.NotNil(t, m.pendingSession, "pendingSession should be set when Context CLI override is provided")
	assert.Equal(t, "test-ctx", m.pendingSession.Context)
	require.Len(t, m.pendingSession.Tabs, 1)
	assert.True(t, m.pendingSession.Tabs[0].AllNamespaces,
		"AllNamespaces should be true when no namespaces are provided")
	assert.Equal(t, "test-ctx", m.pendingSession.Tabs[0].Context)
}

func TestNewModel_SessionClearedByCLIOverride(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := newTestClientForOptions(t)

	// --session foo --context bar: the CLI override replaces the workspace, so
	// activeSession must be cleared to avoid clobbering foo.yaml on quit.
	m := NewModel(client, StartupOptions{Session: "foo", Context: "test-ctx"})
	assert.Equal(t, "", m.activeSession, "CLI override clears the active session")

	// --session foo alone: the named session stays active.
	m = NewModel(client, StartupOptions{Session: "foo"})
	assert.Equal(t, "foo", m.activeSession, "--session alone stays active")
}

func TestNewModel_CLIOverrideNamespacesOnly(t *testing.T) {
	client := newTestClientForOptions(t)

	opts := StartupOptions{Namespaces: []string{"ns1", "ns2"}}
	m := NewModel(client, opts)

	require.NotNil(t, m.pendingSession, "pendingSession should be set when Namespaces CLI override is provided")
	require.Len(t, m.pendingSession.Tabs, 1)

	tab := m.pendingSession.Tabs[0]
	assert.False(t, tab.AllNamespaces,
		"AllNamespaces should be false when specific namespaces are provided")
	assert.Equal(t, []string{"ns1", "ns2"}, tab.SelectedNamespaces)
	assert.Equal(t, "ns1", tab.Namespace, "Namespace should be first from the list")
}

func TestNewModel_CLIOverrideContextAndNamespaces(t *testing.T) {
	client := newTestClientForOptions(t)

	opts := StartupOptions{
		Context:    "test-ctx",
		Namespaces: []string{"staging"},
	}
	m := NewModel(client, opts)

	require.NotNil(t, m.pendingSession)
	assert.Equal(t, "test-ctx", m.pendingSession.Context)
	require.Len(t, m.pendingSession.Tabs, 1)

	tab := m.pendingSession.Tabs[0]
	assert.Equal(t, "test-ctx", tab.Context)
	assert.False(t, tab.AllNamespaces)
	assert.Equal(t, []string{"staging"}, tab.SelectedNamespaces)
}

func TestNewModel_NoCLIOverrides(t *testing.T) {
	// Other tests in this package save real sessions into TestMain's shared
	// XDG sandbox. An isolated state dir keeps loadSession's result (nil,
	// asserted below) independent of run order.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := newTestClientForOptions(t)

	opts := StartupOptions{}
	m := NewModel(client, opts)

	// With an empty XDG_STATE_HOME, loadSession returns nil, so pendingSession
	// should be nil when no CLI overrides are given.
	assert.Nil(t, m.pendingSession,
		"pendingSession should be nil (from loadSession) when no CLI overrides are given")
}

func TestNewModel_LogPreviewVisibleByDefault(t *testing.T) {
	// NewModel seeds logView.previewVisible from ui.ConfigLogShowPreview; pin it
	// to the default so a sibling test that mutates the global can't leak in.
	orig := ui.ConfigLogShowPreview
	t.Cleanup(func() { ui.ConfigLogShowPreview = orig })
	ui.ConfigLogShowPreview = true

	client := newTestClientForOptions(t)
	m := NewModel(client, StartupOptions{})

	assert.True(t, m.logView.previewVisible,
		"log preview side panel should be visible by default; users can toggle off with P")
}

// TestNewModel_LogViewDefaultsFromConfig verifies the log viewer toggles
// (preview/prefixes/timestamps) are seeded from their config globals so users
// can set their preferred startup state (issue #377).
func TestNewModel_LogViewDefaultsFromConfig(t *testing.T) {
	origPreview := ui.ConfigLogShowPreview
	origPrefixes := ui.ConfigLogShowPrefixes
	origTimestamps := ui.ConfigLogShowTimestamps
	t.Cleanup(func() {
		ui.ConfigLogShowPreview = origPreview
		ui.ConfigLogShowPrefixes = origPrefixes
		ui.ConfigLogShowTimestamps = origTimestamps
	})

	ui.ConfigLogShowPreview = false
	ui.ConfigLogShowPrefixes = false
	ui.ConfigLogShowTimestamps = true

	client := newTestClientForOptions(t)
	m := NewModel(client, StartupOptions{})

	assert.False(t, m.logView.previewVisible, "previewVisible should follow log_show_preview")
	assert.True(t, m.logView.hidePrefixes, "hidePrefixes is the inverse of log_show_prefixes")
	assert.True(t, m.logView.timestamps, "timestamps should follow log_show_timestamps")
}

// TestNewModel_ViewerDefaultsFromConfig verifies the YAML / diff / describe
// viewer toggles are seeded from their config globals at model construction.
func TestNewModel_ViewerDefaultsFromConfig(t *testing.T) {
	origYAML := ui.ConfigYAMLViewerWrap
	origDiffWrap := ui.ConfigDiffViewerWrap
	origDiffNums := ui.ConfigDiffViewerLineNumbers
	origDiffUnified := ui.ConfigDiffViewerUnified
	origDescribe := ui.ConfigDescribeViewerWrap
	t.Cleanup(func() {
		ui.ConfigYAMLViewerWrap = origYAML
		ui.ConfigDiffViewerWrap = origDiffWrap
		ui.ConfigDiffViewerLineNumbers = origDiffNums
		ui.ConfigDiffViewerUnified = origDiffUnified
		ui.ConfigDescribeViewerWrap = origDescribe
	})

	ui.ConfigYAMLViewerWrap = true
	ui.ConfigDiffViewerWrap = true
	ui.ConfigDiffViewerLineNumbers = false
	ui.ConfigDiffViewerUnified = true
	ui.ConfigDescribeViewerWrap = true

	client := newTestClientForOptions(t)
	m := NewModel(client, StartupOptions{})

	assert.True(t, m.yamlView.wrap, "yamlView.wrap follows yaml_viewer.wrap")
	assert.True(t, m.diffView.wrap, "diffView.wrap follows diff_viewer.wrap")
	assert.False(t, m.diffView.lineNumbers, "diffView.lineNumbers follows diff_viewer.line_numbers")
	assert.True(t, m.diffView.unified, "diffView.unified follows diff_viewer.unified")
	assert.True(t, m.describeView.wrap, "describeView.wrap follows describe_viewer.wrap")
}

// TestNewModel_SessionDefaultsFromConfig verifies the session-level switches
// seed the model and its initial tab from config (no CLI overrides).
func TestNewModel_SessionDefaultsFromConfig(t *testing.T) {
	origSplit := ui.ConfigSplitPreview
	origWatch := ui.ConfigWatchMode
	origAllNs := ui.ConfigAllNamespaces
	origWarn := ui.ConfigEventsWarningsOnly
	origGroup := ui.ConfigEventsGrouping
	t.Cleanup(func() {
		ui.ConfigSplitPreview = origSplit
		ui.ConfigWatchMode = origWatch
		ui.ConfigAllNamespaces = origAllNs
		ui.ConfigEventsWarningsOnly = origWarn
		ui.ConfigEventsGrouping = origGroup
	})

	ui.ConfigSplitPreview = false
	ui.ConfigWatchMode = false
	ui.ConfigAllNamespaces = false
	ui.ConfigEventsWarningsOnly = false
	ui.ConfigEventsGrouping = false

	client := newTestClientForOptions(t)
	m := NewModel(client, StartupOptions{})

	assert.False(t, m.splitPreview, "splitPreview follows split_preview")
	assert.False(t, m.watchMode, "watchMode follows watch_mode")
	assert.False(t, m.allNamespaces, "allNamespaces follows all_namespaces")
	assert.False(t, m.warningEventsOnly, "warningEventsOnly follows events.warnings_only")
	assert.False(t, m.eventGrouping, "eventGrouping follows events.grouping")
	require.NotEmpty(t, m.tabs)
	assert.False(t, m.tabs[0].splitPreview, "initial tab splitPreview follows config")
	assert.False(t, m.tabs[0].watchMode, "initial tab watchMode follows config")
}

func TestNewModel_CLIOverrideSingleNamespace(t *testing.T) {
	client := newTestClientForOptions(t)

	opts := StartupOptions{Namespaces: []string{"production"}}
	m := NewModel(client, opts)

	require.NotNil(t, m.pendingSession)
	require.Len(t, m.pendingSession.Tabs, 1)

	tab := m.pendingSession.Tabs[0]
	assert.False(t, tab.AllNamespaces)
	assert.Equal(t, "production", tab.Namespace)
	assert.Equal(t, []string{"production"}, tab.SelectedNamespaces)
}

func TestNewModel_KubeconfigOnlyDoesNotCreateSyntheticSession(t *testing.T) {
	// Isolated so a session saved by another test in the shared XDG sandbox
	// cannot make loadSession return non-nil here.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := newTestClientForOptions(t)

	opts := StartupOptions{Kubeconfig: "/some/kubeconfig"}
	m := NewModel(client, opts)

	// Kubeconfig alone does not trigger HasCLIOverrides, so pendingSession
	// should come from loadSession (nil in test environment).
	assert.Nil(t, m.pendingSession,
		"pendingSession should not be overridden when only Kubeconfig is set")
}

func TestNewModel_CLIOverrideUsesCorrectContextForDefaultNamespace(t *testing.T) {
	// When a CLI context override is provided, NewModel should use that context
	// to look up the default namespace, not the kubeconfig's current-context.
	client := newTestClientForOptions(t)

	opts := StartupOptions{Context: "test-ctx"}
	m := NewModel(client, opts)

	// The test client has "test-ctx" with namespace "default".
	assert.Equal(t, "default", m.namespace)
}

func TestNewModel_WatchIntervalPrecedence(t *testing.T) {
	orig := ui.ConfigWatchInterval
	t.Cleanup(func() { ui.ConfigWatchInterval = orig })

	tests := []struct {
		name       string
		cfgValue   time.Duration
		cliValue   time.Duration
		wantModel  time.Duration
		wantReason string
	}{
		{
			name:       "no overrides uses default 2s",
			cfgValue:   ui.DefaultWatchInterval,
			cliValue:   0,
			wantModel:  2 * time.Second,
			wantReason: "default",
		},
		{
			name:       "config value wins when no CLI",
			cfgValue:   5 * time.Second,
			cliValue:   0,
			wantModel:  5 * time.Second,
			wantReason: "config overrides default",
		},
		{
			name:       "CLI value overrides config",
			cfgValue:   5 * time.Second,
			cliValue:   10 * time.Second,
			wantModel:  10 * time.Second,
			wantReason: "CLI wins over config",
		},
		{
			name:       "CLI below min clamps to 500ms",
			cfgValue:   2 * time.Second,
			cliValue:   100 * time.Millisecond,
			wantModel:  500 * time.Millisecond,
			wantReason: "CLI clamped up",
		},
		{
			name:       "CLI above max clamps to 10m",
			cfgValue:   2 * time.Second,
			cliValue:   15 * time.Minute,
			wantModel:  10 * time.Minute,
			wantReason: "CLI clamped down",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ui.ConfigWatchInterval = tc.cfgValue
			client := newTestClientForOptions(t)

			m := NewModel(client, StartupOptions{WatchInterval: tc.cliValue})

			assert.Equal(t, tc.wantModel, m.watchInterval, tc.wantReason)
		})
	}
}

func TestNewModel_BasicFieldsInitialized(t *testing.T) {
	// allNamespaces / splitPreview are seeded from config globals; pin them to
	// their defaults so a sibling test mutating the globals can't leak in.
	origNs, origSplit := ui.ConfigAllNamespaces, ui.ConfigSplitPreview
	t.Cleanup(func() {
		ui.ConfigAllNamespaces = origNs
		ui.ConfigSplitPreview = origSplit
	})
	ui.ConfigAllNamespaces = true
	ui.ConfigSplitPreview = true

	client := newTestClientForOptions(t)

	m := NewModel(client, StartupOptions{})

	assert.NotNil(t, m.cursorMemory)
	assert.NotNil(t, m.itemCache)
	assert.NotNil(t, m.selectedItems)
	assert.NotNil(t, m.execMu)
	assert.Len(t, m.tabs, 1)
	assert.True(t, m.allNamespaces)
	assert.True(t, m.sortAscending)
	assert.True(t, m.splitPreview)

	// Verify the model uses the same sync.Mutex (not nil).
	_ = m.execMu
}
