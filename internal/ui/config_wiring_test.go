package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// fullConfigYAML sets every configFile field to a distinctive, non-default
// value so the wiring test can prove each one flows from YAML through
// LoadConfig into its resolved runtime global. Keep it in sync with
// configFile when adding settings (TestConfigFile_EveryFieldHasWiringCoverage
// enforces this).
const fullConfigYAML = `
colorscheme: dracula
log_path: /tmp/lfk-wiring-test.log
icons: nerdfont
dashboard: false
terminal: mux
scrollback_lines: 8000
pinned_groups: [karpenter.sh]
pinned_types: [argoproj.io/applications]
pinned_summaries: [batch/jobs, argoproj.io/applications]
tips: false
log_tail_lines: 250
log_tail_lines_short: 7
log_render_ansi: false
log_top_default_profile: traefik-json
log_viewer:
  show_preview: false
  show_prefixes: false
  show_timestamps: true
  max_lines: 12345
  preview_live: true
  wrap: true
yaml_viewer:
  wrap: true
diff_viewer:
  wrap: true
  line_numbers: false
  unified: true
describe_viewer:
  wrap: true
object_explorer:
  live: false
  tree: true
api_explorer:
  tree: true
split_preview: false
watch_mode: false
watch_throttle: false
all_namespaces: false
events:
  warnings_only: false
  grouping: false
scrolloff: 9
confirm_on_exit: false
delete_propagation_policy: orphan
dim_overlay: false
row_status_tint: background
transparent_background: true
mouse: false
watch_interval: 3s
background_watch_interval: 45s
foreground_idle_timeout: 300s
metrics_interval: 25s
metrics_sparkline_windows: ["2m", "30m"]
metrics_sparkline_width: 8
metrics_sparkline_interval: 45s
no_color: true
secret_lazy_loading: true
informer_cache: always
min_contrast_ratio: 0.5
read_only: true
field_manager: lfk-ci
show_rare_types: true
kubeconfig_dir: /tmp/lfk-kcfg
kubeconfig_exclusive: false
abbreviations:
  zz: pod
keybindings:
  refresh: ctrl+f5
  which_key_leader: ctrl+k
resource_columns:
  Pod: [IP, Node]
views:
  deployment:
    columns: [Name]
    sort_column: Name
custom_actions:
  Pod:
    - label: Ping
      command: echo ping
      key: ctrl+z
      description: ping the pod
filter_presets:
  Pod:
    - name: Pending
      key: p
      match:
        status: Pending
goto_targets:
  gx:
    kind: Deployment
which_key_enabled: false
which_key_delay_ms: 250
which_key_leader_delay_ms: 450
which_key_grouped: false
monitoring:
  _global:
    node_metrics: prometheus
    prometheus:
      namespaces: [monitoring]
      services: [prom]
      port: "9090"
kubeshark:
  namespace: traffic-ns
security:
  enabled: false
  hide_badges: true
  sources:
    trivy: false
    heuristic: true
  ignore_patterns:
    - cluster: prod
      source: trivy-operator
      comment: noisy
    - labels:
        k8s-app: cilium
      comment: CNI
  heuristic:
    secret_env_include:
      - "*_CONN_STR"
    secret_env_exclude:
      - "LEGACY_*"
    scan_secrets: false
rightsizing_defaults:
  strategy: prom_max_1d
  headroom: 1.5
scheduler:
  workers_per_context: 10
  critical_reserved_slots: 2
  low_reserved_slots: 3
  default_timeout: 45s
  aging_threshold: 4
  show_priority_in_tasks_overlay: false
  k8s_client_qps: 77
  k8s_client_burst: 144
  timeouts_by_kind:
    Metrics: 90s
union_sets:
  staging:
    namespace: stg
    contexts:
      - context: ctxA
        color: green
clusters:
  ctx1:
    read_only: true
    k8s_client_qps: 33
    k8s_client_burst: 66
    resource_columns:
      Pod: [IP, Node]
    views:
      deployment:
        columns: [Name]
        sort_column: Name
    security:
      enabled: false
      hide_badges: true
      sources:
        falco: false
`

