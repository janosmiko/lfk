package ui

import (
	"slices"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

// keysOf extracts the ordered column keys from a collectExtraColumns result.
func keysOf(cols []extraColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.key
	}
	return out
}

func containsKey(cols []extraColumn, key string) bool {
	for _, c := range cols {
		if c.key == key {
			return true
		}
	}
	return false
}

// TestCollectExtraColumns_MandatoryPrinterColumnsSurvive verifies that CRD
// additionalPrinterColumns with priority 0 are treated as mandatory: they are
// not dropped by the width budget even when long resource names would
// otherwise starve trailing columns, and priority > 0 columns are hidden by
// default (matching kubectl).
func TestCollectExtraColumns_MandatoryPrinterColumnsSurvive(t *testing.T) {
	defer func(orig map[string]int) { ActivePrinterColumns = orig }(ActivePrinterColumns)
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	defer func(orig bool) { ActiveFullscreenMode = orig }(ActiveFullscreenMode)
	ActiveSessionColumns = nil
	ActiveFullscreenMode = false

	// Long generated names, like the ChoreTask CRD in issue #305, eat the
	// width budget under the old name-reservation rule.
	const longName = "build-squanderlings-the-bardic-continuum-server-77c9339f39816d8af2-a449f710"
	mkItem := func(suffix string) model.Item {
		return model.Item{
			Name:   longName + suffix,
			Status: "Succeeded",
			Columns: []model.KeyValue{
				{Key: "Duration", Value: "77.091"},
				{Key: "Task", Value: "clone-and-build"},
				{Key: "Catalog", Value: "buildah"},
				{Key: "Error Code", Value: "0"},
				{Key: "Created At", Value: "2026-05-31T00:58:01Z"},
				{Key: "Parent", Value: "top-level"},
			},
		}
	}
	items := []model.Item{mkItem("-a"), mkItem("-b"), mkItem("-c")}

	ActivePrinterColumns = map[string]int{
		"Duration": 0, "Task": 0, "Catalog": 0,
		"Error Code": 0, "Created At": 0,
		"Parent": 1, // priority > 0 -> hidden by default
	}

	// A realistically narrow middle column.
	cols := collectExtraColumns(items, 120, 20, "ChoreTask")

	if containsKey(cols, "Parent") {
		t.Errorf("priority>0 column Parent must be hidden by default, got %v", keysOf(cols))
	}
	if !containsKey(cols, "Created At") {
		t.Errorf("mandatory last-declared column Created At must survive width budget, got %v", keysOf(cols))
	}
}

// TestCollectExtraColumns_ConfiguredOrderNotReshuffled verifies that an
// explicit views.<kind>.columns configuration is authoritative: mandatory
// printer-column protection does not reorder it or hide its priority>0 columns
// (regression for the auto-detect-path-only contract).
func TestCollectExtraColumns_ConfiguredOrderNotReshuffled(t *testing.T) {
	defer func(orig map[string]int) { ActivePrinterColumns = orig }(ActivePrinterColumns)
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	defer func(orig map[string][]string) { ConfigResourceColumns = orig }(ConfigResourceColumns)
	defer func(orig ResourceRef) { ActiveResourceRef = orig }(ActiveResourceRef)
	defer func(orig string) { ActiveContext = orig }(ActiveContext)
	ActiveSessionColumns = nil
	ActiveResourceRef = ResourceRef{}
	ActiveContext = ""

	ActivePrinterColumns = map[string]int{"Duration": 0, "Parent": 1, "Created At": 0}
	// Explicit config order: non-mandatory-first, and includes the priority>0
	// "Parent" the user chose to surface.
	ConfigResourceColumns = map[string][]string{
		"choretask": {"Duration", "Parent", "Created At"},
	}

	items := []model.Item{
		{Name: "ct-a", Columns: []model.KeyValue{
			{Key: "Duration", Value: "1.0"}, {Key: "Parent", Value: "p"}, {Key: "Created At", Value: "2026-05-31T00:00:00Z"},
		}},
		{Name: "ct-b", Columns: []model.KeyValue{
			{Key: "Duration", Value: "2.0"}, {Key: "Parent", Value: "q"}, {Key: "Created At", Value: "2026-05-31T00:01:00Z"},
		}},
	}

	cols := collectExtraColumns(items, 200, 20, "ChoreTask")

	if got := keysOf(cols); len(got) != 3 || got[0] != "Duration" || got[1] != "Parent" || got[2] != "Created At" {
		t.Errorf("configured column order must be preserved verbatim, got %v", got)
	}
}

