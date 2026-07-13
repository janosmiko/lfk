package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func argoApp(health, sync string) model.Item {
	return model.Item{
		Kind: "Application",
		Columns: []model.KeyValue{
			{Key: "Health", Value: health},
			{Key: "Sync Status", Value: sync},
		},
	}
}

func TestBuildListSummary_ArgoApplication(t *testing.T) {
	items := []model.Item{
		argoApp("Healthy", "Synced"),
		argoApp("Healthy", "Synced"),
		argoApp("Degraded", "OutOfSync"),
		argoApp("Progressing", "Synced"),
	}

	s := BuildListSummary("Application", items)

	assert.Equal(t, 4, s.Total)
	require.Len(t, s.Bars, 2, "Application summary should have Health and Sync bars")

	health := s.Bars[0]
	assert.Equal(t, "Health", health.Label)
	assert.Equal(t, 4, health.Total)
	// Worst-first ordering: Degraded (error) before Progressing before Healthy (ok).
	require.Len(t, health.Buckets, 3)
	assert.Equal(t, "Degraded", health.Buckets[0].Value)
	assert.Equal(t, 1, health.Buckets[0].Count)
	assert.Equal(t, "Healthy", health.Buckets[2].Value)
	assert.Equal(t, 2, health.Buckets[2].Count)

	sync := s.Bars[1]
	assert.Equal(t, "Sync", sync.Label)
	assert.Equal(t, 4, sync.Total)
	require.Len(t, sync.Buckets, 2)
	// OutOfSync (progressing/amber) ranks before Synced (ok).
	assert.Equal(t, "OutOfSync", sync.Buckets[0].Value)
	assert.Equal(t, "Synced", sync.Buckets[1].Value)
	assert.Equal(t, 3, sync.Buckets[1].Count)
}

func TestBuildListSummary_GenericStatus(t *testing.T) {
	items := []model.Item{
		{Kind: "Pod", Status: "Running"},
		{Kind: "Pod", Status: "Running"},
		{Kind: "Pod", Status: "CrashLoopBackOff"},
		{Kind: "Pod", Status: "Pending"},
	}

	s := BuildListSummary("Pod", items)

	assert.Equal(t, 4, s.Total)
	require.Len(t, s.Bars, 1)
	bar := s.Bars[0]
	assert.Equal(t, "Status", bar.Label)
	assert.Equal(t, 4, bar.Total)
	require.Len(t, bar.Buckets, 3)
	// Failed first, then Pending (progressing), then Running (ok).
	assert.Equal(t, "CrashLoopBackOff", bar.Buckets[0].Value)
	assert.Equal(t, "Pending", bar.Buckets[1].Value)
	assert.Equal(t, "Running", bar.Buckets[2].Value)
}

// crdWithConditions builds an uncurated CRD item carrying status conditions
// (populated for every kind via appendAllConditions), used to exercise the
// generic conditions fallback (issue #352 follow-up).
func crdWithConditions(conds ...model.ConditionEntry) model.Item {
	return model.Item{Kind: "Widget", Conditions: conds}
}

// TestBuildListSummary_GenericConditionsFallback verifies that an uncurated
// kind whose items expose .status.conditions gets a Ready/NotReady rollup
// derived from the Ready condition, without being added to the curated
// registry (issue #352 follow-up).
func TestBuildListSummary_GenericConditionsFallback(t *testing.T) {
	items := []model.Item{
		crdWithConditions(model.ConditionEntry{Type: "Ready", Status: "True"}),
		crdWithConditions(model.ConditionEntry{Type: "Ready", Status: "True"}),
		crdWithConditions(model.ConditionEntry{Type: "Ready", Status: "False"}),
		crdWithConditions(model.ConditionEntry{Type: "Ready", Status: "Unknown"}),
	}

	s := BuildListSummary("Widget", items)

	assert.Equal(t, 4, s.Total)
	require.Len(t, s.Bars, 1, "uncurated kind with conditions should get one rollup bar")
	bar := s.Bars[0]
	assert.Equal(t, "Status", bar.Label)
	assert.Equal(t, 4, bar.Total)
	values := map[string]int{}
	for _, b := range bar.Buckets {
		values[b.Value] = b.Count
	}
	assert.Equal(t, 2, values["Ready"])
	assert.Equal(t, 1, values["NotReady"])
	assert.Equal(t, 1, values["Unknown"])
}

