package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
)

// Theme defines the color palette for the application.
type Theme struct {
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Text       string `json:"text"`
	SelectedFg string `json:"selected_fg"`
	SelectedBg string `json:"selected_bg"`
	Border     string `json:"border"`
	Dimmed     string `json:"dimmed"`
	Error      string `json:"error"`
	Warning    string `json:"warning"`
	Purple     string `json:"purple"`
	Base       string `json:"base"`
	BarBg      string `json:"bar_bg"`
	Surface    string `json:"surface"`
}

// Keybindings defines configurable keybindings for direct actions.
type Keybindings struct {
	Logs        string `json:"logs"`
	Refresh     string `json:"refresh"`
	Restart     string `json:"restart"`
	Exec        string `json:"exec"`
	Edit        string `json:"edit"`
	Describe    string `json:"describe"`
	Delete      string `json:"delete"`
	ForceDelete string `json:"force_delete"`
	Scale       string `json:"scale"`
}

// DefaultKeybindings returns the default keybinding configuration.
func DefaultKeybindings() Keybindings {
	return Keybindings{
		Logs:        "L",
		Refresh:     "R",
		Restart:     "r",
		Exec:        "s",
		Edit:        "E",
		Describe:    "v",
		Delete:      "D",
		ForceDelete: "X",
		Scale:       "S",
	}
}

// ActiveKeybindings holds the currently active keybinding configuration.
var ActiveKeybindings Keybindings

// ConfigLogPath holds the log_path value from the config file (if any).
var ConfigLogPath string

// SearchAbbreviations maps short abbreviations to full resource type names for search.
var SearchAbbreviations map[string]string

// IconMode controls how resource icons are displayed.
// Valid values: "unicode" (default, show unicode icons), "simple" (show ASCII labels), "emoji" (show emoji), "none" (hide icons).
var IconMode string = "unicode"

// ConfigResourceColumns holds per-resource-type column overrides.
// Keys are lowercase Kind names (e.g. "pod", "deployment"); values are column lists.
var ConfigResourceColumns map[string][]string

// ConfigFilterMatch defines the match criteria for a user-configured filter preset.
type ConfigFilterMatch struct {
	Status      string `json:"status" yaml:"status"`             // substring match against item.Status
	ReadyNot    bool   `json:"ready_not" yaml:"ready_not"`       // true means ready numerator != denominator
	RestartsGt  int    `json:"restarts_gt" yaml:"restarts_gt"`   // match when restarts > this value
	Column      string `json:"column" yaml:"column"`             // column key to check
	ColumnValue string `json:"column_value" yaml:"column_value"` // substring match against column value
}

// ConfigFilterPreset defines a single user-configured filter preset.
type ConfigFilterPreset struct {
	Name  string            `json:"name" yaml:"name"`
	Key   string            `json:"key" yaml:"key"`
	Match ConfigFilterMatch `json:"match" yaml:"match"`
}

// ConfigFilterPresets maps lowercase Kind names to user-configured filter presets.
var ConfigFilterPresets map[string][]ConfigFilterPreset

// ColumnsForKind returns the configured column list for the given resource kind.
// It checks ConfigResourceColumns for the given kind and returns nil if not found (meaning auto-detect).
func ColumnsForKind(kind string) []string {
	if len(ConfigResourceColumns) > 0 && kind != "" {
		if cols, ok := ConfigResourceColumns[strings.ToLower(kind)]; ok {
			return cols
		}
	}
	return nil
}

// ConfigDashboard controls whether to show a cluster dashboard when entering a context.
// Defaults to true.
var ConfigDashboard = true

// ConfigTerminalMode controls how exec/shell commands run: "pty" (default, embedded in TUI) or "exec" (takes over terminal).
var ConfigTerminalMode = "pty"

// CustomAction represents a user-defined action for a specific resource kind.
type CustomAction struct {
	Label       string `json:"label" yaml:"label"`
	Command     string `json:"command" yaml:"command"`
	Key         string `json:"key" yaml:"key"`
	Description string `json:"description" yaml:"description"`
}

// ConfigCustomActions maps resource kinds to user-defined custom actions.
var ConfigCustomActions map[string][]CustomAction

// ConfigPinnedGroups lists CRD API groups that should appear prominently
// right after built-in categories, before other discovered CRDs.
var ConfigPinnedGroups []string

// ConfigTipsEnabled controls whether to show random tips on startup.
var ConfigTipsEnabled = true

// ConfigConfirmOnExit controls whether ctrl+c on the last tab shows a quit confirmation.
var ConfigConfirmOnExit = true