// TestCollectExtraColumns_OrderStableAcrossRowOrder is the regression for the
// sort-cycling column flicker (issue: pressing the sort key reordered or
// dropped extra columns). Column discovery used to follow first-seen order
// across items in their current sorted sequence, so a heterogeneous list
// (e.g. an unscheduled Pod with no NODE column) yielded a different column
// order depending on which row was scanned first. The detected column set and
// order must be a function of the data alone, never the row ordering.
func TestCollectExtraColumns_OrderStableAcrossRowOrder(t *testing.T) {
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	defer func(orig map[string]int) { ActivePrinterColumns = orig }(ActivePrinterColumns)
	defer func(orig bool) { ActiveFullscreenMode = orig }(ActiveFullscreenMode)
	ActiveSessionColumns = nil
	ActivePrinterColumns = nil
	ActiveFullscreenMode = true

	scheduled := func(name string) model.Item {
		return model.Item{
			Name: name, Namespace: "ns", Status: "Running", Kind: "Pod",
			Columns: []model.KeyValue{
				{Key: "QoS", Value: "Burstable"},
				{Key: "Pod IP", Value: "10.0.0.1"},
				{Key: "Node", Value: "node-a"},
				{Key: "Priority Class", Value: "system"},
			},
		}
	}
	// An unscheduled Pod has no NODE column, so scanning it first used to
	// surface "Priority Class" before "Node".
	unscheduled := func(name string) model.Item {
		return model.Item{
			Name: name, Namespace: "ns", Status: "Pending", Kind: "Pod",
			Columns: []model.KeyValue{
				{Key: "QoS", Value: "BestEffort"},
				{Key: "Pod IP", Value: "10.0.0.2"},
				{Key: "Priority Class", Value: "system"},
			},
		}
	}

	forward := []model.Item{scheduled("a"), scheduled("b"), unscheduled("c")}
	reversed := []model.Item{unscheduled("c"), scheduled("b"), scheduled("a")}

	got1 := keysOf(collectExtraColumns(forward, 400, 40, "Pod"))
	got2 := keysOf(collectExtraColumns(reversed, 400, 40, "Pod"))

	if !slices.Equal(got1, got2) {
		t.Errorf("extra-column order must not depend on row order:\n forward  = %v\n reversed = %v", got1, got2)
	}
	// Lock in the canonical data-position order (QoS, Pod IP, Node, Priority
	// Class): columns sort by their position within an item, ties broken by
	// key name so the result is deterministic.
	want := []string{"QoS", "Pod IP", "Node", "Priority Class"}
	if !slices.Equal(got1, want) {
		t.Errorf("canonical column order = %v, want %v", got1, want)
	}
}

