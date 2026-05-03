package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// forceANSIRendering switches lipgloss to the ANSI colour profile so
// styled strings actually emit colour SGR codes during the test —
// otherwise lipgloss strips colour when stdout isn't a TTY (the
// `go test` default), which makes it impossible to distinguish a
// tinted from an untinted render by string comparison.
func forceANSIRendering(t *testing.T) {
	t.Helper()
	r := lipgloss.DefaultRenderer()
	prev := r.ColorProfile()
	r.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { r.SetColorProfile(prev) })
}

func TestClusterColorForActiveContext_ReturnsAssignedColor(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources, Context: "prod-eu"},
		clusterColors: map[string]string{"prod-eu": "red"},
	}
	assert.Equal(t, "red", m.clusterColorForActiveContext(),
		"inside a context with a stored color, the color must surface to the title bar")
}

func TestClusterColorForActiveContext_EmptyAtClusterPicker(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelClusters},
		clusterColors: map[string]string{"prod-eu": "red"},
	}
	assert.Equal(t, "", m.clusterColorForActiveContext(),
		"at the cluster picker there is no active context — return no color so the bar stays neutral")
}

func TestClusterColorForActiveContext_EmptyForUnknownContext(t *testing.T) {
	m := Model{
		nav:           model.NavigationState{Level: model.LevelResources, Context: "scratch"},
		clusterColors: map[string]string{"prod-eu": "red"},
	}
	assert.Equal(t, "", m.clusterColorForActiveContext(),
		"context with no color stored must produce no tint")
}

func TestRenderTitleBar_TintedWhenContextHasColor(t *testing.T) {
	// lipgloss strips colour when not on a TTY, so without forcing the
	// ANSI profile here the tinted and untinted renders would produce
	// identical plain-text strings and the NotEqual would be a
	// tautology even with the test fix CodeRabbit suggested.
	forceANSIRendering(t)

	// Both variants use the SAME context so any difference in output is
	// attributable to the cluster-color tint rather than incidental
	// breadcrumb / context text differences. CodeRabbit caught the prior
	// version using different contexts which made NotEqual a tautology.
	mTinted := minimalRenderableModel()
	mTinted.nav = model.NavigationState{Level: model.LevelResources, Context: "prod-eu"}
	mTinted.clusterColors = map[string]string{"prod-eu": "red"}
	tinted := mTinted.renderTitleBar()

	mPlain := minimalRenderableModel()
	mPlain.nav = model.NavigationState{Level: model.LevelResources, Context: "prod-eu"}
	mPlain.clusterColors = nil // same context, no color: only the tint can differ
	plain := mPlain.renderTitleBar()

	assert.NotEqual(t, plain, tinted,
		"tinted title bar must differ visually from the untinted variant when only the colour assignment changes")
	assert.True(t, strings.Contains(tinted, "prod-eu"),
		"sanity: the breadcrumb section must still render the context name in the tinted variant")
}

func TestRenderTitleBar_NotTintedAtClusterPicker(t *testing.T) {
	m := minimalRenderableModel()
	m.nav = model.NavigationState{Level: model.LevelClusters}
	m.clusterColors = map[string]string{"prod-eu": "red"}

	mPlain := minimalRenderableModel()
	mPlain.nav = model.NavigationState{Level: model.LevelClusters}
	mPlain.clusterColors = nil

	assert.Equal(t, mPlain.renderTitleBar(), m.renderTitleBar(),
		"at the cluster picker the bar must be identical regardless of stored colors")
}

// minimalRenderableModel returns a Model with the bare minimum state for
// renderTitleBar to run without panicking. Width/height are wide enough that
// the breadcrumb truncation logic doesn't trigger and complicate diffs.
func minimalRenderableModel() Model {
	return Model{
		width:  120,
		height: 40,
		nav:    model.NavigationState{Level: model.LevelClusters},
		tabs:   []TabState{{}},
	}
}
