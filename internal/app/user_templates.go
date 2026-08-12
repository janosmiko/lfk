// Package app — user_templates.go
// The user's own resource templates: one YAML file per template in
// <ConfigDir>/templates/, listed in the picker alongside BuiltinTemplates().
//
// The directory is a configuration surface, not application state: users hand
// author these files, keep them in dotfiles, and edit them outside lfk. That is
// why it lives under ConfigDir and why each file is a plain Kubernetes manifest
// rather than a record in a combined list — adding a template is `cp` and
// removing one is `rm`, with no envelope to re-indent into.
//
// The Export Template action writes into this same directory, so there is one
// place to look for a template regardless of where it came from.
//
// The file name is the template name. A file that does not parse as a YAML
// mapping is skipped and logged: a hand-edited directory will contain a broken
// file sooner or later, and one broken file must not empty the picker.
package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/paths"
	"github.com/janosmiko/lfk/internal/ui"
)

// userTemplateCategory is the Category every template in the directory gets.
// Hand-authored files carry no category of their own, and this one value is
// what distinguishes a user template from a built-in of the same name — see
// mergedTemplates.
const userTemplateCategory = "User"

// errInvalidTemplateName rejects a name that would write outside the template
// directory or produce an unreachable file.
var errInvalidTemplateName = errors.New("invalid template name")

// errTemplateNotFound reports that the picker offered a name with no file
// behind it, which means the directory changed under a stale list.
var errTemplateNotFound = errors.New("template not found")

// errTemplateDirUnavailable reports that paths.ConfigDir() failed — an
// environment problem (e.g. $HOME unset), not anything wrong with the
// resource name being saved.
var errTemplateDirUnavailable = errors.New("template directory unavailable")

// clusterScopedSegment stands in for a namespace on a cluster-scoped save.
// The cluster context is deliberately left out of the file name: EKS
// contexts are ARNs containing ":" and "/".
const clusterScopedSegment = "_cluster"

func userTemplateDir() string {
	dir, err := paths.ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "templates")
}

// loadUserTemplates reads every *.yaml / *.yml file in the template directory.
// os.ReadDir returns entries sorted by file name, and the file name is the
// template name, so the picker order matches what the user sees in the
// directory. A missing directory is the normal case and yields nil.
func loadUserTemplates() []model.ResourceTemplate {
	dir := userTemplateDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read template directory", "error", err, "path", dir)
		}
		return nil
	}
	var out []model.ResourceTemplate
	for _, e := range entries {
		if e.IsDir() || !isYAMLFile(e.Name()) {
			continue
		}
		tmpl, ok := readUserTemplate(filepath.Join(dir, e.Name()))
		if !ok {
			continue
		}
		out = append(out, tmpl)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// readUserTemplate turns one file into a template. The name comes from the file
// name and the description from the manifest's kind. Both reach the picker as
// rendered text and both are user-supplied, so both are sanitized here.
func readUserTemplate(path string) (model.ResourceTemplate, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // path is an entry of the user's own template dir
	if err != nil {
		logger.Warn("Skipping unreadable template", "error", err, "path", path)
		return model.ResourceTemplate{}, false
	}
	// Decode only the first document so a multi-document template still yields
	// a kind for the description.
	var obj map[string]any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		if !decodeFirstDocument(data, &obj) {
			logger.Warn("Skipping template that is not valid YAML", "error", err, "path", path)
			return model.ResourceTemplate{}, false
		}
	}
	if len(obj) == 0 {
		logger.Warn("Skipping template that is not a Kubernetes object", "path", path)
		return model.ResourceTemplate{}, false
	}
	name := ui.SanitizeTerminalText(templateNameFromPath(path))
	if name == "" {
		return model.ResourceTemplate{}, false
	}
	namespace := ui.SanitizeTerminalText(templateNamespaceFromPath(path))
	kind, _ := obj["kind"].(string)
	return model.ResourceTemplate{
		Name:        name,
		Description: templateDescription(ui.SanitizeTerminalText(kind), namespace),
		Category:    userTemplateCategory,
		YAML:        string(data),
		Path:        path,
	}, true
}