// TestCollectExtraColumns_CompactBlockedFillSpareWidth verifies the issue-2
// behaviour: on a wide layout, compact blocked columns (e.g. Service Account)
// are revealed as low-priority overflow to fill the space NAME would otherwise
// pad, while verbose blocked columns (Images, Labels) stay hidden. On a narrow
// layout no overflow appears and NAME keeps the slack.
func TestCollectExtraColumns_CompactBlockedFillSpareWidth(t *testing.T) {
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	defer func(orig map[string]int) { ActivePrinterColumns = orig }(ActivePrinterColumns)
	defer func(orig bool) { ActiveFullscreenMode = orig }(ActiveFullscreenMode)
	ActiveSessionColumns = nil
	ActivePrinterColumns = nil
	ActiveFullscreenMode = true

	mk := func(name string) model.Item {
		return model.Item{
			Name: name, Namespace: "ns", Status: "Running", Kind: "Pod",
			Columns: []model.KeyValue{
				{Key: "QoS", Value: "Burstable"},
				{Key: "Service Account", Value: "default"}, // blocked, compact -> overflow
				{Key: "Pod IP", Value: "10.0.0.1"},
				{Key: "Images", Value: "registry.example.com/team/app:v1.2.3"}, // verbose -> hidden
				{Key: "Labels", Value: "app=x,tier=y,env=prod"},                // verbose -> hidden
			},
		}
	}
	items := []model.Item{mk("pod-a"), mk("pod-b"), mk("pod-c")}

	// Wide layout: spare width after the primary columns -> Service Account
	// fills it; the verbose columns stay hidden.
	wide := keysOf(collectExtraColumns(items, 400, 60, "Pod"))
	if !slices.Contains(wide, "Service Account") {
		t.Errorf("compact blocked column Service Account must fill spare width, got %v", wide)
	}
	if slices.Contains(wide, "Images") || slices.Contains(wide, "Labels") {
		t.Errorf("verbose blocked columns must stay hidden, got %v", wide)
	}
	// Overflow is appended after the primary columns.
	if i, j := slices.Index(wide, "Pod IP"), slices.Index(wide, "Service Account"); i >= 0 && j >= 0 && j < i {
		t.Errorf("overflow Service Account must come after primary columns, got %v", wide)
	}

	// Narrow layout: no room beyond the primary columns -> no overflow, and
	// Service Account is not forced in.
	narrow := keysOf(collectExtraColumns(items, 70, 50, "Pod"))
	if slices.Contains(narrow, "Service Account") {
		t.Errorf("compact blocked column must not appear when there is no spare width, got %v", narrow)
	}
}

// TestCollectExtraColumns_ConfiguredColumnsCompressNameInNarrowPane is the
// regression for issue #354: an explicitly configured column set (the column-
// toggle overlay, surfaced here as ActiveSessionColumns) rendered fully in the
// wide fullscreen list but lost trailing columns in the regular three-pane
// list. The default 20-char NAME reservation starved the narrow pane and
// fitExtraColumns dropped the tail. Configured columns are authoritative, so
// NAME must compress to let them all fit; in a wide pane the same config keeps
// rendering every column.
func TestCollectExtraColumns_ConfiguredColumnsCompressNameInNarrowPane(t *testing.T) {
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	defer func(orig map[string]int) { ActivePrinterColumns = orig }(ActivePrinterColumns)
	defer func(orig bool) { ActiveFullscreenMode = orig }(ActiveFullscreenMode)
	defer func(orig bool) { ActiveNameHidden = orig }(ActiveNameHidden)
	ActivePrinterColumns = nil
	ActiveNameHidden = false

	// A moderately long name (35 chars) plus three configured columns whose
	// values are ~7 chars wide. In the narrow pane the old name reservation
	// (longestName+1 = 36) leaves room for at most one column.
	const name = "deployment-frontend-web-server-blue"
	mk := func(suffix string) model.Item {
		return model.Item{
			Name:   name + suffix,
			Status: "Running",
			Columns: []model.KeyValue{
				{Key: "Catalog", Value: "buildah"},
				{Key: "Task", Value: "compile"},
				{Key: "Chore", Value: "nightly"},
			},
		}
	}
	items := []model.Item{mk("-a"), mk("-b"), mk("-c")}

	// User explicitly configured these three columns via the overlay.
	ActiveSessionColumns = []string{"Catalog", "Task", "Chore"}

	// Regular three-pane list: narrow middle column.
	ActiveFullscreenMode = false
	narrow := keysOf(collectExtraColumns(items, 70, 20, "ChoreTask"))
	for _, want := range []string{"Catalog", "Task", "Chore"} {
		if !slices.Contains(narrow, want) {
			t.Errorf("configured column %q must survive in the narrow three-pane list, got %v", want, narrow)
		}
	}

	// Full screen list: the same config must still render every column.
	ActiveFullscreenMode = true
	wide := keysOf(collectExtraColumns(items, 200, 20, "ChoreTask"))
	for _, want := range []string{"Catalog", "Task", "Chore"} {
		if !slices.Contains(wide, want) {
			t.Errorf("configured column %q must survive in the full screen list, got %v", want, wide)
		}
	}
}

