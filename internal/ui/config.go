package ui

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
)

// informerCacheSetting holds the parsed informer_cache config value. It
// accepts both the legacy bool form (true → "always", false → "off") and
// the named-mode form ("off" / "auto" / "always"). Resolved during
// LoadConfig and stored in ConfigInformerCacheMode.
//
// UnmarshalJSON is deliberately tolerant: a typo or unsupported shape is
// captured in raw + invalid rather than aborting the whole config load.
// applyConfigOptions then surfaces a single logger.Warn and falls back to
// the default — same pattern terminal/scrollback_lines use, so a bad
// informer_cache value never silently nukes unrelated keys like
// keybindings or colorscheme.
type informerCacheSetting struct {
	mode    string
	raw     string
	invalid bool
}

// applyInformerCacheSetting writes the resolved mode into
// ConfigInformerCacheMode, or warns and falls back to the default if the
// user supplied an unrecognised shape. Extracted from applyConfigOptions
// to keep that function under the project's cyclomatic-complexity cap.
func applyInformerCacheSetting(s *informerCacheSetting) {
	if s == nil {
		return
	}
	if s.invalid {
		logger.Warn("unrecognised informer_cache value in config; falling back to default",
			"value", s.raw,
			"valid", []string{InformerCacheOff, InformerCacheAuto, InformerCacheAlways},
			"default", ConfigInformerCacheMode)
		return
	}
	if s.mode != "" {
		ConfigInformerCacheMode = s.mode
	}
}

// UnmarshalJSON parses the bool / string union forms into mode. Anything
// else (unknown string, number, object) is recorded on raw + invalid so
// applyConfigOptions can warn-and-fallback. LoadConfig goes through
// sigs.k8s.io/yaml, which converts YAML to JSON before unmarshalling — so
// the unmarshaler hook here is also what handles YAML config files.
func (s *informerCacheSetting) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		if b {
			s.mode = InformerCacheAlways
		} else {
			s.mode = InformerCacheOff
		}
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		// An explicit empty value is treated as "key absent", not a typo —
		// users sometimes leave keys present-but-empty when scaffolding a
		// config file from a template.
		if trimmed == "" {
			return nil
		}
		switch trimmed {
		case InformerCacheOff, InformerCacheAuto, InformerCacheAlways:
			s.mode = trimmed
			return nil
		}
		s.raw = raw
		s.invalid = true
		return nil
	}
	// Truly unparseable shape (number, object, array). Keep the raw bytes
	// so the warning log shows the user what they actually wrote.
	s.raw = strings.TrimSpace(string(data))
	s.invalid = true
	return nil
}

// Watch interval bounds.
const (
	DefaultWatchInterval = 2 * time.Second
	MinWatchInterval     = 500 * time.Millisecond
	MaxWatchInterval     = 10 * time.Minute
)

// ConfigWatchInterval is the resolved polling interval used when watch mode
// is active. Set from config file; CLI flag override is applied later in
// app.NewModel.
var ConfigWatchInterval = DefaultWatchInterval

// clamp01 restricts v to [0.0, 1.0].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ClampWatchInterval restricts d to [MinWatchInterval, MaxWatchInterval].
// A zero or negative duration is returned unchanged so callers can treat it
// as "unset" and fall back to a default.
func ClampWatchInterval(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if d < MinWatchInterval {
		return MinWatchInterval
	}
	if d > MaxWatchInterval {
		return MaxWatchInterval
	}
	return d
}

// ConfigLogPath holds the log_path value from the config file (if any).
var ConfigLogPath string

// SearchAbbreviations maps short abbreviations to full resource type names for search.
var SearchAbbreviations map[string]string

// IconMode controls how resource icons are displayed.
var IconMode = "unicode"