// TestLoadConfig_AllSettingsWired writes a config that exercises every setting
// and asserts each one reaches its resolved runtime global. This is the
// end-to-end proof that YAML keys are wired, not just parsed.
func TestLoadConfig_AllSettingsWired(t *testing.T) {
	restore := snapshotAllConfigGlobals(t)
	defer restore()

	// resolveIconMode honors LFK_ICONS and applyConfigOptions honors NO_COLOR;
	// an empty value reads as "unset" in both (they test != ""), so the
	// assertions reflect the config file alone. t.Setenv auto-restores.
	t.Setenv("LFK_ICONS", "")
	t.Setenv("NO_COLOR", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(fullConfigYAML), 0o600))

	LoadConfig(path)

	// Identity / pipeline-level settings.
	assert.Equal(t, "dracula", ActiveSchemeName, "colorscheme")
	assert.Equal(t, "/tmp/lfk-wiring-test.log", ConfigLogPath, "log_path")
	assert.Equal(t, "ctrl+f5", ActiveKeybindings.Refresh, "keybindings")
	assert.Equal(t, "ctrl+k", ActiveKeybindings.WhichKeyLeader, "keybindings.which_key_leader")
	assert.Equal(t, "pod", SearchAbbreviations["zz"], "abbreviations")

	// Scalar settings (applyConfigOptions).
	assert.Equal(t, "nerdfont", IconMode, "icons")
	assert.False(t, ConfigDashboard, "dashboard")
	assert.Equal(t, TerminalModeMux, ConfigTerminalMode, "terminal")
	assert.Equal(t, 8000, ConfigScrollbackLines, "scrollback_lines")
	assert.Equal(t, []string{"karpenter.sh"}, ConfigPinnedGroups, "pinned_groups")
	assert.Equal(t, []string{"argoproj.io/applications"}, ConfigPinnedTypes, "pinned_types")
	assert.Equal(t, []string{"batch/jobs", "argoproj.io/applications"}, ConfigPinnedSummaries, "pinned_summaries")
	assert.False(t, ConfigTipsEnabled, "tips")
	assert.Equal(t, 250, ConfigLogTailLines, "log_tail_lines")
	assert.Equal(t, 7, ConfigLogTailLinesShort, "log_tail_lines_short")
	assert.False(t, ConfigLogRenderAnsi, "log_render_ansi")
	assert.Equal(t, "traefik-json", ConfigLogTopDefaultProfile, "log_top_default_profile")
	assert.False(t, ConfigLogShowPreview, "log_viewer.show_preview")
	assert.False(t, ConfigLogShowPrefixes, "log_viewer.show_prefixes")
	assert.True(t, ConfigLogShowTimestamps, "log_viewer.show_timestamps")
	assert.Equal(t, 12345, ConfigLogMaxLines, "log_viewer.max_lines")
	assert.True(t, ConfigLogPreviewLive, "log_viewer.preview_live")
	assert.True(t, ConfigLogWrap, "log_viewer.wrap")
	assert.True(t, ConfigYAMLViewerWrap, "yaml_viewer.wrap")
	assert.True(t, ConfigDiffViewerWrap, "diff_viewer.wrap")
	assert.False(t, ConfigDiffViewerLineNumbers, "diff_viewer.line_numbers")
	assert.True(t, ConfigDiffViewerUnified, "diff_viewer.unified")
	assert.True(t, ConfigDescribeViewerWrap, "describe_viewer.wrap")
	assert.False(t, ConfigObjectExplorerLive, "object_explorer.live")
	assert.True(t, ConfigObjectExplorerTree, "object_explorer.tree")
	assert.True(t, ConfigAPIExplorerTree, "api_explorer.tree")
	assert.False(t, ConfigSplitPreview, "split_preview")
	assert.False(t, ConfigWatchMode, "watch_mode")
	assert.False(t, ConfigWatchThrottle, "watch_throttle")
	assert.False(t, ConfigAllNamespaces, "all_namespaces")
	assert.True(t, ConfigAllNamespacesSet, "all_namespaces (set flag)")
	assert.False(t, ConfigEventsWarningsOnly, "events.warnings_only")
	assert.False(t, ConfigEventsGrouping, "events.grouping")
	assert.Equal(t, 9, ConfigScrollOff, "scrolloff")
	assert.False(t, ConfigConfirmOnExit, "confirm_on_exit")
	assert.Equal(t, model.DeletePropagationOrphan, ConfigDeletePropagationPolicy, "delete_propagation_policy")
	assert.False(t, ConfigDimOverlay, "dim_overlay")
	assert.Equal(t, RowStatusTintBackground, ConfigRowStatusTint, "row_status_tint")
	assert.True(t, ConfigTransparentBg, "transparent_background")
	assert.False(t, ConfigMouse, "mouse")
	assert.Equal(t, 3*time.Second, ConfigWatchInterval, "watch_interval")
	assert.Equal(t, 45*time.Second, ConfigBackgroundWatchInterval, "background_watch_interval")
	assert.Equal(t, 300*time.Second, ConfigForegroundIdleTimeout, "foreground_idle_timeout")
	assert.Equal(t, 25*time.Second, ConfigMetricsInterval, "metrics_interval")
	assert.Equal(t, []time.Duration{2 * time.Minute, 30 * time.Minute}, ConfigSparklineWindows, "metrics_sparkline_windows")
	assert.Equal(t, 8, ConfigSparklineWidth, "metrics_sparkline_width")
	assert.Equal(t, 45*time.Second, ConfigSparklineInterval, "metrics_sparkline_interval")
	assert.True(t, ConfigNoColor, "no_color")
	assert.True(t, ConfigSecretLazyLoading, "secret_lazy_loading")
	assert.Equal(t, InformerCacheAlways, ConfigInformerCacheMode, "informer_cache")
	assert.InDelta(t, 0.5, ConfigMinContrastRatio, 1e-9, "min_contrast_ratio")
	assert.True(t, ConfigReadOnly, "read_only")
	assert.Equal(t, "lfk-ci", k8s.FieldManagerOverride, "field_manager")
	assert.Equal(t, "lfk-ci", k8s.FieldManager(), "field_manager reaches the write path")
	assert.True(t, ConfigShowRareTypes, "show_rare_types")
	assert.Equal(t, []string{"/tmp/lfk-kcfg"}, ConfigKubeconfigDirs, "kubeconfig_dir")
	assert.False(t, ConfigKubeconfigExclusive, "kubeconfig_exclusive")
	assert.Equal(t, "traffic-ns", ConfigKubesharkNamespace, "kubeshark")

	// security section.
	assert.False(t, ConfigSecurityEnabled, "security.enabled")
	assert.True(t, ConfigSecurityHideBadges, "security.hide_badges")
	assert.Equal(t, map[string]bool{"trivy": false, "heuristic": true}, ConfigSecuritySources, "security.sources")
	require.Len(t, ConfigSecurityIgnorePatterns, 2, "security.ignore_patterns")
	assert.Equal(t, "prod", ConfigSecurityIgnorePatterns[0].Cluster)
	assert.Equal(t, map[string]string{"k8s-app": "cilium"}, ConfigSecurityIgnorePatterns[1].Labels,
		"security.ignore_patterns[].labels")
	assert.Equal(t, []string{"*_CONN_STR"}, ConfigSecuritySecretEnvInclude, "security.heuristic.secret_env_include")
	assert.Equal(t, []string{"LEGACY_*"}, ConfigSecuritySecretEnvExclude, "security.heuristic.secret_env_exclude")
	assert.False(t, ConfigSecurityScanSecrets, "security.heuristic.scan_secrets")

	// rightsizing_defaults.
	assert.Equal(t, model.StrategyPromMax1D, model.ConfigDefaultRightsizingStrategy, "rightsizing_defaults.strategy")
	assert.InDelta(t, 1.5, model.ConfigDefaultRightsizingHeadroom, 1e-9, "rightsizing_defaults.headroom")

	// scheduler section (+ global k8s client rate).
	assert.Equal(t, 10, scheduler.ConfigWorkersPerContext, "scheduler.workers_per_context")
	assert.Equal(t, 2, scheduler.ConfigCriticalReserved, "scheduler.critical_reserved_slots")
	assert.Equal(t, 3, scheduler.ConfigLowReserved, "scheduler.low_reserved_slots")
	assert.Equal(t, 45*time.Second, scheduler.ConfigDefaultTimeout, "scheduler.default_timeout")
	assert.Equal(t, 4, scheduler.ConfigAgingThreshold, "scheduler.aging_threshold")
	assert.False(t, scheduler.ConfigShowPriorityInOverlay, "scheduler.show_priority_in_tasks_overlay")
	assert.Equal(t, 90*time.Second, scheduler.ConfigTimeoutsByKind[scheduler.KindMetrics], "scheduler.timeouts_by_kind")
	assert.Equal(t, 77, ConfigK8sClientQPS, "scheduler.k8s_client_qps")
	assert.Equal(t, 144, ConfigK8sClientBurst, "scheduler.k8s_client_burst")

	// monitoring.
	require.Contains(t, model.ConfigMonitoring, "_global", "monitoring")
	assert.Equal(t, "prometheus", model.ConfigMonitoring["_global"].NodeMetrics)

	// union_sets (map form).
	require.Len(t, ConfigUnionSets, 1, "union_sets")
	assert.Equal(t, "staging", ConfigUnionSets[0].Name)
	assert.Equal(t, "stg", ConfigUnionSets[0].Namespace)
	require.Len(t, ConfigUnionSets[0].Contexts, 1)
	assert.Equal(t, "ctxA", ConfigUnionSets[0].Contexts[0].Context)
	assert.Equal(t, "green", ConfigUnionSets[0].Contexts[0].Color)

	// Top-level maps (applyConfigMaps).
	assert.Equal(t, []string{"IP", "Node"}, ConfigResourceColumns["pod"], "resource_columns")
	require.Contains(t, ConfigViews, "deployment", "views")
	require.NotNil(t, ConfigViews["deployment"])
	require.Contains(t, ConfigCustomActions, "Pod", "custom_actions")
	assert.Equal(t, "Ping", ConfigCustomActions["Pod"][0].Label)
	require.Contains(t, ConfigFilterPresets, "pod", "filter_presets")
	assert.Equal(t, "Pending", ConfigFilterPresets["pod"][0].Match.Status)
	assert.Equal(t, "Deployment", ConfigGotoTargets["gx"].Kind, "goto_targets")
	assert.False(t, ConfigWhichKeyEnabled, "which_key_enabled")
	assert.Equal(t, 250, ConfigWhichKeyDelayMs, "which_key_delay_ms")
	assert.Equal(t, 450, ConfigWhichKeyLeaderDelayMs, "which_key_leader_delay_ms")
	assert.False(t, ConfigWhichKeyGrouped, "which_key_grouped")

	// Per-cluster overrides (clusters.<ctx>.*).
	assert.True(t, ConfigClusterReadOnly["ctx1"], "clusters.read_only")
	assert.Equal(t, 33, ConfigClusterK8sClientQPS["ctx1"], "clusters.k8s_client_qps")
	assert.Equal(t, 66, ConfigClusterK8sClientBurst["ctx1"], "clusters.k8s_client_burst")
	assert.Equal(t, []string{"IP", "Node"}, ConfigClusterResourceColumns["ctx1"]["pod"], "clusters.resource_columns")
	require.Contains(t, ConfigClusterViews, "ctx1", "clusters.views")
	require.Contains(t, ConfigClusterViews["ctx1"], "deployment")
	assert.False(t, ConfigClusterSecurityEnabled["ctx1"], "clusters.security.enabled")
	assert.True(t, ConfigClusterSecurityHideBadges["ctx1"], "clusters.security.hide_badges")
	assert.Equal(t, map[string]bool{"falco": false}, ConfigClusterSecuritySources["ctx1"], "clusters.security.sources")
}

