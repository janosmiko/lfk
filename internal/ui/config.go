package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
)

// Keybindings defines configurable keybindings for the application.
type Keybindings struct {
	// Navigation
	Left           string `json:"left" yaml:"left"`
	Right          string `json:"right" yaml:"right"`
	Down           string `json:"down" yaml:"down"`
	Up             string `json:"up" yaml:"up"`
	Enter          string `json:"enter" yaml:"enter"`
	JumpTop        string `json:"jump_top" yaml:"jump_top"`
	JumpBottom     string `json:"jump_bottom" yaml:"jump_bottom"`
	PageDown       string `json:"page_down" yaml:"page_down"`
	PageUp         string `json:"page_up" yaml:"page_up"`
	PageForward    string `json:"page_forward" yaml:"page_forward"`
	PageBack       string `json:"page_back" yaml:"page_back"`
	LevelCluster   string `json:"level_cluster" yaml:"level_cluster"`
	LevelTypes     string `json:"level_types" yaml:"level_types"`
	LevelResources string `json:"level_resources" yaml:"level_resources"`
	PreviewDown    string `json:"preview_down" yaml:"preview_down"`
	PreviewUp      string `json:"preview_up" yaml:"preview_up"`
	JumpOwner      string `json:"jump_owner" yaml:"jump_owner"`

	// Views and Modes
	Help            string `json:"help" yaml:"help"`
	Filter          string `json:"filter" yaml:"filter"`
	Search          string `json:"search" yaml:"search"`
	NextMatch       string `json:"next_match" yaml:"next_match"`
	PrevMatch       string `json:"prev_match" yaml:"prev_match"`
	TogglePreview   string `json:"toggle_preview" yaml:"toggle_preview"`
	ResourceMap     string `json:"resource_map" yaml:"resource_map"`
	Fullscreen      string `json:"fullscreen" yaml:"fullscreen"`
	FilterPresets   string `json:"filter_presets" yaml:"filter_presets"`
	ErrorLog        string `json:"error_log" yaml:"error_log"`
	SecretToggle    string `json:"secret_toggle" yaml:"secret_toggle"`
	FinalizerSearch string `json:"finalizer_search" yaml:"finalizer_search"`
	APIExplorer     string `json:"api_explorer" yaml:"api_explorer"`
	RBACBrowser     string `json:"rbac_browser" yaml:"rbac_browser"`
	ThemeSelector   string `json:"theme_selector" yaml:"theme_selector"`
	CommandBar      string `json:"command_bar" yaml:"command_bar"`
	WatchMode       string `json:"watch_mode" yaml:"watch_mode"`
	SortNext        string `json:"sort_next" yaml:"sort_next"`
	SortPrev        string `json:"sort_prev" yaml:"sort_prev"`
	SortFlip        string `json:"sort_flip" yaml:"sort_flip"`
	SortReset       string `json:"sort_reset" yaml:"sort_reset"`
	SaveResource    string `json:"save_resource" yaml:"save_resource"`
	Monitoring      string `json:"monitoring" yaml:"monitoring"`
	Security        string `json:"security" yaml:"security"`
	QuotaDashboard  string `json:"quota_dashboard" yaml:"quota_dashboard"`
	ExpandCollapse  string `json:"expand_collapse" yaml:"expand_collapse"`
	PinGroup        string `json:"pin_group" yaml:"pin_group"`
	ColumnToggle    string `json:"column_toggle" yaml:"column_toggle"`

	// Actions
	NamespaceSelector string `json:"namespace_selector" yaml:"namespace_selector"`
	AllNamespaces     string `json:"all_namespaces" yaml:"all_namespaces"`
	ActionMenu        string `json:"action_menu" yaml:"action_menu"`
	Logs              string `json:"logs" yaml:"logs"`
	LabelEditor       string `json:"label_editor" yaml:"label_editor"`
	SecretEditor      string `json:"secret_editor" yaml:"secret_editor"`
	CreateTemplate    string `json:"create_template" yaml:"create_template"`
	Refresh           string `json:"refresh" yaml:"refresh"`
	Restart           string `json:"restart" yaml:"restart"`
	Exec              string `json:"exec" yaml:"exec"`
	Edit              string `json:"edit" yaml:"edit"`
	Describe          string `json:"describe" yaml:"describe"`
	Delete            string `json:"delete" yaml:"delete"`
	ForceDelete       string `json:"force_delete" yaml:"force_delete"`
	Scale             string `json:"scale" yaml:"scale"`
	OpenBrowser       string `json:"open_browser" yaml:"open_browser"`
	CopyName          string `json:"copy_name" yaml:"copy_name"`
	CopyYAML          string `json:"copy_yaml" yaml:"copy_yaml"`
	PasteApply        string `json:"paste_apply" yaml:"paste_apply"`
	Diff              string `json:"diff" yaml:"diff"`

	// Multi-selection
	ToggleSelect string `json:"toggle_select" yaml:"toggle_select"`
	SelectRange  string `json:"select_range" yaml:"select_range"`
	SelectAll    string `json:"select_all" yaml:"select_all"`

	// Tabs
	NewTab  string `json:"new_tab" yaml:"new_tab"`
	NextTab string `json:"next_tab" yaml:"next_tab"`
	PrevTab string `json:"prev_tab" yaml:"prev_tab"`

	// Bookmarks
	SetMark   string `json:"set_mark" yaml:"set_mark"`
	OpenMarks string `json:"open_marks" yaml:"open_marks"`

	// Terminal mode
	TerminalToggle string `json:"terminal_toggle" yaml:"terminal_toggle"`
}

