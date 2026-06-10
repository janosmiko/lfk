package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// securityGroupPreviewModel builds a model standing on a finding group at
// LevelResources with a loaded affected-resources list and a description long
// enough to need scrolling.
func securityGroupPreviewModel() Model {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_advisor__", APIGroup: "_security"}
	m.middleItems = []model.Item{{
		Name:  "single_replica",
		Kind:  "__security_finding_group__",
		Extra: "single_replica",
		Columns: []model.KeyValue{
			{Key: "Severity", Value: "LOW"},
			{Key: "Affected", Value: "12"},
			{Key: "Description", Value: strings.Repeat("advisory paragraph that wraps over many lines. ", 40)},
		},
	}}
	m.setCursor(0)
	m.rightItems = make([]model.Item, 12)
	for i := range m.rightItems {
		m.rightItems[i] = model.Item{
			Name:      fmt.Sprintf("deploy-%02d", i),
			Kind:      "__security_affected_resource__",
			Namespace: "prod",
			Columns:   []model.KeyValue{{Key: "__severity__", Value: "LOW"}},
		}
	}
	return m
}

// TestSecurityGroupPreviewPinsAffectedWhileDetailsScroll guards the split
// behaviour of the finding-group preview: scrolling the pane (shift+j / mouse
// wheel -> previewScroll) must scroll only the group details below the
// separator; the affected-resources table stays pinned on top. Before the fix
// the whole assembled pane (table + details) scrolled as one buffer, pushing
// the table out of view.
func TestSecurityGroupPreviewPinsAffectedWhileDetailsScroll(t *testing.T) {
	m := securityGroupPreviewModel()

	unscrolled := stripANSI(m.renderRightColumn(60, 24))
	require.Contains(t, unscrolled, "AFFECTED RESOURCES")
	require.Contains(t, unscrolled, "deploy-00")
	require.Contains(t, unscrolled, "single_replica", "details title visible before scrolling")

	m.previewScroll = 3
	scrolled := stripANSI(m.renderRightColumn(60, 24))
	assert.Contains(t, scrolled, "AFFECTED RESOURCES",
		"affected-resources table must stay pinned while the details scroll")
	assert.Contains(t, scrolled, "deploy-00",
		"pinned table rows must not scroll away")
	assert.NotContains(t, scrolled, "single_replica",
		"details title must scroll out of view (only the details portion scrolls)")
}

// TestSecurityGroupPreviewSplitGating verifies the split only engages with
// affected rows present — while loading (no rows) the pane falls back to the
// non-split details rendering.
func TestSecurityGroupPreviewSplitGating(t *testing.T) {
	m := securityGroupPreviewModel()
	assert.True(t, m.hasSplitPreview(), "finding group with affected rows splits")

	m.rightItems = nil
	assert.False(t, m.hasSplitPreview(), "no affected rows -> no split")
}

// TestMeasureScrollableLinesRecomputesOnSelectionChange guards the selName
// memo-key field: switching between two finding groups whose layout
// dimensions are identical (same affected count, level, split state) but
// whose descriptions differ in length must recompute the scrollable line
// count instead of returning the stale memoized value.
func TestMeasureScrollableLinesRecomputesOnSelectionChange(t *testing.T) {
	m := securityGroupPreviewModel()
	short := m.middleItems[0]
	short.Name = "short_group"
	short.Columns = []model.KeyValue{
		{Key: "Severity", Value: "LOW"},
		{Key: "Affected", Value: "12"},
		{Key: "Description", Value: "one line"},
	}
	m.middleItems = append(m.middleItems, short)

	m.setCursor(0) // long description
	longLines := m.measureScrollableLines(60, 20)
	m.setCursor(1) // short description, every other key dimension unchanged
	shortLines := m.measureScrollableLines(60, 20)

	assert.Less(t, shortLines, longLines,
		"selection change must invalidate the memoized line count")
}

// TestSecurityGroupPreviewClampUsesDetailsOnly verifies clampPreviewScroll
// bounds the scroll against the details portion only (not the assembled
// table+details buffer): with a short description the max scroll is small even
// though the affected table adds many lines.
func TestSecurityGroupPreviewClampUsesDetailsOnly(t *testing.T) {
	m := securityGroupPreviewModel()
	m.middleItems[0].Columns = []model.KeyValue{
		{Key: "Severity", Value: "LOW"},
		{Key: "Affected", Value: "12"},
		{Key: "Description", Value: "short"},
	}
	m.previewScroll = 10_000
	m.clampPreviewScroll()
	assert.Less(t, m.previewScroll, 40,
		"clamp must bound scroll to the short details content, not leave it unbounded")
}
