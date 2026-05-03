package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updateMetricsLoaded(msg metricsLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response
	}
	if msg.cpuUsed == 0 && msg.memUsed == 0 {
		m.metricsContent = ""
		return m
	}
	// Calculate available width for the metrics bar.
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	innerW := max(
		// column padding + border
		rightW-4, 20)
	m.metricsContent = ui.RenderResourceUsage(
		msg.cpuUsed, msg.cpuReq, msg.cpuLim,
		msg.memUsed, msg.memReq, msg.memLim,
		innerW,
	)
	return m
}

func (m Model) updatePreviewEventsLoaded(msg previewEventsLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response
	}
	if len(msg.events) == 0 {
		m.previewEventsContent = ""
		return m
	}
	// Calculate available width for the events section.
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	innerW := max(rightW-4, 20)
	entries := make([]ui.EventTimelineEntry, len(msg.events))
	for i, e := range msg.events {
		entries[i] = ui.EventTimelineEntry{
			Timestamp:    e.Timestamp,
			Type:         e.Type,
			Reason:       e.Reason,
			Message:      e.Message,
			Source:       e.Source,
			Count:        e.Count,
			InvolvedName: e.InvolvedName,
			InvolvedKind: e.InvolvedKind,
		}
	}
	m.previewEventsContent = ui.RenderPreviewEvents(entries, innerW)
	return m
}

// updatePreviewServiceEndpointsLoaded injects the rollup into every
// matching middleItems entry as a "Backing Endpoints" summary KV plus
// the multi-line "Endpoints" KV the renderer formats per-line.
//
// Cache stores the latest fresh fetch so the next watch-tick rebuild
// can paint the rollup row immediately from the cache while the new
// fetch lands — see loadPreviewServiceEndpoints's stale-while-
// revalidate pattern. Cache writes only happen on fresh fetches: a
// cache-emit message carries the same pointer that's already in the
// map, so the assignment is a no-op for those.
func (m Model) updatePreviewServiceEndpointsLoaded(msg previewServiceEndpointsLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response; the caller's m.previewLoading stays armed
	}
	if msg.err != nil {
		logger.Info("preview service endpoints load error", "name", msg.name, "err", msg.err)
		return m
	}
	if msg.data == nil {
		return m
	}

	if m.serviceEndpointsCache == nil {
		m.serviceEndpointsCache = make(map[string]*k8s.ServiceEndpoints)
	}
	key := serviceEndpointsCacheKey(msg.ctx, msg.ns, msg.name)
	if msg.fromCache {
		// Cache-emit path: the cache entry is the source of msg.data, so
		// only inject if it's still the cache's current value. A fresher
		// fetch may have already updated the cache (extremely rare race
		// where the fresh response beats the tea.Batch goroutine
		// scheduling); in that case its handler already injected and
		// this stale emit must not clobber it. Don't write the cache
		// either — it already holds msg.data when the guard passes.
		if existing, ok := m.serviceEndpointsCache[key]; !ok || existing != msg.data {
			return m
		}
	} else {
		// Fresh-fetch path: always update the cache so the next watch-
		// tick rebuild can paint instantly from it.
		m.serviceEndpointsCache[key] = msg.data
	}

	m.middleItemsRev++
	for i := range m.middleItems {
		item := &m.middleItems[i]
		if item.Name != msg.name {
			continue
		}
		itemNS := item.Namespace
		if itemNS == "" {
			itemNS = m.namespace
		}
		if itemNS != msg.ns {
			continue
		}
		injectServiceEndpointColumns(item, msg.data)
	}

	return m
}

// injectServiceEndpointColumns rewrites the Service item's Backing
// Endpoints summary + per-endpoint Endpoints multi-line KV. Removes
// any prior values (handles rollup refresh after pods come and go) so
// the column ordering stays stable across hovers.
func injectServiceEndpointColumns(item *model.Item, data *k8s.ServiceEndpoints) {
	filtered := item.Columns[:0]
	for _, kv := range item.Columns {
		if kv.Key == "Backing Endpoints" || kv.Key == "Endpoints" {
			continue
		}
		filtered = append(filtered, kv)
	}
	item.Columns = filtered

	summary := fmt.Sprintf("%d ready / %d not ready", data.Ready, data.NotReady)
	item.Columns = append(item.Columns,
		model.KeyValue{Key: "Backing Endpoints", Value: summary})
	if data.Block != "" {
		item.Columns = append(item.Columns,
			model.KeyValue{Key: "Endpoints", Value: data.Block})
	}
}

