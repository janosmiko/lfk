// Package model defines shared types used across the application.
package model

import (
	"sort"
	"strings"
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

// KeyValue represents an ordered key-value pair for resource summary display.
type KeyValue struct {
	Key   string
	Value string
}

// PinnedGroups lists CRD API groups that should appear right after built-in categories.
// Set from config at startup.
var PinnedGroups []string

// coreCategories lists categories that are always shown regardless of CRD discovery.
// These represent core Kubernetes resources and Helm (which doesn't depend on CRDs).
var coreCategories = map[string]bool{
	"Workloads":      true,
	"Config":         true,
	"Networking":     true,
	"Storage":        true,
	"Access Control": true,
	"Cluster":        true,
	"Helm":           true,
}

// IsCoreCategory returns true if the given category name is a core (always-shown) category.
func IsCoreCategory(name string) bool {
	return coreCategories[name]
}

// Item represents a single navigable entry in any column.
type Item struct {
	Name       string
	Namespace  string     // Namespace of the resource (populated in all-namespaces mode)
	Status     string     // Used for pod/resource status coloring
	Kind       string     // The Kubernetes resource kind
	Extra      string     // Extra metadata (e.g., resource ref "group/version/resource")
	Category   string     // Display category grouping (e.g., "Workloads", "Networking")
	Icon       string     // Unicode icon for display
	Age        string     // Human-readable age (e.g., "5m", "2h", "3d")
	Ready      string     // Ready count (e.g., "2/3" for pods or deployments)
	Restarts      string    // Restart count (for pods)
	LastRestartAt time.Time // Most recent container restart time
	CreatedAt     time.Time // Creation timestamp for sorting
	Columns    []KeyValue // Additional resource fields for summary preview
	Selected   bool       // Whether this item is part of a multi-selection
	Deprecated bool       // Whether this resource uses a deprecated API version
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
	Name         string `json:"name" yaml:"name"`
	Context      string `json:"context" yaml:"context"`
	Namespace    string `json:"namespace" yaml:"namespace"`
	ResourceType string `json:"resource_type" yaml:"resource_type"` // resource ref string (group/version/resource)
	ResourceName string `json:"resource_name,omitempty" yaml:"resource_name,omitempty"`
}

// ActionMenuItem represents an entry in the action menu.
type ActionMenuItem struct {
	Label       string
	Description string
	Key         string // shortcut key
}

// TopLevelResourceTypes returns the curated list of resource types shown at level 1.
func TopLevelResourceTypes() []ResourceCategory {
	return []ResourceCategory{
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
			Name: "Cluster",
			Types: []ResourceTypeEntry{
				{DisplayName: "Namespaces", Kind: "Namespace", APIGroup: "", APIVersion: "v1", Resource: "namespaces", Icon: "▣", Namespaced: false},
				{DisplayName: "Events", Kind: "Event", APIGroup: "", APIVersion: "v1", Resource: "events", Icon: "↯", Namespaced: true},
				{DisplayName: "Nodes", Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Icon: "⬡", Namespaced: false},
				{DisplayName: "Custom Resource Definitions", Kind: "CustomResourceDefinition", APIGroup: "apiextensions.k8s.io", APIVersion: "v1", Resource: "customresourcedefinitions", Icon: "⧫", Namespaced: false},
				{DisplayName: "API Services", Kind: "APIService", APIGroup: "apiregistration.k8s.io", APIVersion: "v1", Resource: "apiservices", Icon: "⧫", Namespaced: false},
			},
		},
		{
			Name: "Helm",
			Types: []ResourceTypeEntry{
				{DisplayName: "Releases", Kind: "HelmRelease", APIGroup: "_helm", APIVersion: "v1", Resource: "releases", Icon: "⎋", Namespaced: true},
			},
		},
		{
			Name: "argoproj.io",
			Types: []ResourceTypeEntry{
				{DisplayName: "Applications", Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications", Icon: "⎈", Namespaced: true},
				{DisplayName: "ApplicationSets", Kind: "ApplicationSet", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applicationsets", Icon: "⎈", Namespaced: true},
				{DisplayName: "AppProjects", Kind: "AppProject", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "appprojects", Icon: "⎈", Namespaced: true},
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
}

// FlattenedResourceTypes returns all resource types as a flat list of Items with category info.
// Conditional groups (ArgoCD, Gateway API, Flux, etc.) are hidden until CRD discovery confirms them.
// Helm is always shown since it doesn't depend on CRDs.
// Individual CRD-dependent entries (e.g., VPA) are hidden before discovery completes.
func FlattenedResourceTypes() []Item {
	return FlattenedResourceTypesFiltered(nil)
}

// FlattenedResourceTypesFiltered returns resource types as a flat list, optionally excluding
// CRD-dependent categories when the cluster doesn't have those CRDs installed.
// Core categories (Workloads, Config, Networking, Storage, Access Control, Cluster, Helm) are
// always shown. Other categories are only shown if their API group name appears in availableGroups.
// Individual resource types marked with RequiresCRD are also filtered out unless their
// API group/resource appears in availableGroups. When availableGroups is nil, CRD-dependent
// entries are hidden (safe default before discovery completes).
func FlattenedResourceTypesFiltered(availableGroups map[string]bool) []Item {
	var items []Item
	// Add Overview dashboard item at the top.
	items = append(items, Item{
		Name:     "Overview",
		Kind:     "__overview__",
		Extra:    "__overview__",
		Category: "",
		Icon:     "◎",
	})
	// Add Monitoring dashboard item right after Overview.
	items = append(items, Item{
		Name:     "Monitoring",
		Kind:     "__monitoring__",
		Extra:    "__monitoring__",
		Category: "",
		Icon:     "⊙",
	})
	for _, cat := range TopLevelResourceTypes() {
		if !coreCategories[cat.Name] {
			// CRD-based category: only show if the API group is detected.
			if availableGroups == nil || !availableGroups[cat.Name] {
				continue
			}
		}
		for _, rt := range cat.Types {
			if rt.RequiresCRD && (availableGroups == nil || !availableGroups[rt.APIGroup+"/"+rt.Resource]) {
				continue
			}
			items = append(items, Item{
				Name:       rt.DisplayName,
				Kind:       rt.Kind,
				Extra:      rt.ResourceRef(),
				Category:   cat.Name,
				Icon:       rt.Icon,
				Deprecated: rt.Deprecated,
			})
		}
	}
	return items
}

// MergeWithCRDs returns the flattened resource type list with discovered CRDs appended
// as additional categories grouped by API group. CRDs that match a built-in resource
// type (same group + resource) are filtered out to avoid duplicates.
func MergeWithCRDs(discovered []ResourceTypeEntry) []Item {
	// Build the set of all API groups and specific resources present as discovered CRDs.
	availableGroups := make(map[string]bool, len(discovered)*2)
	for _, crd := range discovered {
		availableGroups[crd.APIGroup] = true
		availableGroups[crd.APIGroup+"/"+crd.Resource] = true
	}

	// Helm always shows (uses helm binary, not CRDs).
	// No special handling needed — Helm is a core category.

	items := FlattenedResourceTypesFiltered(availableGroups)
	if len(discovered) == 0 {
		return items
	}

	// Build a set of built-in resource identifiers (group/resource) to filter duplicates.
	builtIn := make(map[string]bool)
	for _, cat := range TopLevelResourceTypes() {
		for _, rt := range cat.Types {
			builtIn[rt.APIGroup+"/"+rt.Resource] = true
		}
	}

	// Build a map of discovered CRD versions so built-in entries can be updated
	// to match the version the cluster actually serves.
	discoveredVersion := make(map[string]string, len(discovered))
	for _, crd := range discovered {
		discoveredVersion[crd.APIGroup+"/"+crd.Resource] = crd.APIVersion
	}

	// Update built-in items whose API version differs from what the cluster serves.
	// This prevents stale hardcoded versions from causing "resource not found" errors.
	for i := range items {
		key := items[i].Extra
		if key == "" {
			continue
		}
		// Extra format is "group/version/resource" — extract group and resource.
		parts := strings.SplitN(key, "/", 3)
		if len(parts) != 3 {
			continue
		}
		groupResource := parts[0] + "/" + parts[2]
		if ver, ok := discoveredVersion[groupResource]; ok && ver != parts[1] {
			items[i].Extra = parts[0] + "/" + ver + "/" + parts[2]
		}
	}

	// Build builtInCategoryForGroup dynamically from TopLevelResourceTypes.
	// Maps API groups to their category name so discovered CRDs from the same group
	// get inserted alongside built-in entries.
	builtInCategoryForGroup := make(map[string]string)
	for _, cat := range TopLevelResourceTypes() {
		if coreCategories[cat.Name] {
			continue // Don't map core resource groups
		}
		for _, rt := range cat.Types {
			builtInCategoryForGroup[rt.APIGroup] = cat.Name
		}
	}

	// Group CRDs by API group, filtering out built-in duplicates.
	grouped := make(map[string][]ResourceTypeEntry)
	var groupOrder []string
	for _, crd := range discovered {
		key := crd.APIGroup + "/" + crd.Resource
		if builtIn[key] {
			continue
		}
		if _, seen := grouped[crd.APIGroup]; !seen {
			groupOrder = append(groupOrder, crd.APIGroup)
		}
		grouped[crd.APIGroup] = append(grouped[crd.APIGroup], crd)
	}

	// Separate groups into pinned (user-configured) and unpinned, preserving order.
	pinnedSet := make(map[string]bool, len(PinnedGroups))
	for _, g := range PinnedGroups {
		pinnedSet[g] = true
	}

	var pinnedOrder, unpinnedOrder []string
	for _, group := range groupOrder {
		if pinnedSet[group] {
			pinnedOrder = append(pinnedOrder, group)
		} else {
			unpinnedOrder = append(unpinnedOrder, group)
		}
	}

	// Sort pinnedOrder to match the user's configured order in PinnedGroups.
	pinnedOrderMap := make(map[string]int, len(PinnedGroups))
	for i, g := range PinnedGroups {
		pinnedOrderMap[g] = i
	}
	sort.SliceStable(pinnedOrder, func(i, j int) bool {
		return pinnedOrderMap[pinnedOrder[i]] < pinnedOrderMap[pinnedOrder[j]]
	})

	// Process groups: pinned first, then unpinned.
	orderedGroups := make([]string, 0, len(pinnedOrder)+len(unpinnedOrder))
	orderedGroups = append(orderedGroups, pinnedOrder...)
	orderedGroups = append(orderedGroups, unpinnedOrder...)

	// Build items for each discovered group (non-duplicate CRDs only).
	for _, group := range orderedGroups {
		categoryName, isBuiltInGroup := builtInCategoryForGroup[group]
		if !isBuiltInGroup {
			categoryName = group
		}

		crdItems := make([]Item, 0, len(grouped[group]))
		for _, rt := range grouped[group] {
			crdItems = append(crdItems, Item{
				Name:       rt.DisplayName,
				Kind:       rt.Kind,
				Extra:      rt.ResourceRef(),
				Category:   categoryName,
				Icon:       rt.Icon,
				Deprecated: rt.Deprecated,
			})
		}

		if isBuiltInGroup {
			// Merge extra discovered CRDs into their built-in category.
			insertIdx := -1
			for i, it := range items {
				if it.Category == categoryName {
					insertIdx = i
				}
			}
			if insertIdx >= 0 {
				tail := make([]Item, len(items[insertIdx+1:]))
				copy(tail, items[insertIdx+1:])
				items = append(items[:insertIdx+1], crdItems...)
				items = append(items, tail...)
				continue
			}
		}

		// Append non-built-in discovered groups at the end (sorted below).
		items = append(items, crdItems...)
	}

	// Sort all non-core CRD categories alphabetically by category name,
	// with pinned groups appearing first (in user-configured order).
	// Core categories retain their fixed position at the top.
	var coreItems, pinnedItems, crdItemsList []Item
	for _, it := range items {
		switch {
		case coreCategories[it.Category] || it.Category == "":
			coreItems = append(coreItems, it)
		case pinnedSet[it.Category]:
			pinnedItems = append(pinnedItems, it)
		default:
			crdItemsList = append(crdItemsList, it)
		}
	}

	// Sort pinned items by the user's configured pinned group order.
	sort.SliceStable(pinnedItems, func(i, j int) bool {
		return pinnedOrderMap[pinnedItems[i].Category] < pinnedOrderMap[pinnedItems[j].Category]
	})

	// Sort CRD items alphabetically by category name.
	sort.SliceStable(crdItemsList, func(i, j int) bool {
		return crdItemsList[i].Category < crdItemsList[j].Category
	})

	items = make([]Item, 0, len(coreItems)+len(pinnedItems)+len(crdItemsList))
	items = append(items, coreItems...)
	items = append(items, pinnedItems...)
	items = append(items, crdItemsList...)

	return items
}

// ResourceRef returns the "group/version/resource" reference string.
func (r ResourceTypeEntry) ResourceRef() string {
	return r.APIGroup + "/" + r.APIVersion + "/" + r.Resource
}

// FindResourceTypeByKind searches for a ResourceTypeEntry matching the given kind
// across all built-in types and the provided CRDs.
func FindResourceTypeByKind(kind string, crds []ResourceTypeEntry) (ResourceTypeEntry, bool) {
	for _, cat := range TopLevelResourceTypes() {
		for _, rt := range cat.Types {
			if rt.Kind == kind {
				return rt, true
			}
		}
	}
	for _, crd := range crds {
		if crd.Kind == kind {
			return crd, true
		}
	}
	return ResourceTypeEntry{}, false
}

// FindResourceType looks up a ResourceTypeEntry by its ref string in built-in types.
func FindResourceType(ref string) (ResourceTypeEntry, bool) {
	return FindResourceTypeIn(ref, nil)
}

// FindResourceTypeIn looks up a ResourceTypeEntry by its ref string, searching both
// built-in types and the provided additional entries (e.g., discovered CRDs).
// The ref format is "group/version/resource". If a built-in entry matches by
// group and resource but has a different version (e.g., hardcoded v1beta1 vs
// cluster-served v1), the version from the ref is used.
func FindResourceTypeIn(ref string, additional []ResourceTypeEntry) (ResourceTypeEntry, bool) {
	// Parse the ref to extract version for potential override.
	refParts := strings.SplitN(ref, "/", 3)

	// Build a lookup of discovered CRDs by group/resource for enriching built-in types
	// with PrinterColumns from CRD discovery.
	discoveredByGR := make(map[string]*ResourceTypeEntry, len(additional))
	for i := range additional {
		key := additional[i].APIGroup + "/" + additional[i].Resource
		discoveredByGR[key] = &additional[i]
	}

	for _, cat := range TopLevelResourceTypes() {
		for _, rt := range cat.Types {
			matched := false
			if rt.ResourceRef() == ref {
				matched = true
			} else if len(refParts) == 3 && rt.APIGroup == refParts[0] && rt.Resource == refParts[2] {
				// Match by group/resource, override version from ref.
				rt.APIVersion = refParts[1]
				matched = true
			}
			if matched {
				// Enrich built-in type with PrinterColumns from discovered CRDs.
				grKey := rt.APIGroup + "/" + rt.Resource
				if crd, ok := discoveredByGR[grKey]; ok && len(crd.PrinterColumns) > 0 {
					rt.PrinterColumns = crd.PrinterColumns
				}
				return rt, true
			}
		}
	}
	for _, rt := range additional {
		if rt.ResourceRef() == ref {
			return rt, true
		}
	}
	return ResourceTypeEntry{}, false
}

// IsScaleableKind returns true if the given kind supports the scale operation.
func IsScaleableKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "ReplicaSet":
		return true
	default:
		return false
	}
}

// IsRestartableKind returns true if the given kind supports the restart operation.
func IsRestartableKind(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return true
	default:
		return false
	}
}

// ActionsForContainer returns the action menu items for a container.
func ActionsForContainer() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Logs", Description: "View container logs", Key: "l"},
		{Label: "Exec", Description: "Execute command in container", Key: "e"},
		{Label: "Attach", Description: "Attach to running container", Key: "A"},
		{Label: "Debug", Description: "Debug container with ephemeral container", Key: "b"},
		{Label: "Describe", Description: "Describe parent pod", Key: "d"},
		{Label: "Events", Description: "Show related events", Key: "v"},
	}
}