// TestBuildListSummary_GenericConditionsNegativeType verifies that when the
// primary condition is a negative type (e.g. Degraded), True is treated as
// unhealthy and False as healthy, matching the column the user already sees.
func TestBuildListSummary_GenericConditionsNegativeType(t *testing.T) {
	items := []model.Item{
		crdWithConditions(model.ConditionEntry{Type: "Degraded", Status: "True"}),
		crdWithConditions(model.ConditionEntry{Type: "Degraded", Status: "False"}),
	}

	s := BuildListSummary("Widget", items)

	require.Len(t, s.Bars, 1)
	values := map[string]int{}
	for _, b := range s.Bars[0].Buckets {
		values[b.Value] = b.Count
	}
	assert.Equal(t, 1, values["NotReady"], "Degraded=True is unhealthy")
	assert.Equal(t, 1, values["Ready"], "Degraded=False is healthy")
}

// TestBuildListSummary_GenericConditionsUnknownOnly verifies that a sole
// condition with status Unknown buckets as Unknown (ambiguous), even for a
// negative condition type, rather than being forced to NotReady.
func TestBuildListSummary_GenericConditionsUnknownOnly(t *testing.T) {
	items := []model.Item{
		crdWithConditions(model.ConditionEntry{Type: "Degraded", Status: "Unknown"}),
	}

	s := BuildListSummary("Widget", items)

	require.Len(t, s.Bars, 1)
	require.Len(t, s.Bars[0].Buckets, 1)
	assert.Equal(t, "Unknown", s.Bars[0].Buckets[0].Value)
}

// TestBuildListSummary_GenericConditionsTrueFallback covers the secondary
// fallback: no Ready condition, the last condition is an inactive negative type
// (Failed=False), so the active True condition is the real signal -> Ready.
func TestBuildListSummary_GenericConditionsTrueFallback(t *testing.T) {
	items := []model.Item{
		crdWithConditions(
			model.ConditionEntry{Type: "Established", Status: "True"},
			model.ConditionEntry{Type: "Failed", Status: "False"},
		),
	}

	s := BuildListSummary("Widget", items)

	require.Len(t, s.Bars, 1)
	require.Len(t, s.Bars[0].Buckets, 1)
	assert.Equal(t, "Ready", s.Bars[0].Buckets[0].Value)
}

// TestBuildListSummary_GenericPhaseFallback verifies that an uncurated kind
// whose items expose .status.phase as a Phase column gets a phase rollup.
func TestBuildListSummary_GenericPhaseFallback(t *testing.T) {
	mk := func(phase string) model.Item {
		return model.Item{Kind: "Widget", Columns: []model.KeyValue{{Key: "Phase", Value: phase}}}
	}
	items := []model.Item{mk("Running"), mk("Running"), mk("Failed"), mk("Pending")}

	s := BuildListSummary("Widget", items)

	require.Len(t, s.Bars, 1)
	bar := s.Bars[0]
	assert.Equal(t, "Phase", bar.Label)
	assert.Equal(t, 4, bar.Total)
	// Worst-first: Failed, then Pending, then Running.
	assert.Equal(t, "Failed", bar.Buckets[0].Value)
	assert.Equal(t, "Running", bar.Buckets[2].Value)
}

// TestBuildListSummary_GenericPhaseCardinalityCap verifies the noise guard:
// when the Phase column carries more than maxGenericPhaseValues distinct values
// (likely free-form text, not a status), no Phase bar is built — it falls back
// to conditions when present, otherwise to a count-only band.
func TestBuildListSummary_GenericPhaseCardinalityCap(t *testing.T) {
	mkPhase := func(i int) model.Item {
		return model.Item{Kind: "Widget", Columns: []model.KeyValue{
			{Key: "Phase", Value: fmt.Sprintf("free-form-%d", i)},
		}}
	}
	// More than maxGenericPhaseValues distinct phase strings.
	noisy := make([]model.Item, 0, maxGenericPhaseValues+2)
	for i := range maxGenericPhaseValues + 2 {
		noisy = append(noisy, mkPhase(i))
	}

	// No conditions -> count-only band, no Phase bar.
	s := BuildListSummary("Widget", noisy)
	assert.Equal(t, maxGenericPhaseValues+2, s.Total)
	assert.Empty(t, s.Bars, "over-cap phase cardinality must not render a Phase bar")

	// Same over-cap phases but with conditions present -> fall back to conditions.
	withConds := make([]model.Item, len(noisy))
	for i := range noisy {
		withConds[i] = noisy[i]
		withConds[i].Conditions = []model.ConditionEntry{{Type: "Ready", Status: "True"}}
	}
	s2 := BuildListSummary("Widget", withConds)
	require.Len(t, s2.Bars, 1)
	assert.Equal(t, "Status", s2.Bars[0].Label, "over-cap phase must fall back to the conditions rollup")
}

