package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
	"github.com/janosmiko/lfk/internal/ui"
)

// ViewerPrefsState records the fullscreen-viewer toggles the user last pressed.
// It lives in the state directory, not config.yaml: config.yaml is user-authored
// and seeds each default, this file is the app remembering a keypress. A nil
// field means the user never toggled that setting, so config.yaml still wins for
// it — that is why every field is a pointer and not a plain bool.
type ViewerPrefsState struct {
	LogShowPreview        *bool `json:"log_show_preview,omitempty" yaml:"log_show_preview,omitempty"`
	LogShowPrefixes       *bool `json:"log_show_prefixes,omitempty" yaml:"log_show_prefixes,omitempty"`
	LogShowTimestamps     *bool `json:"log_show_timestamps,omitempty" yaml:"log_show_timestamps,omitempty"`
	LogPreviewLive        *bool `json:"log_preview_live,omitempty" yaml:"log_preview_live,omitempty"`
	LogWrap               *bool `json:"log_wrap,omitempty" yaml:"log_wrap,omitempty"`
	YAMLViewerWrap        *bool `json:"yaml_viewer_wrap,omitempty" yaml:"yaml_viewer_wrap,omitempty"`
	DiffViewerWrap        *bool `json:"diff_viewer_wrap,omitempty" yaml:"diff_viewer_wrap,omitempty"`
	DiffViewerLineNumbers *bool `json:"diff_viewer_line_numbers,omitempty" yaml:"diff_viewer_line_numbers,omitempty"`
	DiffViewerUnified     *bool `json:"diff_viewer_unified,omitempty" yaml:"diff_viewer_unified,omitempty"`
	DescribeViewerWrap    *bool `json:"describe_viewer_wrap,omitempty" yaml:"describe_viewer_wrap,omitempty"`
	ObjectExplorerLive    *bool `json:"object_explorer_live,omitempty" yaml:"object_explorer_live,omitempty"`
	ObjectExplorerTree    *bool `json:"object_explorer_tree,omitempty" yaml:"object_explorer_tree,omitempty"`
	APIExplorerTree       *bool `json:"api_explorer_tree,omitempty" yaml:"api_explorer_tree,omitempty"`
	// MetricsSparkWindow is the chosen sparkline window as a duration, or the
	// empty string for numeric. It is not a bool, so it sits outside
	// viewerPrefBindings. A pointer to "" still marshals, which is what keeps
	// explicit numeric distinct from never chosen.
	MetricsSparkWindow *string `json:"metrics_spark_window,omitempty" yaml:"metrics_spark_window,omitempty"`
}

// viewerPref names one persisted toggle. It indexes both viewerPrefBindings and
// viewerPrefValues, so a caller cannot name a toggle the table does not know.
type viewerPref int

const (
	prefLogShowPreview viewerPref = iota
	prefLogShowPrefixes
	prefLogShowTimestamps
	prefLogPreviewLive
	prefLogWrap
	prefYAMLViewerWrap
	prefDiffViewerWrap
	prefDiffViewerLineNumbers
	prefDiffViewerUnified
	prefDescribeViewerWrap
	prefObjectExplorerLive
	prefObjectExplorerTree
	prefAPIExplorerTree

	// numViewerPrefs sizes viewerPrefValues. Keep it last.
	numViewerPrefs
)

// viewerPrefBinding ties one persisted field to the ui global it seeds.
type viewerPrefBinding struct {
	global *bool
	field  func(*ViewerPrefsState) **bool
}

// viewerPrefBindings is indexed by viewerPref. ApplyViewerPrefs is the only
// writer of these globals, and it runs once at startup, so the viewer seed
// sites that read them stay race-free and stay honest for tests that pin one.
var viewerPrefBindings = []viewerPrefBinding{
	{&ui.ConfigLogShowPreview, func(s *ViewerPrefsState) **bool { return &s.LogShowPreview }},
	{&ui.ConfigLogShowPrefixes, func(s *ViewerPrefsState) **bool { return &s.LogShowPrefixes }},
	{&ui.ConfigLogShowTimestamps, func(s *ViewerPrefsState) **bool { return &s.LogShowTimestamps }},
	{&ui.ConfigLogPreviewLive, func(s *ViewerPrefsState) **bool { return &s.LogPreviewLive }},
	{&ui.ConfigLogWrap, func(s *ViewerPrefsState) **bool { return &s.LogWrap }},
	{&ui.ConfigYAMLViewerWrap, func(s *ViewerPrefsState) **bool { return &s.YAMLViewerWrap }},
	{&ui.ConfigDiffViewerWrap, func(s *ViewerPrefsState) **bool { return &s.DiffViewerWrap }},
	{&ui.ConfigDiffViewerLineNumbers, func(s *ViewerPrefsState) **bool { return &s.DiffViewerLineNumbers }},
	{&ui.ConfigDiffViewerUnified, func(s *ViewerPrefsState) **bool { return &s.DiffViewerUnified }},
	{&ui.ConfigDescribeViewerWrap, func(s *ViewerPrefsState) **bool { return &s.DescribeViewerWrap }},
	{&ui.ConfigObjectExplorerLive, func(s *ViewerPrefsState) **bool { return &s.ObjectExplorerLive }},
	{&ui.ConfigObjectExplorerTree, func(s *ViewerPrefsState) **bool { return &s.ObjectExplorerTree }},
	{&ui.ConfigAPIExplorerTree, func(s *ViewerPrefsState) **bool { return &s.APIExplorerTree }},
}

