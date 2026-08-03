package ui

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
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

// DefaultBackgroundWatchInterval is the watch-tick cadence used while the
// terminal reports the lfk window as unfocused, or while a focused window has
// seen no input for the foreground-idle timeout. Trades freshness for battery.
const DefaultBackgroundWatchInterval = 10 * time.Second

// ConfigBackgroundWatchInterval is the resolved background/idle polling interval.
// Set from config file; CLI flag override is applied in app.NewModel. Clamped
// to [MinWatchInterval, MaxWatchInterval] like the foreground interval.
var ConfigBackgroundWatchInterval = DefaultBackgroundWatchInterval

// DefaultForegroundIdleTimeout is the no-input window after which a focused lfk
// window throttles its watch tick to the background interval. 0 disables
// focused-idle throttling (background throttling still applies).
const DefaultForegroundIdleTimeout = 120 * time.Second

// ConfigForegroundIdleTimeout is the resolved focused-idle timeout.
var ConfigForegroundIdleTimeout = DefaultForegroundIdleTimeout

// DefaultWatchThrottle is the startup default for the focus/idle watch
// throttling feature. When false, the watch tick always uses watch_interval
// regardless of focus or idle state.
const DefaultWatchThrottle = true

// ConfigWatchThrottle enables the background/idle watch throttling. Set false
// to fully disable it (watch_interval is then used unconditionally).
var ConfigWatchThrottle = DefaultWatchThrottle

// ClampForegroundIdleTimeout restricts d to [0, MaxWatchInterval]; 0 disables.
func ClampForegroundIdleTimeout(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	if d > MaxWatchInterval {
		return MaxWatchInterval
	}
	return d
}

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

// ConfigKubeconfigDirs holds the kubeconfig_dir value from the config file (if any).
// The YAML value may be either a single string or a list of strings; both are
// normalised into this slice. Empty (nil) means "no override; use the default".
var ConfigKubeconfigDirs []string

// ConfigKubeconfigExclusive holds kubeconfig_exclusive from the config file:
// whether a set KUBECONFIG suppresses the default kubeconfig discovery
// (kubectl semantics). Defaults to true; CLI flag and env override it at
// client construction (see k8s.ResolveKubeconfigExclusive).
var ConfigKubeconfigExclusive = true

// kubeconfigDirsSetting holds the parsed kubeconfig_dir config value. It accepts
// either a string (single directory) or a list of strings (multiple directories
// to merge). Resolved during LoadConfig and stored in ConfigKubeconfigDirs.
//
// UnmarshalJSON is deliberately tolerant: a typo or unsupported shape is
// captured in raw + invalid rather than aborting the whole config load —
// same pattern informer_cache uses, so a bad kubeconfig_dir value never
// silently nukes unrelated keys like keybindings or colorscheme.
type kubeconfigDirsSetting struct {
	paths   []string
	raw     string
	invalid bool
}

// applyKubeconfigDirsSetting writes the resolved paths into
// ConfigKubeconfigDirs, or warns and falls back when the user supplied an
// unrecognised shape. Extracted from applyConfigOptions to keep that
// dispatcher under the project's cyclomatic-complexity cap.
func applyKubeconfigDirsSetting(s *kubeconfigDirsSetting) {
	if s == nil {
		return
	}
	if s.invalid {
		logger.Warn("unrecognised kubeconfig_dir value in config; ignored",
			"value", s.raw,
			"valid", "string or list of strings")
		return
	}
	if len(s.paths) > 0 {
		ConfigKubeconfigDirs = s.paths
	}
}