// TestBuildListSummary_GenericNoiseRejected verifies the deliberate guard:
// an uncurated kind with no Phase column and no conditions gets no status bar
// even if Item.Status carries a non-health marker (StorageClass sets Status to
// a "default"-class marker). It still gets a count-only band via Total.
func TestBuildListSummary_GenericNoiseRejected(t *testing.T) {
	items := []model.Item{
		{Kind: "StorageClass", Status: "default"},
		{Kind: "StorageClass", Status: ""},
	}

	s := BuildListSummary("StorageClass", items)

	assert.Equal(t, 2, s.Total)
	assert.Empty(t, s.Bars, "uncurated kind without phase/conditions must not get a noisy status bar")
}

// Worst-first ordering must hold even in no-color mode, where the status
// styles carry no foreground — so ordering cannot be derived from color.
func TestBuildListSummary_OrderingHoldsInNoColorMode(t *testing.T) {
	prevNoColor := ConfigNoColor
	prevTheme := ActiveTheme
	ConfigNoColor = true
	applyNoColorTheme()
	t.Cleanup(func() {
		ConfigNoColor = prevNoColor
		ApplyTheme(prevTheme)
	})

	items := []model.Item{
		{Kind: "Pod", Status: "Running"},
		{Kind: "Pod", Status: "Running"},
		{Kind: "Pod", Status: "CrashLoopBackOff"},
		{Kind: "Pod", Status: "Pending"},
	}
	bar := BuildListSummary("Pod", items).Bars[0]
	require.Len(t, bar.Buckets, 3)
	assert.Equal(t, "CrashLoopBackOff", bar.Buckets[0].Value, "failed must sort first regardless of color profile")
	assert.Equal(t, "Pending", bar.Buckets[1].Value)
	assert.Equal(t, "Running", bar.Buckets[2].Value)
}

func TestBuildListSummary_WorkloadReadyRollup(t *testing.T) {
	// StatefulSets/DaemonSets/Deployments carry Ready ("x/y") but no Status.
	items := []model.Item{
		{Kind: "StatefulSet", Ready: "3/3"},
		{Kind: "StatefulSet", Ready: "3/3"},
		{Kind: "StatefulSet", Ready: "1/3"},
		{Kind: "StatefulSet", Ready: "0/0"}, // scaled to zero counts as Ready
	}
	s := BuildListSummary("StatefulSet", items)
	require.Len(t, s.Bars, 1)
	bar := s.Bars[0]
	assert.Equal(t, "Status", bar.Label)
	assert.Equal(t, 4, bar.Total)
	require.Len(t, bar.Buckets, 2)
	// NotReady (progressing) sorts before Ready (ok).
	assert.Equal(t, "NotReady", bar.Buckets[0].Value)
	assert.Equal(t, 1, bar.Buckets[0].Count)
	assert.Equal(t, "Ready", bar.Buckets[1].Value)
	assert.Equal(t, 3, bar.Buckets[1].Count)
}

func TestBuildListSummary_PodUsesStatusNotReady(t *testing.T) {
	// Pods are summarised by Status (phase), not the ready ratio.
	items := []model.Item{
		{Kind: "Pod", Status: "Running", Ready: "1/1"},
		{Kind: "Pod", Status: "CrashLoopBackOff", Ready: "0/1"},
	}
	s := BuildListSummary("Pod", items)
	require.Len(t, s.Bars, 1)
	assert.Equal(t, "CrashLoopBackOff", s.Bars[0].Buckets[0].Value)
	assert.Equal(t, "Running", s.Bars[0].Buckets[1].Value)
}

func TestBuildListSummary_FluxReadyCondition(t *testing.T) {
	mk := func(ready string) model.Item {
		return model.Item{Kind: "Kustomization", Columns: []model.KeyValue{{Key: "Ready", Value: ready}}}
	}
	items := []model.Item{mk("True"), mk("True"), mk("False"), mk("Unknown")}
	s := BuildListSummary("Kustomization", items)
	require.Len(t, s.Bars, 1)
	bar := s.Bars[0]
	require.Len(t, bar.Buckets, 2)
	assert.Equal(t, "NotReady", bar.Buckets[0].Value)
	assert.Equal(t, 2, bar.Buckets[0].Count) // False + Unknown
	assert.Equal(t, "Ready", bar.Buckets[1].Value)
	assert.Equal(t, 2, bar.Buckets[1].Count)
}

