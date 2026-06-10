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
tips: false
log_tail_lines: 250
log_tail_lines_short: 7
log_render_ansi: false
log_viewer:
  show_preview: false
  show_prefixes: false
  show_timestamps: true
  max_lines: 12345
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
split_preview: false
watch_mode: false
all_namespaces: false
events:
  warnings_only: false
  grouping: false
scrolloff: 9
confirm_on_exit: false
dim_overlay: false
transparent_background: true
mouse: false
watch_interval: 3s
no_color: true
secret_lazy_loading: true
informer_cache: always
min_contrast_ratio: 0.5
read_only: true
show_rare_types: true
kubeconfig_dir: /tmp/lfk-kcfg
abbreviations:
  zz: pod
keybindings:
  refresh: ctrl+f5
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
	assert.Equal(t, "pod", SearchAbbreviations["zz"], "abbreviations")

	// Scalar settings (applyConfigOptions).
	assert.Equal(t, "nerdfont", IconMode, "icons")
	assert.False(t, ConfigDashboard, "dashboard")
	assert.Equal(t, TerminalModeMux, ConfigTerminalMode, "terminal")
	assert.Equal(t, 8000, ConfigScrollbackLines, "scrollback_lines")
	assert.Equal(t, []string{"karpenter.sh"}, ConfigPinnedGroups, "pinned_groups")
	assert.Equal(t, []string{"argoproj.io/applications"}, ConfigPinnedTypes, "pinned_types")
	assert.False(t, ConfigTipsEnabled, "tips")
	assert.Equal(t, 250, ConfigLogTailLines, "log_tail_lines")
	assert.Equal(t, 7, ConfigLogTailLinesShort, "log_tail_lines_short")
	assert.False(t, ConfigLogRenderAnsi, "log_render_ansi")
	assert.False(t, ConfigLogShowPreview, "log_viewer.show_preview")
	assert.False(t, ConfigLogShowPrefixes, "log_viewer.show_prefixes")
	assert.True(t, ConfigLogShowTimestamps, "log_viewer.show_timestamps")
	assert.Equal(t, 12345, ConfigLogMaxLines, "log_viewer.max_lines")
	assert.True(t, ConfigYAMLViewerWrap, "yaml_viewer.wrap")
	assert.True(t, ConfigDiffViewerWrap, "diff_viewer.wrap")
	assert.False(t, ConfigDiffViewerLineNumbers, "diff_viewer.line_numbers")
	assert.True(t, ConfigDiffViewerUnified, "diff_viewer.unified")
	assert.True(t, ConfigDescribeViewerWrap, "describe_viewer.wrap")
	assert.False(t, ConfigObjectExplorerLive, "object_explorer.live")
	assert.False(t, ConfigSplitPreview, "split_preview")
	assert.False(t, ConfigWatchMode, "watch_mode")
	assert.False(t, ConfigAllNamespaces, "all_namespaces")
	assert.False(t, ConfigEventsWarningsOnly, "events.warnings_only")
	assert.False(t, ConfigEventsGrouping, "events.grouping")
	assert.Equal(t, 9, ConfigScrollOff, "scrolloff")
	assert.False(t, ConfigConfirmOnExit, "confirm_on_exit")
	assert.False(t, ConfigDimOverlay, "dim_overlay")
	assert.True(t, ConfigTransparentBg, "transparent_background")
	assert.False(t, ConfigMouse, "mouse")
	assert.Equal(t, 3*time.Second, ConfigWatchInterval, "watch_interval")
	assert.True(t, ConfigNoColor, "no_color")
	assert.True(t, ConfigSecretLazyLoading, "secret_lazy_loading")
	assert.Equal(t, InformerCacheAlways, ConfigInformerCacheMode, "informer_cache")
	assert.InDelta(t, 0.5, ConfigMinContrastRatio, 1e-9, "min_contrast_ratio")
	assert.True(t, ConfigReadOnly, "read_only")
	assert.True(t, ConfigShowRareTypes, "show_rare_types")
	assert.Equal(t, []string{"/tmp/lfk-kcfg"}, ConfigKubeconfigDirs, "kubeconfig_dir")
	assert.Equal(t, "traffic-ns", ConfigKubesharkNamespace, "kubeshark")

	// security section.
	assert.False(t, ConfigSecurityEnabled, "security.enabled")
	assert.True(t, ConfigSecurityHideBadges, "security.hide_badges")
	assert.Equal(t, map[string]bool{"trivy": false, "heuristic": true}, ConfigSecuritySources, "security.sources")
	require.Len(t, ConfigSecurityIgnorePatterns, 1, "security.ignore_patterns")
	assert.Equal(t, "prod", ConfigSecurityIgnorePatterns[0].Cluster)

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