// viewerPrefValues is the live set of fullscreen-viewer toggles for one
// session, indexed by viewerPref. It lives on the Model rather than in the
// ui.Config* globals so a keypress in one viewer cannot reach across into an
// unrelated one, and so the test suite stays order-independent.
type viewerPrefValues [numViewerPrefs]bool

// newViewerPrefValues snapshots the startup seeds. ApplyViewerPrefs has already
// folded the persisted file into them by the time NewModel calls this.
func newViewerPrefValues() viewerPrefValues {
	var v viewerPrefValues
	for i, b := range viewerPrefBindings {
		v[i] = *b.global
	}
	return v
}

func viewerPrefsFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "viewer_prefs.yaml")
}

// loadViewerPrefsState reads the file, returning an all-nil state when it is
// missing or corrupt. Nothing here is fatal: a hand-edited state file must never
// stop the app from starting, and an all-nil state means "use config.yaml".
func loadViewerPrefsState() ViewerPrefsState {
	var s ViewerPrefsState
	path := viewerPrefsFilePath()
	if path == "" {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read viewer prefs", "error", err, "path", path)
		}
		return s
	}
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Viewer prefs file is corrupt; ignoring", "error", err, "path", path)
		return ViewerPrefsState{}
	}
	return s
}

// ApplyViewerPrefs overlays the persisted toggles on the config defaults.
// Call it once at startup, right after ui.LoadConfig and before the model seeds
// a viewer from a ui.Config* global. It stays out of NewModel on purpose: a test
// that pins a global must be able to trust that pin.
func ApplyViewerPrefs() {
	s := loadViewerPrefsState()
	for _, b := range viewerPrefBindings {
		if v := *b.field(&s); v != nil {
			*b.global = *v
		}
	}
	if s.MetricsSparkWindow != nil {
		ui.MetricsSparkStartupState = ui.ResolveMetricsSparkState(*s.MetricsSparkWindow)
	}
}

// persistMetricsSparkPref records the display mode for the next start. It
// stores the window DURATION rather than an index, so a shortened
// metrics_sparkline_windows falls back to numeric instead of selecting a
// different window. Same locked read-modify-write as persistViewerPref: two lfk
// instances share a state directory, and an unlocked merge lets the later
// writer drop what the other recorded.
func persistMetricsSparkPref(state ui.MetricsSparkState) {
	path := viewerPrefsFilePath()
	if path == "" {
		return
	}
	withStateFileLock(path, func() {
		s := loadViewerPrefsState()
		w := state.PersistedWindow()
		s.MetricsSparkWindow = &w
		saveViewerPrefs(s)
	})
}

// saveViewerPrefs writes the state file. Best effort: losing a UI preference is
// never worth failing a keypress over.
func saveViewerPrefs(s ViewerPrefsState) {
	path := viewerPrefsFilePath()
	if path == "" {
		return
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		logger.Error("Failed to encode viewer prefs", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		logger.Error("Failed to create viewer prefs directory", "error", err, "path", path)
		return
	}
	if err := writeFileDurable(path, data); err != nil {
		logger.Error("Failed to persist viewer prefs", "error", err, "path", path)
	}
}

// setViewerPref records a toggle for the rest of the session and for the next
// start. Every viewer toggle handler goes through it, so the two halves cannot
// drift apart.
func (m *Model) setViewerPref(p viewerPref, v bool) {
	m.viewerPrefs[p] = v
	persistViewerPref(p, v)
}

// persistViewerPref records one runtime toggle so the next start reopens the
// viewer with it. It merges into the file on disk, so recording one toggle
// never drops another. It deliberately leaves the ui global alone: the global
// is the startup seed, and rewriting it here would make one viewer's keypress
// change what an unrelated viewer opens with later in the same session.
//
// The load and the save sit inside one interprocess lock. Two lfk instances
// share a state directory, so an unlocked merge lets the later writer drop a
// toggle the other instance recorded between its read and its write.
func persistViewerPref(p viewerPref, v bool) {
	path := viewerPrefsFilePath()
	if path == "" {
		return
	}
	withStateFileLock(path, func() {
		s := loadViewerPrefsState()
		*viewerPrefBindings[p].field(&s) = &v
		saveViewerPrefs(s)
	})
}