// ConfigLogTailLines controls how many log lines are initially loaded via --tail.
// When the user scrolls to the top, older logs are fetched in the background.
var ConfigLogTailLines = 1000

// ActiveSchemeName holds the name of the currently active color scheme.
var ActiveSchemeName = "tokyonight"

// ConfigTransparentBg controls whether bar/surface backgrounds are transparent.
// When true, TitleBarStyle, TitleBreadcrumbStyle, and StatusBarBgStyle skip
// setting a background color so the terminal's own background shows through.
var ConfigTransparentBg bool

// BaseBg, BarBg, and SurfaceBg are exported TerminalColor values that inline
// styles can use to set their background, respecting the transparency setting.
// When ConfigTransparentBg is true they are NoColor{}; otherwise they hold
// the theme's Base, BarBg, and Surface colors respectively.
var (
	BaseBg    lipgloss.TerminalColor = lipgloss.NoColor{}
	BarBg     lipgloss.TerminalColor = lipgloss.NoColor{}
	SurfaceBg lipgloss.TerminalColor = lipgloss.NoColor{}
)

type configFile struct {
	// Colorscheme selects a built-in color scheme by name (e.g. "dracula", "nord").
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
	// Terminal controls how exec/shell commands run: "exec" (takes over terminal) or "pty" (embedded in TUI).
	Terminal string `json:"terminal" yaml:"terminal"`
	// PinnedGroups lists CRD API groups that should appear prominently
	// right after built-in categories. Example: ["karpenter.sh", "monitoring.coreos.com"]
	PinnedGroups []string `json:"pinned_groups" yaml:"pinned_groups"`
	// Monitoring maps cluster context names to custom monitoring endpoint config.
	// The special key "default" applies to clusters without explicit config.
	Monitoring map[string]model.MonitoringConfig `json:"monitoring" yaml:"monitoring"`
	// Tips controls whether to show random tips on startup.
	// Defaults to true. Set to false to disable.
	Tips *bool `json:"tips" yaml:"tips"`
	// LogTailLines controls how many log lines are initially loaded via --tail.
	// When the user scrolls to the top, older logs are fetched in the background.
	// Defaults to 1000.
	LogTailLines *int `json:"log_tail_lines" yaml:"log_tail_lines"`
	// ConfirmOnExit controls whether ctrl+c on the last tab shows a quit confirmation.
	// Defaults to true. Set to false to exit immediately on ctrl+c.
	ConfirmOnExit *bool `json:"confirm_on_exit" yaml:"confirm_on_exit"`
	// TransparentBg makes bar and surface backgrounds transparent so the terminal's
	// own background shows through. Selection highlights remain opaque.
	// Defaults to false.
	TransparentBg *bool `json:"transparent_background" yaml:"transparent_background"`
}

