package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- orderedColumnKeys: Name as a first-class column ---

func TestOrderedColumnKeys_NameDefaultFirst(t *testing.T) {
	prevScroll, prevOrder := ActiveMiddleScroll, ActiveColumnOrder
	defer func() { ActiveMiddleScroll, ActiveColumnOrder = prevScroll, prevOrder }()
	ActiveMiddleScroll = -1
	ActiveColumnOrder = nil

	// hasName, hasContext, hasNs, hasReady, hasRestarts, hasStatus, hasAge
	got := orderedColumnKeys(true, false, true, false, false, true, true, nil)
	if assert.NotEmpty(t, got) {
		assert.Equal(t, "Name", got[0], "Name leads the default order")
	}
	assert.Equal(t, []string{"Name", "Namespace", "Status", "Age"}, got)
}

func TestOrderedColumnKeys_NameHidden(t *testing.T) {
	prevScroll, prevOrder := ActiveMiddleScroll, ActiveColumnOrder
	defer func() { ActiveMiddleScroll, ActiveColumnOrder = prevScroll, prevOrder }()
	ActiveMiddleScroll = -1
	ActiveColumnOrder = nil

	// hasName=false: hasName, hasContext, hasNs, hasReady, hasRestarts, hasStatus, hasAge
	got := orderedColumnKeys(false, false, true, false, false, false, true, nil)
	assert.NotContains(t, got, "Name", "hidden Name is omitted entirely")
	assert.Equal(t, []string{"Namespace", "Age"}, got)
}

func TestOrderedColumnKeys_NameReordered(t *testing.T) {
	prevScroll, prevOrder := ActiveMiddleScroll, ActiveColumnOrder
	defer func() { ActiveMiddleScroll, ActiveColumnOrder = prevScroll, prevOrder }()
	ActiveMiddleScroll = 0
	ActiveColumnOrder = []string{"Namespace", "Name", "Age"}

	// hasName, hasContext, hasNs, hasReady, hasRestarts, hasStatus, hasAge
	got := orderedColumnKeys(true, false, true, false, false, false, true, nil)
	assert.Equal(t, []string{"Namespace", "Name", "Age"}, got,
		"Name moves to its position in the saved order")
}

// --- formatTableRowOrdered: Name rendered at its ordered position ---

func TestFormatTableRowOrdered_NameAtPosition(t *testing.T) {
	// order places Namespace before Name: the namespace cell must precede
	// the name in the rendered row.
	order := []string{"Namespace", "Name"}
	got := formatTableRowOrdered("mypod", "myns", "", "", "", "",
		10, 0, 8, 0, 0, 0, 0, order, nil, nil)
	nsIdx := strings.Index(got, "myns")
	nameIdx := strings.Index(got, "mypod")
	assert.GreaterOrEqual(t, nsIdx, 0)
	assert.GreaterOrEqual(t, nameIdx, 0)
	assert.Less(t, nsIdx, nameIdx, "namespace cell precedes name when ordered first")
}

func TestFormatTableRowOrdered_NameOmittedWhenHidden(t *testing.T) {
	// "Name" absent from order: no name cell renders even with a positive
	// nameW. Order presence is the single source of truth for the Name column.
	order := []string{"Namespace"}
	got := formatTableRowOrdered("mypod", "myns", "", "", "", "",
		10, 0, 8, 0, 0, 0, 0, order, nil, nil)
	assert.NotContains(t, got, "mypod", "hidden Name leaves no name cell")
	assert.Contains(t, got, "myns")
}

// --- RenderTable integration: reordered NAME header ---

func TestRenderTable_NameReorderedHeader(t *testing.T) {
	prevScroll := ActiveMiddleScroll
	prevOrder := ActiveColumnOrder
	prevLayout := ActiveTableLayout
	prevHidden := ActiveHiddenBuiltinColumns
	defer func() {
		ActiveMiddleScroll = prevScroll
		ActiveColumnOrder = prevOrder
		ActiveTableLayout = prevLayout
		ActiveHiddenBuiltinColumns = prevHidden
	}()
	ActiveMiddleScroll = 0
	ActiveTableLayout = nil
	// RenderTable derives hasName from this global; isolate it so a prior
	// test leaving {"Name": true} can't fail this assertion for the wrong reason.
	ActiveHiddenBuiltinColumns = nil
	ActiveColumnOrder = []string{"Namespace", "Name"}

	items := []model.Item{
		{Name: "alpha", Namespace: "team-a"},
		{Name: "beta", Namespace: "team-b"},
	}
	out := RenderTable("NAME", items, 0, 120, 20, false, "", "")
	header := strings.SplitN(out, "\n", 2)[0]
	nsIdx := strings.Index(header, "NAMESPACE")
	nameIdx := strings.Index(header, "NAME")
	// "NAMESPACE" contains "NAME"; find the standalone NAME header after it.
	standaloneNameIdx := strings.Index(header[nsIdx+len("NAMESPACE"):], "NAME")
	assert.GreaterOrEqual(t, nsIdx, 0, "NAMESPACE header present")
	assert.GreaterOrEqual(t, standaloneNameIdx, 0, "NAME header rendered after NAMESPACE")
	_ = nameIdx
}

func TestRenderTable_NameHiddenDropsColumn(t *testing.T) {
	prevScroll := ActiveMiddleScroll
	prevHidden := ActiveHiddenBuiltinColumns
	prevLayout := ActiveTableLayout
	prevNameHidden := ActiveNameHidden
	defer func() {
		ActiveMiddleScroll = prevScroll
		ActiveHiddenBuiltinColumns = prevHidden
		ActiveTableLayout = prevLayout
		ActiveNameHidden = prevNameHidden
	}()
	ActiveMiddleScroll = 0
	ActiveTableLayout = nil
	ActiveHiddenBuiltinColumns = map[string]bool{"Name": true}

	items := []model.Item{
		{Name: "alpha-pod", Namespace: "team-a", Age: "5m"},
	}
	out := RenderTable("NAME", items, 0, 120, 20, false, "", "")
	header := strings.SplitN(out, "\n", 2)[0]
	assert.Contains(t, header, "NAMESPACE", "other columns still render")
	// "NAMESPACE" contains the substring "NAME"; mask it before asserting the
	// standalone NAME header is gone.
	masked := strings.ReplaceAll(header, "NAMESPACE", "")
	assert.NotContains(t, masked, "NAME", "NAME header dropped when Name hidden")
	assert.NotContains(t, out, "alpha-pod", "name value not rendered when Name hidden")
}
