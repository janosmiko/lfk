package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func fullscreenDashboardModel(t *testing.T) Model {
	t.Helper()
	got, _ := collapsedSidebarModel().navigateToSelector("__overview__")
	m := got.(Model)
	require.True(t, m.fullscreenDashboard)
	return m
}

func sendKey(t *testing.T, m Model, r rune) Model {
	t.Helper()
	got, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	return got.(Model)
}

func TestGotoChordFromFullscreenDashboardLeavesDashboard(t *testing.T) {
	prev := ui.ConfigGotoTargets
	t.Cleanup(func() { ui.ConfigGotoTargets = prev })
	ui.ConfigGotoTargets = map[string]ui.GotoTargetEntry{
		"ga": {Kind: "Application", Group: "argoproj.io", Name: "ArgoCD Applications"},
	}

	m := sendKey(t, sendKey(t, fullscreenDashboardModel(t), 'g'), 'a')

	require.Equal(t, model.LevelResources, m.nav.Level)
	assert.Equal(t, "applications", m.nav.ResourceType.Resource)
	assert.False(t, m.fullscreenDashboard, "the dashboard must not keep covering the jumped-to list")
}
