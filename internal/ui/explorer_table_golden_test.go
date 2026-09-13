package ui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files in testdata")

// isolateTableGlobals pins every global RenderTable reads to a known state so
// the golden output does not depend on which tests ran before.
func isolateTableGlobals(t *testing.T) {
	t.Helper()
	origNoColor, origTheme := ConfigNoColor, ActiveTheme
	origLayout, origOrder, origHidden := ActiveTableLayout, ActiveColumnOrder, ActiveHiddenBuiltinColumns
	origMiddle, origRight, origRowCache := ActiveMiddleScroll, ActiveRightScroll, ActiveRowCache
	origQuery, origSel, origNyan := ActiveHighlightQuery, ActiveSelectedItems, NyanMode
	origSession, origPrinter, origFullscreen := ActiveSessionColumns, ActivePrinterColumns, ActiveFullscreenMode
	origRef, origCtx, origSecurity := ActiveResourceRef, ActiveContext, ActiveSecurityAvailable
	origSortName, origSortAsc, origTint := ActiveSortColumnName, ActiveSortAscending, ConfigRowStatusTint
	origScrollOff, origNameHidden := ConfigScrollOff, ActiveNameHidden
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ApplyTheme(origTheme)
		ActiveTableLayout, ActiveColumnOrder, ActiveHiddenBuiltinColumns = origLayout, origOrder, origHidden
		ActiveMiddleScroll, ActiveRightScroll, ActiveRowCache = origMiddle, origRight, origRowCache
		ActiveHighlightQuery, ActiveSelectedItems, NyanMode = origQuery, origSel, origNyan
		ActiveSessionColumns, ActivePrinterColumns, ActiveFullscreenMode = origSession, origPrinter, origFullscreen
		ActiveResourceRef, ActiveContext, ActiveSecurityAvailable = origRef, origCtx, origSecurity
		ActiveSortColumnName, ActiveSortAscending, ConfigRowStatusTint = origSortName, origSortAsc, origTint
		ConfigScrollOff, ActiveNameHidden = origScrollOff, origNameHidden
	})

	ConfigNoColor = false
	ApplyTheme(DefaultTheme())
	ActiveTableLayout, ActiveColumnOrder, ActiveHiddenBuiltinColumns = nil, nil, nil
	ActiveMiddleScroll, ActiveRightScroll, ActiveRowCache = -1, -1, nil
	ActiveHighlightQuery, ActiveSelectedItems, NyanMode = "", nil, false
	ActiveSessionColumns, ActivePrinterColumns, ActiveFullscreenMode = nil, nil, false
	ActiveResourceRef, ActiveContext, ActiveSecurityAvailable = ResourceRef{}, "", false
	ActiveSortColumnName, ActiveSortAscending, ConfigRowStatusTint = "", false, RowStatusTintForeground
	ConfigScrollOff = 5
}

func goldenTableItems() []model.Item {
	return []model.Item{
		{
			Name: "api-7d9f", Namespace: "payments", Ready: "1/1", Restarts: "0", Status: "Running", Age: "3d",
			Columns: []model.KeyValue{{Key: "Version", Value: "v1.2.1"}},
		},
		{
			Name: "worker-5c2b", Namespace: "batch-processing", Ready: "0/1", Restarts: "7", Status: "CrashLoopBackOff", Age: "12h",
			Columns: []model.KeyValue{{Key: "Version", Value: "v1.2.2"}},
		},
		{
			Name: "cache-0", Namespace: "infra", Ready: "1/1", Restarts: "1", Status: "ContainerCreating", Age: "5m",
			Columns: []model.KeyValue{{Key: "Version", Value: "v1.2.3"}},
		},
	}
}

func TestRenderTable_BuiltinColumnsGolden(t *testing.T) {
	contextItems := goldenTableItems()
	for i := range contextItems {
		contextItems[i].Columns = append(contextItems[i].Columns, model.KeyValue{Key: "Context", Value: "kind-dev"})
	}

	tests := []struct {
		name  string
		items []model.Item
		width int
		order []string
	}{
		{name: "wide", items: goldenTableItems(), width: 140},
		{name: "narrow", items: goldenTableItems(), width: 48},
		{name: "context_extra", items: contextItems, width: 140},
		{name: "custom_order", items: goldenTableItems(), width: 140, order: []string{"Age", "Status", "Namespace"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateTableGlobals(t)
			ActiveColumnOrder = tt.order

			got := RenderTable("NAME", tt.items, 1, tt.width, 10, false, "", "")
			require.Contains(t, got, "\x1b[", "the golden must pin styled output")

			path := filepath.Join("testdata", "explorer_table_"+tt.name+".golden")
			if *updateGolden {
				require.NoError(t, os.MkdirAll("testdata", 0o755))
				require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err, "run with -update to create the golden file")
			assert.Equal(t, string(want), got, "rendered table:\n%s", strings.ReplaceAll(got, "\x1b", `\e`))
		})
	}
}