func TestBuildListSummary_NoiseKindsHaveNoStatusBars(t *testing.T) {
	// StorageClass/IngressClass/PriorityClass set Status to a "default"-class
	// marker — not health — so they get no status bar (only a count-only band).
	for _, kind := range []string{"StorageClass", "IngressClass", "PriorityClass"} {
		items := []model.Item{
			{Kind: kind, Status: "default"},
			{Kind: kind},
			{Kind: kind},
		}
		s := BuildListSummary(kind, items)
		assert.Truef(t, s.Empty(), "%s should produce no status bars", kind)
		assert.Equal(t, 3, s.Total)
	}
}

func TestBuildListSummary_EmptyList(t *testing.T) {
	s := BuildListSummary("Pod", nil)
	assert.Equal(t, 0, s.Total)
	assert.True(t, s.Empty())
}

func TestBuildListSummary_StatuslessKindHasNoBars(t *testing.T) {
	items := []model.Item{
		{Kind: "ConfigMap", Name: "a"},
		{Kind: "ConfigMap", Name: "b"},
	}
	s := BuildListSummary("ConfigMap", items)
	assert.Equal(t, 2, s.Total)
	assert.True(t, s.Empty(), "kinds whose items carry no status should produce no summary bars")
}

func TestBuildListSummary_PartialValuesIgnoresEmpties(t *testing.T) {
	items := []model.Item{
		argoApp("Healthy", "Synced"),
		argoApp("", ""), // e.g. status not yet populated
	}
	s := BuildListSummary("Application", items)
	require.Len(t, s.Bars, 2)
	// Only the one populated value is counted; empties are skipped.
	assert.Equal(t, 1, s.Bars[0].Total)
	require.Len(t, s.Bars[0].Buckets, 1)
	assert.Equal(t, "Healthy", s.Bars[0].Buckets[0].Value)
}

// TestBuildListSummary_SanitizesTerminalEscapes verifies a CRD-controlled
// status value (.status.phase / condition type) that embeds terminal control
// bytes - the ESC introducer of a cursor-movement/screen-clear sequence, or
// the BEL terminator of an OSC-52 clipboard write - has those control bytes
// stripped before bucketing/counting. ansi.Truncate only trims to width; it
// does not strip non-SGR escapes, and this data reaches the always-visible
// dashboard summary band without ever passing through a viewer's sanitizer.
// Stripping the control bytes is sufficient: without the ESC introducer, the
// remaining printable text ("[2J", "]52;...") is inert - the terminal has no
// escape sequence left to interpret.
func TestBuildListSummary_SanitizesTerminalEscapes(t *testing.T) {
	items := []model.Item{
		argoApp("Healthy\x1b[2J", "Synced"),
		argoApp("Healthy\x1b[2J", "Synced"),
	}
	s := BuildListSummary("Application", items)
	require.Len(t, s.Bars, 2)
	health := s.Bars[0]
	require.Len(t, health.Buckets, 1, "identical escape-laden values sanitize to the same bucket")
	assert.Equal(t, "Healthy[2J", health.Buckets[0].Value, "ESC introducer removed, trailing printable text left inert")
	assert.Equal(t, 2, health.Buckets[0].Count)
	assert.NotContains(t, health.Buckets[0].Value, "\x1b")

	// OSC-52 clipboard-write payload: the BEL (0x07) terminator must also be
	// stripped, not just ESC.
	oscItems := []model.Item{argoApp("Healthy\x1b]52;c;ZXZpbA==\x07", "Synced")}
	oscSummary := BuildListSummary("Application", oscItems)
	require.Len(t, oscSummary.Bars, 2)
	oscValue := oscSummary.Bars[0].Buckets[0].Value
	assert.NotContains(t, oscValue, "\x1b")
	assert.NotContains(t, oscValue, "\x07")
}

func TestRenderListSummary_ContainsCountsAndFitsWidth(t *testing.T) {
	items := []model.Item{
		argoApp("Healthy", "Synced"),
		argoApp("Degraded", "OutOfSync"),
	}
	s := BuildListSummary("Application", items)

	out := RenderListSummary(s, "Applications", 60)
	require.NotEmpty(t, out)

	plain := ansi.Strip(out)
	assert.Contains(t, plain, "Applications")
	assert.Contains(t, plain, "Healthy")
	assert.Contains(t, plain, "Degraded")
	assert.Contains(t, plain, "OutOfSync")

	for line := range strings.SplitSeq(plain, "\n") {
		assert.LessOrEqual(t, ansi.StringWidth(line), 60, "line exceeds width: %q", line)
	}
}

