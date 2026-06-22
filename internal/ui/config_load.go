package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/paths"
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
	// Appearance groups the visual knobs (colorscheme, icons, no_color,
	// transparent_background, min_contrast_ratio, dim_overlay). This is the
	// canonical home; the flat keys of the same name are deprecated aliases
	// kept for backward compatibility. When both are set, the appearance group
	// wins (it is merged down onto the flat fields at load time).
	Appearance    *AppearanceConfig `json:"appearance" yaml:"appearance"`
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
	// Views maps GVR ("apps/v1/deployments") or Kind ("deployment", case-insensitive)
	// to a view config — ordered columns and a default sort column. Subsumes
	// ResourceColumns (which remains supported for backward compat). See the
	// ColumnSpec parser for the per-entry format.
	Views map[string]configView `json:"views" yaml:"views"`
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
	// PinnedTypes lists version-agnostic resource-type pin keys
	// ("group/resource", e.g. "apps/deployments" or "argoproj.io/applications")
	// to move into the top-level Pinned section.
	PinnedTypes []string `json:"pinned_types" yaml:"pinned_types"`
	// Monitoring maps cluster context names to custom monitoring endpoint config.
	// The special key "_global" applies to clusters without explicit config.
	Monitoring map[string]model.MonitoringConfig `json:"monitoring" yaml:"monitoring"`
	// Tips controls whether to show random tips on startup.
	// Defaults to true. Set to false to disable.
	Tips *bool `json:"tips" yaml:"tips"`
	// LogViewer groups all log-viewer settings (tail sizes, ANSI rendering,
	// and the startup-toggle defaults for preview / prefixes / timestamps).
	// This is the canonical home for these knobs; the flat log_tail_lines,
	// log_tail_lines_short, and log_render_ansi keys below are deprecated
	// aliases kept for backward compatibility. When both are set, the
	// log_viewer group wins. Note: log_path is the application's own log
	// file and is intentionally NOT part of this group.
	LogViewer *LogViewerConfig `json:"log_viewer" yaml:"log_viewer"`
	// LogTailLines is the deprecated flat alias for log_viewer.tail_lines.
	// Controls how many log lines are initially loaded via --tail; scrolling
	// to the top fetches older history in the background. Defaults to 1000.
	LogTailLines *int `json:"log_tail_lines" yaml:"log_tail_lines"`
	// LogTailLinesShort is the deprecated flat alias for
	// log_viewer.tail_lines_short. Controls how many log lines the "Tail Logs"
	// action loads via --tail. Defaults to 10. Non-positive values are ignored.
	LogTailLinesShort *int `json:"log_tail_lines_short" yaml:"log_tail_lines_short"`
	// LogRenderAnsi is the deprecated flat alias for log_viewer.render_ansi.
	// Controls whether ANSI SGR sequences (colour, bold, underline) emitted by
	// log producers are rendered. Defaults to true; false strips all ANSI.
	LogRenderAnsi *bool `json:"log_render_ansi" yaml:"log_render_ansi"`
	// LogTopDefaultProfile is the default Log Top parser profile. Valid values:
	// auto | traefik-json | nginx-combined | ingress-nginx | envoy | json | logfmt.
	// Unknown values fall back to "auto". Defaults to "auto".
	LogTopDefaultProfile *string `json:"log_top_default_profile" yaml:"log_top_default_profile"`
	// YAMLViewer holds startup-default toggles for the YAML viewer.
	YAMLViewer *YAMLViewerConfig `json:"yaml_viewer" yaml:"yaml_viewer"`
	// DiffViewer holds startup-default toggles for the diff viewer.
	DiffViewer *DiffViewerConfig `json:"diff_viewer" yaml:"diff_viewer"`
	// DescribeViewer holds startup-default toggles for the describe viewer.
	DescribeViewer *DescribeViewerConfig `json:"describe_viewer" yaml:"describe_viewer"`
	// ObjectExplorer holds startup-default toggles for the Object Explorer.
	ObjectExplorer *ObjectExplorerConfig `json:"object_explorer" yaml:"object_explorer"`
	// APIExplorer holds startup-default toggles for the API Explorer.
	APIExplorer *APIExplorerConfig `json:"api_explorer" yaml:"api_explorer"`
	// SplitPreview is the startup default for the split preview pane (runtime
	// toggle from the explorer). Default true (pane shown).
	SplitPreview *bool `json:"split_preview" yaml:"split_preview"`
	// WatchMode is the startup default for live watch/polling. Default true.
	WatchMode *bool `json:"watch_mode" yaml:"watch_mode"`
	// AllNamespaces is the startup default for the namespace scope: true shows
	// all namespaces, false starts scoped to the context's default namespace.
	// The --namespace CLI flag and per-bookmark/session scope override this.
	// Default true.
	AllNamespaces *bool `json:"all_namespaces" yaml:"all_namespaces"`
	// Events holds startup-default toggles for the events view.
	Events *EventsConfig `json:"events" yaml:"events"`
	// ScrollOff is the number of lines to keep visible above/below the cursor.
	// Defaults to 5.
	ScrollOff *int `json:"scrolloff" yaml:"scrolloff"`
	// ConfirmOnExit controls whether ctrl+c on the last tab shows a quit confirmation.
	// Defaults to true. Set to false to exit immediately on ctrl+c.
	ConfirmOnExit *bool `json:"confirm_on_exit" yaml:"confirm_on_exit"`
	// DimOverlay fades the rest of the screen while any overlay is up,
	// keeping only the bottom hint bar at full intensity. Defaults to true.
	// Set to false for terminals where the SGR faint attribute looks
	// awkward.
	DimOverlay *bool `json:"dim_overlay" yaml:"dim_overlay"`
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
	// ShowRareTypes, when true, surfaces the rarely-used resource types (CSI
	// internals, admission webhooks, leases, runtime classes, and the
	// synthetic "Advanced" category of uncategorized core resources) in the
	// sidebar from startup — as if the ToggleRare key (H) were already pressed
	// — so the full resource-type list is always visible. The runtime H toggle
	// still flips it for the session; it resets to this value on the next
	// launch. Default false.
	ShowRareTypes *bool `json:"show_rare_types" yaml:"show_rare_types"`
	// Security configures the built-in security-findings dashboard. When
	// disabled the Security sidebar category, the SEC badge, and all source
	// probing are turned off. Per-context overrides under
	// clusters.<name>.security take precedence over this global setting.
	Security *securityConfig `json:"security" yaml:"security"`
	// RightsizingDefaults configures the initial strategy + headroom that
	// the right-sizing advisor uses on its first overlay open of the
	// session. Once the user changes strategy or headroom in the overlay,
	// those choices stick across subsequent overlay opens (within the
	// session); restarting lfk falls back to these config values.
	//
	//   strategy: "vpa" | "prom_max_1d" | "prom_avg_1d" | "prom_p95_7d" | "snapshot"
	//   headroom: one of 1.0, 1.1, 1.25, 1.5, 1.75, 2.0
	//
	// Invalid values are ignored (logged once at startup). Both fields
	// optional — when unset, lfk uses the previous picker selection if
	// any (sticky behavior), otherwise falls back to "highest-priority
	// available strategy" + 1.25 headroom.
	RightsizingDefaults *RightsizingDefaultsConfig `json:"rightsizing_defaults" yaml:"rightsizing_defaults"`
	// Kubeshark configures the kubeshark hand-off backend used by the
	// Traffic Capture overlay. Only the namespace is plumbed today;
	// future fields can land here without further config schema changes.
	Kubeshark *KubesharkConfig `json:"kubeshark" yaml:"kubeshark"`
	// Scheduler holds the runtime knobs for the priority task scheduler.
	// All fields are optional; missing keys fall back to the scheduler
	// package's compiled defaults.
	Scheduler *SchedulerConfig `json:"scheduler" yaml:"scheduler"`
	// KubeconfigDir overrides the default kubeconfig directory path (~/.kube/config.d).
	// Accepts either a single string ("/path/to/dir") or a list of strings
	// (["/dir/one", "/dir/two"]); when a list is given, every directory is
	// merged into the discovery set. The --kubeconfig-dir CLI flag takes
	// precedence (repeatable), then the KUBECONFIG_DIR env var (colon-separated),
	// then this config value.
	KubeconfigDir *kubeconfigDirsSetting `json:"kubeconfig_dir" yaml:"kubeconfig_dir"`
	// UnionSets defines named multi-cluster groups for the --union-set CLI
	// flag. Each set bundles a list of contexts and an optional default
	// namespace so users don't have to retype long --union-context lists
	// for the same recurring cluster groups (e.g. blue/green/canary).
	// CLI --namespace overrides the per-set namespace; --union-context and
	// --context are mutually exclusive with --union-set.
	UnionSets UnionSetsConfig `json:"union_sets" yaml:"union_sets"`
	// GotoTargets maps full g-prefix chords (e.g. "gA") to user-defined
	// goto targets. Chords must start with the jump_top key and be exactly
	// 2 runes. Kind is required; Group and Name are optional.
	// User entries override built-ins on chord collision.
	GotoTargets map[string]GotoTargetEntry `json:"goto_targets" yaml:"goto_targets"`
}

