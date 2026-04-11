package model

import (
	"sort"
	"strings"
)

// BuildSidebarItems assembles the navigation sidebar from a discovered
// resource set. It walks every discovered entry, looks up display metadata
// in BuiltInMetadata, and buckets the result into the correct category.
//
// Resources whose group/resource key is not in BuiltInMetadata are:
//   - hidden if the group is in CoreK8sGroups (obscure built-ins)
//   - rendered as generic CRD entries under their API group otherwise
//
// The dashboard pseudo-categories (Cluster overview, Monitoring) are
// injected separately because they are navigation-only items with no
// underlying resource type. Helm "Releases" and the "Port Forwards" view
// are delivered via the discovered set through PseudoResources() so they
// flow through the same metadata-overlay path as real API resources.
func BuildSidebarItems(discovered []ResourceTypeEntry) []Item {
	items := injectPseudoCategoryHeaders()
	items = append(items, injectSecuritySourceItems()...)

	categorized, crdGroups := partitionDiscovered(discovered)
	items = append(items, categorized...)
	items = append(items, crdGroups...)

	return sortSidebarItems(items)
}

// injectSecuritySourceItems returns one sidebar Item per registered security
// source (e.g., Trivy, Heuristic). The items are built from the live
// SecuritySourcesFn hook; when the hook is unset (no sources registered)
// the Security category remains empty but still reserved in CoreCategories.
//
// Each entry uses the virtual _security APIGroup and a synthetic Kind like
// "__security_trivy-operator__". Client.GetResources recognises this group
// and dispatches to the security.Manager.
func injectSecuritySourceItems() []Item {
	if SecuritySourcesFn == nil {
		return nil
	}
	entries := SecuritySourcesFn()
	if len(entries) == 0 {
		return nil
	}
	items := make([]Item, 0, len(entries))
	for _, src := range entries {
		displayName := src.DisplayName
		if src.Count >= 0 {
			displayName = src.DisplayName + " (" + intToStr(src.Count) + ")"
		}
		items = append(items, Item{
			Name:     displayName,
			Kind:     "__security_" + src.SourceName + "__",
			Extra:    SecurityVirtualAPIGroup + "/v1/findings-" + src.SourceName,
			Category: "Security",
			Icon:     src.Icon,
		})
	}
	return items
}

// intToStr is a small fmt-free int-to-string helper. Avoids pulling fmt
// into this package just for a single Sprintf in the security injector.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// partitionDiscovered walks the discovered set and produces two slices:
// items that matched BuiltInMetadata (curated, with category/icon), and
// items for unknown resources in non-core groups that should appear as
// generic CRD entries.
//
// When ShowRareResources is false (default), entries marked Rare in
// BuiltInMetadata are skipped, and uncategorized core Kubernetes
// resources (TokenReview, Binding, ComponentStatus, etc.) remain hidden.
// When ShowRareResources is true, both sets surface: rare curated entries
// appear in their assigned category, and uncategorized core resources
// appear under the synthetic "Advanced" category.
func partitionDiscovered(discovered []ResourceTypeEntry) (categorized, crdGroups []Item) {
	for _, rt := range discovered {
		key := rt.APIGroup + "/" + rt.Resource
		if meta, ok := BuiltInMetadata[key]; ok {
			if meta.Rare && !ShowRareResources {
				continue
			}
			categorized = append(categorized, Item{
				Name:       meta.DisplayName,
				Kind:       rt.Kind,
				Extra:      rt.ResourceRef(),
				Category:   meta.Category,
				Icon:       meta.Icon,
				Deprecated: rt.Deprecated,
			})
			continue
		}
		if CoreK8sGroups[rt.APIGroup] {
			if !ShowRareResources {
				continue // hide obscure built-ins unless the user asked to see them
			}
			// Surface uncategorized core K8s resources under "Advanced".
			categorized = append(categorized, Item{
				Name:       titleCaseFirst(rt.Resource),
				Kind:       rt.Kind,
				Extra:      rt.ResourceRef(),
				Category:   AdvancedCategory,
				Icon:       "⧫",
				Deprecated: rt.Deprecated,
			})
			continue
		}
		// Unknown resource in a CRD group — show with generic icon.
		crdGroups = append(crdGroups, Item{
			Name:       titleCaseFirst(rt.Resource),
			Kind:       rt.Kind,
			Extra:      rt.ResourceRef(),
			Category:   rt.APIGroup,
			Icon:       "⧫",
			Deprecated: rt.Deprecated,
		})
	}
	return categorized, crdGroups
}