func (m Model) updatePreviewSecretDataLoaded(msg previewSecretDataLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response; discard. A newer load is still in flight,
		// so leave previewLoading armed for the next reply.
	}
	// The fetch is no longer in flight for the current gen. Clear the spinner
	// regardless of outcome so the right pane stops saying "Loading...".
	m.previewLoading = false
	if msg.err != nil {
		logger.Info("preview secret data load error", "name", msg.name, "err", msg.err)
		return m // do not cache failures
	}
	if msg.data == nil {
		return m
	}

	// Store in cache so subsequent hovers on the same key (after list refresh)
	// skip the network round-trip.
	if m.secretPreviewCache == nil {
		m.secretPreviewCache = make(map[string]*model.SecretData)
	}
	key := secretPreviewCacheKey(msg.ctx, msg.ns, msg.name)
	m.secretPreviewCache[key] = msg.data

	// Inject secret:<key> columns into every matching middleItems entry.
	m.middleItemsRev++
	for i := range m.middleItems {
		item := &m.middleItems[i]
		if item.Name != msg.name {
			continue
		}
		itemNS := item.Namespace
		if itemNS == "" {
			itemNS = m.namespace
		}
		if itemNS != msg.ns {
			continue
		}

		// Remove any stale secret: columns first to avoid duplicates when
		// the secret data has been updated between fetches.
		filtered := item.Columns[:0]
		for _, kv := range item.Columns {
			if !strings.HasPrefix(kv.Key, "secret:") {
				filtered = append(filtered, kv)
			}
		}
		item.Columns = filtered

		// Append decoded secret entries in key order.
		for _, k := range msg.data.Keys {
			item.Columns = append(item.Columns, model.KeyValue{
				Key:   "secret:" + k,
				Value: msg.data.Data[k],
			})
		}
	}

	return m
}

func (m Model) updatePodMetricsEnriched(msg podMetricsEnrichedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response
	}
	// Don't bail on an empty payload — every visible row still needs to
	// drop into the missing-key branch below so prior CPU/MEM values get
	// reset to "n/a" via clearStalePodMetricsColumns. Returning early here
	// used to leave the previous tick's usage on screen indefinitely
	// whenever metrics-server fell over.
	// Enrich middle items with CPU/Memory usage + percentage columns.
	// Key format: "namespace/name". GetAllPodMetrics uses the same format
	// regardless of query scope (all-namespaces vs single-namespace), so
	// this lookup is consistent. For cluster-scoped items (no namespace)
	// the key collapses to "/name" on both sides.
	m.middleItemsRev++
	for i := range m.middleItems {
		item := &m.middleItems[i]
		key := item.Namespace + "/" + item.Name
		pm, ok := msg.metrics[key]
		if !ok {
			// No fresh metrics for this pod — clear any prior CPU/MEM
			// values so the UI does not keep showing the previous tick's
			// usage as if it were current. Leave non-metrics columns
			// (e.g., raw "CPU Req" / "Mem Lim") untouched so the next
			// successful tick can recompute percentages.
			clearStalePodMetricsColumns(item)
			continue
		}

		// Look up existing request/limit values from item columns.
		var cpuReqStr, cpuLimStr, memReqStr, memLimStr string
		for _, kv := range item.Columns {
			switch kv.Key {
			case "CPU Req":
				cpuReqStr = kv.Value
			case "CPU Lim":
				cpuLimStr = kv.Value
			case "Mem Req":
				memReqStr = kv.Value
			case "Mem Lim":
				memLimStr = kv.Value
			}
		}

		cpuUse := ui.FormatCPU(pm.CPU)
		memUse := ui.FormatMemory(pm.Memory)

		// Detect significant usage trends (arrows before value).
		if m.prevPodMetrics != nil {
			if prev, ok := m.prevPodMetrics[key]; ok {
				cpuDiff := pm.CPU - prev.CPU
				memDiff := pm.Memory - prev.Memory
				// CPU: significant if >10% change AND >20m absolute change.
				if prev.CPU > 0 {
					pctChange := float64(cpuDiff) / float64(prev.CPU)
					if pctChange > 0.10 && cpuDiff > 20 {
						cpuUse = "↑ " + cpuUse
					} else if pctChange < -0.10 && cpuDiff < -20 {
						cpuUse = "↓ " + cpuUse
					}
				}
				// Memory: significant if >10% change AND >20Mi absolute change.
				if prev.Memory > 0 {
					pctChange := float64(memDiff) / float64(prev.Memory)
					if pctChange > 0.10 && memDiff > 20*1024*1024 {
						memUse = "↑ " + memUse
					} else if pctChange < -0.10 && memDiff < -20*1024*1024 {
						memUse = "↓ " + memUse
					}
				}
			}
		}

		cpuReqPct := ui.ComputePctStr(pm.CPU, cpuReqStr, true)
		cpuLimPct := ui.ComputePctStr(pm.CPU, cpuLimStr, true)
		memReqPct := ui.ComputePctStr(pm.Memory, memReqStr, false)
		memLimPct := ui.ComputePctStr(pm.Memory, memLimStr, false)

		// Rebuild columns: replace old CPU/Mem percentage columns with the
		// freshly computed ones. The raw "CPU Req", "CPU Lim", "Mem Req",
		// "Mem Lim" columns are DELIBERATELY preserved — they are always
		// blocked from auto-detected table display (see
		// internal/ui/explorer_format.go) so they do not show up as extra
		// headers, and the next metrics tick reads them to recompute the
		// percentages. Dropping them here was the cause of a regression
		// where CPU/R, CPU/L, MEM/R, MEM/L showed real values on the first
		// tick and flipped to "n/a" on every subsequent tick, because the
		// source data was gone.
		removeCols := map[string]bool{
			"CPU":     true,
			"MEM":     true,
			"CPU Use": true,
			"Mem Use": true,
			"CPU/R":   true, "CPU/L": true, "MEM/R": true, "MEM/L": true,
		}
		var newCols []model.KeyValue
		newCols = append(newCols,
			model.KeyValue{Key: "CPU", Value: cpuUse},
			model.KeyValue{Key: "CPU/R", Value: cpuReqPct},
			model.KeyValue{Key: "CPU/L", Value: cpuLimPct},
			model.KeyValue{Key: "MEM", Value: memUse},
			model.KeyValue{Key: "MEM/R", Value: memReqPct},
			model.KeyValue{Key: "MEM/L", Value: memLimPct},
		)
		for _, kv := range item.Columns {
			if !removeCols[kv.Key] {
				newCols = append(newCols, kv)
			}
		}
		item.Columns = newCols
	}
	// Only update the baseline every 60s so trend arrows persist longer.
	if m.prevPodMetrics == nil || time.Since(m.prevPodMetricsTime) > 60*time.Second {
		m.prevPodMetrics = msg.metrics
		m.prevPodMetricsTime = time.Now()
	}
	// Update cache.
	m.itemCache[m.navKey()] = m.middleItems
	return m
}

