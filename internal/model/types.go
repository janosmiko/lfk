// Package model defines shared types used across the application.
package model

import (
	"fmt"
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
	Icon        string
	Count       int // populated from FindingIndex at render time
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

// ResourceCategory groups related resource types for display.
type ResourceCategory struct {
	Name  string
	Types []ResourceTypeEntry
}

// ResourceTypeEntry represents a single navigable resource type.
type ResourceTypeEntry struct {
	DisplayName    string // e.g., "Deployments"
	Kind           string // e.g., "Deployment"
	APIGroup       string // e.g., "apps"
	APIVersion     string // e.g., "v1"
	Resource       string // e.g., "deployments" (plural lowercase for API calls)
	Icon           string // Unicode icon for display (e.g., "◆")
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

// coreCategories lists categories that are always shown regardless of CRD discovery.
// These represent core Kubernetes resources and Helm (which doesn't depend on CRDs).
var coreCategories = map[string]bool{
	"Dashboards":     true,
	"Cluster":        true,
	"Workloads":      true,
	"Config":         true,
	"Networking":     true,
	"Storage":        true,
	"Access Control": true,
	"Helm":           true,
	"Security":       true,
	"API and CRDs":   true,
}

// IsCoreCategory returns true if the given category name is a core (always-shown) category.
func IsCoreCategory(name string) bool {
	return coreCategories[name]
}

// Item represents a single navigable entry in any column.
type Item struct {
	Name          string
	Namespace     string           // Namespace of the resource (populated in all-namespaces mode)
	Status        string           // Used for pod/resource status coloring
	Kind          string           // The Kubernetes resource kind
	Extra         string           // Extra metadata (e.g., resource ref "group/version/resource")
	Category      string           // Display category grouping (e.g., "Workloads", "Networking")
	Icon          string           // Unicode icon for display
	Age           string           // Human-readable age (e.g., "5m", "2h", "3d")
	Ready         string           // Ready count (e.g., "2/3" for pods or deployments)
	Restarts      string           // Restart count (for pods)
	LastRestartAt time.Time        // Most recent container restart time
	CreatedAt     time.Time        // Creation timestamp for sorting
	Columns       []KeyValue       // Additional resource fields for summary preview
	Conditions    []ConditionEntry // Status conditions for the details pane
	Selected      bool             // Whether this item is part of a multi-selection
	Deprecated    bool             // Whether this resource uses a deprecated API version
	Deleting      bool             // Whether this resource has a deletionTimestamp set
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

// TopLevelResourceTypes returns the curated list of resource types shown at level 1.
func TopLevelResourceTypes() []ResourceCategory {
	cats := []ResourceCategory{ //nolint:prealloc // dynamic Security category appended below
		{
			Name: "Cluster",
			Types: []ResourceTypeEntry{
				{DisplayName: "Nodes", Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Icon: "⬡", Namespaced: false},
				{DisplayName: "Namespaces", Kind: "Namespace", APIGroup: "", APIVersion: "v1", Resource: "namespaces", Icon: "▣", Namespaced: false},
				{DisplayName: "Events", Kind: "Event", APIGroup: "", APIVersion: "v1", Resource: "events", Icon: "↯", Namespaced: true},
			},
		},
		{
			Name: "Workloads",
			Types: []ResourceTypeEntry{
				{DisplayName: "Pods", Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Icon: "⬤", Namespaced: true},
				{DisplayName: "Deployments", Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Icon: "◆", Namespaced: true},
				{DisplayName: "ReplicaSets", Kind: "ReplicaSet", APIGroup: "apps", APIVersion: "v1", Resource: "replicasets", Icon: "◈", Namespaced: true},
				{DisplayName: "StatefulSets", Kind: "StatefulSet", APIGroup: "apps", APIVersion: "v1", Resource: "statefulsets", Icon: "◇", Namespaced: true},
				{DisplayName: "DaemonSets", Kind: "DaemonSet", APIGroup: "apps", APIVersion: "v1", Resource: "daemonsets", Icon: "●", Namespaced: true},
				{DisplayName: "Jobs", Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Icon: "▶", Namespaced: true},
				{DisplayName: "CronJobs", Kind: "CronJob", APIGroup: "batch", APIVersion: "v1", Resource: "cronjobs", Icon: "⟳", Namespaced: true},
			},
		},
		{
			Name: "Config",
			Types: []ResourceTypeEntry{
				{DisplayName: "ConfigMaps", Kind: "ConfigMap", APIGroup: "", APIVersion: "v1", Resource: "configmaps", Icon: "≡", Namespaced: true},
				{DisplayName: "Secrets", Kind: "Secret", APIGroup: "", APIVersion: "v1", Resource: "secrets", Icon: "⊡", Namespaced: true},
				{DisplayName: "HPA", Kind: "HorizontalPodAutoscaler", APIGroup: "autoscaling", APIVersion: "v2", Resource: "horizontalpodautoscalers", Icon: "⇔", Namespaced: true},
				{DisplayName: "ResourceQuotas", Kind: "ResourceQuota", APIGroup: "", APIVersion: "v1", Resource: "resourcequotas", Icon: "⊞", Namespaced: true},
				{DisplayName: "LimitRanges", Kind: "LimitRange", APIGroup: "", APIVersion: "v1", Resource: "limitranges", Icon: "⊟", Namespaced: true},
				{DisplayName: "VPA", Kind: "VerticalPodAutoscaler", APIGroup: "autoscaling.k8s.io", APIVersion: "v1", Resource: "verticalpodautoscalers", Icon: "⇕", Namespaced: true, RequiresCRD: true},
				{DisplayName: "PodDisruptionBudgets", Kind: "PodDisruptionBudget", APIGroup: "policy", APIVersion: "v1", Resource: "poddisruptionbudgets", Icon: "⊘", Namespaced: true},
				{DisplayName: "PriorityClasses", Kind: "PriorityClass", APIGroup: "scheduling.k8s.io", APIVersion: "v1", Resource: "priorityclasses", Icon: "⇑", Namespaced: false},
				{DisplayName: "RuntimeClasses", Kind: "RuntimeClass", APIGroup: "node.k8s.io", APIVersion: "v1", Resource: "runtimeclasses", Icon: "⊙", Namespaced: false},
				{DisplayName: "Leases", Kind: "Lease", APIGroup: "coordination.k8s.io", APIVersion: "v1", Resource: "leases", Icon: "⏱", Namespaced: true},
				{DisplayName: "MutatingWebhookConfigurations", Kind: "MutatingWebhookConfiguration", APIGroup: "admissionregistration.k8s.io", APIVersion: "v1", Resource: "mutatingwebhookconfigurations", Icon: "⚙", Namespaced: false},
				{DisplayName: "ValidatingWebhookConfigurations", Kind: "ValidatingWebhookConfiguration", APIGroup: "admissionregistration.k8s.io", APIVersion: "v1", Resource: "validatingwebhookconfigurations", Icon: "⚙", Namespaced: false},
			},
		},
		{
			Name: "Networking",
			Types: []ResourceTypeEntry{
				{DisplayName: "Services", Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services", Icon: "⇌", Namespaced: true},
				{DisplayName: "Ingresses", Kind: "Ingress", APIGroup: "networking.k8s.io", APIVersion: "v1", Resource: "ingresses", Icon: "↳", Namespaced: true},
				{DisplayName: "NetworkPolicies", Kind: "NetworkPolicy", APIGroup: "networking.k8s.io", APIVersion: "v1", Resource: "networkpolicies", Icon: "⛊", Namespaced: true},
				{DisplayName: "Endpoints", Kind: "Endpoints", APIGroup: "", APIVersion: "v1", Resource: "endpoints", Icon: "⇢", Namespaced: true},
				{DisplayName: "EndpointSlices", Kind: "EndpointSlice", APIGroup: "discovery.k8s.io", APIVersion: "v1", Resource: "endpointslices", Icon: "⇢", Namespaced: true},
				{DisplayName: "IngressClasses", Kind: "IngressClass", APIGroup: "networking.k8s.io", APIVersion: "v1", Resource: "ingressclasses", Icon: "↳", Namespaced: false},
				{DisplayName: "Port Forwards", Kind: "__port_forwards__", APIGroup: "_portforward", APIVersion: "v1", Resource: "portforwards", Icon: "⇋", Namespaced: false},
			},
		},
		{
			Name: "gateway.networking.k8s.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "GatewayClasses", Kind: "GatewayClass", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1", Resource: "gatewayclasses", Icon: "⇶", Namespaced: false},
				{DisplayName: "HTTPRoutes", Kind: "HTTPRoute", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1", Resource: "httproutes", Icon: "⇶", Namespaced: true},
				{DisplayName: "TLSRoutes", Kind: "TLSRoute", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1alpha2", Resource: "tlsroutes", Icon: "⇶", Namespaced: true, RequiresCRD: true},
				{DisplayName: "GRPCRoutes", Kind: "GRPCRoute", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1", Resource: "grpcroutes", Icon: "⇶", Namespaced: true, RequiresCRD: true},
			},
		},
		{
			Name: "Storage",
			Types: []ResourceTypeEntry{
				{DisplayName: "PersistentVolumeClaims", Kind: "PersistentVolumeClaim", APIGroup: "", APIVersion: "v1", Resource: "persistentvolumeclaims", Icon: "⊞", Namespaced: true},
				{DisplayName: "PersistentVolumes", Kind: "PersistentVolume", APIGroup: "", APIVersion: "v1", Resource: "persistentvolumes", Icon: "⊞", Namespaced: false},
				{DisplayName: "StorageClasses", Kind: "StorageClass", APIGroup: "storage.k8s.io", APIVersion: "v1", Resource: "storageclasses", Icon: "▤", Namespaced: false},
			},
		},
		{
			Name: "Access Control",
			Types: []ResourceTypeEntry{
				{DisplayName: "ServiceAccounts", Kind: "ServiceAccount", APIGroup: "", APIVersion: "v1", Resource: "serviceaccounts", Icon: "⊕", Namespaced: true},
				{DisplayName: "Roles", Kind: "Role", APIGroup: "rbac.authorization.k8s.io", APIVersion: "v1", Resource: "roles", Icon: "⚿", Namespaced: true},
				{DisplayName: "RoleBindings", Kind: "RoleBinding", APIGroup: "rbac.authorization.k8s.io", APIVersion: "v1", Resource: "rolebindings", Icon: "⚿", Namespaced: true},
				{DisplayName: "ClusterRoles", Kind: "ClusterRole", APIGroup: "rbac.authorization.k8s.io", APIVersion: "v1", Resource: "clusterroles", Icon: "⚿", Namespaced: false},
				{DisplayName: "ClusterRoleBindings", Kind: "ClusterRoleBinding", APIGroup: "rbac.authorization.k8s.io", APIVersion: "v1", Resource: "clusterrolebindings", Icon: "⚿", Namespaced: false},
			},
		},
		{
			Name: "Helm",
			Types: []ResourceTypeEntry{
				{DisplayName: "Releases", Kind: "HelmRelease", APIGroup: "_helm", APIVersion: "v1", Resource: "releases", Icon: "⎋", Namespaced: true},
			},
		},
		{
			Name: "API and CRDs",
			Types: []ResourceTypeEntry{
				{DisplayName: "API Services", Kind: "APIService", APIGroup: "apiregistration.k8s.io", APIVersion: "v1", Resource: "apiservices", Icon: "⧫", Namespaced: false},
				{DisplayName: "Custom Resource Definitions", Kind: "CustomResourceDefinition", APIGroup: "apiextensions.k8s.io", APIVersion: "v1", Resource: "customresourcedefinitions", Icon: "⧫", Namespaced: false},
			},
		},
		{
			Name: "argoproj.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Applications", Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications", Icon: "⎈", Namespaced: true},
				{DisplayName: "ApplicationSets", Kind: "ApplicationSet", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applicationsets", Icon: "⎈", Namespaced: true},
				{DisplayName: "AppProjects", Kind: "AppProject", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "appprojects", Icon: "⎈", Namespaced: true},
				{DisplayName: "Workflows", Kind: "Workflow", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "workflows", Icon: "⟳", Namespaced: true},
				{DisplayName: "WorkflowTemplates", Kind: "WorkflowTemplate", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "workflowtemplates", Icon: "⟳", Namespaced: true},
				{DisplayName: "ClusterWorkflowTemplates", Kind: "ClusterWorkflowTemplate", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "clusterworkflowtemplates", Icon: "⟳", Namespaced: false},
				{DisplayName: "CronWorkflows", Kind: "CronWorkflow", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "cronworkflows", Icon: "⟳", Namespaced: true},
			},
		},
		{
			Name: "kustomize.toolkit.fluxcd.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Kustomizations", Kind: "Kustomization", APIGroup: "kustomize.toolkit.fluxcd.io", APIVersion: "v1", Resource: "kustomizations", Icon: "⎈", Namespaced: true},
			},
		},
		{
			Name: "helm.toolkit.fluxcd.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "HelmReleases", Kind: "HelmRelease", APIGroup: "helm.toolkit.fluxcd.io", APIVersion: "v2", Resource: "helmreleases", Icon: "⎋", Namespaced: true},
			},
		},
		{
			Name: "source.toolkit.fluxcd.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "GitRepositories", Kind: "GitRepository", APIGroup: "source.toolkit.fluxcd.io", APIVersion: "v1", Resource: "gitrepositories", Icon: "⧫", Namespaced: true},
				{DisplayName: "HelmRepositories", Kind: "HelmRepository", APIGroup: "source.toolkit.fluxcd.io", APIVersion: "v1", Resource: "helmrepositories", Icon: "⧫", Namespaced: true},
				{DisplayName: "HelmCharts", Kind: "HelmChart", APIGroup: "source.toolkit.fluxcd.io", APIVersion: "v1", Resource: "helmcharts", Icon: "⧫", Namespaced: true},
				{DisplayName: "OCIRepositories", Kind: "OCIRepository", APIGroup: "source.toolkit.fluxcd.io", APIVersion: "v1beta2", Resource: "ocirepositories", Icon: "⧫", Namespaced: true},
				{DisplayName: "Buckets", Kind: "Bucket", APIGroup: "source.toolkit.fluxcd.io", APIVersion: "v1beta2", Resource: "buckets", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "notification.toolkit.fluxcd.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Alerts", Kind: "Alert", APIGroup: "notification.toolkit.fluxcd.io", APIVersion: "v1beta3", Resource: "alerts", Icon: "⧫", Namespaced: true},
				{DisplayName: "Providers", Kind: "Provider", APIGroup: "notification.toolkit.fluxcd.io", APIVersion: "v1beta3", Resource: "providers", Icon: "⧫", Namespaced: true},
				{DisplayName: "Receivers", Kind: "Receiver", APIGroup: "notification.toolkit.fluxcd.io", APIVersion: "v1", Resource: "receivers", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "image.toolkit.fluxcd.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "ImageRepositories", Kind: "ImageRepository", APIGroup: "image.toolkit.fluxcd.io", APIVersion: "v1beta2", Resource: "imagerepositories", Icon: "⧫", Namespaced: true},
				{DisplayName: "ImagePolicies", Kind: "ImagePolicy", APIGroup: "image.toolkit.fluxcd.io", APIVersion: "v1beta2", Resource: "imagepolicies", Icon: "⧫", Namespaced: true},
				{DisplayName: "ImageUpdateAutomations", Kind: "ImageUpdateAutomation", APIGroup: "image.toolkit.fluxcd.io", APIVersion: "v1beta2", Resource: "imageupdateautomations", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "cert-manager.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Certificates", Kind: "Certificate", APIGroup: "cert-manager.io", APIVersion: "v1", Resource: "certificates", Icon: "⊡", Namespaced: true, RequiresCRD: true},
				{DisplayName: "Issuers", Kind: "Issuer", APIGroup: "cert-manager.io", APIVersion: "v1", Resource: "issuers", Icon: "⚿", Namespaced: true, RequiresCRD: true},
				{DisplayName: "ClusterIssuers", Kind: "ClusterIssuer", APIGroup: "cert-manager.io", APIVersion: "v1", Resource: "clusterissuers", Icon: "⚿", Namespaced: false, RequiresCRD: true},
				{DisplayName: "CertificateRequests", Kind: "CertificateRequest", APIGroup: "cert-manager.io", APIVersion: "v1", Resource: "certificaterequests", Icon: "⧫", Namespaced: true, RequiresCRD: true},
			},
		},
		{
			Name: "acme.cert-manager.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Orders", Kind: "Order", APIGroup: "acme.cert-manager.io", APIVersion: "v1", Resource: "orders", Icon: "⧫", Namespaced: true, RequiresCRD: true},
				{DisplayName: "Challenges", Kind: "Challenge", APIGroup: "acme.cert-manager.io", APIVersion: "v1", Resource: "challenges", Icon: "⧫", Namespaced: true, RequiresCRD: true},
			},
		},
		{
			Name: "longhorn.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Volumes", Kind: "Volume", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "volumes", Icon: "⬡", Namespaced: true},
				{DisplayName: "Engines", Kind: "Engine", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "engines", Icon: "⬡", Namespaced: true},
				{DisplayName: "Replicas", Kind: "Replica", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "replicas", Icon: "⬡", Namespaced: true},
				{DisplayName: "Longhorn Nodes", Kind: "Node", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "nodes", Icon: "⬡", Namespaced: true},
				{DisplayName: "BackingImages", Kind: "BackingImage", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "backingimages", Icon: "⬡", Namespaced: true},
				{DisplayName: "Backups", Kind: "Backup", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "backups", Icon: "⬡", Namespaced: true},
				{DisplayName: "RecurringJobs", Kind: "RecurringJob", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "recurringjobs", Icon: "⬡", Namespaced: true},
				{DisplayName: "Settings", Kind: "Setting", APIGroup: "longhorn.io", APIVersion: "v1beta2", Resource: "settings", Icon: "⬡", Namespaced: true},
			},
		},
		{
			Name: "networking.istio.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "VirtualServices", Kind: "VirtualService", APIGroup: "networking.istio.io", APIVersion: "v1", Resource: "virtualservices", Icon: "⎈", Namespaced: true},
				{DisplayName: "DestinationRules", Kind: "DestinationRule", APIGroup: "networking.istio.io", APIVersion: "v1", Resource: "destinationrules", Icon: "⎈", Namespaced: true},
				{DisplayName: "Gateways", Kind: "Gateway", APIGroup: "networking.istio.io", APIVersion: "v1", Resource: "gateways", Icon: "⎈", Namespaced: true},
				{DisplayName: "ServiceEntries", Kind: "ServiceEntry", APIGroup: "networking.istio.io", APIVersion: "v1", Resource: "serviceentries", Icon: "⎈", Namespaced: true},
				{DisplayName: "Sidecars", Kind: "Sidecar", APIGroup: "networking.istio.io", APIVersion: "v1", Resource: "sidecars", Icon: "⎈", Namespaced: true},
			},
		},
		{
			Name: "security.istio.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "PeerAuthentications", Kind: "PeerAuthentication", APIGroup: "security.istio.io", APIVersion: "v1", Resource: "peerauthentications", Icon: "⎈", Namespaced: true},
				{DisplayName: "AuthorizationPolicies", Kind: "AuthorizationPolicy", APIGroup: "security.istio.io", APIVersion: "v1", Resource: "authorizationpolicies", Icon: "⎈", Namespaced: true},
				{DisplayName: "RequestAuthentications", Kind: "RequestAuthentication", APIGroup: "security.istio.io", APIVersion: "v1", Resource: "requestauthentications", Icon: "⎈", Namespaced: true},
			},
		},
		{
			Name: "telemetry.istio.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Telemetries", Kind: "Telemetry", APIGroup: "telemetry.istio.io", APIVersion: "v1", Resource: "telemetries", Icon: "⎈", Namespaced: true},
			},
		},
		{
			Name: "cloud.google.com",
			Types: []ResourceTypeEntry{
				{DisplayName: "BackendConfigs", Kind: "BackendConfig", APIGroup: "cloud.google.com", APIVersion: "v1", Resource: "backendconfigs", Icon: "☁", Namespaced: true},
			},
		},
		{
			Name: "networking.gke.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "ManagedCertificates", Kind: "ManagedCertificate", APIGroup: "networking.gke.io", APIVersion: "v1", Resource: "managedcertificates", Icon: "☁", Namespaced: true},
			},
		},
		{
			Name: "vpcresources.k8s.aws",
			Types: []ResourceTypeEntry{
				{DisplayName: "SecurityGroupPolicies", Kind: "SecurityGroupPolicy", APIGroup: "vpcresources.k8s.aws", APIVersion: "v1beta1", Resource: "securitygrouppolicies", Icon: "☁", Namespaced: true},
			},
		},
		{
			Name: "crd.k8s.amazonaws.com",
			Types: []ResourceTypeEntry{
				{DisplayName: "ENIConfigs", Kind: "ENIConfig", APIGroup: "crd.k8s.amazonaws.com", APIVersion: "v1alpha1", Resource: "eniconfigs", Icon: "☁", Namespaced: false},
			},
		},
		{
			Name: "aadpodidentity.k8s.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "AzureIdentities", Kind: "AzureIdentity", APIGroup: "aadpodidentity.k8s.io", APIVersion: "v1", Resource: "azureidentities", Icon: "☁", Namespaced: true},
				{DisplayName: "AzureIdentityBindings", Kind: "AzureIdentityBinding", APIGroup: "aadpodidentity.k8s.io", APIVersion: "v1", Resource: "azureidentitybindings", Icon: "☁", Namespaced: true},
			},
		},
		{
			Name: "infrastructure.cluster.x-k8s.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "AzureManagedClusters", Kind: "AzureManagedCluster", APIGroup: "infrastructure.cluster.x-k8s.io", APIVersion: "v1beta1", Resource: "azuremanagedclusters", Icon: "☁", Namespaced: true},
				{DisplayName: "AzureManagedMachinePools", Kind: "AzureManagedMachinePool", APIGroup: "infrastructure.cluster.x-k8s.io", APIVersion: "v1beta1", Resource: "azuremanagedmachinepools", Icon: "☁", Namespaced: true},
			},
		},
		{
			Name: "karpenter.sh",
			Types: []ResourceTypeEntry{
				{DisplayName: "NodePools", Kind: "NodePool", APIGroup: "karpenter.sh", APIVersion: "v1", Resource: "nodepools", Icon: "⬡", Namespaced: false},
				{DisplayName: "NodeClaims", Kind: "NodeClaim", APIGroup: "karpenter.sh", APIVersion: "v1", Resource: "nodeclaims", Icon: "⬡", Namespaced: false},
			},
		},
		{
			Name: "karpenter.k8s.aws",
			Types: []ResourceTypeEntry{
				{DisplayName: "EC2NodeClasses", Kind: "EC2NodeClass", APIGroup: "karpenter.k8s.aws", APIVersion: "v1", Resource: "ec2nodeclasses", Icon: "⬡", Namespaced: false},
			},
		},
		{
			Name: "monitoring.coreos.com",
			Types: []ResourceTypeEntry{
				{DisplayName: "ServiceMonitors", Kind: "ServiceMonitor", APIGroup: "monitoring.coreos.com", APIVersion: "v1", Resource: "servicemonitors", Icon: "⊙", Namespaced: true},
				{DisplayName: "PodMonitors", Kind: "PodMonitor", APIGroup: "monitoring.coreos.com", APIVersion: "v1", Resource: "podmonitors", Icon: "⊙", Namespaced: true},
				{DisplayName: "PrometheusRules", Kind: "PrometheusRule", APIGroup: "monitoring.coreos.com", APIVersion: "v1", Resource: "prometheusrules", Icon: "⊙", Namespaced: true},
				{DisplayName: "Alertmanagers", Kind: "Alertmanager", APIGroup: "monitoring.coreos.com", APIVersion: "v1", Resource: "alertmanagers", Icon: "⊙", Namespaced: true},
				{DisplayName: "Prometheuses", Kind: "Prometheus", APIGroup: "monitoring.coreos.com", APIVersion: "v1", Resource: "prometheuses", Icon: "⊙", Namespaced: true},
				{DisplayName: "ThanosRulers", Kind: "ThanosRuler", APIGroup: "monitoring.coreos.com", APIVersion: "v1", Resource: "thanosrulers", Icon: "⊙", Namespaced: true},
			},
		},
		{
			Name: "keda.sh",
			Types: []ResourceTypeEntry{
				{DisplayName: "ScaledObjects", Kind: "ScaledObject", APIGroup: "keda.sh", APIVersion: "v1alpha1", Resource: "scaledobjects", Icon: "⚡", Namespaced: true},
				{DisplayName: "ScaledJobs", Kind: "ScaledJob", APIGroup: "keda.sh", APIVersion: "v1alpha1", Resource: "scaledjobs", Icon: "⚡", Namespaced: true},
				{DisplayName: "TriggerAuthentications", Kind: "TriggerAuthentication", APIGroup: "keda.sh", APIVersion: "v1alpha1", Resource: "triggerauthentications", Icon: "⚡", Namespaced: true},
				{DisplayName: "ClusterTriggerAuthentications", Kind: "ClusterTriggerAuthentication", APIGroup: "keda.sh", APIVersion: "v1alpha1", Resource: "clustertriggerauthentications", Icon: "⚡", Namespaced: false},
			},
		},
		{
			Name: "external-secrets.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "ExternalSecrets", Kind: "ExternalSecret", APIGroup: "external-secrets.io", APIVersion: "v1beta1", Resource: "externalsecrets", Icon: "⚿", Namespaced: true},
				{DisplayName: "ClusterExternalSecrets", Kind: "ClusterExternalSecret", APIGroup: "external-secrets.io", APIVersion: "v1beta1", Resource: "clusterexternalsecrets", Icon: "⚿", Namespaced: false},
				{DisplayName: "PushSecrets", Kind: "PushSecret", APIGroup: "external-secrets.io", APIVersion: "v1alpha1", Resource: "pushsecrets", Icon: "⚿", Namespaced: true},
				{DisplayName: "SecretStores", Kind: "SecretStore", APIGroup: "external-secrets.io", APIVersion: "v1beta1", Resource: "secretstores", Icon: "⚿", Namespaced: true},
				{DisplayName: "ClusterSecretStores", Kind: "ClusterSecretStore", APIGroup: "external-secrets.io", APIVersion: "v1beta1", Resource: "clustersecretstores", Icon: "⚿", Namespaced: false},
			},
		},
		{
			Name: "bitnami.com",
			Types: []ResourceTypeEntry{
				{DisplayName: "SealedSecrets", Kind: "SealedSecret", APIGroup: "bitnami.com", APIVersion: "v1alpha1", Resource: "sealedsecrets", Icon: "⚿", Namespaced: true},
			},
		},
		{
			Name: "traefik.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "IngressRoutes", Kind: "IngressRoute", APIGroup: "traefik.io", APIVersion: "v1alpha1", Resource: "ingressroutes", Icon: "⎈", Namespaced: true},
				{DisplayName: "Middlewares", Kind: "Middleware", APIGroup: "traefik.io", APIVersion: "v1alpha1", Resource: "middlewares", Icon: "⎈", Namespaced: true},
				{DisplayName: "IngressRouteTCPs", Kind: "IngressRouteTCP", APIGroup: "traefik.io", APIVersion: "v1alpha1", Resource: "ingressroutetcps", Icon: "⎈", Namespaced: true},
				{DisplayName: "TLSOptions", Kind: "TLSOption", APIGroup: "traefik.io", APIVersion: "v1alpha1", Resource: "tlsoptions", Icon: "⎈", Namespaced: true},
			},
		},
		{
			Name: "externaldns.k8s.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "DNSEndpoints", Kind: "DNSEndpoint", APIGroup: "externaldns.k8s.io", APIVersion: "v1alpha1", Resource: "dnsendpoints", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "apiextensions.crossplane.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Compositions", Kind: "Composition", APIGroup: "apiextensions.crossplane.io", APIVersion: "v1", Resource: "compositions", Icon: "⧫", Namespaced: false},
				{DisplayName: "CompositeResourceDefinitions", Kind: "CompositeResourceDefinition", APIGroup: "apiextensions.crossplane.io", APIVersion: "v1", Resource: "compositeresourcedefinitions", Icon: "⧫", Namespaced: false},
			},
		},
		{
			Name: "pkg.crossplane.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Providers", Kind: "Provider", APIGroup: "pkg.crossplane.io", APIVersion: "v1", Resource: "providers", Icon: "⧫", Namespaced: false},
				{DisplayName: "Configurations", Kind: "Configuration", APIGroup: "pkg.crossplane.io", APIVersion: "v1", Resource: "configurations", Icon: "⧫", Namespaced: false},
			},
		},
		{
			Name: "velero.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Backups", Kind: "Backup", APIGroup: "velero.io", APIVersion: "v1", Resource: "backups", Icon: "⧫", Namespaced: true},
				{DisplayName: "Restores", Kind: "Restore", APIGroup: "velero.io", APIVersion: "v1", Resource: "restores", Icon: "⧫", Namespaced: true},
				{DisplayName: "Schedules", Kind: "Schedule", APIGroup: "velero.io", APIVersion: "v1", Resource: "schedules", Icon: "⧫", Namespaced: true},
				{DisplayName: "BackupStorageLocations", Kind: "BackupStorageLocation", APIGroup: "velero.io", APIVersion: "v1", Resource: "backupstoragelocations", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "tekton.dev",
			Types: []ResourceTypeEntry{
				{DisplayName: "Pipelines", Kind: "Pipeline", APIGroup: "tekton.dev", APIVersion: "v1", Resource: "pipelines", Icon: "⧫", Namespaced: true},
				{DisplayName: "Tasks", Kind: "Task", APIGroup: "tekton.dev", APIVersion: "v1", Resource: "tasks", Icon: "⧫", Namespaced: true},
				{DisplayName: "PipelineRuns", Kind: "PipelineRun", APIGroup: "tekton.dev", APIVersion: "v1", Resource: "pipelineruns", Icon: "⧫", Namespaced: true},
				{DisplayName: "TaskRuns", Kind: "TaskRun", APIGroup: "tekton.dev", APIVersion: "v1", Resource: "taskruns", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "kafka.strimzi.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Kafkas", Kind: "Kafka", APIGroup: "kafka.strimzi.io", APIVersion: "v1beta2", Resource: "kafkas", Icon: "⧫", Namespaced: true},
				{DisplayName: "KafkaTopics", Kind: "KafkaTopic", APIGroup: "kafka.strimzi.io", APIVersion: "v1beta2", Resource: "kafkatopics", Icon: "⧫", Namespaced: true},
				{DisplayName: "KafkaConnects", Kind: "KafkaConnect", APIGroup: "kafka.strimzi.io", APIVersion: "v1beta2", Resource: "kafkaconnects", Icon: "⧫", Namespaced: true},
				{DisplayName: "KafkaUsers", Kind: "KafkaUser", APIGroup: "kafka.strimzi.io", APIVersion: "v1beta2", Resource: "kafkausers", Icon: "⧫", Namespaced: true},
				{DisplayName: "KafkaBridges", Kind: "KafkaBridge", APIGroup: "kafka.strimzi.io", APIVersion: "v1beta2", Resource: "kafkabridges", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "serving.knative.dev",
			Types: []ResourceTypeEntry{
				{DisplayName: "Knative Services", Kind: "Service", APIGroup: "serving.knative.dev", APIVersion: "v1", Resource: "services", Icon: "⧫", Namespaced: true},
				{DisplayName: "Routes", Kind: "Route", APIGroup: "serving.knative.dev", APIVersion: "v1", Resource: "routes", Icon: "⧫", Namespaced: true},
				{DisplayName: "Revisions", Kind: "Revision", APIGroup: "serving.knative.dev", APIVersion: "v1", Resource: "revisions", Icon: "⧫", Namespaced: true},
				{DisplayName: "Configurations", Kind: "Configuration", APIGroup: "serving.knative.dev", APIVersion: "v1", Resource: "configurations", Icon: "⧫", Namespaced: true},
			},
		},
		{
			Name: "eventing.knative.dev",
			Types: []ResourceTypeEntry{
				{DisplayName: "Triggers", Kind: "Trigger", APIGroup: "eventing.knative.dev", APIVersion: "v1", Resource: "triggers", Icon: "⧫", Namespaced: true},
				{DisplayName: "Brokers", Kind: "Broker", APIGroup: "eventing.knative.dev", APIVersion: "v1", Resource: "brokers", Icon: "⧫", Namespaced: true},
			},
		},
	}

	// Security category — dynamically populated from SecuritySourcesFn.
	// Always visible (heuristic source has no external dependency).
	var securityEntries []ResourceTypeEntry
	if SecuritySourcesFn != nil {
		for _, src := range SecuritySourcesFn() {
			displayName := src.DisplayName
			if src.Count >= 0 {
				displayName = fmt.Sprintf("%s (%d)", src.DisplayName, src.Count)
			}
			securityEntries = append(securityEntries, ResourceTypeEntry{
				DisplayName: displayName,
				Kind:        "__security_" + src.SourceName + "__",
				APIGroup:    SecurityVirtualAPIGroup,
				APIVersion:  "v1",
				Resource:    "findings",
				Icon:        src.Icon,
				Namespaced:  false,
			})
		}
	}
	cats = append(cats, ResourceCategory{Name: "Security", Types: securityEntries})

	return cats
}
