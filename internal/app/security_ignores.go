package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// SecurityIgnoreRule represents a single ignore entry. Scope is determined by
// which of Namespace / Resource are set, in order of increasing specificity:
//
//	Namespace == "" && Resource == "" -> whole group, cluster-wide
//	Namespace != "" && Resource == "" -> group within one namespace
//	Resource  != ""                   -> one specific resource (ns encoded in key)
type SecurityIgnoreRule struct {
	Source    string `json:"source" yaml:"source"`                           // Security source name: "heuristic", "trivy-operator", "falco", "policy-report"
	GroupKey  string `json:"group_key" yaml:"group_key"`                     // Finding group key (check label, CVE ID, rule name)
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"` // Namespace scope. Empty = any namespace. Ignored when Resource is set.
	Resource  string `json:"resource,omitempty" yaml:"resource,omitempty"`   // ResourceRef.Key() format: "ns/kind/name". Empty = no resource scope.
	Comment   string `json:"comment,omitempty" yaml:"comment,omitempty"`
	CreatedAt string `json:"created_at" yaml:"created_at"` // RFC3339
}

// SecurityIgnoreState holds ignore rules per cluster context.
type SecurityIgnoreState struct {
	Contexts map[string][]SecurityIgnoreRule `json:"contexts" yaml:"contexts"`
}

// securityIgnoresFilePath returns the path to the security ignores file.
// Uses $XDG_STATE_HOME/lfk/ (defaults to ~/.local/state/lfk/) per XDG specification.
func securityIgnoresFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "security_ignores.yaml")
}

// loadSecurityIgnores reads ignore rules from the YAML file on disk.
// Returns an empty state (never nil) if the file is missing or corrupt.
func loadSecurityIgnores() *SecurityIgnoreState {
	path := securityIgnoresFilePath()
	if path == "" {
		return &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	}

	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	}

	var state SecurityIgnoreState
	if err := yaml.Unmarshal(data, &state); err != nil {
		logger.Info("Failed to parse security ignores file", "path", path, "error", err)
		return &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	}

	if state.Contexts == nil {
		state.Contexts = make(map[string][]SecurityIgnoreRule)
	}

	return &state
}

