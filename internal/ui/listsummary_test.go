package ui

import (
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
