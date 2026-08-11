// Package app — user_templates.go
// The user's own resource templates, saved from a live object by the Export
// Template action.
//
// Shape on disk: one YAML file, DataDir()/templates.yaml, holding a list of
// {name, description, yaml}. A directory of loose manifests would be more
// hand-editable, but each template also carries a name and a description that
// a bare manifest has nowhere to put — and one file is one atomic rename.
// A single marshalled list is also what every other lfk store does
// (bookmarks.yaml, pinned.yaml).
//
// DataDir rather than StateDir: these are authored documents the user would
// want to keep and copy between machines, not derived runtime state.
package app

import (
	"os"
	"path/filepath"
	"sort"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/paths"
	"github.com/janosmiko/lfk/internal/ui"
)

// userTemplateCategory is the Category shown in the picker for every saved
// template. It is what distinguishes a user template from a built-in of the
// same name — see mergedTemplates.
const userTemplateCategory = "User"

// storedTemplate is the on-disk record. Category is not stored: every entry in
// this file is a user template by construction.
type storedTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	YAML        string `json:"yaml"`
}

func userTemplatesPath() string {
	dir, err := paths.DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "templates.yaml")
}

// loadUserTemplates reads the saved templates, sorted by name. A missing or
// unparseable file yields nil so the picker falls back to the built-ins rather
// than failing to open.
func loadUserTemplates() []model.ResourceTemplate {
	path := userTemplatesPath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read user templates", "error", err, "path", path)
		}
		return nil
	}
	var stored []storedTemplate
	if err := yaml.Unmarshal(data, &stored); err != nil {
		logger.Warn("User templates file is corrupt; ignoring", "error", err, "path", path)
		return nil
	}
	out := make([]model.ResourceTemplate, 0, len(stored))
	for _, s := range stored {
		if s.Name == "" {
			continue
		}
		out = append(out, model.ResourceTemplate{
			Name:        s.Name,
			Description: s.Description,
			Category:    userTemplateCategory,
			YAML:        s.YAML,
		})
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// saveUserTemplate writes tmpl to the user template file, replacing any entry
// with the same name. The name and description come from cluster-sourced text,
// so both are stripped of terminal control sequences before they reach disk —
// the picker renders them.
func saveUserTemplate(tmpl model.ResourceTemplate) error {
	path := userTemplatesPath()
	if path == "" {
		return os.ErrNotExist
	}
	name := ui.SanitizeTerminalText(tmpl.Name)
	if name == "" {
		return os.ErrInvalid
	}

	existing := loadUserTemplates()
	stored := make([]storedTemplate, 0, len(existing)+1)
	for _, t := range existing {
		if t.Name == name {
			continue
		}
		stored = append(stored, storedTemplate{Name: t.Name, Description: t.Description, YAML: t.YAML})
	}
	stored = append(stored, storedTemplate{
		Name:        name,
		Description: ui.SanitizeTerminalText(tmpl.Description),
		YAML:        tmpl.YAML,
	})
	sort.Slice(stored, func(i, j int) bool { return stored[i].Name < stored[j].Name })

	data, err := yaml.Marshal(stored)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFileDurable(path, data)
}

// mergedTemplates is what the template picker lists: the user's templates
// first, then the built-ins.
//
// Name collisions are not resolved — both rows stay. A user template never
// replaces a built-in, and the picker's Category column ("User" against
// "Workloads", "Networking", …) is what tells the two rows apart.
func mergedTemplates() []model.ResourceTemplate {
	user := loadUserTemplates()
	builtin := model.BuiltinTemplates()
	out := make([]model.ResourceTemplate, 0, len(user)+len(builtin))
	out = append(out, user...)
	return append(out, builtin...)
}
