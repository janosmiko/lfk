package model

import "strings"

// ResourceRef returns the "group/version/resource" reference string.
func (r ResourceTypeEntry) ResourceRef() string {
	return r.APIGroup + "/" + r.APIVersion + "/" + r.Resource
}

// FindResourceTypeByKindAndGroup searches the given discovered resource set
// for a ResourceTypeEntry matching both Kind and APIGroup. This is the
// disambiguating sibling of FindResourceTypeByKind: when two resources share
// a Kind name across API groups (e.g., VaultDynamicSecret), callers must
// pass the APIGroup to resolve to the right one.
func FindResourceTypeByKindAndGroup(kind, apiGroup string, discovered []ResourceTypeEntry) (ResourceTypeEntry, bool) {
	for _, rt := range discovered {
		if rt.Kind == kind && rt.APIGroup == apiGroup {
			return rt, true
		}
	}
	return ResourceTypeEntry{}, false
}

// FindResourceTypeByKind searches the given discovered resource set for an
// entry matching the given Kind. When multiple groups define the same Kind
// the first match wins — use FindResourceTypeByKindAndGroup to disambiguate.
func FindResourceTypeByKind(kind string, discovered []ResourceTypeEntry) (ResourceTypeEntry, bool) {
	for _, rt := range discovered {
		if rt.Kind == kind {
			return rt, true
		}
	}
	return ResourceTypeEntry{}, false
}

// FindResourceTypeIn searches the given discovered resource set for an entry
// whose ResourceRef() ("group/version/resource") matches ref.
func FindResourceTypeIn(ref string, discovered []ResourceTypeEntry) (ResourceTypeEntry, bool) {
	for _, rt := range discovered {
		if rt.ResourceRef() == ref {
			return rt, true
		}
	}
	return ResourceTypeEntry{}, false
}

// FindResourceType is kept as a convenience wrapper that searches without
// a discovered slice. It exists for callers that don't have access to the
// discovered set yet; most callers should use FindResourceTypeIn directly.
func FindResourceType(ref string) (ResourceTypeEntry, bool) {
	return FindResourceTypeIn(ref, nil)
}

// DisplayNameFor returns a user-friendly label for a ResourceTypeEntry. It
// honors any DisplayName already set on the entry (PseudoResources sets one
// for Helm Releases and Port Forwards), then falls back to the curated name
// in BuiltInMetadata, then the Kind, and finally the raw Resource plural.
//
// Discovery-produced entries do not populate DisplayName themselves, so
// callers that want a label must go through this helper instead of reading
// rt.DisplayName directly.
func DisplayNameFor(rt ResourceTypeEntry) string {
	if rt.DisplayName != "" {
		return rt.DisplayName
	}
	if meta, ok := BuiltInMetadata[rt.APIGroup+"/"+rt.Resource]; ok && meta.DisplayName != "" {
		return meta.DisplayName
	}
	if rt.Kind != "" {
		return rt.Kind
	}
	if rt.Resource == "" {
		return ""
	}
	// Title-case the first byte of the plural resource name. Kubernetes
	// resource plurals are always lowercase ASCII, so this is safe.
	return strings.ToUpper(rt.Resource[:1]) + rt.Resource[1:]
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

// IsForceDeleteableKind returns true if the given kind supports the force delete operation
// (kubectl delete --grace-period=0 --force, without removing finalizers).
func IsForceDeleteableKind(kind string) bool {
	switch kind {
	case "Pod", "Job":
		return true
	default:
		return false
	}
}
