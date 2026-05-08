// Package model defines shared types used across the application.
package model

import (
	"slices"
	"time"
)

// Level represents the current navigation depth in the owner-based hierarchy.
type Level int

const (
	LevelClusters      Level = iota // Top level: list of kube contexts (clusters)
	LevelResourceTypes              // Resource type categories within a cluster
	LevelResources                  // Individual resources of a type (e.g., specific deployments)
	LevelOwned                      // Owned resources (e.g., pods owned by a deployment)
	LevelContainers                 // Containers within a pod
)

// ResourceTypeEntry represents a single navigable resource type.
type ResourceTypeEntry struct {
	DisplayName    string // e.g., "Deployments"
	Kind           string // e.g., "Deployment"
	APIGroup       string // e.g., "apps"
	APIVersion     string // e.g., "v1"
	Resource       string // e.g., "deployments" (plural lowercase for API calls)
	Icon           Icon   // Icon variants for display (see icon.go)
	Namespaced     bool   // true for namespace-scoped resources, false for cluster-scoped
	RequiresCRD    bool   // true if this resource type depends on a CRD being installed
	Deprecated     bool   // true if this API version is deprecated
	DeprecationMsg string // human-readable deprecation message

	// Verbs is the set of verbs the API server reports for this resource
	// (e.g. "get", "list", "watch", "create"). Populated from the discovery
	// API. Empty for LFK pseudo-resources and entries constructed without
	// discovery data — the sidebar treats empty Verbs as listable so those
	// stay visible.
	Verbs []string

	PrinterColumns []PrinterColumn // additionalPrinterColumns from CRD spec
}

// CanList reports whether the API server supports LIST for this
// resource. Entries without Verbs (pseudo-resources, seed entries) are
// treated as listable so they remain visible. Otherwise the verb set
// must explicitly contain "list" — this is what keeps create-only
// Review APIs (tokenreviews, subjectaccessreviews, selfsubject*reviews)
// out of the sidebar.
func (e ResourceTypeEntry) CanList() bool {
	if len(e.Verbs) == 0 {
		return true
	}
	return slices.Contains(e.Verbs, "list")
}

// PrinterColumn represents an additionalPrinterColumn from a CRD spec.
type PrinterColumn struct {
	Name     string
	Type     string // string, integer, number, boolean, date
	JSONPath string // e.g. ".status.phase", ".spec.source.repoURL"
}

// CanIResource represents a single resource type with its RBAC permissions.
type CanIResource struct {
	APIGroup string
	Resource string          // plural name (e.g., "deployments")
	Kind     string          // kind name (e.g., "Deployment")
	Verbs    map[string]bool // verb -> allowed
}

// CanIGroup represents an API group with its resources for the can-i browser.
type CanIGroup struct {
	Name      string         // API group name ("" for core)
	Resources []CanIResource // resources in this group
}

// ExplainField represents a single field from kubectl explain output.
type ExplainField struct {
	Name        string // field name (e.g., "spec", "apiVersion")
	Type        string // field type (e.g., "<string>", "<Object>")
	Description string // human-readable description
	Path        string // dot-separated path (e.g., "spec.template.metadata")
	Required    bool   // true if field has -required- marker
}

// KeyValue represents an ordered key-value pair for resource summary display.
type KeyValue struct {
	Key   string
	Value string
}

// ConditionEntry represents a single status condition for display in the details pane.
type ConditionEntry struct {
	Type    string
	Status  string // "True" or "False"
	Reason  string
	Message string
}

// PinnedGroups lists CRD API groups that should appear right after built-in categories.
// Set from config at startup.
var PinnedGroups []string

// ConfigDefaultRightsizingStrategy is the strategy from the user's lfk
// config (rightsizing_defaults.strategy). Empty when unset or the
// configured value didn't match any known RightsizingStrategy.
//
// Used by executeActionRightsizing as the seed value when there's no
// sticky strategy from a previous overlay open. NOT consulted on every
// open — once the user changes strategy in the overlay, that choice
// sticks for the rest of the session.
var ConfigDefaultRightsizingStrategy RightsizingStrategy

// ConfigDefaultRightsizingHeadroom is the headroom from
// rightsizing_defaults.headroom. Zero when unset or invalid.
// Same sticky-then-fallback semantics as the strategy var above.
var ConfigDefaultRightsizingHeadroom float64

// GroupedRef identifies a single resource within a grouped row (e.g., one
// of the many Event objects collapsed into a single line by event grouping).
type GroupedRef struct {
	Name      string
	Namespace string
}

