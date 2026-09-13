package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestExplorerFrameGolden pins the raw ANSI bytes of one explorer frame so a
// theme style refactor cannot change what the terminal receives. Run with
// UPDATE_GOLDEN=1 to rewrite the golden files.
func TestExplorerFrameGolden(t *testing.T) {
	prevNoColor, prevTransparent, prevContrast := ui.ConfigNoColor, ui.ConfigTransparentBg, ui.ConfigMinContrastRatio
	prevIcons, prevTint, prevLayout := ui.IconMode, ui.ConfigRowStatusTint, ui.ActiveTableLayout
	prevTheme := ui.ActiveTheme
	t.Cleanup(func() {
		ui.ConfigNoColor, ui.ConfigTransparentBg, ui.ConfigMinContrastRatio = prevNoColor, prevTransparent, prevContrast
		ui.IconMode, ui.ConfigRowStatusTint, ui.ActiveTableLayout = prevIcons, prevTint, prevLayout
		ui.ApplyTheme(prevTheme)
	})

	for _, tc := range []struct {
		golden  string
		noColor bool
	}{
		{"explorer_default.golden", false},
		{"explorer_nocolor.golden", true},
	} {
		t.Run(tc.golden, func(t *testing.T) {
			ui.ConfigNoColor, ui.ConfigTransparentBg, ui.ConfigMinContrastRatio = tc.noColor, false, 0
			ui.IconMode, ui.ConfigRowStatusTint, ui.ActiveTableLayout = "unicode", ui.RowStatusTintForeground, nil
			ui.ApplyTheme(ui.DefaultTheme())

			m := goldenExplorerModel()
			got := m.View().Content

			path := filepath.Join("testdata", tc.golden)
			if os.Getenv("UPDATE_GOLDEN") != "" {
				require.NoError(t, os.MkdirAll("testdata", 0o750))
				require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
				return
			}
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, string(want), got)
		})
	}
}

func goldenExplorerModel() Model {
	return Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
			},
		},
		leftItems: []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
		},
		middleItems: []model.Item{
			{Name: "nginx-pod", Status: "Running", Ready: "1/1", Age: "3d"},
			{Name: "crash-pod", Status: "CrashLoopBackOff", Ready: "0/1", Age: "5m"},
			{Name: "pending-pod", Status: "Pending", Ready: "0/1", Age: "2h"},
		},
		width:         160,
		height:        40,
		mode:          modeExplorer,
		namespace:     "default",
		tabs:          []TabState{{}},
		selectedItems: make(map[string]bool),
		cursorMemory:  make(map[string]int),
		itemCache:     make(map[string][]model.Item),
		yamlView: yamlViewState{
			collapsed: make(map[string]bool),
		},
		selectedNamespaces: make(map[string]bool),
	}
}
