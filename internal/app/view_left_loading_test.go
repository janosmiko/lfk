package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// At the root level (cluster list) the left column represents the parent of
// the current list — and clusters have no parent. Even if m.loading is true
// (which happens while we're fetching contexts/clusters), the left column
// must not show a spinner there; the loading state belongs to the middle
// column.
func TestLeftColumnLoadingSuppressedAtRoot(t *testing.T) {
	m := Model{
		nav:     model.NavigationState{Level: model.LevelClusters},
		loading: true,
	}
	assert.False(t, m.leftColumnLoading(),
		"at LevelClusters the left column has no parent; must never show a spinner")
}

// At every other navigation level, the loading flag passes through so the
// left column can visualize progress (e.g. context switch / discovery).
func TestLeftColumnLoadingPropagatesElsewhere(t *testing.T) {
	cases := []model.Level{
		model.LevelResourceTypes,
		model.LevelResources,
		model.LevelOwned,
		model.LevelContainers,
	}
	for _, lvl := range cases {
		m := Model{
			nav:     model.NavigationState{Level: lvl},
			loading: true,
		}
		assert.True(t, m.leftColumnLoading(),
			"loading must be visible in the left column at level %v", lvl)
	}
}

// When not loading, the left column is never loading regardless of level.
func TestLeftColumnLoadingFalseWhenNotLoading(t *testing.T) {
	m := Model{
		nav:     model.NavigationState{Level: model.LevelResourceTypes},
		loading: false,
	}
	assert.False(t, m.leftColumnLoading())
}