// clearStalePodMetricsColumns rebuilds an item's column list with fresh
// "n/a" placeholders for the pod metrics keys, dropping any prior values
// supplied by an earlier tick. Raw "CPU Req"/"CPU Lim"/"Mem Req"/"Mem Lim"
// values are preserved so the next successful tick can recompute the
// percentage columns.
func clearStalePodMetricsColumns(item *model.Item) {
	removeCols := map[string]bool{
		"CPU":     true,
		"MEM":     true,
		"CPU Use": true,
		"Mem Use": true,
		"CPU/R":   true, "CPU/L": true, "MEM/R": true, "MEM/L": true,
	}
	filtered := item.Columns[:0]
	for _, kv := range item.Columns {
		if !removeCols[kv.Key] {
			filtered = append(filtered, kv)
		}
	}
	filtered = append(filtered,
		model.KeyValue{Key: "CPU", Value: "n/a"},
		model.KeyValue{Key: "CPU/R", Value: "n/a"},
		model.KeyValue{Key: "CPU/L", Value: "n/a"},
		model.KeyValue{Key: "MEM", Value: "n/a"},
		model.KeyValue{Key: "MEM/R", Value: "n/a"},
		model.KeyValue{Key: "MEM/L", Value: "n/a"},
	)
	item.Columns = filtered
}

