package app

import "github.com/janosmiko/lfk/internal/model"

// kubectlResourceArg returns how kubectl should address rt on the command
// line: the bare plural for core types, "resource.group" for everything else.
//
// A bare plural is ambiguous the moment two groups define the same Kind —
// kubectl picks whichever group discovery returns first, so an action on one
// CRD lands on the other (issue #562). Every kubectl call site builds its
// resource argument through here; the group is always at hand, because the
// ResourceTypeEntry the action was launched from carries it.
func kubectlResourceArg(rt model.ResourceTypeEntry) string {
	if rt.Resource == "" || rt.APIGroup == "" {
		return rt.Resource
	}
	return rt.Resource + "." + rt.APIGroup
}
