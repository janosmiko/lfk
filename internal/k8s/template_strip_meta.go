// Package k8s — template_strip_meta.go
// Deny-lists throughout, never prefix sweeps: a blanket "kubernetes.io/" strip
// would eat kubernetes.io/ingress.class, which the author wrote.
package k8s

import "strings"

// controllerGeneratedLabels also land in spec.template.metadata.labels, so a
// template that keeps one starts the new object with a stale hash or UID its
// controller then fights.
var controllerGeneratedLabels = []string{
	"pod-template-hash",
	"controller-uid",
	"job-name",
	"batch.kubernetes.io/controller-uid",
	"batch.kubernetes.io/job-name",
	"statefulset.kubernetes.io/pod-name",
	"apps.kubernetes.io/pod-index",
}

// helmChartLabels name Helm in the key itself, so the value never matters.
var helmChartLabels = []string{"helm.sh/chart"}

// helmValuedLabels go only when the value names Helm. app.kubernetes.io/managed-by
// is part of the standard recommended label set — another tool's name there is
// the author's own statement of ownership.
var helmValuedLabels = map[string][]string{
	"app.kubernetes.io/managed-by": {"Helm"},
	"heritage":                     {"Helm", "Tiller"},
}

// helmLegacyLabels are the Helm 2 convention. They are ordinary English words,
// so they go only on an object that carries some other Helm marker.
var helmLegacyLabels = []string{"release", "chart"}

var helmOwnershipAnnotations = []string{
	"meta.helm.sh/release-name",
	"meta.helm.sh/release-namespace",
}

// vendorRuntimeAnnotations are written by a CNI or a vendor controller and hold
// live state — a pod IP, a set of public endpoint addresses — that is a lie the
// moment the template is applied anywhere else.
var vendorRuntimeAnnotations = []string{
	"cni.projectcalico.org/podIP",
	"cni.projectcalico.org/podIPs",
	"cni.projectcalico.org/containerID",
	"k8s.v1.cni.cncf.io/network-status",
	"k8s.v1.cni.cncf.io/networks-status",
	"field.cattle.io/publicEndpoints",
	"cattle.io/timestamp",
	"management.cattle.io/pod-limits",
	"management.cattle.io/pod-requests",
}

// stripControllerLabels also clears spec.selector.matchLabels: a template whose
// pod labels lost the hash but whose selector still demands it selects nothing.
func stripControllerLabels(obj map[string]any) {
	md := childMap(obj, "metadata")
	deleteKeys(childMap(md, "labels"), controllerGeneratedLabels...)
	pruneEmptyChild(md, "labels")

	deleteKeys(selectorMatchLabels(obj), controllerGeneratedLabels...)
	pruneEmptyChild(childMap(childMap(obj, "spec"), "selector"), "matchLabels")

	for _, tmpl := range embeddedTemplates(obj) {
		tmd := childMap(tmpl, "metadata")
		deleteKeys(childMap(tmd, "labels"), controllerGeneratedLabels...)
		pruneEmptyChild(tmd, "labels")
		pruneEmptyChild(tmpl, "metadata")
	}
}

// stripHelmOwnership removes the labels and annotations that claim membership of
// a Helm release. An object carrying them into a cluster it was not installed in
// may be adopted by the next `helm upgrade`, or deleted by `helm uninstall`.
func stripHelmOwnership(obj map[string]any) {
	legacy := hasHelmSignal(obj)
	protected := selectorMatchLabels(obj)

	md := childMap(obj, "metadata")
	dropHelmLabels(childMap(md, "labels"), legacy, nil)
	deleteKeys(childMap(md, "annotations"), helmOwnershipAnnotations...)
	pruneEmptyChild(md, "labels")
	pruneEmptyChild(md, "annotations")

	for _, tmpl := range embeddedTemplates(obj) {
		tmd := childMap(tmpl, "metadata")
		dropHelmLabels(childMap(tmd, "labels"), legacy, protected)
		deleteKeys(childMap(tmd, "annotations"), helmOwnershipAnnotations...)
		pruneEmptyChild(tmd, "labels")
		pruneEmptyChild(tmd, "annotations")
		pruneEmptyChild(tmpl, "metadata")
	}
}

// stripVendorRuntimeAnnotations removes the CNI and vendor-controller
// annotations from the object and from every embedded template.
func stripVendorRuntimeAnnotations(obj map[string]any) {
	md := childMap(obj, "metadata")
	deleteKeys(childMap(md, "annotations"), vendorRuntimeAnnotations...)
	pruneEmptyChild(md, "annotations")

	for _, tmpl := range embeddedTemplates(obj) {
		tmd := childMap(tmpl, "metadata")
		deleteKeys(childMap(tmd, "annotations"), vendorRuntimeAnnotations...)
		pruneEmptyChild(tmd, "annotations")
		pruneEmptyChild(tmpl, "metadata")
	}
}

// dropHelmLabels removes the Helm markers from one labels block. Keys in
// protected stay: a pod-template label that the selector also demands cannot be
// removed from one side only.
func dropHelmLabels(labels map[string]any, legacy bool, protected map[string]any) {
	if labels == nil {
		return
	}
	drop := func(key string) {
		if _, ok := protected[key]; ok {
			return
		}
		delete(labels, key)
	}
	for _, key := range helmChartLabels {
		drop(key)
	}
	for key, values := range helmValuedLabels {
		if matchesAnyValue(labels[key], values) {
			drop(key)
		}
	}
	if !legacy {
		return
	}
	for _, key := range helmLegacyLabels {
		drop(key)
	}
}

// hasHelmSignal reports whether the object carries a marker that names Helm
// unambiguously, which is what licenses stripping the ambiguous legacy labels.
func hasHelmSignal(obj map[string]any) bool {
	md := childMap(obj, "metadata")
	labels := childMap(md, "labels")
	for _, key := range helmChartLabels {
		if _, ok := labels[key]; ok {
			return true
		}
	}
	for key, values := range helmValuedLabels {
		if matchesAnyValue(labels[key], values) {
			return true
		}
	}
	annotations := childMap(md, "annotations")
	for _, key := range helmOwnershipAnnotations {
		if _, ok := annotations[key]; ok {
			return true
		}
	}
	return false
}

func matchesAnyValue(got any, want []string) bool {
	s, ok := got.(string)
	if !ok {
		return false
	}
	for _, w := range want {
		if strings.EqualFold(s, w) {
			return true
		}
	}
	return false
}

// selectorMatchLabels returns spec.selector.matchLabels, or nil when the kind
// has no label selector.
func selectorMatchLabels(obj map[string]any) map[string]any {
	return childMap(childMap(childMap(obj, "spec"), "selector"), "matchLabels")
}
