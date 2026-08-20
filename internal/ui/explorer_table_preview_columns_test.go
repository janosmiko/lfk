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

// resetColumnGlobals snapshots every column-config global RenderTable reads
// and restores it on cleanup, then sets deterministic defaults so the test
// outcome cannot depend on state left behind by other tests.
func resetColumnGlobals(t *testing.T) {
	t.Helper()
	origHidden := ActiveHiddenBuiltinColumns
	origOrder := ActiveColumnOrder
	origSession := ActiveSessionColumns
	origPrinter := ActivePrinterColumns
	origScroll := ActiveMiddleScroll
	origLayout := ActiveTableLayout
	origNameHidden := ActiveNameHidden
	t.Cleanup(func() {
		ActiveHiddenBuiltinColumns = origHidden
		ActiveColumnOrder = origOrder
		ActiveSessionColumns = origSession
		ActivePrinterColumns = origPrinter
		ActiveMiddleScroll = origScroll
		ActiveTableLayout = origLayout
		ActiveNameHidden = origNameHidden
	})
	ActiveHiddenBuiltinColumns = nil
	ActiveColumnOrder = nil
	ActiveSessionColumns = nil
	ActivePrinterColumns = nil
	ActiveMiddleScroll = -1
	ActiveTableLayout = nil
	ActiveNameHidden = false
}

func TestRenderTableCursorlessHonorsHiddenBuiltinColumns(t *testing.T) {
	resetColumnGlobals(t)
	ActiveHiddenBuiltinColumns = map[string]bool{"Status": true, "Age": true}

	items := []model.Item{
		{Name: "pod-a", Kind: "Pod", Status: "Running", Age: "5m"},
		{Name: "pod-b", Kind: "Pod", Status: "Pending", Age: "1m"},
	}
	out := stripANSI(RenderTable("POD", items, -1, 60, 10, false, "", "", false))
	header, _, _ := strings.Cut(out, "\n")
	if strings.Contains(header, "STATUS") || strings.Contains(header, "AGE") {
		t.Fatalf("hidden built-in columns rendered in cursor-less preview header: %q", header)
	}
}

func TestRenderTableCursorlessHonorsColumnOrder(t *testing.T) {
	resetColumnGlobals(t)
	ActiveColumnOrder = []string{"Age", "Status"}

	items := []model.Item{
		{Name: "pod-a", Kind: "Pod", Status: "Running", Age: "5m"},
	}
	out := stripANSI(RenderTable("POD", items, -1, 60, 10, false, "", "", false))
	header, _, _ := strings.Cut(out, "\n")
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
	resetColumnGlobals(t)
	ActiveHiddenBuiltinColumns = map[string]bool{"Name": true}

	items := []model.Item{
		{Name: "pod-a", Kind: "Pod", Status: "Running"},
	}
	out := stripANSI(RenderTable("POD", items, -1, 60, 10, false, "", "", false))
	header, _, _ := strings.Cut(out, "\n")
	if strings.Contains(header, "POD") {
		t.Fatalf("hidden Name column rendered in cursor-less preview header: %q", header)
	}
}
