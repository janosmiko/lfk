// Package security defines the abstraction and shared types for pluggable
// security-finding sources. Adapters live under sub-packages (heuristic,
// trivyop, ...) and register with the Manager at startup.
package security

import "context"

// Category groups a finding by kind so the UI can tab/filter on it.
type Category string

const (
	CategoryVuln       Category = "vuln"
	CategoryMisconfig  Category = "misconfig"
	CategoryPolicy     Category = "policy"
	CategoryCompliance Category = "compliance"
)

// Severity is a 4-level scale; sources map their own scales onto these.
type Severity int

const (
	SeverityUnknown Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// ResourceRef identifies the Kubernetes object a finding is attached to.
// Empty ResourceRef means the finding is cluster-scoped.
type ResourceRef struct {
	Namespace string
	Kind      string
	Name      string
	Container string
}

// Key returns a stable identifier used by FindingIndex aggregation.
// Container is intentionally omitted so findings on multiple containers of
// a pod aggregate into a single row in the explorer SEC column.
func (r ResourceRef) Key() string {
	return r.Namespace + "/" + r.Kind + "/" + r.Name
}

// Finding is the denormalized row the UI renders.
type Finding struct {
	ID         string
	Source     string
	Category   Category
	Severity   Severity
	Title      string
	Resource   ResourceRef
	Summary    string
	Details    string
	References []string
	Labels     map[string]string
}

// SecuritySource is the contract every adapter implements.
// Implementations must be goroutine-safe and honor ctx cancellation.
type SecuritySource interface {
	// Name returns a stable, lowercase identifier used for config keys,
	// logging, and the source column in the UI. E.g., "trivy-operator".
	Name() string

	// Categories returns the set of categories this source produces.
	Categories() []Category

	// IsAvailable returns true if the source's dependencies are reachable.
	// Results are cached by the Manager (default TTL 60s). Must return
	// quickly or honor ctx timeout. Errors are treated as "not available"
	// but logged for diagnostics.
	IsAvailable(ctx context.Context, kubeCtx string) (bool, error)

	// Fetch collects findings for the given cluster context and optional
	// namespace filter. Empty namespace means all-namespaces. Must honor
	// ctx cancellation.
	Fetch(ctx context.Context, kubeCtx, namespace string) ([]Finding, error)
}