// UnionSetsConfig accepts both supported top-level shapes:
//
//	union_sets:
//	  - name: staging
//	    contexts: [...]
//
// and:
//
//	union_sets:
//	  staging:
//	    contexts: [...]
//
// The map form is the preferred shape for copy/paste config because the key is
// exactly what --union-set and the cluster picker reference.
type UnionSetsConfig []UnionSetConfig

func (sets *UnionSetsConfig) UnmarshalJSON(data []byte) error {
	var list []UnionSetConfig
	if err := json.Unmarshal(data, &list); err == nil {
		*sets = list
		return nil
	}

	var mapped map[string]struct {
		Contexts  []UnionSetContextConfig `json:"contexts" yaml:"contexts"`
		Namespace string                  `json:"namespace" yaml:"namespace"`
	}
	if err := json.Unmarshal(data, &mapped); err != nil {
		return err
	}
	out := make([]UnionSetConfig, 0, len(mapped))
	names := make([]string, 0, len(mapped))
	for name := range mapped {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cfg := mapped[name]
		out = append(out, UnionSetConfig{
			Name:      name,
			Contexts:  cfg.Contexts,
			Namespace: cfg.Namespace,
		})
	}
	*sets = out
	return nil
}

// SchedulerConfig holds the runtime knobs for the priority task
// scheduler. All fields are optional; missing keys fall back to the
// scheduler package's compiled defaults.
type SchedulerConfig struct {
	WorkersPerContext int               `json:"workers_per_context" yaml:"workers_per_context"`
	CriticalReserved  int               `json:"critical_reserved_slots" yaml:"critical_reserved_slots"`
	LowReserved       *int              `json:"low_reserved_slots" yaml:"low_reserved_slots"`
	DefaultTimeout    string            `json:"default_timeout" yaml:"default_timeout"`
	TimeoutsByKind    map[string]string `json:"timeouts_by_kind" yaml:"timeouts_by_kind"`
	K8sClientQPS      int               `json:"k8s_client_qps" yaml:"k8s_client_qps"`
	K8sClientBurst    int               `json:"k8s_client_burst" yaml:"k8s_client_burst"`
	ShowPriority      *bool             `json:"show_priority_in_tasks_overlay" yaml:"show_priority_in_tasks_overlay"`
	// AgingThreshold bounds priority starvation: a background (Low) task runs
	// after at most this many higher-priority dispatches. 0 disables aging
	// (strict priority). Pointer so an explicit 0 is distinguishable from
	// "omitted". Omitted keeps the compiled default.
	AgingThreshold *int `json:"aging_threshold" yaml:"aging_threshold"`
}