// TestLoadConfig_PinnedSummariesEmptyListSetsFlag verifies an explicit
// `pinned_summaries: []` is distinguishable from the key being absent: it sets
// ConfigPinnedSummariesSet so loadDashboardFor can tell "no summaries, not
// even defaults" apart from "key absent, use the built-in defaults" - both
// otherwise leave ConfigPinnedSummaries equally empty.
func TestLoadConfig_PinnedSummariesEmptyListSetsFlag(t *testing.T) {
	restore := snapshotAllConfigGlobals(t)
	defer restore()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("pinned_summaries: []\n"), 0o600))

	LoadConfig(path)

	assert.True(t, ConfigPinnedSummariesSet, "an explicit [] must set the flag")
	assert.Empty(t, ConfigPinnedSummaries, "an explicit [] must leave the list empty")
}

// TestLoadConfig_PinnedSummariesAbsentKeyLeavesFlagUnset verifies the key
// being absent from the config file (as opposed to an explicit `[]`) leaves
// ConfigPinnedSummariesSet false, so loadDashboardFor's default-pins gate
// applies.
func TestLoadConfig_PinnedSummariesAbsentKeyLeavesFlagUnset(t *testing.T) {
	restore := snapshotAllConfigGlobals(t)
	defer restore()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("dashboard: true\n"), 0o600))

	LoadConfig(path)

	assert.False(t, ConfigPinnedSummariesSet, "an absent key must not set the flag")
	assert.Empty(t, ConfigPinnedSummaries)
}