// DefaultKeybindings returns the default keybinding configuration.
func DefaultKeybindings() Keybindings {
	return Keybindings{
		// Navigation
		Left: "h", Right: "l", Down: "j", Up: "k",
		Enter: "enter", JumpTop: "g", JumpBottom: "G",
		PageDown: "ctrl+d", PageUp: "ctrl+u",
		PageForward: "ctrl+f", PageBack: "ctrl+b",
		LevelCluster: "0", LevelTypes: "1", LevelResources: "2",
		PreviewDown: "J", PreviewUp: "K", JumpOwner: "o",

		// Views
		Help: "?", Filter: "f", Search: "/",
		NextMatch: "n", PrevMatch: "N",
		TogglePreview: "P", ResourceMap: "M", Fullscreen: "F",
		FilterPresets: ".", ErrorLog: "!", SecretToggle: "ctrl+s",
		FinalizerSearch: "ctrl+g", APIExplorer: "I", RBACBrowser: "U",
		ThemeSelector: "T", CommandBar: ":", WatchMode: "w",
		SortNext: ">", SortPrev: "<", SortFlip: "=", SortReset: "-",
		SaveResource: "W", Monitoring: "@",
		Security:       "#",
		QuotaDashboard: "Q", ExpandCollapse: "z", PinGroup: "p",
		ColumnToggle: ",",

		// Actions
		NamespaceSelector: "\\", AllNamespaces: "A", ActionMenu: "x",
		Logs: "L", LabelEditor: "i", SecretEditor: "e",
		CreateTemplate: "a", Refresh: "R", Restart: "r",
		Exec: "s", Edit: "E", Describe: "v", Delete: "D",
		ForceDelete: "X", Scale: "S",
		OpenBrowser: "ctrl+o", CopyName: "y", CopyYAML: "Y",
		PasteApply: "ctrl+p", Diff: "d",

		// Multi-selection
		ToggleSelect: " ", SelectRange: "ctrl+@", SelectAll: "ctrl+a",

		// Tabs
		NewTab: "t", NextTab: "]", PrevTab: "[",

		// Bookmarks
		SetMark: "m", OpenMarks: "'",

		// Terminal mode
		TerminalToggle: "ctrl+t",
	}
}

// DefaultSecurityConfig returns the default security configuration applied when
// no override is present.
func DefaultSecurityConfig() model.SecurityConfig {
	return model.SecurityConfig{
		Enabled:               true,
		PerResourceIndicators: true,
		PerResourceAction:     true,
		RefreshTTL:            "30s",
		AvailabilityTTL:       "60s",
		Sources: map[string]model.SecuritySourceCfg{
			"heuristic": {Enabled: true, Checks: []string{
				"privileged", "host_namespaces", "host_path", "readonly_root_fs",
				"run_as_root", "allow_priv_esc", "dangerous_caps",
				"missing_resource_limits", "default_sa", "latest_tag",
			}},
			"trivy_operator": {Enabled: true},
		},
	}
}

// MergeKeybindings copies non-empty string fields from src to dst.
func MergeKeybindings(dst, src *Keybindings) {
	dv := reflect.ValueOf(dst).Elem()
	sv := reflect.ValueOf(src).Elem()
	for i := range dv.NumField() {
		sf := sv.Field(i)
		if sf.Kind() == reflect.String && sf.String() != "" {
			dv.Field(i).SetString(sf.String())
		}
	}
}

// ActiveKeybindings holds the currently active keybinding configuration.
var ActiveKeybindings = DefaultKeybindings()

