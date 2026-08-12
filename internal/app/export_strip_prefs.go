package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// ExportStripPrefsState lives in the state directory, not config.yaml: config is
// user-authored, this is the app recording what the user last ticked. A missing
// category falls back to its default, never to "keep everything".
type ExportStripPrefsState struct {
	Categories map[string]bool `json:"categories,omitempty" yaml:"categories,omitempty"`
}

func exportStripPrefsFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "export_strip_prefs.yaml")
}

// loadExportStripPrefs resolves the category set for a new export. Nothing here
// is ever fatal: a user-visible on-disk schema has to survive being edited by
// hand, and the fallback is the default export.
func loadExportStripPrefs() k8s.TemplateStripSet {
	set := k8s.DefaultTemplateStripSet()
	path := exportStripPrefsFilePath()
	if path == "" {
		return set
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read export strip prefs", "error", err, "path", path)
		}
		return set
	}
	var s ExportStripPrefsState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Export strip prefs file is corrupt; ignoring", "error", err, "path", path)
		return set
	}
	for _, cat := range k8s.TemplateCategories {
		if v, ok := s.Categories[string(cat)]; ok {
			set[cat] = v
		}
	}
	return set
}

// saveExportStripPrefs records the tick state. Best-effort: losing a UI
// preference is never worth failing a keypress over. Called from the single
// Bubble Tea Update goroutine.
func saveExportStripPrefs(set k8s.TemplateStripSet) {
	path := exportStripPrefsFilePath()
	if path == "" {
		return
	}
	categories := make(map[string]bool, len(k8s.TemplateCategories))
	for _, cat := range k8s.TemplateCategories {
		categories[string(cat)] = set[cat]
	}
	data, err := yaml.Marshal(ExportStripPrefsState{Categories: categories})
	if err != nil {
		logger.Error("Failed to encode export strip prefs", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		logger.Error("Failed to create export strip prefs directory", "error", err, "path", path)
		return
	}
	if err := writeFileDurable(path, data); err != nil {
		logger.Error("Failed to persist export strip prefs", "error", err, "path", path)
	}
}
