package app

import (
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// updateResourcesLoadedPreview handles a resourcesLoadedMsg destined for the
// right preview pane: it sorts the items the way the drilled-in list renders
// them (issue #408), primes the drill-in itemCache, and applies the Event
// warnings-only filter and grouping before exposing the list as rightItems.
func (m Model) updateResourcesLoadedPreview(msg resourcesLoadedMsg) Model {
	m.previewLoading = false
	msg.items = m.filterLoadedItemsBySelectedNamespaces(msg.items)
	// Copy before sorting: the fresh-cache shortcut in loadResources hands
	// this handler the itemCache slice itself, and sorting that in place
	// would reorder a list other model fields may still alias.
	msg.items = append([]model.Item(nil), msg.items...)
	// Sort before the cache priming below so the drill-in shortcut serves
	// the same order the preview showed.
	m.sortPreviewItems(msg.items, msg.rt)
	// Anti-flap carry-over, mirroring updateResourcesLoadedMain: preview
	// fetches return unenriched items (no CPU/MEM metrics, no Service
	// endpoint rollup), while the drilled list enriches them async. The
	// rt-level watch tick drops the preview fingerprint every interval, so
	// each hover refetch would otherwise flip the auto-detected column set
	// — columns appear and disappear in the preview (#408 follow-up).
	// Carry from the drill-in cache (which the drilled list's metrics
	// ticks keep enriched) and fall back to the currently shown preview.
	// Runs before the cache prime below so the prime keeps the enrichment
	// instead of clobbering the drilled list's enriched cache entry.
	if msg.rt.Kind == "Pod" || msg.rt.Kind == "Node" || msg.rt.Kind == "Service" {
		prev := m.itemCache[m.nav.Context+"/"+msg.rt.Resource]
		if len(prev) == 0 && len(m.rightItems) > 0 && m.rightItems[0].Kind == msg.rt.Kind {
			prev = m.rightItems
		}
		if len(prev) > 0 && prev[0].Kind == msg.rt.Kind {
			if msg.rt.Kind == "Service" {
				carryOverServiceEndpointColumnsFrom(prev, msg.items)
			} else {
				carryOverMetricsColumnsFrom(prev, msg.items)
			}
		}
	}
	// Prime itemCache under the drill-in navKey so loadResources can serve
	// the list instantly and skip a redundant fetch when the user drills
	// in or re-hovers this rt later. Only do this when msg.rt carries a
	// real resource — synthetic previews (port-forwards, dashboards) have
	// a zero-valued rt and must not write an empty-resource key. The
	// fingerprint records the fetch-affecting state so the shortcut can
	// detect later invalidations (namespace switch, allNS toggle,
	// multi-select update) without relying on requestGen, which
	// navigateChild bumps before child handlers even run.
	if msg.rt.Resource != "" {
		drillInKey := m.nav.Context + "/" + msg.rt.Resource
		m.itemCache[drillInKey] = msg.items
		m.cacheFingerprints[drillInKey] = m.fetchFingerprint()
	}
	m.rightItems = msg.items
	// Filter events in children view to warnings-only when enabled.
	if m.warningEventsOnly && len(m.rightItems) > 0 && m.rightItems[0].Kind == "Event" {
		filtered := make([]model.Item, 0, len(m.rightItems))
		for _, item := range m.rightItems {
			if item.Status == "Warning" {
				filtered = append(filtered, item)
			}
		}
		m.rightItems = filtered
	}
	// Collapse duplicate events so noisy pods don't drown out the preview.
	// The preview pane is always a summary, so we follow the main list's
	// grouping toggle without offering a separate control — toggling `z` in
	// the Events view also affects the preview shown for other resources.
	if m.eventGrouping && len(m.rightItems) > 0 && m.rightItems[0].Kind == "Event" {
		m.rightItems = groupEvents(m.rightItems)
	}
	if len(m.rightItems) == 0 {
		logger.Info("No child resources found", "resourceType", m.nav.ResourceType.Kind, "resource", m.nav.ResourceName)
	}
	return m
}