// LogViewerConfig is the on-disk schema for the log_viewer section. Every
// field is a pointer so an omitted key falls through to the compiled default
// (or a deprecated flat alias) rather than overwriting it with a zero value.
type LogViewerConfig struct {
	// TailLines: lines loaded initially via --tail. Default 1000.
	TailLines *int `json:"tail_lines" yaml:"tail_lines"`
	// TailLinesShort: lines loaded by the "Tail Logs" action. Default 10.
	// Non-positive values are ignored.
	TailLinesShort *int `json:"tail_lines_short" yaml:"tail_lines_short"`
	// RenderAnsi: render ANSI SGR sequences from log producers. Default true.
	RenderAnsi *bool `json:"render_ansi" yaml:"render_ansi"`
	// ShowPreview: startup default for the structured preview panel (toggle P).
	// Default true.
	ShowPreview *bool `json:"show_preview" yaml:"show_preview"`
	// ShowPrefixes: startup default for [pod/name/container] prefixes (toggle p).
	// Default true.
	ShowPrefixes *bool `json:"show_prefixes" yaml:"show_prefixes"`
	// ShowTimestamps: startup default for log line timestamps (toggle s).
	// Default false.
	ShowTimestamps *bool `json:"show_timestamps" yaml:"show_timestamps"`
	// MaxLines: max streamed log lines retained per tab before the oldest are
	// dropped. Default 50000. Clamped to [1000, 1000000]. Bounds memory for a
	// long-running follow.
	MaxLines *int `json:"max_lines" yaml:"max_lines"`
	// PreviewLive: startup default for the right-pane live-log preview toggle
	// (runtime toggle: L). When true the explorer opens with live logs streaming
	// in the right pane for the selected pod. Default false.
	PreviewLive *bool `json:"preview_live" yaml:"preview_live"`
}