// saveSecurityIgnores writes ignore rules to the YAML file on disk using an
// atomic write (write to temp file, fsync, then rename) to prevent data loss
// if the process is interrupted mid-write.
func saveSecurityIgnores(state *SecurityIgnoreState) error {
	path := securityIgnoresFilePath()
	if path == "" {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}

	// Atomic write: write to a temp file in the same directory, fsync, then rename.
	tmp, err := os.CreateTemp(dir, ".security_ignores-*.yaml.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	// Fsync to ensure data is flushed to stable storage before rename.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	// Fsync the parent directory so the rename itself is durable; without
	// this, a crash immediately after rename can lose the new directory
	// entry even though the file contents are already on stable storage.
	dirFd, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := dirFd.Sync()
	closeErr := dirFd.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// saveSecurityIgnoresCmd wraps saveSecurityIgnores in a tea.Cmd so the
// fsync at the end of the atomic write does not block the Update goroutine
// — on slow disks (HDDs, networked filesystems) that fsync can stall the
// UI for hundreds of milliseconds. The Cmd emits a securityIgnoresSaveErrMsg
// only on failure; successful saves return nil so the runtime ignores them
// and the optimistic status set at action time stays visible.
func saveSecurityIgnoresCmd(state *SecurityIgnoreState) tea.Cmd {
	return func() tea.Msg {
		if err := saveSecurityIgnores(state); err != nil {
			return securityIgnoresSaveErrMsg{err: err}
		}
		return nil
	}
}

// addSecurityIgnore returns a NEW state with the rule added for the given context.
// Deduplicates by (Source, GroupKey, Resource). Sets CreatedAt if empty.
func addSecurityIgnore(state *SecurityIgnoreState, ctx string, rule SecurityIgnoreRule) *SecurityIgnoreState {
	if rule.CreatedAt == "" {
		rule.CreatedAt = time.Now().Format(time.RFC3339)
	}

	// Deep copy contexts map.
	newContexts := make(map[string][]SecurityIgnoreRule, len(state.Contexts))
	for k, v := range state.Contexts {
		copied := make([]SecurityIgnoreRule, len(v))
		copy(copied, v)
		newContexts[k] = copied
	}

	existing := newContexts[ctx]

	// Deduplicate: replace if same (Source, GroupKey, Namespace, Resource)
	// already exists.
	for i, r := range existing {
		if r.Source == rule.Source && r.GroupKey == rule.GroupKey &&
			r.Namespace == rule.Namespace && r.Resource == rule.Resource {
			existing[i] = rule
			newContexts[ctx] = existing
			return &SecurityIgnoreState{Contexts: newContexts}
		}
	}

	newContexts[ctx] = append(existing, rule)

	return &SecurityIgnoreState{Contexts: newContexts}
}

// removeSecurityIgnore returns a NEW state with the rule matching
// (source, groupKey, namespace, resource) removed.
func removeSecurityIgnore(state *SecurityIgnoreState, ctx, source, groupKey, namespace, resource string) *SecurityIgnoreState {
	newContexts := make(map[string][]SecurityIgnoreRule, len(state.Contexts))
	for k, v := range state.Contexts {
		copied := make([]SecurityIgnoreRule, len(v))
		copy(copied, v)
		newContexts[k] = copied
	}

	existing := newContexts[ctx]
	filtered := make([]SecurityIgnoreRule, 0, len(existing))
	for _, r := range existing {
		if r.Source == source && r.GroupKey == groupKey &&
			r.Namespace == namespace && r.Resource == resource {
			continue
		}
		filtered = append(filtered, r)
	}
	newContexts[ctx] = filtered

	return &SecurityIgnoreState{Contexts: newContexts}
}

// namespaceFromResourceKey extracts the namespace from a ResourceRef.Key()
// ("ns/kind/name"). Cluster-scoped findings have an empty namespace segment.
func namespaceFromResourceKey(resourceKey string) string {
	ns, _, _ := strings.Cut(resourceKey, "/")
	return ns
}

// isGroupIgnored returns true only when the whole group is ignored cluster-wide
// (a rule with neither a Namespace nor a Resource scope) for the given source
// and group key. Namespace- and resource-scoped rules do NOT make the whole
// group ignored, so the group row stays visible.
func isGroupIgnored(state *SecurityIgnoreState, ctx, source, groupKey string) bool {
	for _, r := range state.Contexts[ctx] {
		if r.Source == source && r.GroupKey == groupKey && r.Resource == "" && r.Namespace == "" {
			return true
		}
	}
	return false
}

// isNamespaceIgnored returns true if there is a namespace-scoped ignore rule
// (Namespace set, Resource empty) for the given source/group/namespace.
func isNamespaceIgnored(state *SecurityIgnoreState, ctx, source, groupKey, namespace string) bool {
	if namespace == "" {
		return false
	}
	for _, r := range state.Contexts[ctx] {
		if r.Source == source && r.GroupKey == groupKey && r.Resource == "" && r.Namespace == namespace {
			return true
		}
	}
	return false
}

// isResourceIgnored returns true when a finding on resourceKey is hidden by ANY
// matching rule, checked from least to most specific: a cluster-wide group
// ignore, a namespace-scoped ignore for the resource's namespace, or a
// resource-specific ignore. resourceKey is in ResourceRef.Key() form.
func isResourceIgnored(state *SecurityIgnoreState, ctx, source, groupKey, resourceKey string) bool {
	ns := namespaceFromResourceKey(resourceKey)
	for _, r := range state.Contexts[ctx] {
		if r.Source != source || r.GroupKey != groupKey {
			continue
		}
		switch {
		case r.Resource != "":
			if r.Resource == resourceKey {
				return true
			}
		case r.Namespace != "":
			if r.Namespace == ns {
				return true
			}
		default: // neither scope set -> cluster-wide group ignore
			return true
		}
	}
	return false
}

// isResourceSpecificIgnored reports whether there is a resource-level ignore
// rule (non-empty Resource) for the given source/groupKey/resourceKey. Used
// by the Un-ignore action to decide whether to drop the per-resource rule
// or fall back to the group-level rule.
func isResourceSpecificIgnored(state *SecurityIgnoreState, ctx, source, groupKey, resourceKey string) bool {
	for _, r := range state.Contexts[ctx] {
		if r.Source == source && r.GroupKey == groupKey && r.Resource == resourceKey && r.Resource != "" {
			return true
		}
	}
	return false
}

// countIgnoredGroups returns the count of cluster-wide group ignores (rules
// with neither a namespace nor a resource scope) for the given context.
func countIgnoredGroups(state *SecurityIgnoreState, ctx string) int {
	count := 0
	for _, r := range state.Contexts[ctx] {
		if r.Resource == "" && r.Namespace == "" {
			count++
		}
	}
	return count
}

// modelIgnoreChecker adapts SecurityIgnoreState to the k8s.IgnoreChecker
// interface so the groupFindings engine can filter ignored entries. The
// interface is defined in the k8s package; Go structural typing allows this
// app-layer type to satisfy it without importing k8s. It combines two
// sources: the interactive per-cluster ignore-list (state) and the
// declarative config-file glob patterns (patterns), snapshotted at
// construction. The interactive action menu reads the state functions
// directly, so config patterns never surface a misleading "Un-ignore".
type modelIgnoreChecker struct {
	state    *SecurityIgnoreState
	ctx      string
	patterns []ui.SecurityIgnorePattern
}

// newModelIgnoreChecker builds a checker for one cluster context, capturing a
// snapshot of the config ignore patterns (read-only after load).
func newModelIgnoreChecker(state *SecurityIgnoreState, ctx string) *modelIgnoreChecker {
	return &modelIgnoreChecker{
		state:    state,
		ctx:      ctx,
		patterns: ui.ConfigSecurityIgnorePatterns,
	}
}

// IsGroupIgnored returns true when the entire group is ignored cluster-wide,
// by either an interactive rule or an any-namespace config pattern.
func (c *modelIgnoreChecker) IsGroupIgnored(source, groupKey string) bool {
	return isGroupIgnored(c.state, c.ctx, source, groupKey) ||
		patternIgnoresGroup(c.patterns, c.ctx, source, groupKey)
}

// IsResourceIgnored returns true when the specific resource within a group is
// ignored — by an interactive rule (group / namespace / resource scope) or a
// matching config pattern.
func (c *modelIgnoreChecker) IsResourceIgnored(source, groupKey, resourceKey string) bool {
	if isResourceIgnored(c.state, c.ctx, source, groupKey, resourceKey) {
		return true
	}
	return patternIgnoresResource(c.patterns, c.ctx, source, groupKey, namespaceFromResourceKey(resourceKey))
}