// wiringCoveredFields records, for every configFile json field, where its
// YAML->runtime wiring is asserted. TestConfigFile_EveryFieldHasWiringCoverage
// fails if a field is added (or removed) without updating this map, forcing new
// settings to ship with a wiring test.
var wiringCoveredFields = map[string]string{
	"appearance":             "config_appearance_test.go (TestAppearance_*)",
	"colorscheme":            "TestLoadConfig_AllSettingsWired + TestApplyColorscheme_*",
	"theme":                  "TestMergeThemeOverrides (mergeThemeOverrides is the LoadConfig wiring point)",
	"keybindings":            "TestLoadConfig_AllSettingsWired + config_keybindings_test.go",
	"log_path":               "TestLoadConfig_AllSettingsWired",
	"abbreviations":          "TestLoadConfig_AllSettingsWired",
	"icons":                  "TestLoadConfig_AllSettingsWired",
	"resource_columns":       "TestLoadConfig_AllSettingsWired",
	"views":                  "TestLoadConfig_AllSettingsWired",
	"dashboard":              "TestLoadConfig_AllSettingsWired",
	"custom_actions":         "TestLoadConfig_AllSettingsWired",
	"filter_presets":         "TestLoadConfig_AllSettingsWired",
	"terminal":               "TestLoadConfig_AllSettingsWired",
	"scrollback_lines":       "TestLoadConfig_AllSettingsWired",
	"pinned_groups":          "TestLoadConfig_AllSettingsWired",
	"pinned_types":           "TestLoadConfig_AllSettingsWired",
	"monitoring":             "TestLoadConfig_AllSettingsWired",
	"tips":                   "TestLoadConfig_AllSettingsWired",
	"log_tail_lines":         "TestLoadConfig_AllSettingsWired",
	"log_tail_lines_short":   "TestLoadConfig_AllSettingsWired",
	"log_render_ansi":        "TestLoadConfig_AllSettingsWired (deprecated flat alias)",
	"log_viewer":             "TestLoadConfig_AllSettingsWired + config_log_viewer_test.go (TestLogViewer_*)",
	"yaml_viewer":            "TestLoadConfig_AllSettingsWired",
	"diff_viewer":            "TestLoadConfig_AllSettingsWired",
	"describe_viewer":        "TestLoadConfig_AllSettingsWired",
	"object_explorer":        "TestLoadConfig_AllSettingsWired",
	"split_preview":          "TestLoadConfig_AllSettingsWired",
	"watch_mode":             "TestLoadConfig_AllSettingsWired",
	"all_namespaces":         "TestLoadConfig_AllSettingsWired",
	"events":                 "TestLoadConfig_AllSettingsWired",
	"scrolloff":              "TestLoadConfig_AllSettingsWired",
	"confirm_on_exit":        "TestLoadConfig_AllSettingsWired",
	"dim_overlay":            "TestLoadConfig_AllSettingsWired",
	"transparent_background": "TestLoadConfig_AllSettingsWired",
	"mouse":                  "TestLoadConfig_AllSettingsWired",
	"watch_interval":         "TestLoadConfig_AllSettingsWired",
	"clusters":               "TestLoadConfig_AllSettingsWired",
	"no_color":               "TestLoadConfig_AllSettingsWired",
	"secret_lazy_loading":    "TestLoadConfig_AllSettingsWired",
	"informer_cache":         "TestLoadConfig_AllSettingsWired",
	"min_contrast_ratio":     "TestLoadConfig_AllSettingsWired",
	"read_only":              "TestLoadConfig_AllSettingsWired",
	"show_rare_types":        "TestLoadConfig_AllSettingsWired",
	"security":               "TestLoadConfig_AllSettingsWired",
	"rightsizing_defaults":   "TestLoadConfig_AllSettingsWired",
	"kubeshark":              "TestLoadConfig_AllSettingsWired",
	"scheduler":              "TestLoadConfig_AllSettingsWired",
	"kubeconfig_dir":         "TestLoadConfig_AllSettingsWired",
	"union_sets":             "TestLoadConfig_AllSettingsWired",
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
func snapshotAllConfigGlobals(t *testing.T) func() {
	t.Helper()

	origTheme := ActiveTheme
	origScheme := ActiveSchemeName
	origKB := ActiveKeybindings
	origAbbr := SearchAbbreviations
	origLogPath := ConfigLogPath
	origIcon := IconMode
	origDashboard := ConfigDashboard
	origTerminal := ConfigTerminalMode
	origScrollback := ConfigScrollbackLines
	origPinnedGroups := ConfigPinnedGroups
	origPinnedTypes := ConfigPinnedTypes
	origUnionSets := ConfigUnionSets
	origTips := ConfigTipsEnabled
	origTail := ConfigLogTailLines
	origTailShort := ConfigLogTailLinesShort
	origAnsi := ConfigLogRenderAnsi
	origShowPreview := ConfigLogShowPreview
	origShowPrefixes := ConfigLogShowPrefixes
	origShowTimestamps := ConfigLogShowTimestamps
	origLogMaxLines := ConfigLogMaxLines
	origYAMLWrap := ConfigYAMLViewerWrap
	origDiffWrap := ConfigDiffViewerWrap
	origDiffLineNums := ConfigDiffViewerLineNumbers
	origDiffUnified := ConfigDiffViewerUnified
	origDescribeWrap := ConfigDescribeViewerWrap
	origObjectExplorerLive := ConfigObjectExplorerLive
	origSplitPreview := ConfigSplitPreview
	origWatchMode := ConfigWatchMode
	origAllNamespaces := ConfigAllNamespaces
	origEventsWarn := ConfigEventsWarningsOnly
	origEventsGroup := ConfigEventsGrouping
	origScrollOff := ConfigScrollOff
	origConfirm := ConfigConfirmOnExit
	origDim := ConfigDimOverlay
	origTransparent := ConfigTransparentBg
	origMouse := ConfigMouse
	origWatch := ConfigWatchInterval
	origNoColor := ConfigNoColor
	origKubeconfig := ConfigKubeconfigDirs
	origSecretLazy := ConfigSecretLazyLoading
	origKubeshark := ConfigKubesharkNamespace
	origInformer := ConfigInformerCacheMode
	origContrast := ConfigMinContrastRatio
	origReadOnly := ConfigReadOnly
	origShowRare := ConfigShowRareTypes
	origSecEnabled := ConfigSecurityEnabled
	origSecHideBadges := ConfigSecurityHideBadges
	origSecSources := ConfigSecuritySources
	origSecIgnore := ConfigSecurityIgnorePatterns
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
		ActiveKeybindings = origKB
		SearchAbbreviations = origAbbr
		ConfigLogPath = origLogPath
		IconMode = origIcon
		ConfigDashboard = origDashboard
		ConfigTerminalMode = origTerminal
		ConfigScrollbackLines = origScrollback
		ConfigPinnedGroups = origPinnedGroups
		ConfigPinnedTypes = origPinnedTypes
		ConfigUnionSets = origUnionSets
		ConfigTipsEnabled = origTips
		ConfigLogTailLines = origTail
		ConfigLogTailLinesShort = origTailShort
		ConfigLogRenderAnsi = origAnsi
		ConfigLogShowPreview = origShowPreview
		ConfigLogShowPrefixes = origShowPrefixes
		ConfigLogShowTimestamps = origShowTimestamps
		ConfigLogMaxLines = origLogMaxLines
		ConfigYAMLViewerWrap = origYAMLWrap
		ConfigDiffViewerWrap = origDiffWrap
		ConfigDiffViewerLineNumbers = origDiffLineNums
		ConfigDiffViewerUnified = origDiffUnified
		ConfigDescribeViewerWrap = origDescribeWrap
		ConfigObjectExplorerLive = origObjectExplorerLive
		ConfigSplitPreview = origSplitPreview
		ConfigWatchMode = origWatchMode
		ConfigAllNamespaces = origAllNamespaces
		ConfigEventsWarningsOnly = origEventsWarn
		ConfigEventsGrouping = origEventsGroup
		ConfigScrollOff = origScrollOff
		ConfigConfirmOnExit = origConfirm
		ConfigDimOverlay = origDim
		ConfigTransparentBg = origTransparent
		ConfigMouse = origMouse
		ConfigWatchInterval = origWatch
		ConfigNoColor = origNoColor
		ConfigKubeconfigDirs = origKubeconfig
		ConfigSecretLazyLoading = origSecretLazy
		ConfigKubesharkNamespace = origKubeshark
		ConfigInformerCacheMode = origInformer
		ConfigMinContrastRatio = origContrast
		ConfigReadOnly = origReadOnly
		ConfigShowRareTypes = origShowRare
		ConfigSecurityEnabled = origSecEnabled
		ConfigSecurityHideBadges = origSecHideBadges
		ConfigSecuritySources = origSecSources
		ConfigSecurityIgnorePatterns = origSecIgnore
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