// injectPseudoCategoryHeaders returns navigation-only items that do not
// correspond to any resource type: the cluster overview dashboard and the
// monitoring dashboard. Helm releases and port forwards flow through the
// discovered resource set via PseudoResources() and are rendered by the
// normal metadata-overlay path, not by this function.
func injectPseudoCategoryHeaders() []Item {
	return []Item{
		{Name: "Cluster", Kind: "__overview__", Extra: "__overview__", Category: "Dashboards", Icon: "◎"},
		{Name: "Monitoring", Kind: "__monitoring__", Extra: "__monitoring__", Category: "Dashboards", Icon: "⊙"},
	}
}

// titleCaseFirst capitalizes the first character of s. Used to produce a
// display name for uncategorized CRD entries (Kubernetes resource plurals
// are always lowercase ASCII, so the simple first-byte transformation is
// safe). Uses strings.ToUpper on the first byte so the function is robust
// against any input without relying on manual ASCII arithmetic.
func titleCaseFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sortSidebarItems orders sidebar items: core categories in fixed order,
// items within a core category in BuiltInOrderRank order (falling back to
// alphabetical by display name for entries without a curated rank), pinned
// CRD groups next (respecting PinnedGroups config), then remaining CRD
// groups alphabetical by category and item name.
func sortSidebarItems(items []Item) []Item {
	coreOrder := make(map[string]int, len(CoreCategories))
	for i, name := range CoreCategories {
		coreOrder[name] = i
	}
	pinnedOrder := make(map[string]int, len(PinnedGroups))
	for i, g := range PinnedGroups {
		pinnedOrder[g] = i
	}

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aCoreRank, aCore := coreOrder[a.Category]
		bCoreRank, bCore := coreOrder[b.Category]
		switch {
		case aCore && bCore:
			if aCoreRank != bCoreRank {
				return aCoreRank < bCoreRank
			}
			// Same core category: use the curated BuiltInOrderRank so
			// items appear in their declared order (e.g., Pods before
			// Deployments). Items without a rank fall back to alphabetical.
			aOrd, aHasOrd := itemOrderRank(a)
			bOrd, bHasOrd := itemOrderRank(b)
			switch {
			case aHasOrd && bHasOrd:
				if aOrd != bOrd {
					return aOrd < bOrd
				}
			case aHasOrd:
				return true
			case bHasOrd:
				return false
			}
		case aCore:
			return true
		case bCore:
			return false
		default:
			// Both non-core: pinned before unpinned; within pinned, follow PinnedGroups order; otherwise alphabetical by category.
			aPinRank, aPin := pinnedOrder[a.Category]
			bPinRank, bPin := pinnedOrder[b.Category]
			switch {
			case aPin && bPin:
				if aPinRank != bPinRank {
					return aPinRank < bPinRank
				}
			case aPin:
				return true
			case bPin:
				return false
			default:
				if a.Category != b.Category {
					return a.Category < b.Category
				}
			}
		}
		return a.Name < b.Name
	})
	return items
}

// itemOrderRank returns the BuiltInOrderRank for a sidebar item, derived
// from its Extra field ("group/version/resource" → "group/resource").
// Returns false for items whose Extra is not in the standard ref format
// (e.g., dashboard pseudo-items with Extra == "__overview__").
func itemOrderRank(it Item) (int, bool) {
	parts := strings.SplitN(it.Extra, "/", 3)
	if len(parts) != 3 {
		return 0, false
	}
	key := parts[0] + "/" + parts[2]
	rank, ok := BuiltInOrderRank[key]
	return rank, ok
}
