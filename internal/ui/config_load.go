package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
)

type configFile struct {
	// Colorscheme selects a built-in color scheme by name (e.g. "dracula",
	// "nord"). Supports Ghostty-style dual-mode syntax to enable automatic
	// dark/light switching via CSI 996/2031:
	//
	//   colorscheme: "dark:Rose Pine,light:Rose Pine Dawn"
	//
	// Either segment may be omitted. Without the prefix syntax the value is
	// used as a plain scheme name and dark/light switching is disabled.
	// Custom theme overrides in the "theme" section are applied on top.
	Colorscheme   string            `json:"colorscheme" yaml:"colorscheme"`
	Theme         Theme             `json:"theme" yaml:"theme"`
	Keybindings   Keybindings       `json:"keybindings" yaml:"keybindings"`
	LogPath       string            `json:"log_path" yaml:"log_path"`
	Abbreviations map[string]string `json:"abbreviations" yaml:"abbreviations"`
	// Icons controls icon display mode: "unicode" (default), "simple" (ASCII labels), "emoji" (emoji), "none" (no icons).
	Icons string `json:"icons" yaml:"icons"`
	// ResourceColumns maps resource Kind names (case-insensitive, e.g. "Pod", "Deployment")
	// to per-type column lists. When set, these override the global Columns setting for that kind.
	ResourceColumns map[string][]string `json:"resource_columns" yaml:"resource_columns"`
	// Dashboard controls whether to show a cluster dashboard when entering a context.
	// Defaults to true. Set to false to go directly to resource types.
	Dashboard *bool `json:"dashboard" yaml:"dashboard"`
	// CustomActions maps resource Kind names (e.g. "Pod", "Deployment") to a list of
	// user-defined actions. Each action specifies a label, shell command template,
	// shortcut key, and description.
	CustomActions map[string][]CustomAction `json:"custom_actions" yaml:"custom_actions"`
	// FilterPresets maps resource Kind names (case-insensitive, e.g. "Pod", "Deployment")
	// to user-defined quick filter presets that appear alongside the built-in presets.
	FilterPresets map[string][]ConfigFilterPreset `json:"filter_presets" yaml:"filter_presets"`
	// Terminal controls how exec/shell commands run: "pty" (embedded in
	// TUI), "exec" (takes over the terminal), or "mux" (open in a new
	// tmux/zellij window or pane — requires lfk to be running inside a
	// supported multiplexer).
	Terminal string `json:"terminal" yaml:"terminal"`
	// ScrollbackLines is the per-tab capacity of the embedded PTY
	// scrollback ring buffer. Default 5000; clamped to
	// [ScrollbackLinesMin, ScrollbackLinesMax]. Only meaningful in pty
	// mode — exec and mux delegate scrollback to the host terminal.
	ScrollbackLines int `json:"scrollback_lines" yaml:"scrollback_lines"`
	// PinnedGroups lists CRD API groups that should appear prominently
	// right after built-in categories. Example: ["karpenter.sh", "monitoring.coreos.com"]
	PinnedGroups []string `json:"pinned_groups" yaml:"pinned_groups"`
	// Monitoring maps cluster context names to custom monitoring endpoint config.
	// The special key "_global" applies to clusters without explicit config.
	Monitoring map[string]model.MonitoringConfig `json:"monitoring" yaml:"monitoring"`
	// Tips controls whether to show random tips on startup.
	// Defaults to true. Set to false to disable.
	Tips *bool `json:"tips" yaml:"tips"`
	// LogTailLines controls how many log lines are initially loaded via --tail.
	// When the user scrolls to the top, older logs are fetched in the background.
	// Defaults to 1000.
	LogTailLines *int `json:"log_tail_lines" yaml:"log_tail_lines"`
	// LogTailLinesShort controls how many log lines the "Tail Logs" action menu
	// entry loads via --tail. Intended for quick peeks without the full history
	// hit. Defaults to 10. Non-positive values are ignored (default is kept).
	LogTailLinesShort *int `json:"log_tail_lines_short" yaml:"log_tail_lines_short"`
	// LogRenderAnsi controls whether ANSI SGR sequences (colour, bold,
	// underline) emitted by log producers are rendered in the viewer.
	// Defaults to true. Set to false to strip all ANSI escapes, matching
	// the historical behaviour where the sanitizer replaced every ESC
	// byte with U+FFFD.
	LogRenderAnsi *bool `json:"log_render_ansi" yaml:"log_render_ansi"`
	// ScrollOff is the number of lines to keep visible above/below the cursor.
	// Defaults to 5.
	ScrollOff *int `json:"scrolloff" yaml:"scrolloff"`
	// ConfirmOnExit controls whether ctrl+c on the last tab shows a quit confirmation.
	// Defaults to true. Set to false to exit immediately on ctrl+c.
	ConfirmOnExit *bool `json:"confirm_on_exit" yaml:"confirm_on_exit"`
	// TransparentBg makes bar and surface backgrounds transparent so the terminal's
	// own background shows through. Selection highlights remain opaque.
	// Defaults to false.
	TransparentBg *bool `json:"transparent_background" yaml:"transparent_background"`
	// Mouse controls whether the TUI captures mouse input for click navigation
	// and scroll. Defaults to true. Set to false to allow native terminal text
	// selection (useful in Terminal.app where shift+click doesn't work).
	Mouse *bool `json:"mouse" yaml:"mouse"`
	// WatchInterval is the polling interval used in watch mode, expressed as
	// a Go duration string (e.g. "2s", "500ms", "1m"). Clamped to [500ms, 10m].
	// Defaults to 2s when unset or invalid.
	WatchInterval string `json:"watch_interval" yaml:"watch_interval"`
	// Clusters maps context names to per-cluster configuration overrides.
	Clusters map[string]clusterConfig `json:"clusters" yaml:"clusters"`
	// NoColor, when true, strips foreground/background colors from all styles
	// so the UI renders in terminal-native monochrome. Emphasis is preserved
	// via bold/underline/reverse SGR codes. The NO_COLOR env var (per
	// https://no-color.org) takes precedence over this field.
	NoColor *bool `json:"no_color" yaml:"no_color"`
	// SecretLazyLoading controls how Secret resources are fetched. When false
	// (default), Secrets behave like every other resource type: full objects
	// are pulled and data is eagerly decoded into the list. When true, only
	// metadata is fetched for the list and decoded values are loaded on hover.
	// Turn on in clusters with many Helm release secrets to cut list latency;
	// the trade-off is a per-hover GET and a brief blank-data frame until the
	// fetch resolves.
	SecretLazyLoading *bool `json:"secret_lazy_loading" yaml:"secret_lazy_loading"`
	// InformerCache controls how lists are routed: "off" round-trips every
	// time (matches kubectl), "auto" (default) starts in direct mode per
	// (context, GVR) and promotes to a shared informer once a list crosses
	// 1000 items — demoting again when the list shrinks below 500 for three
	// consecutive cached calls — and "always" eagerly opens a watch on the
	// first list. Accepts the legacy bool form for compatibility: `true`
	// maps to "always", `false` maps to "off". Issue #86 was the original
	// motivation: on a 7k-pod cluster a namespace switch goes from a 1–2s
	// round trip to an in-process slice walk under "auto"/"always".
	InformerCache *informerCacheSetting `json:"informer_cache" yaml:"informer_cache"`
	// MinContrastRatio is a normalized readability knob in [0.0, 1.0]. When set
	// above zero, ApplyTheme nudges each foreground color's HSL lightness until
	// the fg/bg pair meets the derived WCAG contrast ratio:
	//
	//   wcagTarget = 1.0 + value * 20.0
	//
	// Examples: 0.175 ≈ WCAG AA (4.5:1), 0.3 ≈ AAA (7.0:1), 1.0 = maximum.
	// Values outside [0, 1] are clamped. Hue and saturation are preserved.
	MinContrastRatio *float64 `json:"min_contrast_ratio" yaml:"min_contrast_ratio"`
	// ReadOnly disables all mutating actions (delete, edit, scale, restart,
	// exec, port-forward, drain, cordon, etc.) for every context. Per-context
	// overrides under clusters.<name>.read_only take precedence; the
	// --read-only CLI flag wins over both.
	ReadOnly *bool `json:"read_only" yaml:"read_only"`
}

