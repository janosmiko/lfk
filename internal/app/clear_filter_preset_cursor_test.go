package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestClearFilterPresetKeepsCursorOnItem(t *testing.T) {
	clearers := map[string]func(Model) (tea.Model, tea.Cmd){
		"dot key": func(m Model) (tea.Model, tea.Cmd) {
			mdl, cmd, _ := m.handleExplorerActionKeyFilterPresets()
			return mdl, cmd
		},
		"esc": Model.handleExplorerEsc,
	}
	for name, clear := range clearers {
		t.Run(name, func(t *testing.T) {
			m := baseModelOverlay()
			m.middleItems = []model.Item{
				{Name: "job-1", Status: "Complete"},
				{Name: "job-2", Status: "Failed"},
				{Name: "job-3", Status: "Complete"},
				{Name: "job-4", Status: "Failed"},
			}
			failed := FilterPreset{
				Name:    "Failed",
				MatchFn: func(item model.Item) bool { return item.Status == "Failed" },
			}
			res, _ := m.applyFilterPreset(failed)
			m = res.(Model)
			m.setCursor(1) // job-4 in the filtered list

			res, _ = clear(m)
			m = res.(Model)

			require.Nil(t, m.activeFilterPreset)
			require.Len(t, m.middleItems, 4)
			sel := m.selectedMiddleItem()
			require.NotNil(t, sel)
			assert.Equal(t, "job-4", sel.Name)
		})
	}
}