// Item represents a single navigable entry in any column.
type Item struct {
	Name          string
	Namespace     string           // Namespace of the resource (populated in all-namespaces mode)
	Status        string           // Used for pod/resource status coloring
	Kind          string           // The Kubernetes resource kind
	Extra         string           // Extra metadata (e.g., resource ref "group/version/resource")
	Category      string           // Display category grouping (e.g., "Workloads", "Networking")
	Icon          Icon             // Icon variants for display (see icon.go)
	Age           string           // Human-readable age (e.g., "5m", "2h", "3d")
	Ready         string           // Ready count (e.g., "2/3" for pods or deployments)
	Restarts      string           // Restart count (for pods)
	LastRestartAt time.Time        // Most recent container restart time
	CreatedAt     time.Time        // Creation timestamp for sorting (Events: first observed timestamp in the series)
	LastSeen      time.Time        // Most recent observation (Events only — drives the "Last Seen" column)
	Columns       []KeyValue       // Additional resource fields for summary preview
	Conditions    []ConditionEntry // Status conditions for the details pane
	Selected      bool             // Whether this item is part of a multi-selection
	Deprecated    bool             // Whether this resource uses a deprecated API version
	Deleting      bool             // Whether this resource has a deletionTimestamp set
	ReadOnly      bool             // Whether this item represents a context locked in read-only mode (renders as a [RO] suffix in the picker)
	ClusterColor  string           // Optional named color (one of ui.ClusterColorNames) for context rows; empty = no swatch.
	// LocalClusterStatus is "running" / "stopped" / "" — populated only
	// for cluster-picker rows whose context name is recognised in
	// Model.localClusterCache. The renderer prepends a filled-circle
	// glyph for "running" and a hollow-circle glyph for "stopped"; an
	// empty string means the row is not a local cluster and the
	// renderer skips the icon entirely.
	LocalClusterStatus string
	// IsContext flags that this Item represents a kubeconfig context
	// (a row in the cluster picker at LevelClusters). Stamped by
	// updateContextsLoaded so renderers can lay out the row in a
	// columned shape — current marker, local-cluster status, RO tag,
	// and color swatch each occupy a fixed-width slot so rows stay
	// aligned regardless of which markers are present.
	IsContext   bool
	GroupedRefs []GroupedRef // For grouped rows (Events): all underlying resource identifiers
}

// MissingRefStatus is the Status string assigned to a ResourceNode whose
// referenced object (Secret/ConfigMap/PVC/ServiceAccount) does not exist on
// the cluster. The k8s package writes it; the ui package matches on it in
// StatusStyle to render the node red. Keep these two callers in sync via
// this single constant.
const MissingRefStatus = "MissingRef"

// ResourceNode represents a node in a resource relationship tree.
type ResourceNode struct {
	Name      string
	Kind      string
	Namespace string
	Status    string
	// Group categorizes children for rendering. Empty (default) and "owned"
	// behave the same — owner-chain descendants like ReplicaSet/Pod/Container.
	// "refs" marks Secret/ConfigMap/PVC/ServiceAccount nodes attached to a
	// Pod via env, envFrom, volumes, or serviceAccountName. The renderer
	// uses this to emit a mixed-kind badge like "(2 Container, 3 refs)".
	Group    string
	Children []*ResourceNode
}

// NavigationState holds the full state of where the user is in the hierarchy.
type NavigationState struct {
	Level        Level
	Context      string
	Namespace    string
	ResourceType ResourceTypeEntry // The selected resource type
	ResourceName string            // The selected resource name
	OwnedName    string            // The selected owned resource name (e.g., pod name)
}

// SecretData holds the decoded key-value pairs of a Kubernetes secret.
type SecretData struct {
	Keys []string          // ordered list of keys
	Data map[string]string // key -> decoded value
}

// ConfigMapData holds the key-value pairs of a Kubernetes ConfigMap.
type ConfigMapData struct {
	Keys []string          // ordered list of keys
	Data map[string]string // key -> value
}

// LabelAnnotationData holds labels and annotations for a resource.
type LabelAnnotationData struct {
	Labels      map[string]string
	LabelKeys   []string // ordered
	Annotations map[string]string
	AnnotKeys   []string // ordered
}

// PodMetrics holds CPU and memory usage for a pod.
type PodMetrics struct {
	Name      string
	Namespace string
	CPU       int64 // in millicores
	Memory    int64 // in bytes
}

// Bookmark represents a saved navigation path for quick access.
type Bookmark struct {
	Name         string   `json:"name" yaml:"name"`
	Context      string   `json:"context,omitempty" yaml:"context,omitempty"`
	Namespace    string   `json:"namespace" yaml:"namespace"`
	Namespaces   []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	ResourceType string   `json:"resource_type" yaml:"resource_type"` // resource ref string (group/version/resource)
	ResourceName string   `json:"resource_name,omitempty" yaml:"resource_name,omitempty"`
	Slot         string   `json:"slot,omitempty" yaml:"slot,omitempty"` // single char key for vim-style named marks (a-z, A-Z, 0-9)
}

// IsContextAware reports whether this bookmark is anchored to a specific
// kube context. Context-aware bookmarks switch to their stored context on
// jump; context-free bookmarks use whatever context is currently active.
func (b Bookmark) IsContextAware() bool {
	return b.Context != ""
}

