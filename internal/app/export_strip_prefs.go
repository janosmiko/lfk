package app

import (
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
)

const exportStripPrefsFileName = "export_strip_prefs.yaml"

// ExportStripPrefsState lives in the state directory, not config.yaml: config is
// user-authored, this is the app recording what the user last ticked. A missing
// category falls back to its default, never to "keep everything".
type ExportStripPrefsState struct {
	Categories map[string]bool `json:"categories,omitempty" yaml:"categories,omitempty"`
}

func exportStripPrefsFilePath() string {
	return stateFilePath(exportStripPrefsFileName)
}

// loadExportStripPrefs resolves the category set for a new export. Nothing here
// is ever fatal: a user-visible on-disk schema has to survive being edited by
// hand, and the fallback is the default export.
func loadExportStripPrefs() k8s.TemplateStripSet {
	set := k8s.DefaultTemplateStripSet()
	s := loadStateFile[ExportStripPrefsState](exportStripPrefsFileName)
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
	categories := make(map[string]bool, len(k8s.TemplateCategories))
	for _, cat := range k8s.TemplateCategories {
		categories[string(cat)] = set[cat]
	}
	if err := saveStateFile(exportStripPrefsFileName, ExportStripPrefsState{Categories: categories}); err != nil {
		logger.Error("Failed to persist export strip prefs", "error", err)
	}
}
