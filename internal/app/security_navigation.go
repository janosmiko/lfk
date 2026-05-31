// Package app — security_navigation.go
// Helpers used by the explorer's navigation dispatch to resolve security
// sidebar Items into synthetic ResourceTypeEntry values. Security source
// items are injected by the SecuritySourcesFn hook and never appear in
// the discovered API resource set, so the regular FindResourceTypeIn
// lookup misses them — this file fills the gap.
package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

// securityResourceTypeForItem returns the synthetic ResourceTypeEntry for
// a security source sidebar Item (Kind shaped "__security_<source>__"),
// or false when the item is not a security entry. The Item's Extra field
// already encodes "_security/v1/findings-<source>"; we rebuild a
// ResourceTypeEntry with that group/version/resource so GetResources
// dispatches via the _security virtual APIGroup branch.
func securityResourceTypeForItem(sel *model.Item) (model.ResourceTypeEntry, bool) {
	if sel == nil {
		return model.ResourceTypeEntry{}, false
	}
	// The loader sentinel sits in the sidebar while the availability
	// probe is in flight; it is not a real source so navigation must
	// no-op rather than dispatch a doomed fetch.
	if sel.Kind == model.SecurityLoaderKind {
		return model.ResourceTypeEntry{}, false
	}
	const prefix = "__security_"
	const suffix = "__"
	if !strings.HasPrefix(sel.Kind, prefix) || !strings.HasSuffix(sel.Kind, suffix) {
		return model.ResourceTypeEntry{}, false
	}
	// Reject kinds too short for prefix and suffix to occupy disjoint spans;
	// e.g. "__security__" (len 12) satisfies both Has* checks but its markers
	// overlap, so the slice below would have low > high and panic.
	if len(sel.Kind) < len(prefix)+len(suffix) {
		return model.ResourceTypeEntry{}, false
	}
	source := sel.Kind[len(prefix) : len(sel.Kind)-len(suffix)]
	if source == "" {
		return model.ResourceTypeEntry{}, false
	}
	return model.ResourceTypeEntry{
		DisplayName: sel.Name,
		Kind:        sel.Kind,
		APIGroup:    model.SecurityVirtualAPIGroup,
		APIVersion:  "v1",
		Resource:    "findings-" + source,
		Namespaced:  true,
		Icon:        sel.Icon,
	}, true
}