// UnmarshalJSON parses the string or []string union forms into paths.
// Whitespace-only entries are dropped so a YAML value like
// `kubeconfig_dir: " "` does not silently shadow a CLI flag or env var —
// that mirrors the trimming applied to the env var / CLI surfaces.
//
// LoadConfig goes through sigs.k8s.io/yaml, which converts YAML to JSON
// before unmarshalling, so this hook also handles YAML files.
func (s *kubeconfigDirsSetting) UnmarshalJSON(data []byte) error {
	// String form first.
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if v := strings.TrimSpace(single); v != "" {
			s.paths = []string{v}
		}
		return nil
	}
	// List form.
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		for _, p := range list {
			if v := strings.TrimSpace(p); v != "" {
				s.paths = append(s.paths, v)
			}
		}
		return nil
	}
	// Truly unparseable shape (number, object, etc.). Capture for warning.
	s.raw = strings.TrimSpace(string(data))
	s.invalid = true
	return nil
}

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
// Fields are AND-ed together. When `Invert` is true the resulting boolean is
// negated, so e.g. `status: Bound` + `invert: true` matches everything that is
// NOT Bound. The pre-existing `ReadyNot` flag remains for fine-grained Ready
// negation; rule-level `Invert` is the general escape hatch.
type ConfigFilterMatch struct {
	Status      string `json:"status" yaml:"status"`
	ReadyNot    bool   `json:"ready_not" yaml:"ready_not"`
	RestartsGt  int    `json:"restarts_gt" yaml:"restarts_gt"`
	Column      string `json:"column" yaml:"column"`
	ColumnValue string `json:"column_value" yaml:"column_value"`
	Invert      bool   `json:"invert" yaml:"invert"`
}

// ConfigFilterPreset defines a single user-configured filter preset.
type ConfigFilterPreset struct {
	Name  string            `json:"name" yaml:"name"`
	Key   string            `json:"key" yaml:"key"`
	Match ConfigFilterMatch `json:"match" yaml:"match"`
}

// ConfigFilterPresets maps lowercase Kind names to user-configured filter presets.
var ConfigFilterPresets map[string][]ConfigFilterPreset

// ColumnsForKind returns the configured column list for the given resource
// and cluster context. Resolution order: per-cluster resource_columns,
// per-cluster views (GVR then Kind), global resource_columns, global views
// (GVR then Kind). Per-cluster wins over global; the legacy resource_columns
// surface wins over views within a scope so users with both keys configured
// see the explicit resource_columns override. GVR keys win over Kind keys
// within the views surface, matching ResolveView.
func ColumnsForKind(rt ResourceRef, context string) []string {
	gvrKey := rt.GVRKey()
	kindKey := rt.KindKey()
	if context != "" {
		if len(ConfigClusterResourceColumns) > 0 {
			if clusterCols, ok := ConfigClusterResourceColumns[context]; ok {
				if cols, ok := clusterCols[kindKey]; ok {
					return cols
				}
			}
		}
		if cols := viewColumnNames(ConfigClusterViews[context], gvrKey, kindKey); cols != nil {
			return cols
		}
	}
	if len(ConfigResourceColumns) > 0 && kindKey != "" {
		if cols, ok := ConfigResourceColumns[kindKey]; ok {
			return cols
		}
	}
	if cols := viewColumnNames(ConfigViews, gvrKey, kindKey); cols != nil {
		return cols
	}
	return nil
}

// HiddenBuiltinsForView returns the set of built-in column keys that should
// be hidden when rendering rt under the given cluster context. When a view
// is configured and its columns list omits a given built-in (Context,
// Namespace, Ready, Restarts, Age, Status), that built-in is hidden so the
// table renders only what the user asked for. Returns nil when no view is
// configured for this resource in this scope. Resolution follows ResolveView:
// per-cluster GVR > per-cluster Kind > global GVR > global Kind.
func HiddenBuiltinsForView(rt ResourceRef, context string) map[string]bool {
	if rt.Kind == "" && rt.Resource == "" {
		return nil
	}
	v, ok := ResolveView(rt, context)
	if !ok || v == nil {
		return nil
	}
	listed := make(map[string]bool, len(v.Columns))
	for _, c := range v.Columns {
		listed[c.Name] = true
	}
	hidden := make(map[string]bool, 6)
	for _, name := range []string{"Context", "Namespace", "Ready", "Restarts", "Age", "Status"} {
		if !listed[name] {
			hidden[name] = true
		}
	}
	if len(hidden) == 0 {
		return nil
	}
	return hidden
}

