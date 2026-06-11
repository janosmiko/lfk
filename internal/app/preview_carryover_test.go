package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

// Issue #408 follow-up: preview refetches return unenriched items (no
// CPU/MEM metrics, no Service endpoint rollup), while the drilled list's
// items are enriched asynchronously. Without carrying the enriched columns
// over, every hover refetch (the rt-level watch tick drops the preview
// fingerprint each interval) flips the auto-detected column set — columns
// appear and disappear in the preview.

func columnsByKey(item model.Item) map[string]string {
	out := make(map[string]string, len(item.Columns))
	for _, kv := range item.Columns {
		out[kv.Key] = kv.Value
	}
	return out
}

func enrichedPodItems() []model.Item {
	return []model.Item{
		{Name: "web-0", Namespace: "default", Kind: "Pod", Status: "Running", Columns: []model.KeyValue{
			{Key: "CPU", Value: "12m"},
			{Key: "CPU/R", Value: "12%"},
			{Key: "CPU/L", Value: "6%"},
			{Key: "MEM", Value: "30Mi"},
			{Key: "MEM/R", Value: "15%"},
			{Key: "MEM/L", Value: "7%"},
			{Key: "Node", Value: "n1"},
		}},
		{Name: "web-1", Namespace: "default", Kind: "Pod", Status: "Running", Columns: []model.KeyValue{
			{Key: "CPU", Value: "7m"},
			{Key: "CPU/R", Value: "7%"},
			{Key: "CPU/L", Value: "3%"},
			{Key: "MEM", Value: "22Mi"},
			{Key: "MEM/R", Value: "11%"},
			{Key: "MEM/L", Value: "5%"},
			{Key: "Node", Value: "n2"},
		}},
	}
}

func unenrichedPodItems() []model.Item {
	return []model.Item{
		{Name: "web-0", Namespace: "default", Kind: "Pod", Status: "Running", Columns: []model.KeyValue{
			{Key: "Node", Value: "n1"},
		}},
		{Name: "web-1", Namespace: "default", Kind: "Pod", Status: "Running", Columns: []model.KeyValue{
			{Key: "Node", Value: "n2"},
		}},
	}
}

func TestPreviewCarriesMetricsColumnsFromDrillCache(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	m.itemCache["test-ctx/pods"] = enrichedPodItems()

	res, _ := m.Update(resourcesLoadedMsg{items: unenrichedPodItems(), forPreview: true, rt: previewPodsRT()})
	rm := res.(Model)

	if len(rm.rightItems) != 2 {
		t.Fatalf("rightItems = %d items, want 2", len(rm.rightItems))
	}
	cols := columnsByKey(rm.rightItems[0])
	if cols["CPU"] != "12m" || cols["MEM"] != "30Mi" {
		t.Fatalf("metrics columns not carried over from the drill-in cache: CPU=%q MEM=%q", cols["CPU"], cols["MEM"])
	}
	if cols["Node"] != "n1" {
		t.Fatalf("non-metrics column lost in carry-over: Node=%q", cols["Node"])
	}
	// The cache prime must keep the enrichment too — overwriting the
	// drilled list's enriched cache with unenriched preview items made the
	// next drill-in flash the same surprise columns. The prime writes a
	// fresh slice (the handler copies msg.items), so asserting on its
	// contents genuinely exercises the primed entry, not the pre-Update one.
	primed := rm.itemCache["test-ctx/pods"]
	if len(primed) != 2 {
		t.Fatalf("primed cache entry = %d items, want 2", len(primed))
	}
	if pc := columnsByKey(primed[0]); pc["CPU"] != "12m" {
		t.Fatalf("primed cache lost metrics columns: CPU=%q", pc["CPU"])
	}
}

func TestPreviewCarriesNodeMetricsColumnsFromDrillCache(t *testing.T) {
	nodeRT := model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", APIVersion: "v1"}
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	m.itemCache["test-ctx/nodes"] = []model.Item{
		{Name: "n1", Kind: "Node", Columns: []model.KeyValue{
			{Key: "CPU", Value: "1200m"},
			{Key: "CPU%", Value: "30%"},
			{Key: "MEM", Value: "4Gi"},
			{Key: "MEM%", Value: "50%"},
		}},
	}

	fresh := []model.Item{{Name: "n1", Kind: "Node", Columns: []model.KeyValue{
		{Key: "Roles", Value: "control-plane"},
	}}}
	res, _ := m.Update(resourcesLoadedMsg{items: fresh, forPreview: true, rt: nodeRT})
	rm := res.(Model)

	cols := columnsByKey(rm.rightItems[0])
	if cols["CPU"] != "1200m" || cols["MEM%"] != "50%" {
		t.Fatalf("node metrics not carried over: CPU=%q MEM%%=%q", cols["CPU"], cols["MEM%"])
	}
	if cols["Roles"] != "control-plane" {
		t.Fatalf("non-metrics column lost: Roles=%q", cols["Roles"])
	}
}

func TestPreviewCarriesMetricsColumnsFromCurrentRightItems(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	m.rightItems = enrichedPodItems() // currently shown preview, no cache entry

	res, _ := m.Update(resourcesLoadedMsg{items: unenrichedPodItems(), forPreview: true, rt: previewPodsRT()})
	rm := res.(Model)

	cols := columnsByKey(rm.rightItems[0])
	if cols["CPU"] != "12m" || cols["MEM/L"] != "7%" {
		t.Fatalf("metrics columns not carried over from previous rightItems: CPU=%q MEM/L=%q", cols["CPU"], cols["MEM/L"])
	}
}

func TestPreviewCarriesServiceEndpointColumns(t *testing.T) {
	svcRT := model.ResourceTypeEntry{Kind: "Service", Resource: "services", APIVersion: "v1", Namespaced: true}
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	m.itemCache["test-ctx/services"] = []model.Item{
		{Name: "api", Namespace: "default", Kind: "Service", Columns: []model.KeyValue{
			{Key: "Type", Value: "ClusterIP"},
			{Key: "Backing Endpoints", Value: "3/3"},
			{Key: "Endpoints", Value: "10.0.0.1:8080"},
		}},
	}

	fresh := []model.Item{
		{Name: "api", Namespace: "default", Kind: "Service", Columns: []model.KeyValue{
			{Key: "Type", Value: "ClusterIP"},
		}},
	}
	res, _ := m.Update(resourcesLoadedMsg{items: fresh, forPreview: true, rt: svcRT})
	rm := res.(Model)

	cols := columnsByKey(rm.rightItems[0])
	if cols["Backing Endpoints"] != "3/3" {
		t.Fatalf("endpoint rollup not carried over: %q", cols["Backing Endpoints"])
	}
}

func TestPreviewCarryoverSkipsKindMismatchRightItems(t *testing.T) {
	m := basePush80v2Model()
	m.nav.Level = model.LevelResourceTypes
	// Previous hover showed Nodes (different kind) — their metrics must not
	// leak into a pods preview that matches on ns+name by accident.
	m.rightItems = []model.Item{
		{Name: "web-0", Namespace: "default", Kind: "Node", Columns: []model.KeyValue{
			{Key: "CPU", Value: "999m"},
		}},
	}

	res, _ := m.Update(resourcesLoadedMsg{items: unenrichedPodItems(), forPreview: true, rt: previewPodsRT()})
	rm := res.(Model)

	cols := columnsByKey(rm.rightItems[0])
	if got, ok := cols["CPU"]; ok {
		t.Fatalf("metrics carried over from a different kind's preview: CPU=%q", got)
	}
}
