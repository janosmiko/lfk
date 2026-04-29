package app

import (
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
)

// SecurityIgnoreRule represents a single ignore entry.
type SecurityIgnoreRule struct {
	Source    string `json:"source" yaml:"source"`                         // Security source name: "heuristic", "trivy-operator", "falco", "policy-report"
	GroupKey  string `json:"group_key" yaml:"group_key"`                   // Finding group key (check label, CVE ID, rule name)
	Resource  string `json:"resource,omitempty" yaml:"resource,omitempty"` // ResourceRef.Key() format: "ns/kind/name". Empty = global ignore.
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
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

	return os.Rename(tmpPath, path)
}

// addSecurityIgnore returns a NEW state with the rule added for the given context.
// Deduplicates by (GroupKey, Resource). Sets CreatedAt if empty.
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

	// Deduplicate: replace if same (Source, GroupKey, Resource) already exists.
	for i, r := range existing {
		if r.Source == rule.Source && r.GroupKey == rule.GroupKey && r.Resource == rule.Resource {
			existing[i] = rule
			newContexts[ctx] = existing
			return &SecurityIgnoreState{Contexts: newContexts}
		}
	}

	newContexts[ctx] = append(existing, rule)

	return &SecurityIgnoreState{Contexts: newContexts}
}

// removeSecurityIgnore returns a NEW state with the matching rule removed.
func removeSecurityIgnore(state *SecurityIgnoreState, ctx, source, groupKey, resource string) *SecurityIgnoreState {
	newContexts := make(map[string][]SecurityIgnoreRule, len(state.Contexts))
	for k, v := range state.Contexts {
		copied := make([]SecurityIgnoreRule, len(v))
		copy(copied, v)
		newContexts[k] = copied
	}

	existing := newContexts[ctx]
	filtered := make([]SecurityIgnoreRule, 0, len(existing))
	for _, r := range existing {
		if r.Source == source && r.GroupKey == groupKey && r.Resource == resource {
			continue
		}
		filtered = append(filtered, r)
	}
	newContexts[ctx] = filtered

	return &SecurityIgnoreState{Contexts: newContexts}
}

// isGroupIgnored returns true if there is a global ignore rule (empty Resource)
// for the given source and group key in the specified context.
func isGroupIgnored(state *SecurityIgnoreState, ctx, source, groupKey string) bool {
	for _, r := range state.Contexts[ctx] {
		if r.Source == source && r.GroupKey == groupKey && r.Resource == "" {
			return true
		}
	}
	return false
}

// isResourceIgnored returns true if EITHER a global ignore (empty Resource)
// OR a resource-specific ignore matches the given source, group key and resource key.
func isResourceIgnored(state *SecurityIgnoreState, ctx, source, groupKey, resourceKey string) bool {
	for _, r := range state.Contexts[ctx] {
		if r.Source != source || r.GroupKey != groupKey {
			continue
		}
		if r.Resource == "" || r.Resource == resourceKey {
			return true
		}
	}
	return false
}

// isResourceSpecificIgnored returns true only when there is a resource-level
// ignore rule (non-empty Resource) for the given source, group key and resource key.
func isResourceSpecificIgnored(state *SecurityIgnoreState, ctx, source, groupKey, resourceKey string) bool {
	for _, r := range state.Contexts[ctx] {
		if r.Source == source && r.GroupKey == groupKey && r.Resource == resourceKey && r.Resource != "" {
			return true
		}
	}
	return false
}

// countIgnoredGroups returns the count of global ignores (rules with empty Resource)
// for the given context.
func countIgnoredGroups(state *SecurityIgnoreState, ctx string) int {
	count := 0
	for _, r := range state.Contexts[ctx] {
		if r.Resource == "" {
			count++
		}
	}
	return count
}

// modelIgnoreChecker adapts SecurityIgnoreState to the k8s.IgnoreChecker
// interface so the groupFindings engine can filter ignored entries. The
// interface is defined in the k8s package; Go structural typing allows this
// app-layer type to satisfy it without importing k8s.
type modelIgnoreChecker struct {
	state *SecurityIgnoreState
	ctx   string
}

// IsGroupIgnored returns true when the entire group is globally ignored.
func (c *modelIgnoreChecker) IsGroupIgnored(source, groupKey string) bool {
	return isGroupIgnored(c.state, c.ctx, source, groupKey)
}

// IsResourceIgnored returns true when the specific resource within a group
// is ignored (either via a resource-level rule or a global group ignore).
func (c *modelIgnoreChecker) IsResourceIgnored(source, groupKey, resourceKey string) bool {
	return isResourceIgnored(c.state, c.ctx, source, groupKey, resourceKey)
}
