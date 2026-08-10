package k8s

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// dependentSummaryKinds is how many kinds the summary names before it gives up
// and prints one total. Four kinds already overflow the confirm box, and a
// truncated list reads as a wrong list.
const dependentSummaryKinds = 3

// DependentRef is one object in the owner graph, reduced to what the walk
// needs: its own identity, its kind for the summary, and the owners that would
// take it with them.
type DependentRef struct {
	Kind      string
	UID       string
	OwnerUIDs []string
}

// DependentCount is how many objects a cascading delete removes along with the
// targets, grouped by kind.
type DependentCount struct {
	Total  int
	ByKind map[string]int

	// Uncounted is how many selected rows the walk could not resolve, so a
	// bulk total says what it left out rather than reading as complete.
	Uncounted int
}

// CountDependents walks the owner graph down from roots and counts everything
// the garbage collector would follow. It makes no cluster calls, so the caller
// fetches the candidate objects first.
//
// The walk is breadth-first over ownerReferences, so a deep chain
// (Deployment to ReplicaSet to Pod) is counted in full. An object reached by
// two owners is counted once, and an owner cycle terminates rather than
// looping.
func CountDependents(objects []DependentRef, roots []string) DependentCount {
	out := DependentCount{ByKind: make(map[string]int)}

	byOwner := make(map[string][]int, len(objects))
	for i, o := range objects {
		if o.UID == "" {
			// Nothing can reach an object with no UID, and nothing owns one.
			continue
		}
		for _, owner := range o.OwnerUIDs {
			byOwner[owner] = append(byOwner[owner], i)
		}
	}

	// The roots start out seen, so a target that appears among the candidate
	// objects is not counted as its own dependent.
	seen := make(map[string]bool, len(roots))
	queue := make([]string, 0, len(roots))
	for _, r := range roots {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		queue = append(queue, r)
	}

	for len(queue) > 0 {
		owner := queue[0]
		queue = queue[1:]
		for _, i := range byOwner[owner] {
			child := objects[i]
			if seen[child.UID] {
				continue
			}
			seen[child.UID] = true
			out.Total++
			out.ByKind[child.Kind]++
			queue = append(queue, child.UID)
		}
	}
	return out
}

// Summary names what the count holds, biggest group first. Past
// dependentSummaryKinds it states one total instead, because a confirm box has
// one line for this and a half-listed set of kinds is misleading.
func (d DependentCount) Summary() string {
	if d.Total == 0 {
		return ""
	}
	if len(d.ByKind) > dependentSummaryKinds {
		return fmt.Sprintf("%d %s", d.Total, pluralKind("object", d.Total))
	}

	kinds := make([]string, 0, len(d.ByKind))
	for kind := range d.ByKind {
		kinds = append(kinds, kind)
	}
	// Biggest group first; ties by kind so the line does not reshuffle
	// between renders.
	sort.Slice(kinds, func(i, j int) bool {
		if d.ByKind[kinds[i]] != d.ByKind[kinds[j]] {
			return d.ByKind[kinds[i]] > d.ByKind[kinds[j]]
		}
		return kinds[i] < kinds[j]
	})

	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		n := d.ByKind[kind]
		parts = append(parts, fmt.Sprintf("%d %s", n, pluralKind(kind, n)))
	}
	return strings.Join(parts, ", ")
}

// alreadyPluralKinds are Kinds the API spells as a plural already. Adding the
// usual suffix to one of these produces "endpointses".
var alreadyPluralKinds = map[string]bool{"endpoints": true}

// pluralKind renders a Kubernetes Kind the way the resource name reads:
// lowercase, and pluralized the way the API does it.
func pluralKind(kind string, n int) string {
	lower := strings.ToLower(kind)
	if n == 1 || alreadyPluralKinds[lower] {
		return lower
	}
	// Matches the API's own plural forms: ingresses, not ingresss.
	for _, suffix := range []string{"s", "x", "z", "ch", "sh"} {
		if strings.HasSuffix(lower, suffix) {
			return lower + "es"
		}
	}
	if len(lower) > 1 && strings.HasSuffix(lower, "y") && !isVowel(lower[len(lower)-2]) {
		return lower[:len(lower)-1] + "ies"
	}
	return lower + "s"
}

func isVowel(b byte) bool {
	return slices.Contains([]byte("aeiou"), b)
}
