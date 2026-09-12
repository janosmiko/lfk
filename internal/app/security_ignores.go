package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

const securityIgnoresFileName = "security_ignores.yaml"

// SecurityIgnoreRule represents a single ignore entry. Scope is determined by
// which of Namespace / Resource are set, in order of increasing specificity:
//
//	Namespace == "" && Resource == "" -> whole group, cluster-wide
//	Namespace != "" && Resource == "" -> group within one namespace
//	Resource  != ""                   -> one specific resource (ns encoded in key)
type SecurityIgnoreRule struct {
	Source    string `json:"source" yaml:"source"`                           // Security source name: "heuristic", "advisor", "rbac", "trivy-operator", "falco", "policy-report"
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

// loadSecurityIgnores reads ignore rules from the YAML file on disk.
// Returns an empty state (never nil) if the file is missing or corrupt.
func loadSecurityIgnores() *SecurityIgnoreState {
	state := loadStateFile[SecurityIgnoreState](securityIgnoresFileName)
	if state.Contexts == nil {
		state.Contexts = make(map[string][]SecurityIgnoreRule)
	}
	return &state
}

// saveSecurityIgnores writes ignore rules to the YAML file on disk durably
// (fsynced tmp file, then rename) to prevent data loss if the process is
// interrupted mid-write.
func saveSecurityIgnores(state *SecurityIgnoreState) error {
	return saveStateFile(securityIgnoresFileName, state)
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
// matching config pattern. labels are the target object's Kubernetes labels
// (nil when the source did not expose them), consulted only by label-match
// config patterns.
func (c *modelIgnoreChecker) IsResourceIgnored(source, groupKey, resourceKey string, labels map[string]string) bool {
	if isResourceIgnored(c.state, c.ctx, source, groupKey, resourceKey) {
		return true
	}
	return patternIgnoresResource(c.patterns, c.ctx, source, groupKey, namespaceFromResourceKey(resourceKey), labels)
}
