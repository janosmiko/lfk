package ui

import (
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