// TestLoadConfig_AllNamespacesAbsentKeyLeavesFlagUnset verifies the key
// being absent from the config file leaves ConfigAllNamespacesSet false, so
// resolveStartupAllNamespaces falls through to a kubeconfig-pinned namespace.
func TestLoadConfig_AllNamespacesAbsentKeyLeavesFlagUnset(t *testing.T) {
	restore := snapshotAllConfigGlobals(t)
	defer restore()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("dashboard: true\n"), 0o600))

	LoadConfig(path)

	assert.False(t, ConfigAllNamespacesSet, "an absent key must not set the flag")
	assert.True(t, ConfigAllNamespaces, "an absent key must keep the compiled default")
}

// TestLoadConfig_AllNamespacesExplicitFalseSetsFlag verifies an explicit
// `all_namespaces: false` sets ConfigAllNamespacesSet, distinguishing it from
// the key being absent.
func TestLoadConfig_AllNamespacesExplicitFalseSetsFlag(t *testing.T) {
	restore := snapshotAllConfigGlobals(t)
	defer restore()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("all_namespaces: false\n"), 0o600))

	LoadConfig(path)

	assert.True(t, ConfigAllNamespacesSet, "an explicit false must set the flag")
	assert.False(t, ConfigAllNamespaces)
}

// wiringCoveredFields records, for every configFile json field, where its
// YAML->runtime wiring is asserted. TestConfigFile_EveryFieldHasWiringCoverage
// fails if a field is added (or removed) without updating this map, forcing new
// settings to ship with a wiring test.
var wiringCoveredFields = map[string]string{
	"appearance":                 "config_appearance_test.go (TestAppearance_*)",
	"colorscheme":                "TestLoadConfig_AllSettingsWired + TestApplyColorscheme_*",
	"theme":                      "TestMergeThemeOverrides (mergeThemeOverrides is the LoadConfig wiring point)",
	"keybindings":                "TestLoadConfig_AllSettingsWired + config_keybindings_test.go",
	"log_path":                   "TestLoadConfig_AllSettingsWired",
	"abbreviations":              "TestLoadConfig_AllSettingsWired",
	"icons":                      "TestLoadConfig_AllSettingsWired",
	"resource_columns":           "TestLoadConfig_AllSettingsWired",
	"views":                      "TestLoadConfig_AllSettingsWired",
	"dashboard":                  "TestLoadConfig_AllSettingsWired",
	"custom_actions":             "TestLoadConfig_AllSettingsWired",
	"filter_presets":             "TestLoadConfig_AllSettingsWired",
	"terminal":                   "TestLoadConfig_AllSettingsWired",
	"scrollback_lines":           "TestLoadConfig_AllSettingsWired",
	"pinned_groups":              "TestLoadConfig_AllSettingsWired",
	"pinned_types":               "TestLoadConfig_AllSettingsWired",
	"pinned_summaries":           "TestLoadConfig_AllSettingsWired + TestLoadConfig_PinnedSummariesEmptyListSetsFlag + TestLoadConfig_PinnedSummariesAbsentKeyLeavesFlagUnset",
	"monitoring":                 "TestLoadConfig_AllSettingsWired",
	"tips":                       "TestLoadConfig_AllSettingsWired",
	"log_tail_lines":             "TestLoadConfig_AllSettingsWired",
	"log_tail_lines_short":       "TestLoadConfig_AllSettingsWired",
	"log_render_ansi":            "TestLoadConfig_AllSettingsWired (deprecated flat alias)",
	"log_top_default_profile":    "TestLoadConfig_AllSettingsWired",
	"log_viewer":                 "TestLoadConfig_AllSettingsWired + config_log_viewer_test.go (TestLogViewer_*)",
	"yaml_viewer":                "TestLoadConfig_AllSettingsWired",
	"diff_viewer":                "TestLoadConfig_AllSettingsWired",
	"describe_viewer":            "TestLoadConfig_AllSettingsWired",
	"object_explorer":            "TestLoadConfig_AllSettingsWired",
	"api_explorer":               "TestLoadConfig_AllSettingsWired",
	"split_preview":              "TestLoadConfig_AllSettingsWired",
	"watch_mode":                 "TestLoadConfig_AllSettingsWired",
	"watch_throttle":             "TestLoadConfig_AllSettingsWired",
	"all_namespaces":             "TestLoadConfig_AllSettingsWired + TestLoadConfig_AllNamespacesExplicitFalseSetsFlag + TestLoadConfig_AllNamespacesAbsentKeyLeavesFlagUnset",
	"events":                     "TestLoadConfig_AllSettingsWired",
	"scrolloff":                  "TestLoadConfig_AllSettingsWired",
	"confirm_on_exit":            "TestLoadConfig_AllSettingsWired",
	"delete_propagation_policy":  "TestLoadConfig_AllSettingsWired + TestDeletePropagationPolicy_InvalidFallsBack",
	"dim_overlay":                "TestLoadConfig_AllSettingsWired",
	"row_status_tint":            "TestLoadConfig_AllSettingsWired + TestRowStatusTint_InvalidFallsBack",
	"transparent_background":     "TestLoadConfig_AllSettingsWired",
	"mouse":                      "TestLoadConfig_AllSettingsWired",
	"watch_interval":             "TestLoadConfig_AllSettingsWired",
	"background_watch_interval":  "TestLoadConfig_AllSettingsWired",
	"foreground_idle_timeout":    "TestLoadConfig_AllSettingsWired",
	"metrics_interval":           "TestLoadConfig_AllSettingsWired + TestApplyMetricsIntervalConfig",
	"metrics_sparkline_windows":  "TestLoadConfig_AllSettingsWired",
	"metrics_sparkline_width":    "TestLoadConfig_AllSettingsWired + TestClampSparklineWidth",
	"metrics_sparkline_interval": "TestLoadConfig_AllSettingsWired + TestClampSparklineInterval",
	"clusters":                   "TestLoadConfig_AllSettingsWired",
	"no_color":                   "TestLoadConfig_AllSettingsWired",
	"secret_lazy_loading":        "TestLoadConfig_AllSettingsWired",
	"informer_cache":             "TestLoadConfig_AllSettingsWired",
	"min_contrast_ratio":         "TestLoadConfig_AllSettingsWired",
	"read_only":                  "TestLoadConfig_AllSettingsWired",
	"field_manager":              "TestLoadConfig_AllSettingsWired + TestLoadConfig_FieldManagerBlankKeepsDefault",
	"show_rare_types":            "TestLoadConfig_AllSettingsWired",
	"security":                   "TestLoadConfig_AllSettingsWired",
	"rightsizing_defaults":       "TestLoadConfig_AllSettingsWired",
	"kubeshark":                  "TestLoadConfig_AllSettingsWired",
	"scheduler":                  "TestLoadConfig_AllSettingsWired",
	"kubeconfig_dir":             "TestLoadConfig_AllSettingsWired",
	"kubeconfig_exclusive":       "TestLoadConfig_AllSettingsWired",
	"union_sets":                 "TestLoadConfig_AllSettingsWired",
	"goto_targets":               "TestLoadConfig_AllSettingsWired + TestLoadConfig_GotoTargets",
	"which_key_enabled":          "TestLoadConfig_AllSettingsWired + TestLoadConfig_WhichKey",
	"which_key_delay_ms":         "TestLoadConfig_AllSettingsWired + TestLoadConfig_WhichKey",
	"which_key_leader_delay_ms":  "TestLoadConfig_AllSettingsWired + TestLoadConfig_WhichKeyLeaderDelay*",
	"which_key_grouped":          "TestLoadConfig_AllSettingsWired + TestLoadConfig_WhichKeyGrouped",
}