func TestAllocateCells_KeepsRareBucketsVisible(t *testing.T) {
	// Real case: 391 pods, worst-first order. Error and PodInitializing are a
	// tiny fraction but must still get a visible cell.
	counts := []int{6, 2, 318, 64} // Error, PodInitializing, Running, Succeeded
	cells := allocateCells(counts, 391, summaryBarCells)

	sum := 0
	for i, c := range counts {
		assert.GreaterOrEqualf(t, cells[i], 1, "nonzero bucket %d must get >=1 cell", i)
		sum += cells[i]
		_ = c
	}
	assert.Equal(t, summaryBarCells, sum, "cells must sum to the bar width")
}

func TestAllocateCells_MoreBucketsThanCells(t *testing.T) {
	counts := []int{5, 4, 3, 2, 1} // 5 buckets, only 3 cells
	cells := allocateCells(counts, 15, 3)

	sum := 0
	for _, c := range cells {
		sum += c
	}
	assert.Equal(t, 3, sum)
	// The first three (most severe, by input order) each get a cell.
	assert.Equal(t, []int{1, 1, 1, 0, 0}, cells)
}

func TestRenderListSummary_CountOnlyForStatuslessKind(t *testing.T) {
	s := BuildListSummary("ConfigMap", []model.Item{{Kind: "ConfigMap"}, {Kind: "ConfigMap"}})
	out := ansi.Strip(RenderListSummary(s, "ConfigMaps", 60))
	assert.Contains(t, out, "2 ConfigMaps")
	assert.NotContains(t, out, "Status", "statusless kinds get a header-only band")
	assert.Equal(t, 1, strings.Count(out, "\n")+1, "header line only, no bars")
}

func TestRenderListSummary_BlankWhenNoItems(t *testing.T) {
	s := BuildListSummary("ConfigMap", nil)
	assert.Empty(t, RenderListSummary(s, "ConfigMaps", 60))
}

func TestBuildListSummary_NodeAndNamespace(t *testing.T) {
	nodes := BuildListSummary("Node", []model.Item{
		{Kind: "Node", Status: "Ready"},
		{Kind: "Node", Status: "Ready"},
		{Kind: "Node", Status: "NotReady"},
	})
	require.Len(t, nodes.Bars, 1)
	assert.Equal(t, "NotReady", nodes.Bars[0].Buckets[0].Value)
	assert.Equal(t, "Ready", nodes.Bars[0].Buckets[1].Value)

	ns := BuildListSummary("Namespace", []model.Item{
		{Kind: "Namespace", Status: "Active"},
		{Kind: "Namespace", Status: "Terminating"},
	})
	require.Len(t, ns.Bars, 1)
	assert.Equal(t, "Terminating", ns.Bars[0].Buckets[0].Value)
	assert.Equal(t, "Active", ns.Bars[0].Buckets[1].Value)
}

func TestRenderKindSummary(t *testing.T) {
	items := []model.Item{
		argoApp("Healthy", "Synced"),
		argoApp("Degraded", "OutOfSync"),
	}
	s := BuildListSummary("Application", items)

	out := ansi.Strip(RenderKindSummary(s, "Applications", 80))
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 3, "header + Health bar + Sync bar")
	assert.Contains(t, lines[0], "Applications")
	assert.Contains(t, lines[0], "2")
	assert.NotContains(t, lines[0], "SUMMARY", "dashboard header shows the kind, not the band title")
	assert.Contains(t, lines[1], "Health")
	assert.Contains(t, lines[1], "1 Degraded")
	assert.Contains(t, lines[2], "Sync")
}

func TestRenderKindSummary_ZeroTotalStillRendersHeader(t *testing.T) {
	s := BuildListSummary("Job", nil)
	out := ansi.Strip(RenderKindSummary(s, "Jobs", 80))
	assert.Equal(t, "Jobs  0", out)
}

func TestRenderKindSummary_ZeroWidth(t *testing.T) {
	s := BuildListSummary("Job", []model.Item{{Kind: "Job", Status: "Complete"}})
	assert.Empty(t, RenderKindSummary(s, "Jobs", 0))
}