// YAMLViewerConfig is the on-disk schema for the yaml_viewer section. Every
// field is a pointer so an omitted key keeps the compiled default.
type YAMLViewerConfig struct {
	// Wrap: startup default for line wrapping (toggle: z). Default false.
	Wrap *bool `json:"wrap" yaml:"wrap"`
}

// DiffViewerConfig is the on-disk schema for the diff_viewer section.
type DiffViewerConfig struct {
	// Wrap: startup default for line wrapping (toggle: Ctrl+W / >). Default false.
	Wrap *bool `json:"wrap" yaml:"wrap"`
	// LineNumbers: startup default for the gutter line numbers (toggle: #).
	// Default true.
	LineNumbers *bool `json:"line_numbers" yaml:"line_numbers"`
	// Unified: startup default for unified (vs side-by-side) layout
	// (toggle: u). Default false.
	Unified *bool `json:"unified" yaml:"unified"`
}

// DescribeViewerConfig is the on-disk schema for the describe_viewer section.
type DescribeViewerConfig struct {
	// Wrap: startup default for line wrapping (toggle: z). Default false.
	Wrap *bool `json:"wrap" yaml:"wrap"`
}

// ObjectExplorerConfig is the on-disk schema for the object_explorer section.
type ObjectExplorerConfig struct {
	// Live: startup default for live-refreshing the browsed object as the
	// resource changes under watch mode (runtime toggle: w). Default true.
	Live *bool `json:"live" yaml:"live"`
	// Tree: startup default for the ASCII-art tree view (runtime toggle: T).
	// Default false (flat Miller-columns level list).
	Tree *bool `json:"tree" yaml:"tree"`
}

// APIExplorerConfig is the on-disk schema for the api_explorer section.
type APIExplorerConfig struct {
	// Tree: startup default for the ASCII-art tree view (runtime toggle: T).
	// Default false (flat field list).
	Tree *bool `json:"tree" yaml:"tree"`
}

