package app

import (
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

// whoCanCollectResources flattens the Can-I groups into a sorted,
// de-duplicated list of resource names. The Who-Can picker uses this
// as the canonical "what resources can I query?" list — same source
// of truth as the Can-I browser, so the user sees a familiar set.
//
// Resources without slashes (e.g. "pods") and subresources ("pods/exec")
// both appear; dedupe is by the full string so the user can pick a
// subresource explicitly if they care to.
func whoCanCollectResources(groups []model.CanIGroup) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 64)
	for _, g := range groups {
		for _, r := range g.Resources {
			name := r.Resource
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// whoCanFilterResources narrows the full resource list to entries that
// contain the (case-insensitive) substring in q. Empty q returns the
// full list unchanged. Substring match (not prefix) so users can find
// "pods/exec" by typing "exec".
func whoCanFilterResources(all []string, q string) []string {
	if q == "" {
		return all
	}
	q = strings.ToLower(q)
	out := make([]string, 0, len(all))
	for _, r := range all {
		if strings.Contains(strings.ToLower(r), q) {
			out = append(out, r)
		}
	}
	return out
}
