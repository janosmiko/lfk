package ui

import (
	"maps"
	"math"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// applyColorscheme selects a built-in colorscheme if specified in config.
//
// The colorscheme field supports two formats:
//
//  1. Plain name – "dracula"
//     Applies the scheme and leaves dark/light switching disabled.
//
//  2. Ghostty-style dual-mode – "dark:Rose Pine,light:Rose Pine Dawn"
//     Parses each comma-separated segment for a "dark:" or "light:" prefix.
//     Both, one, or neither segment may be present; order does not matter.
//     ConfigDarkColorscheme / ConfigLightColorscheme are set accordingly.
//     No default scheme is applied immediately; the terminal's first CSI 997
//     notification will trigger the initial switch.
func applyColorscheme(theme *Theme, cfg configFile) {
	if cfg.Colorscheme == "" {
		return
	}
	dark, light, isDual := parseDualColorscheme(cfg.Colorscheme)
	if isDual {
		ConfigDarkColorscheme = dark
		ConfigLightColorscheme = light
		return
	}
	lower := normalizeScheme(cfg.Colorscheme)
	if scheme, ok := BuiltinSchemes()[lower]; ok {
		*theme = scheme
		ActiveSchemeName = lower
	}
}

// parseDualColorscheme parses a Ghostty-style "dark:X,light:Y" colorscheme
// string. It returns the dark and light scheme names (normalized to lowercase
// with spaces replaced by hyphens, matching built-in scheme map keys) and
// isDual=true when the string contains at least one "dark:" or "light:" prefix.
// Segment order and surrounding whitespace are both tolerated.
func parseDualColorscheme(s string) (dark, light string, isDual bool) {
	parts := strings.SplitSeq(s, ",")
	for p := range parts {
		p = strings.TrimSpace(p)
		lower := strings.ToLower(p)
		switch {
		case strings.HasPrefix(lower, "dark:"):
			dark = normalizeScheme(p[len("dark:"):])
			isDual = true
		case strings.HasPrefix(lower, "light:"):
			light = normalizeScheme(p[len("light:"):])
			isDual = true
		}
	}
	return dark, light, isDual
}

// normalizeScheme converts a user-supplied scheme name to the lowercase,
// hyphenated form used as keys in BuiltinSchemes (e.g. "Rose Pine" → "rose-pine").
func normalizeScheme(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
}

// resolveIconMode determines the icon mode from the environment and config.
// Priority:
//  1. LFK_ICONS env var (if valid) — unconditional override.
//  2. cfg.Icons if explicit non-auto.
//  3. Otherwise, detectIconMode() for auto.
//  4. Fallback: unicode.
func resolveIconMode(cfgIcons string) string {
	if envMode := strings.ToLower(os.Getenv("LFK_ICONS")); envMode != "" {
		switch envMode {
		case "unicode", "nerdfont", "simple", "emoji", "none":
			return envMode
		}
	}
	cfgMode := strings.ToLower(cfgIcons)
	if cfgMode == "" || cfgMode == "auto" {
		return detectIconMode()
	}
	switch cfgMode {
	case "unicode", "nerdfont", "simple", "emoji", "none":
		return cfgMode
	}
	return "unicode"
}

// applyConfigOptions applies scalar config options (icons, terminal, tips, etc.).
func applyConfigOptions(cfg configFile) {
	IconMode = resolveIconMode(cfg.Icons)

	if cfg.Dashboard != nil {
		ConfigDashboard = *cfg.Dashboard
	}
	if cfg.Terminal != "" {
		mode, warning := resolveTerminalMode(cfg.Terminal, runtime.GOOS, ConfigTerminalMode)
		if warning != "" {
			// Raw cfg.Terminal is intentionally not logged — log
			// redaction policy. The "valid" list tells the user what
			// is accepted; their own config file is the source of
			// truth for what they typed.
			logger.Warn(warning,
				"valid", []string{TerminalModePTY, TerminalModeExec, TerminalModeMux},
				"applied", mode)
		}
		ConfigTerminalMode = mode
	}
	if cfg.ScrollbackLines != 0 {
		v := cfg.ScrollbackLines
		clamped := v
		if v < ScrollbackLinesMin {
			clamped = ScrollbackLinesMin
		} else if v > ScrollbackLinesMax {
			clamped = ScrollbackLinesMax
		}
		if clamped != v {
			logger.Warn("scrollback_lines out of range; clamped",
				"value", v,
				"min", ScrollbackLinesMin,
				"max", ScrollbackLinesMax,
				"applied", clamped)
		}
		ConfigScrollbackLines = clamped
	}
	if len(cfg.PinnedGroups) > 0 {
		ConfigPinnedGroups = cfg.PinnedGroups
	}
	if len(cfg.PinnedTypes) > 0 {
		ConfigPinnedTypes = cfg.PinnedTypes
	}
	if cfg.PinnedSummaries != nil { // nil vs "[]" distinguishes defaults-on from defaults-off; see ConfigPinnedSummariesSet
		ConfigPinnedSummaries = cfg.PinnedSummaries
		ConfigPinnedSummariesSet = true
	}
	if len(cfg.UnionSets) > 0 {
		ConfigUnionSets = sanitizeUnionSets([]UnionSetConfig(cfg.UnionSets))
	}
	if cfg.Monitoring != nil {
		model.ConfigMonitoring = cfg.Monitoring
	}
	if cfg.Tips != nil {
		ConfigTipsEnabled = *cfg.Tips
	}
	applyLogViewerConfig(cfg)
	applyViewerDefaults(cfg)
	applySessionDefaults(cfg)
	if cfg.ScrollOff != nil && *cfg.ScrollOff >= 0 {
		ConfigScrollOff = *cfg.ScrollOff
	}
	if cfg.ConfirmOnExit != nil {
		ConfigConfirmOnExit = *cfg.ConfirmOnExit
	}
	applyDeletePropagationPolicy(cfg.DeletePropagationPolicy)
	if cfg.DimOverlay != nil {
		ConfigDimOverlay = *cfg.DimOverlay
	}
	applyRowStatusTint(cfg.RowStatusTint)
	if cfg.TransparentBg != nil {
		ConfigTransparentBg = *cfg.TransparentBg
	}
	if cfg.Mouse != nil {
		ConfigMouse = *cfg.Mouse
	}
	applyWatchIntervalConfig(cfg.WatchInterval)
	applyBackgroundWatchIntervalConfig(cfg.BackgroundWatchInterval)
	applyForegroundIdleTimeoutConfig(cfg.ForegroundIdleTimeout)
	if cfg.NoColor != nil {
		ConfigNoColor = *cfg.NoColor
	}
	applyKubeconfigDirsSetting(cfg.KubeconfigDir)
	if cfg.KubeconfigExclusive != nil {
		ConfigKubeconfigExclusive = *cfg.KubeconfigExclusive
	}
	applyDataAccessConfig(cfg)
	applyInformerCacheSetting(cfg.InformerCache)
	if cfg.MinContrastRatio != nil {
		ConfigMinContrastRatio = clamp01(*cfg.MinContrastRatio)
	}
	if cfg.ReadOnly != nil {
		ConfigReadOnly = *cfg.ReadOnly
	}
	// Trim first so `field_manager: " "` cannot replace the default with
	// whitespace, which the apiserver would store verbatim.
	if fm := strings.TrimSpace(cfg.FieldManager); fm != "" {
		k8s.FieldManagerOverride = fm
	}
	if cfg.ShowRareTypes != nil {
		ConfigShowRareTypes = *cfg.ShowRareTypes
	}
	if cfg.LogTopDefaultProfile != nil {
		switch *cfg.LogTopDefaultProfile {
		case "auto", "traefik-json", "ingress-nginx", "nginx-combined", "envoy", "json", "logfmt":
			ConfigLogTopDefaultProfile = *cfg.LogTopDefaultProfile
		default:
			ConfigLogTopDefaultProfile = "auto"
		}
	}
	applySecurityConfig(cfg)
	applyRightsizingDefaults(cfg.RightsizingDefaults)
	if cfg.Scheduler != nil {
		applySchedulerConfig(cfg.Scheduler)
	}
	applyWhichKey(cfg)
	if os.Getenv("NO_COLOR") != "" {
		// Per https://no-color.org, the presence of NO_COLOR (regardless of
		// value) disables color. Env takes precedence over the config file
		// field; CLI flag is applied later in main.go.
		ConfigNoColor = true
	}
}

// applySecurityConfig applies the global `security` section: the dashboard
// enable toggle and the per-source overrides. Per-cluster overrides under
// clusters.<name>.security are applied in the Clusters loop.
func applySecurityConfig(cfg configFile) {
	if cfg.Security == nil {
		return
	}
	if cfg.Security.Enabled != nil {
		ConfigSecurityEnabled = *cfg.Security.Enabled
	}
	if cfg.Security.HideBadges != nil {
		ConfigSecurityHideBadges = *cfg.Security.HideBadges
	}
	if cfg.Security.Sources != nil {
		ConfigSecuritySources = cfg.Security.Sources
	}
	// Drop all-empty patterns so an accidental blank list item can't hide
	// every finding.
	patterns := make([]SecurityIgnorePattern, 0, len(cfg.Security.IgnorePatterns))
	for _, p := range cfg.Security.IgnorePatterns {
		if p.IsEmpty() {
			continue
		}
		patterns = append(patterns, p)
	}
	ConfigSecurityIgnorePatterns = patterns
	if cfg.Security.Heuristic != nil {
		ConfigSecuritySecretEnvInclude = trimNonEmpty(cfg.Security.Heuristic.SecretEnvInclude)
		ConfigSecuritySecretEnvExclude = trimNonEmpty(cfg.Security.Heuristic.SecretEnvExclude)
		if cfg.Security.Heuristic.ScanSecrets != nil {
			ConfigSecurityScanSecrets = *cfg.Security.Heuristic.ScanSecrets
		}
	}
}

// applyClusterSecurityConfig applies one clusters.<name>.security override
// block. Extracted from applyConfigMaps to keep it under the gocyclo cap.
func applyClusterSecurityConfig(ctx string, sec *securityConfig) {
	if sec.Enabled != nil {
		ConfigClusterSecurityEnabled[ctx] = *sec.Enabled
	}
	if sec.HideBadges != nil {
		ConfigClusterSecurityHideBadges[ctx] = *sec.HideBadges
	}
	if sec.Sources != nil {
		ConfigClusterSecuritySources[ctx] = sec.Sources
	}
	// ignore_patterns is honored only in the top-level security
	// section; per-cluster scoping is expressed via each pattern's
	// `cluster` glob instead. Warn rather than silently drop.
	if len(sec.IgnorePatterns) > 0 {
		logger.Warn("clusters.<ctx>.security.ignore_patterns is ignored; "+
			"set ignore_patterns at the top-level security section and use the per-pattern 'cluster' glob",
			"context", ctx)
	}
	if sec.Heuristic != nil {
		logger.Warn("clusters.<ctx>.security.heuristic is ignored; "+
			"set it at the top-level security section",
			"context", ctx)
	}
}

// trimNonEmpty returns the entries trimmed of whitespace with empties dropped,
// so a blank YAML list item can't become a match-nothing (or, worse,
// match-everything) pattern.
func trimNonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// applyDataAccessConfig applies the cluster-data-access settings:
// secret lazy-loading and the kubeshark hub namespace. Extracted from
// applyConfigOptions so the dispatcher stays under the gocyclo cap.
func applyDataAccessConfig(cfg configFile) {
	if cfg.SecretLazyLoading != nil {
		ConfigSecretLazyLoading = *cfg.SecretLazyLoading
	}
	if cfg.Kubeshark != nil {
		// Trim before checking emptiness so a YAML value like `namespace: " "`
		// doesn't silently overwrite the default with whitespace — the K8s
		// API would then reject the lookup with a confusing error.
		if ns := strings.TrimSpace(cfg.Kubeshark.Namespace); ns != "" {
			ConfigKubesharkNamespace = ns
		}
	}
}

// applyLogViewerConfig wires every log-viewer setting (tail sizes, ANSI
// rendering, and the preview / prefixes / timestamps startup defaults) into
// its runtime global. The deprecated flat keys (log_tail_lines,
// log_tail_lines_short, log_render_ansi) are applied first so the canonical
// log_viewer group, applied second, wins when a config sets both.
func applyLogViewerConfig(cfg configFile) {
	// Deprecated flat aliases first.
	applyTailLines(cfg.LogTailLines, &ConfigLogTailLines)
	applyTailLines(cfg.LogTailLinesShort, &ConfigLogTailLinesShort)
	applyBoolPtr(cfg.LogRenderAnsi, &ConfigLogRenderAnsi)

	// Canonical log_viewer group overrides any flat alias above.
	lv := cfg.LogViewer
	if lv == nil {
		return
	}
	applyTailLines(lv.TailLines, &ConfigLogTailLines)
	applyTailLines(lv.TailLinesShort, &ConfigLogTailLinesShort)
	applyBoolPtr(lv.RenderAnsi, &ConfigLogRenderAnsi)
	applyBoolPtr(lv.ShowPreview, &ConfigLogShowPreview)
	applyBoolPtr(lv.ShowPrefixes, &ConfigLogShowPrefixes)
	applyBoolPtr(lv.ShowTimestamps, &ConfigLogShowTimestamps)
	applyBoolPtr(lv.PreviewLive, &ConfigLogPreviewLive)
	applyLogMaxLines(lv.MaxLines)
}

// applyLogMaxLines clamps a log_viewer.max_lines override into
// [LogMaxLinesMin, LogMaxLinesMax] and applies it. A nil or zero value keeps
// the compiled default so an omitted (or stray 0) key never disables the cap.
func applyLogMaxLines(src *int) {
	if src == nil || *src == 0 {
		return
	}
	v := *src
	clamped := v
	if v < LogMaxLinesMin {
		clamped = LogMaxLinesMin
	} else if v > LogMaxLinesMax {
		clamped = LogMaxLinesMax
	}
	if clamped != v {
		logger.Warn("log_viewer.max_lines out of range; clamped",
			"value", v,
			"min", LogMaxLinesMin,
			"max", LogMaxLinesMax,
			"applied", clamped)
	}
	ConfigLogMaxLines = clamped
}

// applyViewerDefaults wires the YAML / diff / describe viewer startup-toggle
// defaults from their config groups into the matching runtime globals.
func applyViewerDefaults(cfg configFile) {
	if yv := cfg.YAMLViewer; yv != nil {
		applyBoolPtr(yv.Wrap, &ConfigYAMLViewerWrap)
	}
	if dv := cfg.DiffViewer; dv != nil {
		applyBoolPtr(dv.Wrap, &ConfigDiffViewerWrap)
		applyBoolPtr(dv.LineNumbers, &ConfigDiffViewerLineNumbers)
		applyBoolPtr(dv.Unified, &ConfigDiffViewerUnified)
	}
	if dv := cfg.DescribeViewer; dv != nil {
		applyBoolPtr(dv.Wrap, &ConfigDescribeViewerWrap)
	}
	if oe := cfg.ObjectExplorer; oe != nil {
		applyBoolPtr(oe.Live, &ConfigObjectExplorerLive)
		applyBoolPtr(oe.Tree, &ConfigObjectExplorerTree)
	}
	if ae := cfg.APIExplorer; ae != nil {
		applyBoolPtr(ae.Tree, &ConfigAPIExplorerTree)
	}
}

// applySessionDefaults wires the session-level startup defaults (split preview,
// watch mode, namespace scope, and the events-view toggles) into their runtime
// globals.
func applySessionDefaults(cfg configFile) {
	applyBoolPtr(cfg.SplitPreview, &ConfigSplitPreview)
	applyBoolPtr(cfg.WatchMode, &ConfigWatchMode)
	applyBoolPtr(cfg.WatchThrottle, &ConfigWatchThrottle)
	applyBoolPtr(cfg.AllNamespaces, &ConfigAllNamespaces)
	if ev := cfg.Events; ev != nil {
		applyBoolPtr(ev.WarningsOnly, &ConfigEventsWarningsOnly)
		applyBoolPtr(ev.Grouping, &ConfigEventsGrouping)
	}
}

// applyTailLines copies a positive tail-line override into dst; non-positive
// values are ignored so a stray `0` keeps the compiled default.
func applyTailLines(src *int, dst *int) {
	if src != nil && *src > 0 {
		*dst = *src
	}
}

// applyBoolPtr copies an optional bool override into dst when present.
func applyBoolPtr(src *bool, dst *bool) {
	if src != nil {
		*dst = *src
	}
}

// applyConfigMaps applies map-based config settings (columns, actions, presets, abbreviations, clusters).
func applyConfigMaps(cfg configFile, abbr map[string]string) {
	if len(cfg.ResourceColumns) > 0 {
		ConfigResourceColumns = make(map[string][]string, len(cfg.ResourceColumns))
		for k, v := range cfg.ResourceColumns {
			ConfigResourceColumns[strings.ToLower(k)] = v
		}
	}
	if len(cfg.Views) > 0 {
		ConfigViews = make(map[string]*View, len(cfg.Views))
		for k, cv := range cfg.Views {
			v, err := BuildView(&cv)
			if err != nil {
				logger.Warn("ignoring invalid view config",
					"key", k,
					"err", err.Error())
				continue
			}
			ConfigViews[strings.ToLower(k)] = v
			logger.Debug("loaded view",
				"key", strings.ToLower(k),
				"columns", len(v.Columns),
				"sort_column", v.SortColumn)
		}
	}
	for k, v := range cfg.Abbreviations {
		abbr[strings.ToLower(k)] = strings.ToLower(v)
	}
	if len(cfg.CustomActions) > 0 {
		ConfigCustomActions = cfg.CustomActions
	}
	if len(cfg.FilterPresets) > 0 {
		ConfigFilterPresets = make(map[string][]ConfigFilterPreset, len(cfg.FilterPresets))
		for k, v := range cfg.FilterPresets {
			ConfigFilterPresets[strings.ToLower(k)] = v
		}
	}
	if len(cfg.Clusters) > 0 {
		ConfigClusterResourceColumns = make(map[string]map[string][]string, len(cfg.Clusters))
		ConfigClusterReadOnly = make(map[string]bool, len(cfg.Clusters))
		ConfigClusterSecurityEnabled = make(map[string]bool, len(cfg.Clusters))
		ConfigClusterSecurityHideBadges = make(map[string]bool, len(cfg.Clusters))
		ConfigClusterSecuritySources = make(map[string]map[string]bool, len(cfg.Clusters))
		ConfigClusterK8sClientQPS = make(map[string]int, len(cfg.Clusters))
		ConfigClusterK8sClientBurst = make(map[string]int, len(cfg.Clusters))
		for ctx, cc := range cfg.Clusters {
			if len(cc.ResourceColumns) > 0 {
				cols := make(map[string][]string, len(cc.ResourceColumns))
				for k, v := range cc.ResourceColumns {
					cols[strings.ToLower(k)] = v
				}
				ConfigClusterResourceColumns[ctx] = cols
			}
			if cc.ReadOnly != nil {
				ConfigClusterReadOnly[ctx] = *cc.ReadOnly
			}
			if cc.K8sClientQPS != nil && *cc.K8sClientQPS > 0 {
				ConfigClusterK8sClientQPS[ctx] = *cc.K8sClientQPS
			}
			if cc.K8sClientBurst != nil && *cc.K8sClientBurst > 0 {
				ConfigClusterK8sClientBurst[ctx] = *cc.K8sClientBurst
			}
			if cc.Security != nil {
				applyClusterSecurityConfig(ctx, cc.Security)
			}
			if len(cc.Views) > 0 {
				if ConfigClusterViews == nil {
					ConfigClusterViews = make(map[string]map[string]*View, len(cfg.Clusters))
				}
				perCluster := make(map[string]*View, len(cc.Views))
				for k, cv := range cc.Views {
					v, err := BuildView(&cv)
					if err != nil {
						logger.Warn("ignoring invalid view config",
							"context", ctx,
							"key", k,
							"err", err.Error())
						continue
					}
					perCluster[strings.ToLower(k)] = v
				}
				ConfigClusterViews[ctx] = perCluster
			}
		}
	}

	// Bridge resource_columns into views as a deprecated alias. views: wins when
	// both are present for the same key. A single deprecation warning is emitted
	// if any bridging occurs (not once per entry).
	if bridgeResourceColumnsToViews(cfg) {
		logger.Warn("resource_columns is deprecated; migrate to views: (see docs/config-reference.md)")
	}
}

// bridgeResourceColumnsToViews populates ConfigViews and ConfigClusterViews from
// resource_columns entries that have no corresponding views: entry. Returns true
// if any bridging was performed so the caller can emit a single deprecation warning.
func bridgeResourceColumnsToViews(cfg configFile) bool {
	bridged := false
	if len(cfg.ResourceColumns) > 0 {
		if ConfigViews == nil {
			ConfigViews = make(map[string]*View, len(cfg.ResourceColumns))
		}
		for kind, cols := range cfg.ResourceColumns {
			key := strings.ToLower(kind)
			if _, exists := ConfigViews[key]; exists {
				continue // views: wins
			}
			v, err := BuildView(&configView{Columns: cols})
			if err != nil {
				logger.Warn("ignoring invalid resource_columns config",
					"key", key,
					"err", err.Error())
				continue
			}
			ConfigViews[key] = v
			bridged = true
		}
	}
	for ctx, cc := range cfg.Clusters {
		if len(cc.ResourceColumns) == 0 {
			continue
		}
		if ConfigClusterViews == nil {
			ConfigClusterViews = make(map[string]map[string]*View, len(cfg.Clusters))
		}
		if ConfigClusterViews[ctx] == nil {
			ConfigClusterViews[ctx] = make(map[string]*View, len(cc.ResourceColumns))
		}
		for kind, cols := range cc.ResourceColumns {
			key := strings.ToLower(kind)
			if _, exists := ConfigClusterViews[ctx][key]; exists {
				continue
			}
			v, err := BuildView(&configView{Columns: cols})
			if err != nil {
				logger.Warn("ignoring invalid resource_columns config",
					"context", ctx,
					"key", key,
					"err", err.Error())
				continue
			}
			ConfigClusterViews[ctx][key] = v
			bridged = true
		}
	}
	return bridged
}

// applyRightsizingDefaults validates the rightsizing_defaults config section
// and pushes accepted values into the model package-level vars consumed by
// executeActionRightsizing's sticky-then-config-then-builtin fallback chain.
//
// A nil section is a no-op (omitting rightsizing_defaults must NOT clobber an
// already-set value - important for tests and future reload paths). Invalid
// strategy literals or off-preset headroom values are dropped with a warning;
// the model var is left at zero so the runtime falls back through the chain.
func applyRightsizingDefaults(cfg *RightsizingDefaultsConfig) {
	if cfg == nil {
		return
	}
	if cfg.Strategy != "" {
		// Reset before parse so an invalid retry-supplied value clears
		// any previously-accepted default (rather than silently keeping
		// the stale one and contradicting the documented contract).
		model.ConfigDefaultRightsizingStrategy = ""
		if s, ok := parseRightsizingStrategy(cfg.Strategy); ok {
			model.ConfigDefaultRightsizingStrategy = s
		} else {
			logger.Warn("unknown rightsizing_defaults.strategy in config; ignored",
				"value", cfg.Strategy,
				"valid", rightsizingStrategyLiterals())
		}
	}
	if cfg.Headroom != 0 {
		model.ConfigDefaultRightsizingHeadroom = 0
		if h, ok := parseRightsizingHeadroom(cfg.Headroom); ok {
			model.ConfigDefaultRightsizingHeadroom = h
		} else {
			logger.Warn("invalid rightsizing_defaults.headroom in config; ignored",
				"value", cfg.Headroom,
				"valid", model.RightsizingHeadrooms)
		}
	}
}

// parseRightsizingStrategy resolves a config string against the known
// strategy literals (strict match, no case folding — predictable for
// users typing config files by hand). Returns the matched strategy
// and true on success; ("", false) for unknown values.
func parseRightsizingStrategy(s string) (model.RightsizingStrategy, bool) {
	candidate := model.RightsizingStrategy(s)
	if slices.Contains(model.AllRightsizingStrategies, candidate) {
		return candidate, true
	}
	return "", false
}

// parseRightsizingHeadroom validates a config float against the
// preset values in model.RightsizingHeadrooms using a 1e-9 epsilon so
// 1.25 typed as 1.250000000000001 still matches. Returns the canonical
// preset value (not the raw input) so any cache key derived from it
// is stable across config-file rewrites.
func parseRightsizingHeadroom(v float64) (float64, bool) {
	for _, preset := range model.RightsizingHeadrooms {
		if math.Abs(v-preset) < 1e-9 {
			return preset, true
		}
	}
	return 0, false
}

// rightsizingStrategyLiterals returns the user-facing string form of
// every known strategy, used in warning logs so the user can see what
// they should have typed.
func rightsizingStrategyLiterals() []string {
	out := make([]string, 0, len(model.AllRightsizingStrategies))
	for _, s := range model.AllRightsizingStrategies {
		out = append(out, string(s))
	}
	return out
}

// applySchedulerConfig pushes the YAML scheduler section into the
// scheduler package-globals consumed by FromGlobals at request time.
// A nil section is a no-op; zero/negative values are ignored so
// omitted fields don't clobber the compiled defaults.
// K8sClientQPS and K8sClientBurst set the global foreground REST client rate
// (per-cluster overrides are applied from the clusters section). The k8s
// package reads the resolved value through the RateLimitForContext hook wired
// in this package's init().
func applySchedulerConfig(s *SchedulerConfig) {
	if s.WorkersPerContext > 0 {
		scheduler.ConfigWorkersPerContext = scheduler.ClampWorkers(s.WorkersPerContext)
	}
	if s.CriticalReserved > 0 {
		scheduler.ConfigCriticalReserved = scheduler.ClampCriticalReserved(s.CriticalReserved, scheduler.ClampWorkers(scheduler.ConfigWorkersPerContext))
	}
	if s.LowReserved != nil {
		workers := scheduler.ClampWorkers(scheduler.ConfigWorkersPerContext)
		crit := scheduler.ClampCriticalReserved(scheduler.ConfigCriticalReserved, workers)
		scheduler.ConfigLowReserved = scheduler.ClampLowReserved(*s.LowReserved, workers, crit)
	}
	if d, err := time.ParseDuration(s.DefaultTimeout); err == nil && d > 0 {
		scheduler.ConfigDefaultTimeout = d
	}
	if len(s.TimeoutsByKind) > 0 {
		// Merge YAML overrides into the existing per-Kind map instead
		// of replacing it. Compiled-in defaults (e.g. APIDiscovery=60s,
		// Mutation=120s) should remain in effect for any Kind the user
		// didn't list explicitly.
		out := make(map[scheduler.Kind]time.Duration, len(scheduler.ConfigTimeoutsByKind)+len(s.TimeoutsByKind))
		maps.Copy(out, scheduler.ConfigTimeoutsByKind)
		for name, dur := range s.TimeoutsByKind {
			d, err := time.ParseDuration(dur)
			if err != nil || d <= 0 {
				continue
			}
			k, ok := schedulerKindByName(name)
			if !ok {
				continue
			}
			out[k] = d
		}
		scheduler.ConfigTimeoutsByKind = out
	}
	if s.K8sClientQPS > 0 {
		ConfigK8sClientQPS = s.K8sClientQPS
	}
	if s.K8sClientBurst > 0 {
		ConfigK8sClientBurst = s.K8sClientBurst
	}
	if s.ShowPriority != nil {
		scheduler.ConfigShowPriorityInOverlay = *s.ShowPriority
	}
	if s.AgingThreshold != nil {
		scheduler.ConfigAgingThreshold = scheduler.ClampAgingThreshold(*s.AgingThreshold)
	}
}

// schedulerKindByName maps a YAML key string to the corresponding
// scheduler.Kind constant. Returns (0, false) for unknown names so
// the caller can skip unrecognised entries without panicking.
func schedulerKindByName(name string) (scheduler.Kind, bool) {
	switch name {
	case "ResourceList":
		return scheduler.KindResourceList, true
	case "YAMLFetch":
		return scheduler.KindYAMLFetch, true
	case "Metrics":
		return scheduler.KindMetrics, true
	case "ResourceTree":
		return scheduler.KindResourceTree, true
	case "Dashboard":
		return scheduler.KindDashboard, true
	case "Containers":
		return scheduler.KindContainers, true
	case "Mutation":
		return scheduler.KindMutation, true
	case "Subprocess":
		return scheduler.KindSubprocess, true
	case "APIDiscovery":
		return scheduler.KindAPIDiscovery, true
	case "NamespaceList":
		return scheduler.KindNamespaceList, true
	case "RBACCheck":
		return scheduler.KindRBACCheck, true
	case "SecurityScan":
		return scheduler.KindSecurityScan, true
	default:
		return 0, false
	}
}

// sanitizeUnionSets validates a raw UnionSets list from config and emits
// warnings for entries that won't be usable (missing name, no contexts,
// duplicate name, malformed per-cluster entries, unknown color names).
// Invalid entries are dropped rather than failing config load — a typo
// in one set should not prevent lfk from starting. Last-name-wins on
// duplicates so the user can override a global set per-project.
//
// Per-set context existence in the kubeconfig and namespace presence are
// NOT checked here: existence depends on the kubeconfig (which the ui
// package doesn't see), and namespace can come from the CLI. Both are
// validated when --union-set is resolved at runTUI time via
// app.ValidateUnionOptions.
func sanitizeUnionSets(in []UnionSetConfig) []UnionSetConfig {
	out := make([]UnionSetConfig, 0, len(in))
	seen := make(map[string]int, len(in))
	for _, s := range in {
		s.Name = strings.TrimSpace(s.Name)
		s.Namespace = strings.TrimSpace(s.Namespace)
		if s.Name == "" {
			logger.Warn("union_sets entry has no name; skipping", "contexts", s.Contexts)
			continue
		}
		// Drop nameless cluster entries, drop invalid color names, dedupe
		// repeated context references within the same set. The result is
		// a list whose Context fields are non-empty and whose Color fields
		// are either empty (untinted) or a valid ClusterColorNames value.
		cleanCtxs := make([]UnionSetContextConfig, 0, len(s.Contexts))
		seenCtx := make(map[string]struct{}, len(s.Contexts))
		for _, c := range s.Contexts {
			c.Context = strings.TrimSpace(c.Context)
			c.Color = strings.TrimSpace(c.Color)
			c.Namespace = strings.TrimSpace(c.Namespace)
			if c.Context == "" {
				logger.Warn("union_sets entry has nameless context; skipping that entry", "set", s.Name)
				continue
			}
			if _, dup := seenCtx[c.Context]; dup {
				logger.Warn("union_sets entry repeats a context; keeping first occurrence",
					"set", s.Name, "context", c.Context)
				continue
			}
			seenCtx[c.Context] = struct{}{}
			if c.Color != "" && !IsValidClusterColor(c.Color) {
				logger.Warn("union_sets entry has unknown color; leaving cluster untinted",
					"set", s.Name, "context", c.Context, "color", c.Color,
					"valid", ClusterColorNames)
				c.Color = ""
			}
			cleanCtxs = append(cleanCtxs, c)
		}
		if len(cleanCtxs) == 0 {
			logger.Warn("union_sets entry has no usable contexts; skipping", "name", s.Name)
			continue
		}
		s.Contexts = cleanCtxs
		if idx, dup := seen[s.Name]; dup {
			logger.Warn("duplicate union_sets name; later definition wins", "name", s.Name)
			out[idx] = s
			continue
		}
		seen[s.Name] = len(out)
		out = append(out, s)
	}
	return out
}
