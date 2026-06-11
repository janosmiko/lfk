package ui

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

// Issue #408: the right preview pane renders cursor-less with
// ActiveMiddleScroll parked at -1. The user's column config — hidden
// built-ins and explicit order, applied via withSessionColumnsForKind —
// must still take effect so the preview matches the drilled-in list.

func TestRenderTableCursorlessHonorsHiddenBuiltinColumns(t *testing.T) {
	origHidden := ActiveHiddenBuiltinColumns
	origScroll := ActiveMiddleScroll
	t.Cleanup(func() {
		ActiveHiddenBuiltinColumns = origHidden
		ActiveMiddleScroll = origScroll
	})
	ActiveMiddleScroll = -1
	ActiveHiddenBuiltinColumns = map[string]bool{"Status": true, "Age": true}

	items := []model.Item{
		{Name: "pod-a", Kind: "Pod", Status: "Running", Age: "5m"},
		{Name: "pod-b", Kind: "Pod", Status: "Pending", Age: "1m"},
	}
	out := stripANSI(RenderTable("POD", items, -1, 60, 10, false, "", "", false))
	header := strings.SplitN(out, "\n", 2)[0]
	if strings.Contains(header, "STATUS") || strings.Contains(header, "AGE") {
		t.Fatalf("hidden built-in columns rendered in cursor-less preview header: %q", header)
	}
}

func TestRenderTableCursorlessHonorsColumnOrder(t *testing.T) {
	origOrder := ActiveColumnOrder
	origScroll := ActiveMiddleScroll
	t.Cleanup(func() {
		ActiveColumnOrder = origOrder
		ActiveMiddleScroll = origScroll
	})
	ActiveMiddleScroll = -1
	ActiveColumnOrder = []string{"Age", "Status"}

	items := []model.Item{
		{Name: "pod-a", Kind: "Pod", Status: "Running", Age: "5m"},
	}
	out := stripANSI(RenderTable("POD", items, -1, 60, 10, false, "", "", false))
	header := strings.SplitN(out, "\n", 2)[0]
	ageIdx := strings.Index(header, "AGE")
	statusIdx := strings.Index(header, "STATUS")
	if ageIdx < 0 || statusIdx < 0 {
		t.Fatalf("expected AGE and STATUS in header, got %q", header)
	}
	if ageIdx > statusIdx {
		t.Fatalf("ActiveColumnOrder ignored in cursor-less preview: header %q", header)
	}
}

func TestRenderTableCursorlessHonorsHiddenNameColumn(t *testing.T) {
	origHidden := ActiveHiddenBuiltinColumns
	origScroll := ActiveMiddleScroll
	t.Cleanup(func() {
		ActiveHiddenBuiltinColumns = origHidden
		ActiveMiddleScroll = origScroll
		ActiveNameHidden = false
	})
	ActiveMiddleScroll = -1
	ActiveHiddenBuiltinColumns = map[string]bool{"Name": true}

	items := []model.Item{
		{Name: "pod-a", Kind: "Pod", Status: "Running"},
	}
	out := stripANSI(RenderTable("POD", items, -1, 60, 10, false, "", "", false))
	header := strings.SplitN(out, "\n", 2)[0]
	if strings.Contains(header, "POD") {
		t.Fatalf("hidden Name column rendered in cursor-less preview header: %q", header)
	}
}
