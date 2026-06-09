package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Manual refresh (R) in the YAML viewer re-fetches the resource without
// clobbering the viewer's content/scroll up front: updateYamlLoaded swaps the
// content in place when the fetch lands, so the user keeps their position.
func TestYAMLViewer_ManualRefreshKeepsPositionAndFetches(t *testing.T) {
	m := basePush80Model()
	m.mode = modeYAML
	m.yamlView.content = "apiVersion: v1\nkind: Pod\n"
	m.yamlView.scroll = 7

	mdl, cmd := m.handleYAMLRefresh()
	m = mdl.(Model)

	require.NotNil(t, cmd, "refresh must dispatch a fetch command")
	assert.Equal(t, "apiVersion: v1\nkind: Pod\n", m.yamlView.content, "content unchanged until the fetch lands")
	assert.Equal(t, 7, m.yamlView.scroll, "scroll position preserved")
	assert.Equal(t, "Refreshing YAML…", m.statusMessage)
}
