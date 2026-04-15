// Package model defines shared types used across the application.
package model

import (
	"time"
)

// SecurityVirtualAPIGroup is the APIGroup used by synthetic security
// resource types. Client.GetResources dispatches on this value.
const SecurityVirtualAPIGroup = "_security"

// SecuritySourceEntry describes one entry shown under the Security category
// in the middle column. Populated at startup by the app layer.
type SecuritySourceEntry struct {
	DisplayName string // "Trivy", "Kyverno", "Heuristic"
	SourceName  string // matches security.SecuritySource.Name() — "trivy-operator", "heuristic", "policy-report"
	Icon        Icon   // Icon variants for display (see icon.go)
	Count       int    // populated from FindingIndex at render time
}

// SecuritySourcesFn returns the list of security source entries to display
// in the Security category. Set by the app at startup. When nil or empty,
// the Security category is still shown (it's a core category) but empty.
var SecuritySourcesFn func() []SecuritySourceEntry

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

	PrinterColumns []PrinterColumn // additionalPrinterColumns from CRD spec
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
	GroupedRefs   []GroupedRef     // For grouped rows (Events): all underlying resource identifiers
}

// ColumnValue returns the value of the named column, or "" if the column
// is not present. Used by callers that need to read fields out of an Item
// without knowing the column's position.
func (i Item) ColumnValue(key string) string {
	for _, c := range i.Columns {
		if c.Key == key {
			return c.Value
		}
	}
	return ""
}

// ResourceNode represents a node in a resource relationship tree.
type ResourceNode struct {
	Name      string
	Kind      string
	Namespace string
	Status    string
	Children  []*ResourceNode
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
