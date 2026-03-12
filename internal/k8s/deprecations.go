// Package k8s provides Kubernetes API access for the TUI application.
package k8s

// DeprecationInfo describes a deprecated API version.
type DeprecationInfo struct {
	RemovedIn   string // Kubernetes version where the API was removed (e.g., "1.22")
	Replacement string // The replacement API group/version (e.g., "networking.k8s.io/v1")
	Message     string // Human-readable deprecation message
}

// deprecatedAPIs maps "group/version/resource" to deprecation info.
// This covers the most common Kubernetes API deprecations.
var deprecatedAPIs = map[string]DeprecationInfo{
	// Removed in 1.22
	"extensions/v1beta1/ingresses": {
		RemovedIn:   "1.22",
		Replacement: "networking.k8s.io/v1",
		Message:     "Ingress extensions/v1beta1 removed in 1.22, use networking.k8s.io/v1",
	},
	"networking.k8s.io/v1beta1/ingresses": {
		RemovedIn:   "1.22",
		Replacement: "networking.k8s.io/v1",
		Message:     "Ingress networking.k8s.io/v1beta1 removed in 1.22, use networking.k8s.io/v1",
	},
	"rbac.authorization.k8s.io/v1beta1/roles": {
		RemovedIn:   "1.22",
		Replacement: "rbac.authorization.k8s.io/v1",
		Message:     "RBAC v1beta1 removed in 1.22, use v1",
	},
	"rbac.authorization.k8s.io/v1beta1/rolebindings": {
		RemovedIn:   "1.22",
		Replacement: "rbac.authorization.k8s.io/v1",
		Message:     "RBAC v1beta1 removed in 1.22, use v1",
	},
	"rbac.authorization.k8s.io/v1beta1/clusterroles": {
		RemovedIn:   "1.22",
		Replacement: "rbac.authorization.k8s.io/v1",
		Message:     "RBAC v1beta1 removed in 1.22, use v1",
	},
	"rbac.authorization.k8s.io/v1beta1/clusterrolebindings": {
		RemovedIn:   "1.22",
		Replacement: "rbac.authorization.k8s.io/v1",
		Message:     "RBAC v1beta1 removed in 1.22, use v1",
	},
	"admissionregistration.k8s.io/v1beta1/mutatingwebhookconfigurations": {
		RemovedIn:   "1.22",
		Replacement: "admissionregistration.k8s.io/v1",
		Message:     "AdmissionRegistration v1beta1 removed in 1.22, use v1",
	},
	"admissionregistration.k8s.io/v1beta1/validatingwebhookconfigurations": {
		RemovedIn:   "1.22",
		Replacement: "admissionregistration.k8s.io/v1",
		Message:     "AdmissionRegistration v1beta1 removed in 1.22, use v1",
	},
	// Removed in 1.25
	"policy/v1beta1/podsecuritypolicies": {
		RemovedIn:   "1.25",
		Replacement: "Pod Security Admission",
		Message:     "PodSecurityPolicy removed in 1.25, use Pod Security Admission",
	},
	"batch/v1beta1/cronjobs": {
		RemovedIn:   "1.25",
		Replacement: "batch/v1",
		Message:     "CronJob batch/v1beta1 removed in 1.25, use batch/v1",
	},
	// Removed in 1.26
	"autoscaling/v2beta2/horizontalpodautoscalers": {
		RemovedIn:   "1.26",
		Replacement: "autoscaling/v2",
		Message:     "HPA autoscaling/v2beta2 removed in 1.26, use autoscaling/v2",
	},
	// Removed in 1.27
	"storage.k8s.io/v1beta1/csistoragebuckets": {
		RemovedIn:   "1.27",
		Replacement: "storage.k8s.io/v1",
		Message:     "CSI storage v1beta1 removed in 1.27, use v1",
	},
	// Removed in 1.29
	"flowcontrol.apiserver.k8s.io/v1beta2/flowschemas": {
		RemovedIn:   "1.29",
		Replacement: "flowcontrol.apiserver.k8s.io/v1",
		Message:     "FlowControl v1beta2 removed in 1.29, use v1",
	},
	"flowcontrol.apiserver.k8s.io/v1beta2/prioritylevelconfigurations": {
		RemovedIn:   "1.29",
		Replacement: "flowcontrol.apiserver.k8s.io/v1",
		Message:     "FlowControl v1beta2 removed in 1.29, use v1",
	},
}

// CheckDeprecation looks up whether a given API group/version/resource is deprecated.
// Returns the deprecation info and true if found, zero value and false otherwise.
func CheckDeprecation(apiGroup, apiVersion, resource string) (DeprecationInfo, bool) {
	key := apiGroup + "/" + apiVersion + "/" + resource
	info, found := deprecatedAPIs[key]
	return info, found
}