// detectIconMode inspects the environment and returns the icon mode to use
// when the resolved config value is "auto". It honors an explicit LFK_ICONS
// env override first (accepting any valid mode string), then sniffs TERM for
// known Nerd-Font-shipping terminals (substring match — this survives
// tmux-through-ghostty setups where TERM=xterm-ghostty but TERM_PROGRAM=tmux),
// then TERM_PROGRAM for direct launches, and falls back to "unicode".
//
// Priority order:
//  1. LFK_ICONS env override (if valid, returned directly — can force any mode).
//  2. TERM substring match: "ghostty", "kitty", "wezterm".
//  3. TERM_PROGRAM match: "ghostty", "WezTerm", "kitty".
//  4. Fallback: "unicode".
func detectIconMode() string {
	if v := strings.ToLower(os.Getenv("LFK_ICONS")); v != "" {
		switch v {
		case "nerdfont", "unicode", "simple", "emoji", "none":
			return v
		}
	}
	if term := strings.ToLower(os.Getenv("TERM")); term != "" {
		if strings.Contains(term, "ghostty") ||
			strings.Contains(term, "kitty") ||
			strings.Contains(term, "wezterm") {
			return "nerdfont"
		}
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "WezTerm", "kitty":
		return "nerdfont"
	}
	return "unicode"
}

// ConfigResourceColumns holds global per-resource-type column overrides.
var ConfigResourceColumns map[string][]string

// ConfigClusterResourceColumns holds per-cluster per-resource-type column overrides.
// Keys: context name -> lowercase kind -> column list.
var ConfigClusterResourceColumns map[string]map[string][]string

// ConfigFilterMatch defines the match criteria for a user-configured filter preset.
type ConfigFilterMatch struct {
	Status      string `json:"status" yaml:"status"`
	ReadyNot    bool   `json:"ready_not" yaml:"ready_not"`
	RestartsGt  int    `json:"restarts_gt" yaml:"restarts_gt"`
	Column      string `json:"column" yaml:"column"`
	ColumnValue string `json:"column_value" yaml:"column_value"`
}

// ConfigFilterPreset defines a single user-configured filter preset.
type ConfigFilterPreset struct {
	Name  string            `json:"name" yaml:"name"`
	Key   string            `json:"key" yaml:"key"`
	Match ConfigFilterMatch `json:"match" yaml:"match"`
}

// ConfigFilterPresets maps lowercase Kind names to user-configured filter presets.
var ConfigFilterPresets map[string][]ConfigFilterPreset

// ColumnsForKind returns the configured column list for the given resource kind
// and cluster context. Per-cluster config takes priority over global config.
func ColumnsForKind(kind, context string) []string {
	lk := strings.ToLower(kind)
	// Per-cluster override first.
	if context != "" && len(ConfigClusterResourceColumns) > 0 {
		if clusterCols, ok := ConfigClusterResourceColumns[context]; ok {
			if cols, ok := clusterCols[lk]; ok {
				return cols
			}
		}
	}
	// Global override.
	if len(ConfigResourceColumns) > 0 && kind != "" {
		if cols, ok := ConfigResourceColumns[lk]; ok {
			return cols
		}
	}
	return nil
}

// ConfigDashboard controls whether to show a cluster dashboard when entering a context.
var ConfigDashboard = true

// ConfigSecretLazyLoading controls how Secret resources are listed.
// When false (default), Secret lists fetch full objects and eagerly decode
// their data into item columns — matching the behaviour of every other
// resource type.
// When true, Secret lists fetch metadata only (no data payload over the
// wire) and decoded values are lazy-loaded on hover. This is much faster in
// clusters with many Helm release secrets or large TLS payloads, at the
// cost of an extra GET per hovered secret and a brief empty-data window
// between hover and fetch completion.
var ConfigSecretLazyLoading bool

// Recognised string values for the informer_cache config knob. The Go
// constants are duplicated from internal/k8s.InformerCacheMode to keep the
// ui package free of a k8s dependency — main.go does the conversion.
const (
	InformerCacheOff    = "off"
	InformerCacheAuto   = "auto"
	InformerCacheAlways = "always"
)

// ConfigInformerCacheMode resolves to one of "off"/"auto"/"always" after
// LoadConfig. Default "auto" so users on large clusters (issue #86) get the
// namespace-switch perf win for free; small clusters pay nothing because
// auto-mode only promotes a (context, GVR) to the cache once a list crosses
// 1000 items, and demotes it again when the list shrinks.
var ConfigInformerCacheMode = InformerCacheAuto