// ActionMenuItem represents an entry in the action menu.
type ActionMenuItem struct {
	Label       string
	Description string
	Key         string // shortcut key
}

// RightsizingStrategy enumerates the supported recommendation algorithms.
// String type so the value flows through cache keys / log lines cleanly.
type RightsizingStrategy string

const (
	StrategyVPA       RightsizingStrategy = "vpa"
	StrategyPromMax1D RightsizingStrategy = "prom_max_1d"
	StrategyPromAvg1D RightsizingStrategy = "prom_avg_1d"
	StrategyPromP957D RightsizingStrategy = "prom_p95_7d"
	StrategySnapshot  RightsizingStrategy = "snapshot"
)

// AllRightsizingStrategies in priority order (first = preferred default).
// The picker walks this list, skipping strategies not present in the
// per-workload AvailableStrategies slice. The order matches the
// user-confirmed default: VPA (history-based recommender) > Prometheus
// peak/avg/p95 windows > snapshot fallback.
var AllRightsizingStrategies = []RightsizingStrategy{
	StrategyVPA,
	StrategyPromMax1D,
	StrategyPromAvg1D,
	StrategyPromP957D,
	StrategySnapshot,
}

// RightsizingHeadrooms enumerates the headroom multipliers the picker
// cycles through. Stored ascending so a `>` press in the overlay reads
// as "give me more safety margin" and `<` reads as "tighten the
// recommendation."
//
// 1.25 is the default (DefaultRightsizingHeadroom) — the closest entry
// to the previous hardcoded 1.2 factor, so the migration is visually
// invisible. Users can tune up to 2.0 (double the observed peak) for
// generous padding or down to 1.0 (raw VPA / metrics value) for the
// tightest possible spec.
var RightsizingHeadrooms = []float64{1.0, 1.1, 1.25, 1.5, 1.75, 2.0}

// DefaultRightsizingHeadroom is the headroom multiplier seeded when
// the right-sizing overlay opens for the first time. Lives in
// RightsizingHeadrooms so the [N/M] picker chip renders a position on
// first open instead of "?".
const DefaultRightsizingHeadroom = 1.25

// HumanLabel returns the short label shown in the overlay header.
func (s RightsizingStrategy) HumanLabel() string {
	switch s {
	case StrategyVPA:
		return "VPA"
	case StrategyPromMax1D:
		return "1d-max"
	case StrategyPromAvg1D:
		return "1d-avg"
	case StrategyPromP957D:
		return "7d-p95"
	case StrategySnapshot:
		return "snapshot"
	}
	return string(s)
}

// MethodologyHint returns the longer explanation appended to the
// header line so the user understands what window / source backs
// the recommendation. The headroom multiplier is intentionally NOT
// in this string — the UI appends " x <H> headroom" using the
// data's Headroom field, so this hint stays headroom-agnostic and
// the user can see the actual multiplier they selected.
func (s RightsizingStrategy) MethodologyHint() string {
	switch s {
	case StrategyVPA:
		return "VPA recommender (history-based)"
	case StrategyPromMax1D:
		return "Prometheus peak (max) over last 1d"
	case StrategyPromAvg1D:
		return "Prometheus avg over last 1d"
	case StrategyPromP957D:
		return "Prometheus p95 over last 7d"
	case StrategySnapshot:
		return "current usage"
	}
	return string(s)
}

// Rightsizing is the per-workload right-sizing recommendation
// payload. Source is the legacy human-readable label kept for
// external readers; new code should branch on Strategy instead.
type Rightsizing struct {
	Source              string                // legacy human-readable label (set from Strategy.HumanLabel())
	Strategy            RightsizingStrategy   // which algorithm produced these numbers
	AvailableStrategies []RightsizingStrategy // subset of AllRightsizingStrategies usable on this workload + cluster
	Headroom            float64               // headroom multiplier that produced these numbers (e.g. 1.25 = 25% padding above usage)
	PodCount            int                   // pods aggregated (always 1 for Pod kind)
	Window              string                // sampling window (e.g. "30s" for metrics-server, "1d"/"7d" for Prometheus); empty for VPA
	Containers          []ContainerRec
}

// ContainerRec carries the recommendation for a single container,
// keyed by name to match the corresponding container in the pod
// spec template (used by the "y copy" path to splice back into a
// YAML block).
type ContainerRec struct {
	Name string
	CPU  ResourceRec
	Mem  ResourceRec
}

// ResourceRec is the current-vs-recommended pair for one resource
// (CPU or memory). Strings are k8s canonical form ("100m", "256Mi").
// Empty string = "unset" — current values absent in spec, or no
// recommendation available. Bounds are populated only on the VPA path.
type ResourceRec struct {
	Usage              string // peak observed usage from metrics-server (empty if no live data)
	CurrentRequest     string
	CurrentLimit       string
	RecommendedRequest string
	RecommendedLimit   string
	LowerBound         string // VPA only
	UpperBound         string // VPA only
}