// TestCollectExtraColumns_ConfiguredColumnSurvivesVeryNarrowPane covers the
// boundary where the auto-detect "available < 8" bail-out would otherwise drop
// an explicitly configured column that still physically fits. The bail-out is
// an auto-detect heuristic and must not apply to authoritative configured sets.
func TestCollectExtraColumns_ConfiguredColumnSurvivesVeryNarrowPane(t *testing.T) {
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	defer func(orig map[string]int) { ActivePrinterColumns = orig }(ActivePrinterColumns)
	defer func(orig bool) { ActiveFullscreenMode = orig }(ActiveFullscreenMode)
	defer func(orig bool) { ActiveNameHidden = orig }(ActiveNameHidden)
	ActivePrinterColumns = nil
	ActiveNameHidden = false
	ActiveFullscreenMode = false

	// Short name and one configured column with a short value (colW = 5).
	items := []model.Item{
		{Name: "po", Columns: []model.KeyValue{{Key: "Task", Value: "ok"}}},
		{Name: "px", Columns: []model.KeyValue{{Key: "Task", Value: "no"}}},
	}
	ActiveSessionColumns = []string{"Task"}

	// totalWidth 30, usedWidth 20 -> available 5, below the 8-char bail-out.
	cols := keysOf(collectExtraColumns(items, 30, 20, "Pod"))
	if !slices.Contains(cols, "Task") {
		t.Errorf("configured column must survive a very narrow pane that fits it, got %v", cols)
	}
}

// TestSelectColumnCandidates_AutoDetectFlag verifies the fromAutoDetect signal
// is true only on the auto-detect path, not for session or config overrides.
func TestSelectColumnCandidates_AutoDetectFlag(t *testing.T) {
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	defer func(orig map[string][]string) { ConfigResourceColumns = orig }(ConfigResourceColumns)
	defer func(orig ResourceRef) { ActiveResourceRef = orig }(ActiveResourceRef)
	defer func(orig string) { ActiveContext = orig }(ActiveContext)
	ActiveResourceRef = ResourceRef{}
	ActiveContext = ""

	seen := map[string]*colInfo{"Duration": {key: "Duration", count: 2}, "Task": {key: "Task", count: 2}}
	order := []string{"Duration", "Task"}
	items := []model.Item{{}, {}}

	// Session override -> not auto-detect.
	ActiveSessionColumns = []string{"Duration"}
	ConfigResourceColumns = nil
	if _, auto := selectColumnCandidates(seen, order, "Foo", items); auto {
		t.Error("session override must report fromAutoDetect=false")
	}

	// Config override -> not auto-detect.
	ActiveSessionColumns = nil
	ConfigResourceColumns = map[string][]string{"foo": {"Duration"}}
	if _, auto := selectColumnCandidates(seen, order, "Foo", items); auto {
		t.Error("config override must report fromAutoDetect=false")
	}

	// No overrides -> auto-detect.
	ConfigResourceColumns = nil
	if _, auto := selectColumnCandidates(seen, order, "Foo", items); !auto {
		t.Error("default path must report fromAutoDetect=true")
	}
}

// TestCollectExtraColumns_NonCRDUnaffected verifies that with no printer-column
// metadata the heuristic pipeline behaves exactly as before (no regression).
func TestCollectExtraColumns_NonCRDUnaffected(t *testing.T) {
	defer func(orig map[string]int) { ActivePrinterColumns = orig }(ActivePrinterColumns)
	defer func(orig []string) { ActiveSessionColumns = orig }(ActiveSessionColumns)
	ActivePrinterColumns = nil
	ActiveSessionColumns = nil

	// Keys not in blockedColumnsForMode so the default auto-detect surfaces them.
	items := []model.Item{
		{Name: "svc-a", Columns: []model.KeyValue{{Key: "Ports", Value: "80/TCP"}, {Key: "Replicas", Value: "3"}}},
		{Name: "svc-b", Columns: []model.KeyValue{{Key: "Ports", Value: "443/TCP"}, {Key: "Replicas", Value: "2"}}},
	}
	cols := collectExtraColumns(items, 120, 20, "Service")
	if !containsKey(cols, "Ports") || !containsKey(cols, "Replicas") {
		t.Errorf("expected Ports and Replicas columns, got %v", keysOf(cols))
	}
}