// ConfigMinContrastRatio is the normalized readability knob in [0.0, 1.0].
// When greater than zero, ApplyTheme nudges foreground colors in HSL lightness
// space so each fg/bg pair meets a minimum WCAG contrast ratio. The mapping is:
//
//	wcagTarget = 1.0 + value * 20.0
//
// Concrete examples:
//
//	0.0   off (default) — theme colors used as-is
//	0.175 approx. WCAG AA threshold (4.5:1) for normal text
//	0.3   approx. WCAG AAA threshold (7.0:1)
//	1.0   maximum — forces fg toward pure black or white against any bg
//
// Values outside [0, 1] are clamped. Only HSL lightness is adjusted; hue and
// saturation are preserved at moderate values.
var ConfigMinContrastRatio float64

// Terminal-mode constants control how exec/shell commands run. They are
// the only valid values for ConfigTerminalMode and the `terminal:` config
// key.
const (
	// TerminalModePTY embeds the shell in lfk's TUI via an internal vt10x
	// terminal. Output stays inside lfk; selection works via host-terminal
	// shift+drag. Default.
	TerminalModePTY = "pty"
	// TerminalModeExec hands the host terminal to the shell via
	// tea.ExecProcess and resumes lfk after the shell exits. Selection,
	// scrollback, and copy/paste work natively but lfk is suspended for
	// the duration.
	TerminalModeExec = "exec"
	// TerminalModeMux opens the shell in a new window/pane of the
	// surrounding multiplexer (tmux or zellij), so lfk stays foregrounded
	// alongside the shell. Errors out if no multiplexer is detected — use
	// pty or exec in that case.
	TerminalModeMux = "mux"
)

// ConfigTerminalMode controls how exec/shell commands run. One of
// TerminalModePTY, TerminalModeExec, TerminalModeMux.
var ConfigTerminalMode = TerminalModePTY

// ScrollbackLines clamps for the embedded PTY scrollback ring. The
// default of 5000 covers an extended interactive session without
// running the parent process out of memory; the floor stops a typo in
// the config from disabling scrollback entirely; the ceiling caps
// memory at roughly 10MB even with very long lines.
const (
	ScrollbackLinesDefault = 5000
	ScrollbackLinesMin     = 100
	ScrollbackLinesMax     = 100_000
)

// ConfigScrollbackLines is the per-tab capacity of the PTY scrollback
// ring (in lines). Set via the `scrollback_lines:` config key.
var ConfigScrollbackLines = ScrollbackLinesDefault

// CustomAction represents a user-defined action for a specific resource kind.
type CustomAction struct {
	Label       string `json:"label" yaml:"label"`
	Command     string `json:"command" yaml:"command"`
	Key         string `json:"key" yaml:"key"`
	Description string `json:"description" yaml:"description"`
	// ReadOnlySafe declares the action does not change cluster state.
	// Defaults to false (treated as mutating) so custom actions are blocked
	// in read-only mode unless the user explicitly opts in. Set to true for
	// view-only commands (port-forward listings, "kubectl describe", etc.).
	ReadOnlySafe bool `json:"read_only_safe" yaml:"read_only_safe"`
}

// ConfigCustomActions maps resource kinds to user-defined custom actions.
var ConfigCustomActions map[string][]CustomAction

// ConfigPinnedGroups lists CRD API groups that should appear prominently.
var ConfigPinnedGroups []string

// ConfigTipsEnabled controls whether to show random tips on startup.
var ConfigTipsEnabled = true

// ConfigConfirmOnExit controls whether ctrl+c on the last tab shows a quit confirmation.
var ConfigConfirmOnExit = true

// ConfigLogTailLines controls how many log lines are initially loaded via --tail.
var ConfigLogTailLines = 1000

// ConfigLogTailLinesShort is the tail line count used by the "Tail Logs" action
// menu entry. It intentionally defaults to a small value (10) so users get a
// lightweight peek at recent output without the full 1000-line hit.
var ConfigLogTailLinesShort = 10