// DefaultAbbreviations returns the default search abbreviation map.
func DefaultAbbreviations() map[string]string {
	return map[string]string{
		"pvc":    "persistentvolumeclaim",
		"pv":     "persistentvolume",
		"hpa":    "horizontalpodautoscaler",
		"vpa":    "verticalpodautoscaler",
		"ds":     "daemonset",
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

// DefaultTheme returns the default Tokyonight-inspired theme.
func DefaultTheme() Theme {
	return Theme{
		Primary:    "#7aa2f7",
		Secondary:  "#9ece6a",
		Text:       "#c0caf5",
		SelectedFg: "#1a1b26",
		SelectedBg: "#7aa2f7",
		Border:     "#3b4261",
		Dimmed:     "#565f89",
		Error:      "#f7768e",
		Warning:    "#e0af68",
		Purple:     "#bb9af7",
		Base:       "#1a1b26",
		BarBg:      "#24283b",
		Surface:    "#1f2335",
	}
}

// LoadAndApplyTheme loads the theme and keybindings from config file and applies them.
func LoadAndApplyTheme() {
	theme := DefaultTheme()
	kb := DefaultKeybindings()

	abbr := DefaultAbbreviations()

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			ApplyTheme(theme)
			ActiveKeybindings = kb
			SearchAbbreviations = abbr
			return
		}
		configDir = filepath.Join(home, ".config")
	}

	configPath := filepath.Join(configDir, "lfk", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		ApplyTheme(theme)
		ActiveKeybindings = kb
		SearchAbbreviations = abbr
		return
	}

	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		ApplyTheme(theme)
		ActiveKeybindings = kb
		SearchAbbreviations = abbr
		return
	}

	ConfigLogPath = cfg.LogPath

	// If a built-in colorscheme is specified, use it as the base theme.
	if cfg.Colorscheme != "" {
		if scheme, ok := BuiltinSchemes()[strings.ToLower(cfg.Colorscheme)]; ok {
			theme = scheme
			ActiveSchemeName = strings.ToLower(cfg.Colorscheme)
		}
	}

	// Merge theme: only override non-empty values (custom overrides on top of scheme).
	mergeThemeOverrides(&theme, cfg.Theme)

	// Merge keybindings: only override non-empty values.
	if cfg.Keybindings.Logs != "" {
		kb.Logs = cfg.Keybindings.Logs
	}
	if cfg.Keybindings.Refresh != "" {
		kb.Refresh = cfg.Keybindings.Refresh
	}
	if cfg.Keybindings.Restart != "" {
		kb.Restart = cfg.Keybindings.Restart
	}
	if cfg.Keybindings.Exec != "" {
		kb.Exec = cfg.Keybindings.Exec
	}
	if cfg.Keybindings.Describe != "" {
		kb.Describe = cfg.Keybindings.Describe
	}
	if cfg.Keybindings.Delete != "" {
		kb.Delete = cfg.Keybindings.Delete
	}
	if cfg.Keybindings.ForceDelete != "" {
		kb.ForceDelete = cfg.Keybindings.ForceDelete
	}
	if cfg.Keybindings.Scale != "" {
		kb.Scale = cfg.Keybindings.Scale
	}

	// Apply icon mode from config.
	if cfg.Icons != "" {
		switch strings.ToLower(cfg.Icons) {
		case "unicode", "simple", "emoji", "none":
			IconMode = strings.ToLower(cfg.Icons)
		default:
			// Invalid value; keep default "unicode".
		}
	}

	// Apply per-resource-type column overrides (normalize keys to lowercase).
	if len(cfg.ResourceColumns) > 0 {
		ConfigResourceColumns = make(map[string][]string, len(cfg.ResourceColumns))
		for k, v := range cfg.ResourceColumns {
			ConfigResourceColumns[strings.ToLower(k)] = v
		}
	}

	// Apply dashboard config.
	if cfg.Dashboard != nil {
		ConfigDashboard = *cfg.Dashboard
	}

	// Merge abbreviations: user config overrides/extends defaults.
	for k, v := range cfg.Abbreviations {
		abbr[strings.ToLower(k)] = strings.ToLower(v)
	}

	// Apply custom actions from config.
	if len(cfg.CustomActions) > 0 {
		ConfigCustomActions = cfg.CustomActions
	}

	// Apply filter presets from config (normalize keys to lowercase).
	if len(cfg.FilterPresets) > 0 {
		ConfigFilterPresets = make(map[string][]ConfigFilterPreset, len(cfg.FilterPresets))
		for k, v := range cfg.FilterPresets {
			ConfigFilterPresets[strings.ToLower(k)] = v
		}
	}

	// Apply terminal mode from config.
	if cfg.Terminal != "" {
		switch strings.ToLower(cfg.Terminal) {
		case "pty", "exec":
			ConfigTerminalMode = strings.ToLower(cfg.Terminal)
		}
	}

	// Apply pinned groups from config.
	if len(cfg.PinnedGroups) > 0 {
		ConfigPinnedGroups = cfg.PinnedGroups
	}

	// Apply monitoring endpoint overrides.
	if cfg.Monitoring != nil {
		model.ConfigMonitoring = cfg.Monitoring
	}

	// Apply tips setting.
	if cfg.Tips != nil {
		ConfigTipsEnabled = *cfg.Tips
	}

	// Apply log tail lines setting.
	if cfg.LogTailLines != nil && *cfg.LogTailLines > 0 {
		ConfigLogTailLines = *cfg.LogTailLines
	}

	// Apply confirm on exit setting.
	if cfg.ConfirmOnExit != nil {
		ConfigConfirmOnExit = *cfg.ConfirmOnExit
	}

	// Apply transparent background setting.
	if cfg.TransparentBg != nil {
		ConfigTransparentBg = *cfg.TransparentBg
	}

	ApplyTheme(theme)
	ActiveKeybindings = kb
	SearchAbbreviations = abbr
}