// ActionsForBulk returns the action menu items available for bulk operations.
func ActionsForBulk() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Logs", Description: "Stream logs from selected resources", Key: "L"},
		{Label: "Delete", Description: "Delete selected resources", Key: "D"},
		{Label: "Force Delete", Description: "Force delete selected resources", Key: "X"},
		{Label: "Scale", Description: "Scale selected resources", Key: "s"},
		{Label: "Restart", Description: "Restart selected resources", Key: "r"},
		{Label: "Labels / Annotations", Description: "Edit labels and annotations", Key: "l"},
		{Label: "Diff", Description: "Compare YAML of two resources", Key: "d"},
	}
}

// ActionsForKind returns the action menu items appropriate for a given resource kind.
func ActionsForKind(kind string) []ActionMenuItem {
	switch kind {
	case "Pod":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in container", Key: "e"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Debug", Description: "Debug pod with ephemeral container", Key: "b"},
			{Label: "Port Forward", Description: "Forward local port to pod", Key: "p"},
			{Label: "Startup Analysis", Description: "Analyze pod startup timing", Key: "S"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this pod", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Deployment":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "e"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Scale", Description: "Scale replica count", Key: "s"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Rollback", Description: "Rollback to previous revision", Key: "R"},
			{Label: "Port Forward", Description: "Forward local port to deployment pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this deployment", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "ReplicaSet":
		return []ActionMenuItem{
			{Label: "Scale", Description: "Scale replica count", Key: "s"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this replicaset", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Node":
		return []ActionMenuItem{
			{Label: "Cordon", Description: "Mark node as unschedulable", Key: "c"},
			{Label: "Uncordon", Description: "Mark node as schedulable", Key: "u"},
			{Label: "Drain", Description: "Drain node (evict pods)", Key: "n"},
			{Label: "Taint", Description: "Add taint to node", Key: "t"},
			{Label: "Untaint", Description: "Remove taint from node", Key: "T"},
			{Label: "Shell", Description: "Open shell on node via debug pod", Key: "s"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "HorizontalPodAutoscaler":
		return []ActionMenuItem{
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this HPA", Key: "D"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "StatefulSet":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "e"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Scale", Description: "Scale replica count", Key: "s"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Port Forward", Description: "Forward local port to statefulset pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this statefulset", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "DaemonSet":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "e"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Restart", Description: "Rolling restart", Key: "r"},
			{Label: "Port Forward", Description: "Forward local port to daemonset pod", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this daemonset", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Job":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View job logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "e"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this job", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "CronJob":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View cronjob logs", Key: "l"},
			{Label: "Exec", Description: "Execute command in pod container", Key: "e"},
			{Label: "Attach", Description: "Attach to running container", Key: "A"},
			{Label: "Trigger", Description: "Create a Job from this CronJob", Key: "t"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this cronjob", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Service":
		return []ActionMenuItem{
			{Label: "Logs", Description: "View aggregated pod logs", Key: "l"},
			{Label: "Exec", Description: "Exec into pod behind service", Key: "e"},
			{Label: "Attach", Description: "Attach to pod behind service", Key: "A"},
			{Label: "Port Forward", Description: "Forward local port to service", Key: "p"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this service", Key: "D"},
			{Label: "Debug Pod", Description: "Run alpine debug pod in namespace", Key: "b"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Application":
		return []ActionMenuItem{
			{Label: "Sync", Description: "Sync application", Key: "s"},
			{Label: "Diff", Description: "Diff live vs desired state", Key: "f"},
			{Label: "Refresh", Description: "Hard refresh application", Key: "R"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this application", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "PersistentVolumeClaim":
		return []ActionMenuItem{
			{Label: "Debug Mount", Description: "Run debug pod with this PVC mounted", Key: "m"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this PVC", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Ingress":
		return []ActionMenuItem{
			{Label: "Open in Browser", Description: "Open first host URL in browser", Key: "o"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this ingress", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "HelmRelease":
		return []ActionMenuItem{
			{Label: "Values", Description: "View user-supplied values", Key: "V"},
			{Label: "All Values", Description: "View all values (including defaults)", Key: "A"},
			{Label: "Edit Values", Description: "Edit values in $EDITOR", Key: "E"},
			{Label: "Rollback", Description: "Rollback to previous revision", Key: "R"},
			{Label: "Describe", Description: "Show release info", Key: "d"},
			{Label: "Delete", Description: "Uninstall this release", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Kustomization":
		// FluxCD Kustomization
		return []ActionMenuItem{
			{Label: "Reconcile", Description: "Trigger reconciliation", Key: "r"},
			{Label: "Suspend", Description: "Suspend reconciliation", Key: "s"},
			{Label: "Resume", Description: "Resume reconciliation", Key: "R"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "GitRepository", "HelmRepository", "HelmChart", "OCIRepository", "Bucket":
		// FluxCD Source resources
		return []ActionMenuItem{
			{Label: "Reconcile", Description: "Trigger reconciliation", Key: "r"},
			{Label: "Suspend", Description: "Suspend reconciliation", Key: "s"},
			{Label: "Resume", Description: "Resume reconciliation", Key: "R"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Alert", "Provider", "Receiver":
		// FluxCD Notification resources
		return []ActionMenuItem{
			{Label: "Reconcile", Description: "Trigger reconciliation", Key: "r"},
			{Label: "Suspend", Description: "Suspend reconciliation", Key: "s"},
			{Label: "Resume", Description: "Resume reconciliation", Key: "R"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "ImageRepository", "ImagePolicy", "ImageUpdateAutomation":
		// FluxCD Image resources
		return []ActionMenuItem{
			{Label: "Reconcile", Description: "Trigger reconciliation", Key: "r"},
			{Label: "Suspend", Description: "Suspend reconciliation", Key: "s"},
			{Label: "Resume", Description: "Resume reconciliation", Key: "R"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Certificate", "CertificateRequest":
		// cert-manager Certificate resources
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Issuer", "ClusterIssuer":
		// cert-manager Issuer resources
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "Order", "Challenge":
		// cert-manager ACME resources
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
		}
	case "NetworkPolicy":
		return []ActionMenuItem{
			{Label: "Visualize", Description: "Visualize network policy rules", Key: "V"},
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this network policy", Key: "D"},
			{Label: "Events", Description: "Show related events", Key: "v"},
			{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
		}
	default:
		return []ActionMenuItem{
			{Label: "Describe", Description: "Describe resource", Key: "d"},
			{Label: "Edit", Description: "Edit resource YAML", Key: "E"},
			{Label: "Delete", Description: "Delete this resource", Key: "D"},
			{Label: "Labels / Annotations", Description: "Edit labels and annotations", Key: "l"},
			{Label: "Events", Description: "Show related events", Key: "v"},
			{Label: "Permissions", Description: "Check RBAC permissions", Key: "P"},
		}
	}

	// Note: "Permissions" action is also available for all kinds — it's appended
	// by the action dispatch logic if not present in the kind-specific list.
}

// ActionsForPortForward returns the action menu items for a port forward entry.
func ActionsForPortForward() []ActionMenuItem {
	return []ActionMenuItem{
		{Label: "Stop", Description: "Stop this port forward", Key: "s"},
		{Label: "Restart", Description: "Restart this port forward", Key: "r"},
		{Label: "Remove", Description: "Remove this entry", Key: "D"},
		{Label: "Open in Browser", Description: "Open localhost port in browser", Key: "O"},
	}
}