// TestConfigFile_EveryFieldHasWiringCoverage is a forcing function: it fails if
// a configFile field lacks an entry in wiringCoveredFields (a new setting was
// added without a wiring test) or if the map names a field that no longer
// exists (stale entry).
func TestConfigFile_EveryFieldHasWiringCoverage(t *testing.T) {
	fields := map[string]bool{}
	for f := range reflect.TypeFor[configFile]().Fields() {
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		fields[name] = true
		require.Containsf(t, wiringCoveredFields, name,
			"config field %q has no wiring-test coverage entry; add an assertion in config_wiring_test.go and record it in wiringCoveredFields", name)
	}
	for name := range wiringCoveredFields {
		require.Truef(t, fields[name], "wiringCoveredFields names %q, which is not a configFile field anymore; remove the stale entry", name)
	}
}

// snapshotAllConfigGlobals captures every runtime global the config apply path
// can mutate and returns a restore closure. Scalars are restored first, then
// ApplyTheme(origTheme) last so it rebuilds style globals against the restored
// no-color / contrast settings.
//
// Every test that calls LoadConfig needs this, including one that asserts a
// single unrelated scalar. Saving just that scalar is not enough: LoadConfig
// also applies the theme and the keybindings, and several tests pass
// "colorscheme: dracula" as the filler key that proves their own value stayed
// at its default. ColorError and its siblings derive from the theme, so putting
// the scalar back leaves them holding the other scheme's colours.
//
// Two seeds reproduced that before the callers below were fixed:
// -shuffle=1787629791574481000 failed TestNoColor_StripsAgeStyleColors with
// ColorError #fdd6dd instead of #f7768e, and -shuffle=1000013 failed
// TestHelpSections_EveryEntryHasAKey on leaked keybindings. Both pass now.
func snapshotAllConfigGlobals(t *testing.T) func() {
	t.Helper()

	origTheme := ActiveTheme
	origScheme := ActiveSchemeName
	origDarkScheme := ConfigDarkColorscheme
	origLightScheme := ConfigLightColorscheme
	origKB := ActiveKeybindings
	origAbbr := SearchAbbreviations
	origLogPath := ConfigLogPath
	origIcon := IconMode
	origDashboard := ConfigDashboard
	origTerminal := ConfigTerminalMode
	origScrollback := ConfigScrollbackLines
	origPinnedGroups := ConfigPinnedGroups
	origPinnedTypes := ConfigPinnedTypes
	origPinnedSummaries := ConfigPinnedSummaries
	origPinnedSummariesSet := ConfigPinnedSummariesSet
	origUnionSets := ConfigUnionSets
	origTips := ConfigTipsEnabled
	origTail := ConfigLogTailLines
	origTailShort := ConfigLogTailLinesShort
	origAnsi := ConfigLogRenderAnsi
	origLogTopProfile := ConfigLogTopDefaultProfile
	origShowPreview := ConfigLogShowPreview
	origShowPrefixes := ConfigLogShowPrefixes
	origShowTimestamps := ConfigLogShowTimestamps
	origLogMaxLines := ConfigLogMaxLines
	origLogPreviewLive := ConfigLogPreviewLive
	origYAMLWrap := ConfigYAMLViewerWrap
	origDiffWrap := ConfigDiffViewerWrap
	origDiffLineNums := ConfigDiffViewerLineNumbers
	origDiffUnified := ConfigDiffViewerUnified
	origDescribeWrap := ConfigDescribeViewerWrap
	origObjectExplorerLive := ConfigObjectExplorerLive
	origObjectExplorerTree := ConfigObjectExplorerTree
	origAPIExplorerTree := ConfigAPIExplorerTree
	origSplitPreview := ConfigSplitPreview
	origWatchMode := ConfigWatchMode
	origWatchThrottle := ConfigWatchThrottle
	origAllNamespaces := ConfigAllNamespaces
	origAllNamespacesSet := ConfigAllNamespacesSet
	origEventsWarn := ConfigEventsWarningsOnly
	origEventsGroup := ConfigEventsGrouping
	origScrollOff := ConfigScrollOff
	origConfirm := ConfigConfirmOnExit
	origDim := ConfigDimOverlay
	origTransparent := ConfigTransparentBg
	origMouse := ConfigMouse
	origWatch := ConfigWatchInterval
	origBackgroundWatch := ConfigBackgroundWatchInterval
	origForegroundIdle := ConfigForegroundIdleTimeout
	origMetricsInterval := ConfigMetricsInterval
	origNoColor := ConfigNoColor
	origKubeconfig := ConfigKubeconfigDirs
	origKubeconfigExclusive := ConfigKubeconfigExclusive
	origSecretLazy := ConfigSecretLazyLoading
	origKubeshark := ConfigKubesharkNamespace
	origInformer := ConfigInformerCacheMode
	origContrast := ConfigMinContrastRatio
	origReadOnly := ConfigReadOnly
	origFieldManager := k8s.FieldManagerOverride
	origShowRare := ConfigShowRareTypes
	origSecEnabled := ConfigSecurityEnabled
	origSecHideBadges := ConfigSecurityHideBadges
	origSecSources := ConfigSecuritySources
	origSecIgnore := ConfigSecurityIgnorePatterns
	origSecEnvInclude := ConfigSecuritySecretEnvInclude
	origSecEnvExclude := ConfigSecuritySecretEnvExclude
	origSecScanSecrets := ConfigSecurityScanSecrets
	origQPS := ConfigK8sClientQPS
	origBurst := ConfigK8sClientBurst
	origResCols := ConfigResourceColumns
	origViews := ConfigViews
	origCustom := ConfigCustomActions
	origPresets := ConfigFilterPresets
	origClusterReadOnly := ConfigClusterReadOnly
	origClusterResCols := ConfigClusterResourceColumns
	origClusterViews := ConfigClusterViews
	origClusterSecEnabled := ConfigClusterSecurityEnabled
	origClusterSecHideBadges := ConfigClusterSecurityHideBadges
	origClusterSecSources := ConfigClusterSecuritySources
	origClusterQPS := ConfigClusterK8sClientQPS
	origClusterBurst := ConfigClusterK8sClientBurst
	origGotoTargets := ConfigGotoTargets
	origWhichKeyEnabled := ConfigWhichKeyEnabled
	origWhichKeyDelayMs := ConfigWhichKeyDelayMs
	origSparkWindows := ConfigSparklineWindows
	origSparkWidth := ConfigSparklineWidth
	origSparkInterval := ConfigSparklineInterval

	origMonitoring := model.ConfigMonitoring
	origRSStrategy := model.ConfigDefaultRightsizingStrategy
	origRSHeadroom := model.ConfigDefaultRightsizingHeadroom

	origWorkers := scheduler.ConfigWorkersPerContext
	origCritical := scheduler.ConfigCriticalReserved
	origLow := scheduler.ConfigLowReserved
	origDefTimeout := scheduler.ConfigDefaultTimeout
	origByKind := scheduler.ConfigTimeoutsByKind
	origShowPrio := scheduler.ConfigShowPriorityInOverlay
	origAging := scheduler.ConfigAgingThreshold

	return func() {
		ActiveSchemeName = origScheme
		ConfigDarkColorscheme = origDarkScheme
		ConfigLightColorscheme = origLightScheme
		ActiveKeybindings = origKB
		SearchAbbreviations = origAbbr
		ConfigLogPath = origLogPath
		IconMode = origIcon
		ConfigDashboard = origDashboard
		ConfigTerminalMode = origTerminal
		ConfigScrollbackLines = origScrollback
		ConfigPinnedGroups = origPinnedGroups
		ConfigPinnedTypes = origPinnedTypes
		ConfigPinnedSummaries = origPinnedSummaries
		ConfigPinnedSummariesSet = origPinnedSummariesSet
		ConfigUnionSets = origUnionSets
		ConfigTipsEnabled = origTips
		ConfigLogTailLines = origTail
		ConfigLogTailLinesShort = origTailShort
		ConfigLogRenderAnsi = origAnsi
		ConfigLogTopDefaultProfile = origLogTopProfile
		ConfigLogShowPreview = origShowPreview
		ConfigLogShowPrefixes = origShowPrefixes
		ConfigLogShowTimestamps = origShowTimestamps
		ConfigLogMaxLines = origLogMaxLines
		ConfigLogPreviewLive = origLogPreviewLive
		ConfigYAMLViewerWrap = origYAMLWrap
		ConfigDiffViewerWrap = origDiffWrap
		ConfigDiffViewerLineNumbers = origDiffLineNums
		ConfigDiffViewerUnified = origDiffUnified
		ConfigDescribeViewerWrap = origDescribeWrap
		ConfigObjectExplorerLive = origObjectExplorerLive
		ConfigObjectExplorerTree = origObjectExplorerTree
		ConfigAPIExplorerTree = origAPIExplorerTree
		ConfigSplitPreview = origSplitPreview
		ConfigWatchMode = origWatchMode
		ConfigWatchThrottle = origWatchThrottle
		ConfigAllNamespaces = origAllNamespaces
		ConfigAllNamespacesSet = origAllNamespacesSet
		ConfigEventsWarningsOnly = origEventsWarn
		ConfigEventsGrouping = origEventsGroup
		ConfigScrollOff = origScrollOff
		ConfigConfirmOnExit = origConfirm
		ConfigDimOverlay = origDim
		ConfigTransparentBg = origTransparent
		ConfigMouse = origMouse
		ConfigWatchInterval = origWatch
		ConfigBackgroundWatchInterval = origBackgroundWatch
		ConfigForegroundIdleTimeout = origForegroundIdle
		ConfigMetricsInterval = origMetricsInterval
		ConfigNoColor = origNoColor
		ConfigKubeconfigDirs = origKubeconfig
		ConfigKubeconfigExclusive = origKubeconfigExclusive
		ConfigSecretLazyLoading = origSecretLazy
		ConfigKubesharkNamespace = origKubeshark
		ConfigInformerCacheMode = origInformer
		ConfigMinContrastRatio = origContrast
		ConfigReadOnly = origReadOnly
		k8s.FieldManagerOverride = origFieldManager
		ConfigShowRareTypes = origShowRare
		ConfigSecurityEnabled = origSecEnabled
		ConfigSecurityHideBadges = origSecHideBadges
		ConfigSecuritySources = origSecSources
		ConfigSecurityIgnorePatterns = origSecIgnore
		ConfigSecuritySecretEnvInclude = origSecEnvInclude
		ConfigSecuritySecretEnvExclude = origSecEnvExclude
		ConfigSecurityScanSecrets = origSecScanSecrets
		ConfigK8sClientQPS = origQPS
		ConfigK8sClientBurst = origBurst
		ConfigResourceColumns = origResCols
		ConfigViews = origViews
		ConfigCustomActions = origCustom
		ConfigFilterPresets = origPresets
		ConfigClusterReadOnly = origClusterReadOnly
		ConfigClusterResourceColumns = origClusterResCols
		ConfigClusterViews = origClusterViews
		ConfigClusterSecurityEnabled = origClusterSecEnabled
		ConfigClusterSecurityHideBadges = origClusterSecHideBadges
		ConfigClusterSecuritySources = origClusterSecSources
		ConfigClusterK8sClientQPS = origClusterQPS
		ConfigClusterK8sClientBurst = origClusterBurst
		ConfigGotoTargets = origGotoTargets
		ConfigWhichKeyEnabled = origWhichKeyEnabled
		ConfigWhichKeyDelayMs = origWhichKeyDelayMs
		ConfigSparklineWindows = origSparkWindows
		ConfigSparklineWidth = origSparkWidth
		ConfigSparklineInterval = origSparkInterval

		model.ConfigMonitoring = origMonitoring
		model.ConfigDefaultRightsizingStrategy = origRSStrategy
		model.ConfigDefaultRightsizingHeadroom = origRSHeadroom

		scheduler.ConfigWorkersPerContext = origWorkers
		scheduler.ConfigCriticalReserved = origCritical
		scheduler.ConfigLowReserved = origLow
		scheduler.ConfigDefaultTimeout = origDefTimeout
		scheduler.ConfigTimeoutsByKind = origByKind
		scheduler.ConfigShowPriorityInOverlay = origShowPrio
		scheduler.ConfigAgingThreshold = origAging

		ApplyTheme(origTheme)
	}
}

