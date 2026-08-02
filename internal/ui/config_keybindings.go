package ui

import (
	"reflect"
	"slices"
	"strings"
	"unicode"
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
	JumpBack       string `json:"jump_back" yaml:"jump_back"`

	// Views and Modes
	Help          string `json:"help" yaml:"help"`
	Filter        string `json:"filter" yaml:"filter"`
	Search        string `json:"search" yaml:"search"`
	SeverityUp    string `json:"severity_up" yaml:"severity_up"`
	SeverityDown  string `json:"severity_down" yaml:"severity_down"`
	NextMatch     string `json:"next_match" yaml:"next_match"`
	PrevMatch     string `json:"prev_match" yaml:"prev_match"`
	TogglePreview string `json:"toggle_preview" yaml:"toggle_preview"`
	ToggleWrap    string `json:"toggle_wrap" yaml:"toggle_wrap"`
	// Viewer display toggles (YAML / diff / describe / log / event viewers).
	ToggleLineNumbers string `json:"toggle_line_numbers" yaml:"toggle_line_numbers"`
	ToggleFold        string `json:"toggle_fold" yaml:"toggle_fold"`
	ToggleFoldAll     string `json:"toggle_fold_all" yaml:"toggle_fold_all"`
	ToggleFollow      string `json:"toggle_follow" yaml:"toggle_follow"`
	ToggleTimestamps  string `json:"toggle_timestamps" yaml:"toggle_timestamps"`
	TogglePrefixes    string `json:"toggle_prefixes" yaml:"toggle_prefixes"`
	ToggleUnified     string `json:"toggle_unified" yaml:"toggle_unified"`
	ResourceMap       string `json:"resource_map" yaml:"resource_map"`
	Fullscreen        string `json:"fullscreen" yaml:"fullscreen"`
	FilterPresets     string `json:"filter_presets" yaml:"filter_presets"`
	ErrorLog          string `json:"error_log" yaml:"error_log"`
	SecretToggle      string `json:"secret_toggle" yaml:"secret_toggle"`
	FinalizerSearch   string `json:"finalizer_search" yaml:"finalizer_search"`
	APIExplorer       string `json:"api_explorer" yaml:"api_explorer"`
	ObjectExplorer    string `json:"object_explorer" yaml:"object_explorer"`
	TreeView          string `json:"tree_view" yaml:"tree_view"`
	RBACBrowser       string `json:"rbac_browser" yaml:"rbac_browser"`
	ThemeSelector     string `json:"theme_selector" yaml:"theme_selector"`
	CommandBar        string `json:"command_bar" yaml:"command_bar"`
	WatchMode         string `json:"watch_mode" yaml:"watch_mode"`
	SortNext          string `json:"sort_next" yaml:"sort_next"`
	SortPrev          string `json:"sort_prev" yaml:"sort_prev"`
	SortFlip          string `json:"sort_flip" yaml:"sort_flip"`
	SortReset         string `json:"sort_reset" yaml:"sort_reset"`
	SaveResource      string `json:"save_resource" yaml:"save_resource"`
	Monitoring        string `json:"monitoring" yaml:"monitoring"`
	QuotaDashboard    string `json:"quota_dashboard" yaml:"quota_dashboard"`
	TasksOverlay      string `json:"tasks_overlay" yaml:"tasks_overlay"`
	ExpandCollapse    string `json:"expand_collapse" yaml:"expand_collapse"`
	PinGroup          string `json:"pin_group" yaml:"pin_group"`
	ColumnToggle      string `json:"column_toggle" yaml:"column_toggle"`
	ToggleRare        string `json:"toggle_rare" yaml:"toggle_rare"`
	OrphanOverlay     string `json:"orphan_overlay" yaml:"orphan_overlay"`
	SessionManager    string `json:"session_manager" yaml:"session_manager"`

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
	CopyField         string `json:"copy_field" yaml:"copy_field"`
	PasteApply        string `json:"paste_apply" yaml:"paste_apply"`
	Diff              string `json:"diff" yaml:"diff"`

	// Multi-selection
	ToggleSelect string `json:"toggle_select" yaml:"toggle_select"`
	SelectRange  string `json:"select_range" yaml:"select_range"`
	SelectAll    string `json:"select_all" yaml:"select_all"`

	// Tabs
	NewTab       string `json:"new_tab" yaml:"new_tab"`
	NextTab      string `json:"next_tab" yaml:"next_tab"`
	PrevTab      string `json:"prev_tab" yaml:"prev_tab"`
	MoveTabLeft  string `json:"move_tab_left" yaml:"move_tab_left"`
	MoveTabRight string `json:"move_tab_right" yaml:"move_tab_right"`

	// Bookmarks
	SetMark   string `json:"set_mark" yaml:"set_mark"`
	OpenMarks string `json:"open_marks" yaml:"open_marks"`

	// Terminal mode
	TerminalToggle string `json:"terminal_toggle" yaml:"terminal_toggle"`

	// MouseToggle suspends/resumes mouse capture at runtime so the user can
	// make a native terminal text selection (and use the terminal's own
	// scrollback / copy) without restarting with --no-mouse.
	MouseToggle string `json:"mouse_toggle" yaml:"mouse_toggle"`

	// Read-only mode
	ReadOnlyToggle string `json:"readonly_toggle" yaml:"readonly_toggle"`

	// Cluster color picker (Level=Clusters only): assigns a background tint
	// to the highlighted cluster row, persisted across restarts.
	ClusterColorPicker string `json:"cluster_color_picker" yaml:"cluster_color_picker"`

	// Local cluster manager (Level=Clusters only): opens the kind/k3d/
	// minikube manager overlay so users can list, create, start, stop,
	// and delete local clusters without leaving the TUI.
	LocalClusterManager string `json:"local_cluster_manager" yaml:"local_cluster_manager"`

	// SecurityIgnoreToggle flips the show/hide-ignored-findings state on
	// the active security view and triggers a refresh. Dispatched only when
	// the user is on a security view, so it can reuse a key bound elsewhere
	// (it shadows LabelEditor, which is meaningless on synthetic finding
	// rows) — mirroring how ClusterColorPicker reuses the Logs key at the
	// cluster picker. Must not be a terminal control-alias (ctrl+i/m/[);
	// see TestDefaultKeybindingsNoUnreachableControlAliases.
	SecurityIgnoreToggle string `json:"security_ignore_toggle" yaml:"security_ignore_toggle"`

	// SecurityBadgeToggle shows/hides the per-resource SEC severity badge on
	// explorer rows without affecting the Security dashboard or source probing.
	SecurityBadgeToggle string `json:"security_badge_toggle" yaml:"security_badge_toggle"`

	// TogglePreviewLogs toggles the right-pane live-log preview for the
	// selected pod. Bound to "L" (shift+l) at deeper levels (resources,
	// containers); at Level=Clusters the same key opens ClusterColorPicker
	// and the dispatcher gates accordingly. The fullscreen log viewer
	// (kb.Logs) uses "ctrl+l" so the plain "L" is free for this toggle.
	TogglePreviewLogs string `json:"toggle_preview_logs" yaml:"toggle_preview_logs"`

	// LogTop opens the Log Top aggregation viewer directly from the open log
	// viewer over the lines already buffered. "T" is free in modeLogs
	// (ThemeSelector's "T" only dispatches in explorer/error-log modes).
	LogTop string `json:"log_top" yaml:"log_top"`

	// Goto navigation: vim-style g-prefix chords that switch the active
	// resource type in the current context. Each holds the full chord
	// (e.g. "gp"). Reserved: "gg" jumps to list top, "G" to list bottom.
	GotoPods         string `json:"goto_pods" yaml:"goto_pods"`
	GotoDeployments  string `json:"goto_deployments" yaml:"goto_deployments"`
	GotoServices     string `json:"goto_services" yaml:"goto_services"`
	GotoNodes        string `json:"goto_nodes" yaml:"goto_nodes"`
	GotoNamespaces   string `json:"goto_namespaces" yaml:"goto_namespaces"`
	GotoIngresses    string `json:"goto_ingresses" yaml:"goto_ingresses"`
	GotoJobs         string `json:"goto_jobs" yaml:"goto_jobs"`
	GotoCronJobs     string `json:"goto_cronjobs" yaml:"goto_cronjobs"`
	GotoReplicaSets  string `json:"goto_replicasets" yaml:"goto_replicasets"`
	GotoDaemonSets   string `json:"goto_daemonsets" yaml:"goto_daemonsets"`
	GotoStatefulSets string `json:"goto_statefulsets" yaml:"goto_statefulsets"`
	GotoConfigMaps   string `json:"goto_configmaps" yaml:"goto_configmaps"`
	GotoSecrets      string `json:"goto_secrets" yaml:"goto_secrets"`
	GotoHPAs         string `json:"goto_hpas" yaml:"goto_hpas"`
	GotoPVCs         string `json:"goto_pvcs" yaml:"goto_pvcs"`
	GotoPVs          string `json:"goto_pvs" yaml:"goto_pvs"`
	GotoPDBs         string `json:"goto_pdbs" yaml:"goto_pdbs"`

	// PreviousNamespace is a g-prefix chord that swaps the namespace scope with
	// the one in effect before the last change (toggle back and forth). Holds
	// the full chord (e.g. "g\\").
	PreviousNamespace string `json:"previous_namespace" yaml:"previous_namespace"`
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
		JumpBack: "backspace",

		// Views
		Help: "?", Filter: "f", Search: "/",
		SeverityUp: "o", SeverityDown: "i",
		NextMatch: "n", PrevMatch: "N",
		TogglePreview: "P", ToggleWrap: ">", ResourceMap: "M", Fullscreen: "F",
		ToggleLineNumbers: "#", ToggleFold: "z", ToggleFoldAll: "Z",
		ToggleFollow: "F", ToggleTimestamps: "s", TogglePrefixes: "p", ToggleUnified: "u",
		FilterPresets: ".", ErrorLog: "!", SecretToggle: "ctrl+s",
		FinalizerSearch: "ctrl+g", APIExplorer: "I", ObjectExplorer: "O", RBACBrowser: "U",
		// "T" is also the explorer-level ThemeSelector; TreeView only dispatches
		// inside the Object/API Explorer modes, which have their own handlers.
		TreeView:      "T",
		ThemeSelector: "T", CommandBar: ":", WatchMode: "w",
		SortNext: ">", SortPrev: "<", SortFlip: "=", SortReset: "-",
		SaveResource: "W", Monitoring: "@",
		QuotaDashboard: "Q", TasksOverlay: "`",
		ExpandCollapse: "z", PinGroup: "p",
		ColumnToggle: ",", ToggleRare: "H",
		OrphanOverlay:  "Z",
		SessionManager: "C",

		// Actions
		NamespaceSelector: "\\", AllNamespaces: "A", ActionMenu: "x",
		Logs: "ctrl+l", LabelEditor: "i", SecretEditor: "e",
		CreateTemplate: "a", Refresh: "R", Restart: "r",
		Exec: "s", Edit: "E", Describe: "v", Delete: "D",
		ForceDelete: "X", Scale: "S",
		OpenBrowser: "ctrl+o", CopyName: "y", CopyYAML: "Y",
		CopyField:  "ctrl+y",
		PasteApply: "ctrl+p", Diff: "d",

		// Multi-selection
		ToggleSelect: "space", SelectRange: "ctrl+@", SelectAll: "ctrl+a",

		// Tabs. Move keys mirror the switch keys: "}" (shift+]) moves the
		// active tab one slot right, "{" (shift+[) one slot left.
		NewTab: "t", NextTab: "]", PrevTab: "[",
		MoveTabLeft: "{", MoveTabRight: "}",

		// Bookmarks
		SetMark: "m", OpenMarks: "'",

		// Terminal mode
		TerminalToggle: "ctrl+t",

		// Mouse capture toggle. Ctrl+Option+Y (Ctrl+Alt+Y) avoids the Ctrl+X
		// collision with the bookmark-overlay delete and is hard to hit by
		// accident; rebindable like any other key. Bubble Tea prints modifiers
		// in a fixed order (ctrl, alt, shift, ...), so the stored string is
		// "ctrl+alt+y".
		MouseToggle: "ctrl+alt+y",

		// Read-only mode
		ReadOnlyToggle: "ctrl+r",

		// "i" (not "ctrl+i": a terminal sends ctrl+i as Tab, so that binding
		// could never fire). Gated to security views in the dispatcher, where
		// it shadows the otherwise-meaningless LabelEditor ("i").
		SecurityIgnoreToggle: "i",
		SecurityBadgeToggle:  "B",

		// Cluster color picker. Bound to "L" (shift+l) because the picker
		// only exists at Level=Clusters. At deeper levels the same key
		// opens the live-log preview pane (TogglePreviewLogs). The dispatch
		// case in handleExplorerNavKey is gated on Level=Clusters and breaks
		// at deeper levels so handleExplorerUIKey can pick up "L" for the
		// live-log toggle. The fullscreen log viewer uses "ctrl+l".
		ClusterColorPicker: "L",

		// Local cluster manager. Ctrl+N is gated on Level=Clusters in
		// the dispatcher so it doesn't shadow other keys at deeper
		// levels.
		LocalClusterManager: "ctrl+n",

		// Live-log preview pane toggle. "L" (shift+l) at resource/container
		// levels; at Level=Clusters the same key opens ClusterColorPicker
		// (gated in handleExplorerNavKey). The fullscreen log viewer
		// (kb.Logs) is "ctrl+l".
		TogglePreviewLogs: "L",

		// Log Top from log viewer. "T" is free in modeLogs.
		LogTop: "T",

		// Goto navigation chords (g prefix).
		GotoPods: "gp", GotoDeployments: "gd", GotoServices: "gs",
		GotoNodes: "gn", GotoNamespaces: "gN", GotoIngresses: "gi",
		GotoJobs: "gj", GotoCronJobs: "gc", GotoReplicaSets: "gr",
		GotoDaemonSets: "gD", GotoStatefulSets: "gt", GotoConfigMaps: "gC",
		GotoSecrets: "gS", GotoHPAs: "gh", GotoPVCs: "gv", GotoPVs: "gV",
		GotoPDBs: "gb",

		// Jump to previous namespace (g-prefix chord, sits next to the "\\"
		// namespace selector).
		PreviousNamespace: "g\\",
	}
}