// AppearanceConfig is the on-disk schema for the appearance section. Every
// field is a pointer (colorscheme/icons too) so an omitted key falls through to
// the deprecated flat alias or the compiled default rather than overwriting it.
type AppearanceConfig struct {
	Colorscheme      *string  `json:"colorscheme" yaml:"colorscheme"`
	Icons            *string  `json:"icons" yaml:"icons"`
	NoColor          *bool    `json:"no_color" yaml:"no_color"`
	TransparentBg    *bool    `json:"transparent_background" yaml:"transparent_background"`
	MinContrastRatio *float64 `json:"min_contrast_ratio" yaml:"min_contrast_ratio"`
	DimOverlay       *bool    `json:"dim_overlay" yaml:"dim_overlay"`
}

// mergeAppearanceConfig folds a present appearance group down onto the flat
// appearance fields so all downstream apply/theme logic can keep reading the
// flat fields unchanged. The group wins over a flat alias when both are set.
func mergeAppearanceConfig(cfg configFile) configFile {
	a := cfg.Appearance
	if a == nil {
		return cfg
	}
	if a.Colorscheme != nil {
		cfg.Colorscheme = *a.Colorscheme
	}
	if a.Icons != nil {
		cfg.Icons = *a.Icons
	}
	if a.NoColor != nil {
		cfg.NoColor = a.NoColor
	}
	if a.TransparentBg != nil {
		cfg.TransparentBg = a.TransparentBg
	}
	if a.MinContrastRatio != nil {
		cfg.MinContrastRatio = a.MinContrastRatio
	}
	if a.DimOverlay != nil {
		cfg.DimOverlay = a.DimOverlay
	}
	return cfg
}

// EventsConfig is the on-disk schema for the events section.
type EventsConfig struct {
	// WarningsOnly: start the events view filtered to Warning events only
	// (runtime toggle from the events view). Default true.
	WarningsOnly *bool `json:"warnings_only" yaml:"warnings_only"`
	// Grouping: start with related events grouped by reason. Default true.
	Grouping *bool `json:"grouping" yaml:"grouping"`
}

// KubesharkConfig is the on-disk schema for the kubeshark section.
type KubesharkConfig struct {
	// Namespace where Service kubeshark-hub lives. Empty / unset falls
	// back to DefaultKubesharkNamespace ("kubeshark").
	Namespace string `json:"namespace" yaml:"namespace"`
}

// UnionSetConfig is the on-disk schema for one entry under union_sets.
type UnionSetConfig struct {
	// Name is the identifier used by --union-set to reference this entry.
	// Must be unique across UnionSets; duplicates are dropped at apply
	// time (last wins) with a startup warning.
	Name string `json:"name" yaml:"name"`
	// Contexts is the list of cluster entries to merge in this union view.
	// Subject to the same MaxUnionContexts cap and existence check as
	// repeated --union-context flags. Each entry carries the kubeconfig
	// context name plus an optional per-cluster color used to paint the
	// 1-cell row tile in the merged view.
	Contexts []UnionSetContextConfig `json:"contexts" yaml:"contexts"`
	// Namespace is the namespace lfk opens in when this set is selected.
	// Optional: when empty, lfk can still use a member entry namespace or
	// an explicit kubeconfig context namespace. When set, --namespace on
	// the CLI overrides this value.
	Namespace string `json:"namespace" yaml:"namespace"`
}

// UnionSetContextConfig identifies one cluster within a union set, plus
// optional per-set metadata used when this named view is activated.
// The color lives inside the set rather than the global cluster_colors map
// so users can pick deliberate "traffic light" semantics per view (e.g. the
// canary is green in this set, the prod-blue marker stays blue) without
// affecting the cluster picker's global per-context tints.
//
// The color name must be one of ui.ClusterColorNames; invalid values are
// dropped at sanitize time with a warning, leaving the entry usable but
// untinted (the row gets a blank reserved cell instead of a colored tile).
type UnionSetContextConfig struct {
	Context   string `json:"context" yaml:"context"`
	Color     string `json:"color"   yaml:"color"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

func (c *UnionSetContextConfig) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		c.Context = name
		c.Color = ""
		c.Namespace = ""
		return nil
	}
	var obj struct {
		Context   string `json:"context" yaml:"context"`
		Name      string `json:"name" yaml:"name"`
		Color     string `json:"color" yaml:"color"`
		Namespace string `json:"namespace" yaml:"namespace"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	c.Context = obj.Context
	if c.Context == "" {
		c.Context = obj.Name
	}
	c.Color = obj.Color
	c.Namespace = obj.Namespace
	return nil
}

