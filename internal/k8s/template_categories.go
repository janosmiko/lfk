// Package k8s — template_categories.go
// The optional half of the template strip: what template_strip.go removes
// unconditionally is a correctness requirement, what is here is the user's call.
package k8s

import "maps"

// TemplateCategory names one group of fields the template strip may remove.
// The string values are the on-disk keys of the user's saved choice, so they
// are stable.
type TemplateCategory string

const (
	TemplateNamespace     TemplateCategory = "namespace"
	TemplateLabels        TemplateCategory = "labels"
	TemplateAnnotations   TemplateCategory = "annotations"
	TemplateHelmOwnership TemplateCategory = "helm_ownership"
	TemplateVendorRuntime TemplateCategory = "vendor_runtime"
	TemplateSecretValues  TemplateCategory = "secret_values"
)

// TemplateCategories is every optional category, in the order the picker shows.
var TemplateCategories = []TemplateCategory{
	TemplateNamespace,
	TemplateLabels,
	TemplateAnnotations,
	TemplateHelmOwnership,
	TemplateVendorRuntime,
	TemplateSecretValues,
}

// TemplateStripSet records which optional categories the strip removes. An
// absent key means "keep", so a partially-written or hand-edited saved choice
// degrades to keeping a field rather than to silently dropping one.
type TemplateStripSet map[TemplateCategory]bool

// DefaultTemplateStripSet is what the two-keystroke export does for a user who
// never opens the picker.
func DefaultTemplateStripSet() TemplateStripSet {
	return TemplateStripSet{
		TemplateNamespace:     true,
		TemplateLabels:        false,
		TemplateAnnotations:   false,
		TemplateHelmOwnership: true,
		TemplateVendorRuntime: true,
		TemplateSecretValues:  true,
	}
}

// Clone returns an independent copy, so an edit in the picker cannot reach the
// set an in-flight export already captured.
func (s TemplateStripSet) Clone() TemplateStripSet {
	out := make(TemplateStripSet, len(s))
	maps.Copy(out, s)
	return out
}

// stripAllLabels removes the author's labels from the object, and from every
// embedded template except the keys the selector demands — dropping one of
// those would leave the workload selecting nothing.
func stripAllLabels(obj map[string]any) {
	md := childMap(obj, "metadata")
	delete(md, "labels")

	protected := selectorMatchLabels(obj)
	for _, tmpl := range embeddedTemplates(obj) {
		tmd := childMap(tmpl, "metadata")
		for key := range childMap(tmd, "labels") {
			if _, ok := protected[key]; !ok {
				delete(childMap(tmd, "labels"), key)
			}
		}
		pruneEmptyChild(tmd, "labels")
		pruneEmptyChild(tmpl, "metadata")
	}
}

func stripAllAnnotations(obj map[string]any) {
	delete(childMap(obj, "metadata"), "annotations")
	for _, tmpl := range embeddedTemplates(obj) {
		tmd := childMap(tmpl, "metadata")
		delete(tmd, "annotations")
		pruneEmptyChild(tmpl, "metadata")
	}
}
