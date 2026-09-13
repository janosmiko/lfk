// Package k8s provides Kubernetes API access for the TUI application.
package k8s

// DeprecationInfo describes a deprecated API version.
type DeprecationInfo struct {
	Message string // Human-readable deprecation message
}

// deprecatedAPIs maps "group/version/resource" to deprecation info.
// This covers the most common Kubernetes API deprecations.
var deprecatedAPIs = map[string]DeprecationInfo{
	// Removed in 1.22
	"extensions/v1beta1/ingresses": {
		Message: "Ingress extensions/v1beta1 removed in 1.22, use networking.k8s.io/v1",
	},
	"networking.k8s.io/v1beta1/ingresses": {
		Message: "Ingress networking.k8s.io/v1beta1 removed in 1.22, use networking.k8s.io/v1",
	},
	"rbac.authorization.k8s.io/v1beta1/roles": {
		Message: "RBAC v1beta1 removed in 1.22, use v1",
	},
	"rbac.authorization.k8s.io/v1beta1/rolebindings": {
		Message: "RBAC v1beta1 removed in 1.22, use v1",
	},
	"rbac.authorization.k8s.io/v1beta1/clusterroles": {
		Message: "RBAC v1beta1 removed in 1.22, use v1",
	},
	"rbac.authorization.k8s.io/v1beta1/clusterrolebindings": {
		Message: "RBAC v1beta1 removed in 1.22, use v1",
	},
	"admissionregistration.k8s.io/v1beta1/mutatingwebhookconfigurations": {
		Message: "AdmissionRegistration v1beta1 removed in 1.22, use v1",
	},
	"admissionregistration.k8s.io/v1beta1/validatingwebhookconfigurations": {
		Message: "AdmissionRegistration v1beta1 removed in 1.22, use v1",
	},
	// Removed in 1.25
	"policy/v1beta1/podsecuritypolicies": {
		Message: "PodSecurityPolicy removed in 1.25, use Pod Security Admission",
	},
	"batch/v1beta1/cronjobs": {
		Message: "CronJob batch/v1beta1 removed in 1.25, use batch/v1",
	},
	// Removed in 1.26
	"autoscaling/v2beta2/horizontalpodautoscalers": {
		Message: "HPA autoscaling/v2beta2 removed in 1.26, use autoscaling/v2",
	},
	// Removed in 1.27
	"storage.k8s.io/v1beta1/csistoragebuckets": {
		Message: "CSI storage v1beta1 removed in 1.27, use v1",
	},
	// Removed in 1.29
	"flowcontrol.apiserver.k8s.io/v1beta2/flowschemas": {
		Message: "FlowControl v1beta2 removed in 1.29, use v1",
	},
	"flowcontrol.apiserver.k8s.io/v1beta2/prioritylevelconfigurations": {
		Message: "FlowControl v1beta2 removed in 1.29, use v1",
	},
}

// CheckDeprecation looks up whether a given API group/version/resource is deprecated.
// Returns the deprecation info and true if found, zero value and false otherwise.
func CheckDeprecation(apiGroup, apiVersion, resource string) (DeprecationInfo, bool) {
	key := apiGroup + "/" + apiVersion + "/" + resource
	info, found := deprecatedAPIs[key]
	return info, found
}