// clusterConfig holds per-cluster configuration overrides.
type clusterConfig struct {
	ResourceColumns map[string][]string   `json:"resource_columns" yaml:"resource_columns"`
	Views           map[string]configView `json:"views" yaml:"views"`
	// ReadOnly, when set, overrides the global read_only setting for this
	// context only. Useful for marking specific clusters (e.g. "prod") as
	// read-only while leaving others mutable.
	ReadOnly *bool `json:"read_only" yaml:"read_only"`
	// Security, when set, overrides the global security settings for this
	// context only — e.g. disabling the dashboard on a cluster where the
	// kubeconfig credential plugin is noisy, or enabling only specific
	// sources per cluster.
	Security *securityConfig `json:"security" yaml:"security"`
	// K8sClientQPS / K8sClientBurst override the foreground REST client rate
	// for this context only — e.g. a higher ceiling on a big cluster, or a
	// lower one on a shared/throttled API server. Omitted = use the global.
	K8sClientQPS   *int `json:"k8s_client_qps" yaml:"k8s_client_qps"`
	K8sClientBurst *int `json:"k8s_client_burst" yaml:"k8s_client_burst"`
}

// securityConfig is the on-disk schema for the global `security` section and
// the per-cluster `clusters.<name>.security` override.
type securityConfig struct {
	// Enabled turns the whole security dashboard on or off. Defaults to true
	// (omitted = enabled).
	Enabled *bool `json:"enabled" yaml:"enabled"`
	// HideBadges sets the default for suppressing the per-resource SEC severity
	// badge. The dashboard still scans; only the inline row badge is hidden.
	// Toggleable at runtime. Settable globally and per-cluster (a per-cluster
	// value overrides the global default for that context).
	HideBadges *bool `json:"hide_badges" yaml:"hide_badges"`
	// Sources enables or disables individual sources by name. Keys accept the
	// friendly names (heuristic, advisor, rbac, trivy, kyverno, kubescape, falco, gatekeeper)
	// or the internal source ids (trivy-operator, policy-report). Any source
	// omitted from the map defaults to enabled.
	Sources map[string]bool `json:"sources" yaml:"sources"`
	// IgnorePatterns are declarative, glob-based ignore rules applied at load.
	// They complement the interactive per-cluster ignore-list (managed with
	// the action menu): config patterns are read-only and cannot be
	// un-ignored from the UI, but the show-ignored toggle still reveals them.
	// Honored ONLY in the top-level `security` section — per-cluster scoping
	// is expressed via each pattern's `cluster` glob. A per-cluster block that
	// sets ignore_patterns is ignored (with a warning).
	IgnorePatterns []SecurityIgnorePattern `json:"ignore_patterns" yaml:"ignore_patterns"`
	// Heuristic holds tuning options for the built-in heuristic source.
	// Honored ONLY in the top-level `security` section, like IgnorePatterns.
	Heuristic *heuristicConfig `json:"heuristic" yaml:"heuristic"`
}

