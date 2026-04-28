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
	client := newTestClientForOptions(t)

	opts := StartupOptions{}
	m := NewModel(client, opts)

	// With an empty XDG_STATE_HOME (set in TestMain), loadSession returns nil,
	// so pendingSession should be nil when no CLI overrides are given.
	assert.Nil(t, m.pendingSession,
		"pendingSession should be nil (from loadSession) when no CLI overrides are given")
}

func TestNewModel_LogPreviewVisibleByDefault(t *testing.T) {
	client := newTestClientForOptions(t)
	m := NewModel(client, StartupOptions{})

	assert.True(t, m.logPreviewVisible,
		"log preview side panel should be visible by default; users can toggle off with P")
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