// ConfigLogPath holds the log_path value from the config file (if any).
var ConfigLogPath string

// SearchAbbreviations maps short abbreviations to full resource type names for search.
var SearchAbbreviations map[string]string

// IconMode controls how resource icons are displayed.
var IconMode = "unicode"

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

// ConfigTerminalMode controls how exec/shell commands run.
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

// ConfigPinnedGroups lists CRD API groups that should appear prominently.
var ConfigPinnedGroups []string

// ConfigTipsEnabled controls whether to show random tips on startup.
var ConfigTipsEnabled = true

// ConfigConfirmOnExit controls whether ctrl+c on the last tab shows a quit confirmation.
var ConfigConfirmOnExit = true

// ConfigLogTailLines controls how many log lines are initially loaded via --tail.
var ConfigLogTailLines = 1000

// ConfigScrollOff is the number of lines to keep visible above/below the cursor.
// Used by all views with cursor-based navigation.
var ConfigScrollOff = 5

// ActiveSchemeName holds the name of the currently active color scheme.
var ActiveSchemeName = "tokyonight-storm"

// ConfigTransparentBg controls whether bar/surface backgrounds are transparent.
var ConfigTransparentBg bool

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
	// Security maps cluster context names to security dashboard configuration.
	// The special key "default" applies to clusters without explicit config.
	Security map[string]model.SecurityConfig `json:"security" yaml:"security"`
	// Tips controls whether to show random tips on startup.
	// Defaults to true. Set to false to disable.
	Tips *bool `json:"tips" yaml:"tips"`
	// LogTailLines controls how many log lines are initially loaded via --tail.
	// When the user scrolls to the top, older logs are fetched in the background.
	// Defaults to 1000.
	LogTailLines *int `json:"log_tail_lines" yaml:"log_tail_lines"`
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
	// Clusters maps context names to per-cluster configuration overrides.
	Clusters map[string]clusterConfig `json:"clusters" yaml:"clusters"`
}

// clusterConfig holds per-cluster configuration overrides.
type clusterConfig struct {
	ResourceColumns map[string][]string `json:"resource_columns" yaml:"resource_columns"`
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

// LoadConfig loads the config file (theme, keybindings, abbreviations, etc.) and applies them.
func LoadConfig() {
	theme := DefaultTheme()
	kb := DefaultKeybindings()
	abbr := DefaultAbbreviations()

	cfg, ok := loadConfigFile()
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
func loadConfigFile() (configFile, bool) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return configFile{}, false
		}
		configDir = filepath.Join(home, ".config")
	}

	configPath := filepath.Join(configDir, "lfk", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return configFile{}, false
	}

	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return configFile{}, false
	}
	return cfg, true
}

// applyColorscheme selects a built-in colorscheme if specified in config.
func applyColorscheme(theme *Theme, cfg configFile) {
	if cfg.Colorscheme != "" {
		if scheme, ok := BuiltinSchemes()[strings.ToLower(cfg.Colorscheme)]; ok {
			*theme = scheme
			ActiveSchemeName = strings.ToLower(cfg.Colorscheme)
		}
	}
}

// applyConfigOptions applies scalar config options (icons, terminal, tips, etc.).
func applyConfigOptions(cfg configFile) {
	if cfg.Icons != "" {
		switch strings.ToLower(cfg.Icons) {
		case "unicode", "simple", "emoji", "none":
			IconMode = strings.ToLower(cfg.Icons)
		}
	}
	if cfg.Dashboard != nil {
		ConfigDashboard = *cfg.Dashboard
	}
	if cfg.Terminal != "" {
		switch strings.ToLower(cfg.Terminal) {
		case "pty", "exec":
			ConfigTerminalMode = strings.ToLower(cfg.Terminal)
		}
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
	if cfg.ScrollOff != nil && *cfg.ScrollOff >= 0 {
		ConfigScrollOff = *cfg.ScrollOff
	}
	if cfg.ConfirmOnExit != nil {
		ConfigConfirmOnExit = *cfg.ConfirmOnExit
	}
	if cfg.TransparentBg != nil {
		ConfigTransparentBg = *cfg.TransparentBg
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
		for ctx, cc := range cfg.Clusters {
			if len(cc.ResourceColumns) > 0 {
				cols := make(map[string][]string, len(cc.ResourceColumns))
				for k, v := range cc.ResourceColumns {
					cols[strings.ToLower(k)] = v
				}
				ConfigClusterResourceColumns[ctx] = cols
			}
		}
	}
}