// ApplyTheme updates all style variables with the given theme colors.
func ApplyTheme(t Theme) {
	// baseBg is applied to all column/content text styles so the theme
	// background shows behind text (ANSI resets from inner styled content
	// would otherwise clear the container background). NoColor when transparent.
	var baseBg lipgloss.TerminalColor = lipgloss.NoColor{}
	if !ConfigTransparentBg {
		baseBg = lipgloss.Color(t.Base)
	}

	ActiveColumnStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Primary)).
		BorderBackground(baseBg).
		Background(baseBg)

	InactiveColumnStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Border)).
		BorderBackground(baseBg).
		Background(baseBg)

	SelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.SelectedBg))

	NormalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(baseBg)

	DimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(baseBg)

	CategoryStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Bold(true).
		Italic(true).
		Background(baseBg)

	IconStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Background(baseBg)

	StatusRunning = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Secondary)).Background(baseBg)
	StatusProgressing = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Primary)).Background(baseBg)
	StatusFailed = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Background(baseBg)
	StatusOther = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dimmed)).Background(baseBg)

	var barBg lipgloss.TerminalColor = lipgloss.NoColor{}
	if !ConfigTransparentBg {
		barBg = lipgloss.Color(t.BarBg)
	}

	BarDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(barBg)

	TitleBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(barBg).
		Padding(0, 1)

	TitleBreadcrumbStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Background(barBg)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Background(barBg).
		Padding(0, 1)

	NamespaceBadgeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.Primary)).
		Bold(true).
		Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Underline(true).
		Background(baseBg)

	HeaderIconStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Background(baseBg)

	NamespaceStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Warning)).
		Bold(true).
		Background(barBg).
		Padding(0, 1)

	YamlViewStyle = lipgloss.NewStyle().
		Background(baseBg).
		Padding(1, 2)

	YamlKeyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Bold(true).
		Background(baseBg)

	YamlValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(baseBg)

	YamlPunctuationStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(baseBg)

	YamlCommentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Italic(true).
		Background(baseBg)

	StatusBarBgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(barBg).
		Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(barBg).
		Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Bold(true)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Error)).
		Background(baseBg).
		Bold(true)

	CurrentMarkerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Background(baseBg).
		Bold(true)

	var surfaceBg lipgloss.TerminalColor = lipgloss.NoColor{}
	if !ConfigTransparentBg {
		surfaceBg = lipgloss.Color(t.Surface)
	}

	// Export background colors so inline styles across the codebase can use them.
	BaseBg = baseBg
	BarBg = barBg
	SurfaceBg = surfaceBg

	OverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Primary)).
		BorderBackground(surfaceBg).
		Background(surfaceBg).
		Padding(1, 2)

	innerPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Border)).
		BorderBackground(surfaceBg).
		Background(surfaceBg).
		Padding(0, 1)

	OverlayTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Padding(0, 0, 1, 0).
		Background(surfaceBg)

	OverlaySelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.SelectedBg))

	OverlayNormalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(surfaceBg)

	OverlayFilterStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Bold(true).
		Background(surfaceBg)

	OverlayDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(surfaceBg)

	OverlayWarningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Error)).
		Bold(true).
		Background(surfaceBg)

	OverlayInputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Bold(true).
		Underline(true).
		Background(surfaceBg)

	ParentHighlightStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.Border))

	StatusMessageOkStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Background(barBg).
		Bold(true)

	StatusMessageErrStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Error)).
		Background(barBg).
		Bold(true)

	SearchHighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.Warning)).
		Foreground(lipgloss.Color(t.Base)).
		Bold(true)

	SelectedSearchHighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.Warning)).
		Foreground(lipgloss.Color(t.SelectedBg)).
		Bold(true).
		Underline(true)

	SelectionMarkerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Background(baseBg).
		Bold(true)

	SelectionCountStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.Secondary)).
		Bold(true)

	DeprecationStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Warning)).
		Background(baseBg)
}

// mergeThemeOverrides applies non-empty fields from overrides onto base.
func mergeThemeOverrides(base *Theme, overrides Theme) {
	if overrides.Primary != "" {
		base.Primary = overrides.Primary
	}
	if overrides.Secondary != "" {
		base.Secondary = overrides.Secondary
	}
	if overrides.Text != "" {
		base.Text = overrides.Text
	}
	if overrides.SelectedFg != "" {
		base.SelectedFg = overrides.SelectedFg
	}
	if overrides.SelectedBg != "" {
		base.SelectedBg = overrides.SelectedBg
	}
	if overrides.Border != "" {
		base.Border = overrides.Border
	}
	if overrides.Dimmed != "" {
		base.Dimmed = overrides.Dimmed
	}
	if overrides.Error != "" {
		base.Error = overrides.Error
	}
	if overrides.Warning != "" {
		base.Warning = overrides.Warning
	}
	if overrides.Purple != "" {
		base.Purple = overrides.Purple
	}
	if overrides.Base != "" {
		base.Base = overrides.Base
	}
	if overrides.BarBg != "" {
		base.BarBg = overrides.BarBg
	}
	if overrides.Surface != "" {
		base.Surface = overrides.Surface
	}
}