// ensureNodeMetricsColumnsPlaceholder adds CPU/CPU%/MEM/MEM% columns to a node
// item using "n/a" placeholders when metrics-server returned no data for it.
// Stable column visibility is the contract — without these placeholders,
// autoDetectColumns drops the metrics columns whenever every visible row
// lacks them, and the user sees the column set blink in and out as
// metrics-server health fluctuates.
func ensureNodeMetricsColumnsPlaceholder(item *model.Item) {
	// Strip any prior CPU/CPU%/MEM/MEM% values so a node that has just
	// dropped out of metrics-server output does not keep showing stale
	// numbers from the previous tick — autoDetectColumns already keeps
	// the columns visible thanks to the placeholders we re-append below.
	removeCols := map[string]bool{"CPU": true, "CPU%": true, "MEM": true, "MEM%": true}
	filtered := item.Columns[:0]
	for _, kv := range item.Columns {
		if !removeCols[kv.Key] {
			filtered = append(filtered, kv)
		}
	}
	filtered = append(filtered,
		model.KeyValue{Key: "CPU", Value: "n/a"},
		model.KeyValue{Key: "CPU%", Value: "n/a"},
		model.KeyValue{Key: "MEM", Value: "n/a"},
		model.KeyValue{Key: "MEM%", Value: "n/a"},
	)
	item.Columns = filtered
}

func (m Model) updateNodeMetricsEnriched(msg nodeMetricsEnrichedMsg) Model {
	if msg.gen != m.requestGen {
		return m
	}
	m.middleItemsRev++
	for i := range m.middleItems {
		item := &m.middleItems[i]
		nm, ok := msg.metrics[item.Name]
		if !ok {
			// Metrics-server didn't return data for this node (or not yet).
			// Touch the item so CPU/CPU%/MEM/MEM% columns exist with "n/a"
			// values; otherwise autoDetectColumns hides the columns
			// entirely whenever metrics are unavailable, and they pop
			// in/out as metrics-server churns.
			ensureNodeMetricsColumnsPlaceholder(item)
			continue
		}

		// Look up allocatable values from item columns.
		var cpuAllocStr, memAllocStr string
		for _, kv := range item.Columns {
			switch kv.Key {
			case "CPU Alloc":
				cpuAllocStr = kv.Value
			case "Mem Alloc":
				memAllocStr = kv.Value
			}
		}

		cpuUse := ui.FormatCPU(nm.CPU)
		memUse := ui.FormatMemory(nm.Memory)

		// Detect significant usage trends (arrows before value).
		if m.prevNodeMetrics != nil {
			if prev, ok := m.prevNodeMetrics[item.Name]; ok {
				cpuDiff := nm.CPU - prev.CPU
				memDiff := nm.Memory - prev.Memory
				// CPU: significant if >10% change AND >20m absolute change.
				if prev.CPU > 0 {
					pctChange := float64(cpuDiff) / float64(prev.CPU)
					if pctChange > 0.10 && cpuDiff > 20 {
						cpuUse = "↑ " + cpuUse
					} else if pctChange < -0.10 && cpuDiff < -20 {
						cpuUse = "↓ " + cpuUse
					}
				}
				// Memory: significant if >10% change AND >20Mi absolute change.
				if prev.Memory > 0 {
					pctChange := float64(memDiff) / float64(prev.Memory)
					if pctChange > 0.10 && memDiff > 20*1024*1024 {
						memUse = "↑ " + memUse
					} else if pctChange < -0.10 && memDiff < -20*1024*1024 {
						memUse = "↓ " + memUse
					}
				}
			}
		}

		cpuPct := ui.ComputePctStr(nm.CPU, cpuAllocStr, true)
		memPct := ui.ComputePctStr(nm.Memory, memAllocStr, false)

		// Strip only the columns we're about to re-emit. CPU Alloc / Mem Alloc
		// stay in place: they're populator-supplied capacity data the right-
		// pane summary needs whenever the user navigates to a node, and
		// removing them used to leave a window after metrics enrichment but
		// before the next watch-tick list refresh where the preview had no
		// alloc info to render.
		removeCols := map[string]bool{
			"CPU": true, "CPU%": true, "MEM": true, "MEM%": true,
		}
		var newCols []model.KeyValue
		newCols = append(newCols,
			model.KeyValue{Key: "CPU", Value: cpuUse},
			model.KeyValue{Key: "CPU%", Value: cpuPct},
			model.KeyValue{Key: "MEM", Value: memUse},
			model.KeyValue{Key: "MEM%", Value: memPct},
		)
		for _, kv := range item.Columns {
			if !removeCols[kv.Key] {
				newCols = append(newCols, kv)
			}
		}
		item.Columns = newCols
	}
	// Only update the baseline every 60s so trend arrows persist longer.
	if m.prevNodeMetrics == nil || time.Since(m.prevNodeMetricsTime) > 60*time.Second {
		m.prevNodeMetrics = msg.metrics
		m.prevNodeMetricsTime = time.Now()
	}
	m.itemCache[m.navKey()] = m.middleItems
	return m
}
