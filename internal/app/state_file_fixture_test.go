package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pinnedFixtureYAML = `contexts:
  prod:
  - apps/deployments
  - batch/jobs
union_sets:
  all:
  - apps/deployments
`

const portForwardFixtureYAML = `port_forwards:
- context: prod
  local_port: "8080"
  namespace: default
  remote_port: "80"
  resource_kind: pod
  resource_name: web-0
`

const securityIgnoresFixtureYAML = `contexts:
  prod:
  - comment: known false positive
    created_at: "2026-01-01T00:00:00Z"
    group_key: CVE-2024-1234
    source: heuristic
`

const columnPrefsFixtureYAML = `contexts:
  prod:
    apps/deployments:
      hidden_builtins:
      - labels
      order:
      - name
      - ready
      visible_extras:
      - age
`

const hiddenTypesFixtureYAML = `contexts:
  prod:
  - networking.k8s.io/ingresses
union_sets:
  all:
  - /limitranges
`

func writeStateFixture(t *testing.T, name, content string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LFK_STATE_DIR", dir)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestPinnedState_LoadsMainFixture(t *testing.T) {
	writeStateFixture(t, "pinned.yaml", pinnedFixtureYAML)

	got := loadPinnedState()

	assert.Equal(t, []string{"apps/deployments", "batch/jobs"}, got.Contexts["prod"])
	assert.Equal(t, []string{"apps/deployments"}, got.UnionSets["all"])
}

func TestPinnedState_SaveReproducesFixtureBytes(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	state := &PinnedState{
		Contexts:  map[string][]string{"prod": {"apps/deployments", "batch/jobs"}},
		UnionSets: map[string][]string{"all": {"apps/deployments"}},
	}

	require.NoError(t, savePinnedState(state))
	got, err := os.ReadFile(pinnedFilePath())

	require.NoError(t, err)
	assert.Equal(t, pinnedFixtureYAML, string(got))
}

func TestPinnedSummariesState_LoadsMainFixture(t *testing.T) {
	writeStateFixture(t, "pinned_summaries.yaml", pinnedFixtureYAML)

	got := loadPinnedSummariesState()

	assert.Equal(t, []string{"apps/deployments", "batch/jobs"}, got.Contexts["prod"])
	assert.Equal(t, []string{"apps/deployments"}, got.UnionSets["all"])
}

func TestPinnedSummariesState_SaveReproducesFixtureBytes(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	state := &PinnedState{
		Contexts:  map[string][]string{"prod": {"apps/deployments", "batch/jobs"}},
		UnionSets: map[string][]string{"all": {"apps/deployments"}},
	}

	require.NoError(t, savePinnedSummariesState(state))
	got, err := os.ReadFile(stateFilePath(pinnedSummariesFileName))

	require.NoError(t, err)
	assert.Equal(t, pinnedFixtureYAML, string(got))
}

func TestPortForwardState_LoadsMainFixture(t *testing.T) {
	writeStateFixture(t, "portforwards.yaml", portForwardFixtureYAML)

	got := loadPortForwardState()

	require.Len(t, got.PortForwards, 1)
	assert.Equal(t, PortForwardState{
		ResourceKind: "pod", ResourceName: "web-0", Namespace: "default",
		Context: "prod", LocalPort: "8080", RemotePort: "80",
	}, got.PortForwards[0])
}

func TestPortForwardState_SaveReproducesFixtureBytes(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	state := &PortForwardStates{PortForwards: []PortForwardState{
		{ResourceKind: "pod", ResourceName: "web-0", Namespace: "default", Context: "prod", LocalPort: "8080", RemotePort: "80"},
	}}

	require.NoError(t, savePortForwardState(state))
	got, err := os.ReadFile(portForwardStatePath())

	require.NoError(t, err)
	assert.Equal(t, portForwardFixtureYAML, string(got))
}

func TestSecurityIgnoreState_LoadsMainFixture(t *testing.T) {
	writeStateFixture(t, "security_ignores.yaml", securityIgnoresFixtureYAML)

	got := loadSecurityIgnores()

	require.Len(t, got.Contexts["prod"], 1)
	assert.Equal(t, SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "CVE-2024-1234",
		Comment: "known false positive", CreatedAt: "2026-01-01T00:00:00Z",
	}, got.Contexts["prod"][0])
}

func TestSecurityIgnoreState_SaveReproducesFixtureBytes(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	state := &SecurityIgnoreState{Contexts: map[string][]SecurityIgnoreRule{
		"prod": {{Source: "heuristic", GroupKey: "CVE-2024-1234", Comment: "known false positive", CreatedAt: "2026-01-01T00:00:00Z"}},
	}}

	require.NoError(t, saveSecurityIgnores(state))
	got, err := os.ReadFile(stateFilePath(securityIgnoresFileName))

	require.NoError(t, err)
	assert.Equal(t, securityIgnoresFixtureYAML, string(got))
}

func TestColumnPrefsState_LoadsMainFixture(t *testing.T) {
	writeStateFixture(t, "column_prefs.yaml", columnPrefsFixtureYAML)

	got := loadColumnPrefsState()

	assert.Equal(t, persistedColumnPrefs{
		Order:          []string{"name", "ready"},
		VisibleExtras:  []string{"age"},
		HiddenBuiltins: []string{"labels"},
	}, got.Contexts["prod"]["apps/deployments"])
}

func TestColumnPrefsState_SaveReproducesFixtureBytes(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	state := ColumnPrefsState{Contexts: map[string]map[string]persistedColumnPrefs{
		"prod": {"apps/deployments": {
			Order:          []string{"name", "ready"},
			VisibleExtras:  []string{"age"},
			HiddenBuiltins: []string{"labels"},
		}},
	}}

	require.NoError(t, saveColumnPrefsState(state))
	got, err := os.ReadFile(columnPrefsFilePath())

	require.NoError(t, err)
	assert.Equal(t, columnPrefsFixtureYAML, string(got))
}

func TestHiddenTypesState_LoadsMainFixture(t *testing.T) {
	writeStateFixture(t, "hidden_types.yaml", hiddenTypesFixtureYAML)

	got := loadHiddenTypesState()

	assert.Equal(t, []string{"networking.k8s.io/ingresses"}, got.Contexts["prod"])
	assert.Equal(t, []string{"/limitranges"}, got.UnionSets["all"])
}

func TestHiddenTypesState_SaveReproducesFixtureBytes(t *testing.T) {
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	state := &HiddenTypesState{
		Contexts:  map[string][]string{"prod": {"networking.k8s.io/ingresses"}},
		UnionSets: map[string][]string{"all": {"/limitranges"}},
	}

	require.NoError(t, saveHiddenTypesState(state))
	got, err := os.ReadFile(hiddenTypesFilePath())

	require.NoError(t, err)
	assert.Equal(t, hiddenTypesFixtureYAML, string(got))
}