// clusterConfig holds per-cluster configuration overrides.
type clusterConfig struct {
	ResourceColumns map[string][]string `json:"resource_columns" yaml:"resource_columns"`
	// ReadOnly, when set, overrides the global read_only setting for this
	// context only. Useful for marking specific clusters (e.g. "prod") as
	// read-only while leaving others mutable.
	ReadOnly *bool `json:"read_only" yaml:"read_only"`
}

// LoadConfig loads the config file (theme, keybindings, abbreviations, etc.) and applies them.
func LoadConfig(configOverride string) {
	theme := DefaultTheme()
	kb := DefaultKeybindings()
	abbr := DefaultAbbreviations()

	cfg, ok := loadConfigFile(configOverride)
	if !ok {
		ApplyTheme(theme)
		ActiveKeybindings = kb
		SearchAbbreviations = abbr
		return
	}

	ConfigLogPath = cfg.LogPath
	applyColorscheme(&theme, cfg)
	mergeThemeOverrides(&theme, cfg.Theme)
	MergeKeybindings(&kb, &cfg.Keybindings)
	applyConfigOptions(cfg)
	applyConfigMaps(cfg, abbr)

	ApplyTheme(theme)
	ActiveKeybindings = kb
	SearchAbbreviations = abbr
}

// loadConfigFile reads and parses the YAML config file.
// When configOverride is non-empty, it is used directly instead of the default
// XDG-based path.
func loadConfigFile(configOverride string) (configFile, bool) {
	var configPath string
	if configOverride != "" {
		configPath = configOverride
	} else {
		configDir := os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return configFile{}, false
			}
			configDir = filepath.Join(home, ".config")
		}
		configPath = filepath.Join(configDir, "lfk", "config.yaml")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return configFile{}, false
	}

	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Surface YAML parse errors directly to stderr: LoadConfig runs
		// before logger.Init in main.go, so logger.Warn here would go to
		// io.Discard. Dropping the entire config silently was the previous
		// behaviour and made typos very hard to debug.
		fmt.Fprintf(os.Stderr,
			"lfk: could not parse config %s: %v\nfalling back to built-in defaults\n",
			configPath, err)
		return configFile{}, false
	}
	return cfg, true
}