// MergeKeybindings copies non-empty string fields from src to dst,
// normalising each value first.
func MergeKeybindings(dst, src *Keybindings) {
	dv := reflect.ValueOf(dst).Elem()
	sv := reflect.ValueOf(src).Elem()
	for i := range dv.NumField() {
		sf := sv.Field(i)
		if sf.Kind() == reflect.String && sf.String() != "" {
			dv.Field(i).SetString(NormalizeKeybinding(sf.String()))
		}
	}
}

// modifierOrder is the order Bubble Tea prints modifiers in. Bindings are
// matched by string, so a config written in any other order never fires.
var modifierOrder = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

// NormalizeKeybinding rewrites a configured keystroke into the spelling the
// terminal layer reports, so a binding written the natural way still fires:
//
//   - a literal space used to arrive as " " and now arrives as "space"
//   - modifiers used to appear in the order the terminal sent them
//     ("alt+ctrl+y") and are now always printed in modifierOrder ("ctrl+alt+y")
//   - a shift chord reports the SHIFTED character, so "ctrl+shift+x" has to
//     become "ctrl+shift+X" — the lowercase form never matches
//   - shift on its own is not reported as a modifier at all; the terminal just
//     sends the shifted character, so "shift+x" becomes plain "X"
//
// A binding naming an unrecognised modifier is returned untouched rather than
// silently reordered into something that also would not match.
func NormalizeKeybinding(s string) string {
	if s == " " {
		return "space"
	}
	if !strings.Contains(s, "+") {
		return s
	}
	parts := strings.Split(s, "+")
	key, mods := parts[len(parts)-1], parts[:len(parts)-1]
	if key == "" && len(mods) > 0 {
		// The key itself is "+", e.g. "ctrl++".
		key, mods = "+", mods[:len(mods)-1]
	}
	if len(mods) == 0 {
		return s
	}
	seen := make(map[string]bool, len(mods))
	for _, m := range mods {
		seen[m] = true
	}
	ordered := make([]string, 0, len(mods)+1)
	for _, m := range modifierOrder {
		if seen[m] {
			ordered = append(ordered, m)
			delete(seen, m)
		}
	}
	if len(seen) > 0 {
		return s
	}

	// Shift is expressed in the character itself, not as a prefix, whenever the
	// key is a single letter. Named keys (tab, up, f1) keep the prefix.
	if isSingleLetter(key) && slices.Contains(ordered, "shift") {
		key = strings.ToUpper(key)
		if len(ordered) == 1 {
			return key
		}
	}
	return strings.Join(append(ordered, key), "+")
}

// isSingleLetter reports whether s is exactly one alphabetic rune.
func isSingleLetter(s string) bool {
	r := []rune(s)
	return len(r) == 1 && unicode.IsLetter(r[0])
}

// ActiveKeybindings holds the currently active keybinding configuration.
var ActiveKeybindings = DefaultKeybindings()