// decodeFirstDocument retries a multi-document file, where a whole-file
// Unmarshal fails on the second `---`.
func decodeFirstDocument(data []byte, obj *map[string]any) bool {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	return dec.Decode(obj) == nil
}

func templateBaseFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// templateNameFromPath maps a file to the name the picker shows for it. "_"
// is illegal in a DNS-1123 object name or namespace, so splitting on the
// LAST "__" is unambiguous. A basename with none is unchanged.
func templateNameFromPath(path string) string {
	base := templateBaseFromPath(path)
	if idx := strings.LastIndex(base, "__"); idx >= 0 {
		return base[idx+2:]
	}
	return base
}

// templateNamespaceFromPath recovers the namespace saveUserTemplate encoded
// into the file name, or "" for a hand-authored file or a cluster-scoped save.
func templateNamespaceFromPath(path string) string {
	base := templateBaseFromPath(path)
	idx := strings.LastIndex(base, "__")
	if idx < 0 {
		return ""
	}
	if ns := base[:idx]; ns != clusterScopedSegment {
		return ns
	}
	return ""
}

// templateDescription matches the "<namespace>/<name>" order used elsewhere
// in the app (e.g. explorer.go) for the picker's Description column.
func templateDescription(kind, namespace string) string {
	if namespace == "" {
		return kind
	}
	return namespace + "/" + kind
}

func isYAMLFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".yaml", ".yml":
		return true
	}
	return false
}

// templateFileBase sanitizes namespace and name before they reach the
// filesystem: both are resource identifiers read from the cluster.
func templateFileBase(namespace, name string) (string, bool) {
	cleanName := ui.SanitizeTerminalText(name)
	if !isSafeTemplateName(cleanName) {
		return "", false
	}
	nsSegment := clusterScopedSegment
	if namespace != "" {
		nsSegment = ui.SanitizeTerminalText(namespace)
		if !isSafeTemplateName(nsSegment) {
			return "", false
		}
	}
	return nsSegment + "__" + cleanName, true
}

// templateSavePath resolves where saveUserTemplate would write namespace and
// name, without touching the filesystem — the export picker checks this path
// for an existing file before it commits to an overwrite.
func templateSavePath(namespace, name string) (string, bool) {
	dir := userTemplateDir()
	if dir == "" {
		return "", false
	}
	base, ok := templateFileBase(namespace, name)
	if !ok {
		return "", false
	}
	return filepath.Join(dir, base+".yaml"), true
}

// saveUserTemplate writes manifest to <template dir>/<namespace>__<name>.yaml
// (or _cluster__<name>.yaml for a cluster-scoped resource), replacing any
// file already at that path.
func saveUserTemplate(namespace, name, manifest string) error {
	dir := userTemplateDir()
	if dir == "" {
		return errTemplateDirUnavailable
	}
	base, ok := templateFileBase(namespace, name)
	if !ok {
		return errInvalidTemplateName
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return writeFileDurable(filepath.Join(dir, base+".yaml"), []byte(manifest))
}

// deleteUserTemplate removes one template file by path. Deleting by name would
// take web.yaml and web.yml together, and the picker lists those as two rows.
// The path arrives through Model state, so the directory is re-checked here.
func deleteUserTemplate(path string) error {
	dir := userTemplateDir()
	if dir == "" {
		return errTemplateDirUnavailable
	}
	if path == "" || filepath.Dir(path) != dir || !isSafeTemplateName(filepath.Base(path)) {
		return errInvalidTemplateName
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return errTemplateNotFound
		}
		return err
	}
	return nil
}

// isSafeTemplateName reports whether name is a single path element that stays
// inside the template directory.
func isSafeTemplateName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, os.PathSeparator) {
		return false
	}
	return name == filepath.Base(name)
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