// heuristicConfig is the on-disk schema for `security.heuristic`, tuning
// options for the built-in heuristic source's checks.
type heuristicConfig struct {
	// SecretEnvInclude / SecretEnvExclude tune the secret_env check with
	// case-insensitive env-var name globs (`*`, `?`). Include adds names to
	// flag on top of the built-in credential keywords (an explicit match
	// overrides a built-in exemption); Exclude adds names to never flag and
	// wins over Include.
	SecretEnvInclude []string `json:"secret_env_include" yaml:"secret_env_include"`
	SecretEnvExclude []string `json:"secret_env_exclude" yaml:"secret_env_exclude"`
	// ScanSecrets gates the heuristic checks that list Secret objects
	// (legacy_sa_token_secret, tls_secret_expiry). Defaults to true; set
	// false to keep the source from ever listing Secrets.
	ScanSecrets *bool `json:"scan_secrets" yaml:"scan_secrets"`
}

// SecurityIgnorePattern is a declarative ignore rule from the config file.
// Each field is a glob (`*` and `?`); an empty field matches anything. A
// finding is ignored when every non-empty field matches it. A pattern with
// every field empty is treated as a no-op (skipped) to avoid silently hiding
// all findings.
type SecurityIgnorePattern struct {
	// Cluster globs the kube-context name. Empty = any cluster.
	Cluster string `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	// Source globs the security source id (heuristic, trivy-operator,
	// policy-report, falco, ...). Empty = any source.
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	// Group globs the finding group key (CVE id, check label, rule name).
	// Empty = any group.
	Group string `json:"group,omitempty" yaml:"group,omitempty"`
	// Namespace globs the resource namespace. Empty (or `*`) = any namespace,
	// which hides the whole group; a specific value scopes the ignore to that
	// namespace only. Cluster-scoped findings have an empty namespace.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Labels match the target resource's Kubernetes labels: each entry is a
	// label key mapped to a glob value pattern, and ALL entries must match
	// (AND). A label constraint is resource-scoped — it hides matching
	// resources within a group, never the whole group — and matches when the
	// finding's resource has resolvable labels: heuristic-observed pods (plus
	// same-resource findings), and workload labels resolved from the live
	// object for standard kinds when a label pattern is set. Empty = no label
	// constraint.
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	// Comment is a free-text note explaining why the rule exists.
	Comment string `json:"comment,omitempty" yaml:"comment,omitempty"`
}

// IsEmpty reports whether the pattern has no match constraints (every glob
// field empty and no label constraints). Such a pattern would match every
// finding, so it is dropped at load and skipped at match time.
func (p SecurityIgnorePattern) IsEmpty() bool {
	return p.Cluster == "" && p.Source == "" && p.Group == "" &&
		p.Namespace == "" && len(p.Labels) == 0
}

// RightsizingDefaultsConfig is the on-disk schema for the
// rightsizing_defaults section. Both fields optional — leaving them
// out keeps the runtime fallback chain (sticky -> built-in) intact.
//
// Strategy values mirror model.AllRightsizingStrategies (string form).
// Headroom values must match one of model.RightsizingHeadrooms within
// 1e-9 epsilon; off-preset values are dropped at apply time so the
// picker never displays an out-of-list multiplier on first open.
type RightsizingDefaultsConfig struct {
	Strategy string  `json:"strategy" yaml:"strategy"`
	Headroom float64 `json:"headroom" yaml:"headroom"`
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

	cfg = mergeAppearanceConfig(cfg)
	ConfigLogPath = cfg.LogPath
	applyColorscheme(&theme, cfg)
	mergeThemeOverrides(&theme, cfg.Theme)
	MergeKeybindings(&kb, &cfg.Keybindings)
	applyGotoTargets(cfg, kb.JumpTop)
	applyConfigOptions(cfg)
	applyConfigMaps(cfg, abbr)

	ApplyTheme(theme)
	ActiveKeybindings = kb
	SearchAbbreviations = abbr
}

// loadConfigFile reads and parses the YAML config file.
// When configOverride is non-empty, it is used directly instead of the
// resolved config directory (see internal/paths).
func loadConfigFile(configOverride string) (configFile, bool) {
	var configPath string
	if configOverride != "" {
		configPath = configOverride
	} else {
		dir, err := paths.ConfigDir()
		if err != nil {
			return configFile{}, false
		}
		configPath = filepath.Join(dir, "config.yaml")
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