// ConfigLogRenderAnsi controls whether the log viewer preserves ANSI SGR
// escape sequences (colour, bold, underline) emitted by log producers.
// When true, the sanitizer keeps valid SGR runs verbatim so coloured
// output from applications renders in the viewer. When false, ESC bytes
// are treated the same as other control bytes and replaced with U+FFFD,
// matching the historical safe-but-noisy behaviour. Toggle at runtime
// with `:set ansi` / `:set noansi`.
var ConfigLogRenderAnsi = true

// ConfigScrollOff is the number of lines to keep visible above/below the cursor.
// Used by all views with cursor-based navigation.
var ConfigScrollOff = 5

// ActiveSchemeName holds the name of the currently active color scheme.
var ActiveSchemeName = "tokyonight-storm"

// ConfigTransparentBg controls whether bar/surface backgrounds are transparent.
var ConfigTransparentBg bool

// ConfigMouse controls whether mouse input is captured by the TUI.
// Defaults to true. Set to false to disable mouse capture, allowing native
// terminal text selection (shift+click, drag-to-select).
var ConfigMouse = true

// ConfigReadOnly is the global default for read-only mode. When true, every
// mutating action is blocked unless overridden per-context.
var ConfigReadOnly bool

// ConfigClusterReadOnly maps context names to per-cluster read-only overrides.
// A value here takes precedence over ConfigReadOnly for that specific context.
var ConfigClusterReadOnly = map[string]bool{}

// ResolveReadOnly returns the effective read-only state for a given context.
// Precedence: CLI flag > per-context config > global config.
func ResolveReadOnly(context string, cliFlag bool) bool {
	if cliFlag {
		return true
	}
	if v, ok := ConfigClusterReadOnly[context]; ok {
		return v
	}
	return ConfigReadOnly
}

// ConfigNoColor, when true, builds the theme without foreground or background
// colors. Emphasis is conveyed with bold, underline, and reverse video so the
// selection and other highlights remain visible in monochrome terminals.
// Controlled by the NO_COLOR environment variable (https://no-color.org),
// the no_color config field, or the --no-color CLI flag.
var ConfigNoColor bool

// ConfigDarkColorscheme is the built-in scheme name applied when the terminal
// reports dark mode. Populated by parsing the "dark:X" segment of colorscheme.
var ConfigDarkColorscheme string

// ConfigLightColorscheme is the built-in scheme name applied when the terminal
// reports light mode. Populated by parsing the "light:X" segment of colorscheme.
var ConfigLightColorscheme string

// SetNoColor updates ConfigNoColor and rebuilds the active theme so style
// globals reflect the new setting. No-op when the value is unchanged.
func SetNoColor(v bool) {
	if v == ConfigNoColor {
		return
	}
	ConfigNoColor = v
	ApplyTheme(ActiveTheme)
}

// DefaultAbbreviations returns the default search abbreviation map.
func DefaultAbbreviations() map[string]string {
	return map[string]string{
		"pvc":    "persistentvolumeclaim",
		"pv":     "persistentvolume",
		"hpa":    "horizontalpodautoscaler",
		"vpa":    "verticalpodautoscaler",
		"ds":     "daemonset",
		"dp":     "deployment",
		"dep":    "deployment",
		"deploy": "deployment",
		"sts":    "statefulset",
		"svc":    "service",
		"ep":     "endpoint",
		"eps":    "endpointslice",
		"ns":     "namespace",
		"no":     "node",
		"po":     "pod",
		"rs":     "replicaset",
		"rc":     "replicationcontroller",
		"sa":     "serviceaccount",
		"cm":     "configmap",
		"sec":    "secret",
		"ing":    "ingress",
		"netpol": "networkpolicy",
		"sc":     "storageclass",
		"cj":     "cronjob",
		"job":    "job",
		"crd":    "customresourcedefinition",
		"ev":     "event",
		"rb":     "rolebinding",
		"crb":    "clusterrolebinding",
		"cr":     "clusterrole",
		"role":   "role",
		"limit":  "limitrange",
		"quota":  "resourcequota",
		"pdb":    "poddisruptionbudget",
	}
}
