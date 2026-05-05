package ui

import "reflect"

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
	QuotaDashboard  string `json:"quota_dashboard" yaml:"quota_dashboard"`
	TasksOverlay    string `json:"tasks_overlay" yaml:"tasks_overlay"`
	ExpandCollapse  string `json:"expand_collapse" yaml:"expand_collapse"`
	PinGroup        string `json:"pin_group" yaml:"pin_group"`
	ColumnToggle    string `json:"column_toggle" yaml:"column_toggle"`
	ToggleRare      string `json:"toggle_rare" yaml:"toggle_rare"`
	OrphanOverlay   string `json:"orphan_overlay" yaml:"orphan_overlay"`

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

	// Read-only mode
	ReadOnlyToggle string `json:"readonly_toggle" yaml:"readonly_toggle"`

	// Cluster color picker (Level=Clusters only): assigns a background tint
	// to the highlighted cluster row, persisted across restarts.
	ClusterColorPicker string `json:"cluster_color_picker" yaml:"cluster_color_picker"`
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
		QuotaDashboard: "Q", TasksOverlay: "`",
		ExpandCollapse: "z", PinGroup: "p",
		ColumnToggle: ",", ToggleRare: "H",
		OrphanOverlay: "O",

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

		// Read-only mode
		ReadOnlyToggle: "ctrl+r",

		// Cluster color picker. Bound to Shift+L because the picker only
		// exists at Level=Clusters and "L" is otherwise the Logs action
		// (which has no meaning at the cluster picker — no pods to
		// stream from). The dispatch case is gated on Level=Clusters
		// and breaks out at deeper levels so "L" continues to open
		// Logs everywhere else.
		ClusterColorPicker: "L",
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