// TestLoadConfig_FieldManagerBlankKeepsDefault proves a blank or whitespace
// value cannot replace the derived "lfk:<user>" name with an empty string,
// which the apiserver would store verbatim.
func TestLoadConfig_FieldManagerBlankKeepsDefault(t *testing.T) {
	for _, value := range []string{`field_manager: ""`, `field_manager: "   "`, ""} {
		t.Run(value, func(t *testing.T) {
			restore := snapshotAllConfigGlobals(t)
			defer restore()

			k8s.FieldManagerOverride = ""
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(value+"\n"), 0o600))

			LoadConfig(path)

			assert.Empty(t, k8s.FieldManagerOverride)
			assert.True(t, strings.HasPrefix(k8s.FieldManager(), "lfk:"), "got %q", k8s.FieldManager())
		})
	}
}

// TestLoadConfig_FieldManagerRemovedKeyClearsTheOverride proves a reload after
// the key is deleted returns to the derived "lfk:<user>" name. A stale override
// would keep signing writes with an identity the config no longer names.
func TestLoadConfig_FieldManagerRemovedKeyClearsTheOverride(t *testing.T) {
	restore := snapshotAllConfigGlobals(t)
	defer restore()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	require.NoError(t, os.WriteFile(path, []byte("field_manager: lfk-ci\n"), 0o600))
	LoadConfig(path)
	require.Equal(t, "lfk-ci", k8s.FieldManagerOverride)

	for _, second := range []string{"", `field_manager: ""`, `field_manager: "   "`} {
		require.NoError(t, os.WriteFile(path, []byte(second+"\n"), 0o600))
		LoadConfig(path)

		assert.Empty(t, k8s.FieldManagerOverride, "reload with %q", second)
		assert.True(t, strings.HasPrefix(k8s.FieldManager(), "lfk:"), "got %q", k8s.FieldManager())
	}
}
