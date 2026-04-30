package ui

import (
	"math"
	"os"
	"slices"
	"strings"
	"time"

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
		mode := strings.ToLower(cfg.Terminal)
		switch mode {
		case TerminalModePTY, TerminalModeExec, TerminalModeMux:
			ConfigTerminalMode = mode
		default:
			logger.Warn("unrecognised terminal mode in config; falling back to default",
				"value", cfg.Terminal,
				"valid", []string{TerminalModePTY, TerminalModeExec, TerminalModeMux},
				"default", ConfigTerminalMode)
		}
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
	if cfg.Monitoring != nil {
		model.ConfigMonitoring = cfg.Monitoring
	}
	if cfg.Tips != nil {
		ConfigTipsEnabled = *cfg.Tips
	}
	if cfg.LogTailLines != nil && *cfg.LogTailLines > 0 {
		ConfigLogTailLines = *cfg.LogTailLines
	}
	if cfg.LogTailLinesShort != nil && *cfg.LogTailLinesShort > 0 {
		ConfigLogTailLinesShort = *cfg.LogTailLinesShort
	}
	if cfg.LogRenderAnsi != nil {
		ConfigLogRenderAnsi = *cfg.LogRenderAnsi
	}
	if cfg.ScrollOff != nil && *cfg.ScrollOff >= 0 {
		ConfigScrollOff = *cfg.ScrollOff
	}
	if cfg.ConfirmOnExit != nil {
		ConfigConfirmOnExit = *cfg.ConfirmOnExit
	}
	if cfg.DimOverlay != nil {
		ConfigDimOverlay = *cfg.DimOverlay
	}
	if cfg.TransparentBg != nil {
		ConfigTransparentBg = *cfg.TransparentBg
	}
	if cfg.Mouse != nil {
		ConfigMouse = *cfg.Mouse
	}
	if cfg.WatchInterval != "" {
		if d, err := time.ParseDuration(cfg.WatchInterval); err == nil {
			if clamped := ClampWatchInterval(d); clamped > 0 {
				ConfigWatchInterval = clamped
			}
		}
	}
	if cfg.NoColor != nil {
		ConfigNoColor = *cfg.NoColor
	}
	if cfg.SecretLazyLoading != nil {
		ConfigSecretLazyLoading = *cfg.SecretLazyLoading
	}
	applyInformerCacheSetting(cfg.InformerCache)
	if cfg.MinContrastRatio != nil {
		ConfigMinContrastRatio = clamp01(*cfg.MinContrastRatio)
	}
	if cfg.ReadOnly != nil {
		ConfigReadOnly = *cfg.ReadOnly
	}
	applyRightsizingDefaults(cfg.RightsizingDefaults)
	if os.Getenv("NO_COLOR") != "" {
		// Per https://no-color.org, the presence of NO_COLOR (regardless of
		// value) disables color. Env takes precedence over the config file
		// field; CLI flag is applied later in main.go.
		ConfigNoColor = true
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
		}
	}
}

// applyRightsizingDefaults validates the rightsizing_defaults config
// section and pushes accepted values into the model package-level vars
// consumed by executeActionRightsizing's sticky-then-config-then-builtin
// fallback chain.
//
// A nil section is a no-op (omitting rightsizing_defaults must NOT
// clobber an already-set value — important for tests and for future
// reload paths). Invalid strategy literals or off-preset headroom
// values are dropped with a warning so the user gets a single, visible
// signal at startup rather than a silent fallthrough; the model var
// is left at zero in that case so the runtime falls back through the
// rest of the chain.
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