// viewColumnNames returns the ordered list of column Name fields from the
// view stored under gvrKey (preferred) or kindKey (fallback), or nil when
// no view exists under either. Matches the GVR-then-Kind resolution order
// used by ResolveView. Columns flagged |W (FlagWideOnly) are omitted unless
// ActiveFullscreenMode is set, so users can keep wide-only columns in their
// config without crowding narrow table layouts.
func viewColumnNames(views map[string]*View, gvrKey, kindKey string) []string {
	if len(views) == 0 {
		return nil
	}
	v := views[gvrKey]
	if v == nil && kindKey != "" {
		v = views[kindKey]
	}
	if v == nil || len(v.Columns) == 0 {
		return nil
	}
	names := make([]string, 0, len(v.Columns))
	for _, c := range v.Columns {
		if c.Flags&FlagWideOnly != 0 && !ActiveFullscreenMode {
			continue
		}
		names = append(names, c.Name)
	}
	return names
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

// DefaultKubesharkNamespace is the namespace probed for Service
// kubeshark-hub when no override is configured. Matches the default of
// `helm install kubeshark kubeshark/kubeshark --namespace kubeshark`.
const DefaultKubesharkNamespace = "kubeshark"

// ConfigKubesharkNamespace is the namespace where the Traffic Capture
// overlay looks for the kubeshark hub Service. Set via the kubeshark
// config block (see configFile.Kubeshark). Falls back to
// DefaultKubesharkNamespace.
var ConfigKubesharkNamespace = DefaultKubesharkNamespace

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
// TerminalModePTY, TerminalModeExec, TerminalModeMux. Initialised from
// defaultTerminalMode() so Windows users — where the embedded PTY
// driver (github.com/creack/pty) has no working backend — start in
// Exec mode by default and don't hit "failed to start PTY: unsupported"
// the first time they trigger an interactive shell (issue #194).
var ConfigTerminalMode = defaultTerminalMode()

// defaultTerminalMode returns the package-level default for
// ConfigTerminalMode based on the current runtime OS.
func defaultTerminalMode() string {
	return defaultTerminalModeForOS(runtime.GOOS)
}

// defaultTerminalModeForOS is the testable inner form. Windows must
// default to Exec because creack/pty's Windows StartWithSize returns
// ErrUnsupported; every other platform gets the richer embedded PTY.
func defaultTerminalModeForOS(goos string) string {
	if goos == "windows" {
		return TerminalModeExec
	}
	return TerminalModePTY
}

// resolveTerminalMode validates the `terminal:` config value against the
// runtime OS. It returns (effectiveMode, warning):
//   - effectiveMode is always a valid mode the caller can assign to
//     ConfigTerminalMode. When the input is empty or unrecognised the
//     caller's currentMode is returned unchanged.
//   - warning is non-empty when a fallback was applied; the caller is
//     expected to log it at Warn level so the user sees why their
//     configured value didn't stick.
//
// `pty` on Windows is silently downgraded to `exec` because the embedded
// PTY backend (github.com/creack/pty) has no working Windows
// implementation — accepting the option would just trap users in a
// state where every interactive action fails (issue #194).
func resolveTerminalMode(configValue, goos, currentMode string) (mode string, warning string) {
	normalized := strings.ToLower(strings.TrimSpace(configValue))
	if normalized == "" {
		return currentMode, ""
	}
	switch normalized {
	case TerminalModePTY:
		if goos == "windows" {
			return TerminalModeExec, "terminal: pty is not supported on Windows (no PTY backend); using exec"
		}
		return TerminalModePTY, ""
	case TerminalModeExec, TerminalModeMux:
		return normalized, ""
	default:
		// The raw configValue is intentionally NOT embedded in the
		// warning — log redaction policy. Users can check their config
		// file to see what they typed; the "valid" list logged by the
		// caller tells them what is accepted.
		return currentMode, "unrecognised terminal mode; using " + currentMode
	}
}

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

// ConfigPinnedTypes lists version-agnostic resource-type pin keys
// ("group/resource") that should move into the top-level Pinned section.
var ConfigPinnedTypes []string

// ConfigPinnedSummaries lists resource-type pin keys ("group/resource") from
// the config file whose status summaries render on the cluster dashboard.
// Merged at load time with the per-context / per-union-set state file.
var ConfigPinnedSummaries []string

// ConfigPinnedSummariesSet reports whether the config file set pinned_summaries
// at all, distinguishing an explicit `pinned_summaries: []` ("no summaries,
// not even the built-in defaults") from the key being absent ("use the
// defaults"). ConfigPinnedSummaries alone can't tell these apart - both leave
// it empty.
var ConfigPinnedSummariesSet bool

// ConfigUnionSets holds named multi-cluster groups defined in config.
// Resolved by --union-set into a list of contexts + optional namespace.
// Empty by default; populated by applyConfigOptions when union_sets is
// present in the YAML config.
var ConfigUnionSets []UnionSetConfig

// ConfigTipsEnabled controls whether to show random tips on startup.
var ConfigTipsEnabled = true

// ConfigConfirmOnExit controls whether ctrl+c on the last tab shows a quit confirmation.
var ConfigConfirmOnExit = true

// ConfigDimOverlay controls whether the rest of the screen fades to a dim
// foreground colour while ANY overlay is up, leaving only the bottom hint
// bar at full intensity. On by default — flip to false via the
// `dim_overlay:` config key for terminals where the SGR faint attribute
// looks awkward.
var ConfigDimOverlay = true

// ConfigLogTailLines controls how many log lines are initially loaded via
// --tail (per container). Kept small by default — older history loads on
// scroll-up — so opening logs on a multi-container pod doesn't dump tens of
// thousands of lines at once.
var ConfigLogTailLines = 100

// ConfigLogTopDefaultProfile is the default Log Top parser:
// auto | traefik-json | nginx-combined | ingress-nginx | envoy | json | logfmt.
var ConfigLogTopDefaultProfile = "auto"

// ConfigLogTailLinesShort is the tail line count used by the "Tail Logs" action
// menu entry. It intentionally defaults to a small value (10) so users get a
// lightweight peek at recent output without the full 100-line hit.
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

// ConfigWhichKeyEnabled toggles the which-key popup (chords work regardless).
var ConfigWhichKeyEnabled = true

// ConfigWhichKeyDelayMs delays the popup after a prefix is pressed (0..2000).
var ConfigWhichKeyDelayMs = 0

// ConfigWhichKeyLeaderDelayMs delays the leader panel (0..2000). Zero by
// default: the leader key does nothing except open the panel, so there is no
// burst of side-effect presses to keep it out of the way of.
var ConfigWhichKeyLeaderDelayMs = 0

// ConfigShowRareTypes is the startup default for the "show rarely-used resource
// types" toggle (the ToggleRare / H key). When true the sidebar surfaces the
// rare and "Advanced" resource types from launch. The runtime H toggle
// overrides it for the rest of the session.
var ConfigShowRareTypes bool

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

// ConfigK8sClientQPS / ConfigK8sClientBurst are the global foreground REST
// client rate limits. Defaults mirror k8s.DefaultClientQPS/Burst; raising the
// stock client-go 5/10 keeps foreground lists responsive while background
// scans run. Per-cluster overrides take precedence.
var (
	ConfigK8sClientQPS   = 50
	ConfigK8sClientBurst = 100
)

// ConfigClusterK8sClientQPS / ConfigClusterK8sClientBurst map context names to
// per-cluster overrides of the foreground client rate. A value here wins over
// the global default for that context.
var (
	ConfigClusterK8sClientQPS   = map[string]int{}
	ConfigClusterK8sClientBurst = map[string]int{}
)

// init wires the foreground rate resolver into the k8s package so every REST
// client picks up the configured (or default) QPS/Burst. Set unconditionally
// at package load; resolution is lazy, so config applied later is honored.
func init() {
	k8s.RateLimitForContext = ResolveK8sClientRate
}

// ResolveK8sClientRate returns the effective foreground QPS/Burst for a
// context. Precedence: per-cluster override > global > compiled default.
// Returned as (float32, int) to match rest.Config fields; this is the resolver
// wired into k8s.RateLimitForContext.
func ResolveK8sClientRate(context string) (qps float32, burst int) {
	q, b := ConfigK8sClientQPS, ConfigK8sClientBurst
	if v, ok := ConfigClusterK8sClientQPS[context]; ok && v > 0 {
		q = v
	}
	if v, ok := ConfigClusterK8sClientBurst[context]; ok && v > 0 {
		b = v
	}
	return float32(q), b
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
