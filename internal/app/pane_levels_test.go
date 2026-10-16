package app

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func perLevelModel() Model {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.paneResize.perLevel = true
	return m
}

func dragAndRelease(t *testing.T, m Model, x int) Model {
	t.Helper()
	return release(dragLeftTo(t, m, x), x, 5)
}

func TestPanePerLevelDragStaysOnItsLevel(t *testing.T) {
	isolateViewerPrefs(t)
	m := dragAndRelease(t, perLevelModel(), 33)
	dragged := widthsOf(m)
	require.NotEqual(t, defaultWidths(120), dragged)

	m.nav.Level = model.LevelResources
	assert.Equal(t, defaultWidths(120), widthsOf(m))

	m.nav.Level = model.LevelResourceTypes
	assert.Equal(t, dragged, widthsOf(m))
}

func TestPanePerLevelOffKeepsGlobalSplit(t *testing.T) {
	isolateViewerPrefs(t)
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m = dragAndRelease(t, m, 33)
	dragged := widthsOf(m)

	m.nav.Level = model.LevelResources
	assert.Equal(t, dragged, widthsOf(m))
}

func TestPanePerLevelPersistsLevelKey(t *testing.T) {
	path := isolateViewerPrefs(t)
	m := dragAndRelease(t, perLevelModel(), 33)

	s := readPrefsFile(t, path)
	assert.Nil(t, s.ExplorerLeftPermille)
	assert.Nil(t, s.ExplorerRightPermille)
	require.Contains(t, s.ExplorerPaneWidths, "resource_types")
	p := m.paneResize.levels[model.LevelResourceTypes]
	assert.Equal(t, PaneWidthsPref{Left: p[0], Right: p[1]}, s.ExplorerPaneWidths["resource_types"])
}

func TestPanePerLevelRoundTrip(t *testing.T) {
	isolateViewerPrefs(t)
	t.Cleanup(func() { explorerPaneStartup = paneResizeState{}; ui.ConfigPaneWidthsPerLevel = false })
	m := dragAndRelease(t, perLevelModel(), 33)
	want := widthsOf(m)

	ui.ConfigPaneWidthsPerLevel = true
	ApplyViewerPrefs()
	m2 := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	m2.width, m2.height = 120, 40
	m2.nav.Level = model.LevelResourceTypes
	assert.Equal(t, want, widthsOf(m2))
	m2.nav.Level = model.LevelClusters
	assert.Equal(t, defaultWidths(120), widthsOf(m2))
}

func TestPanePerLevelStartupValidation(t *testing.T) {
	st := panePerLevelStartup(map[string]PaneWidthsPref{
		"resources":      {Left: 250, Right: 300},
		"owned":          {Left: 0, Right: 1000},
		"containers":     {Left: -3},
		"made_up_level":  {Left: 200, Right: 200},
		"resource_types": {Left: 5000, Right: 400},
	})
	assert.True(t, st.perLevel)
	assert.Equal(t, map[model.Level][2]int{
		model.LevelResources:     {250, 300},
		model.LevelResourceTypes: {0, 400},
	}, st.levels)
}

func TestPanePerLevelDoubleClickResetsOnlyCurrentLevel(t *testing.T) {
	path := isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := dragAndRelease(t, perLevelModel(), 33)
	m.nav.Level = model.LevelResources
	m = dragAndRelease(t, m, 40)
	resTypes := m.paneResize.levels[model.LevelResourceTypes]
	*clock = clock.Add(5 * time.Second)

	leftEnd, _ := m.columnBoundaries()
	m = doubleClick(m, clock, leftEnd-1, 100*time.Millisecond)

	assert.Equal(t, defaultWidths(120), widthsOf(m))
	assert.Zero(t, m.paneResize.levels[model.LevelResources][0])
	assert.Equal(t, resTypes, m.paneResize.levels[model.LevelResourceTypes])
	s := readPrefsFile(t, path)
	assert.Zero(t, s.ExplorerPaneWidths["resources"].Left)
	assert.Contains(t, s.ExplorerPaneWidths, "resource_types")
}

func TestPanePerLevelMapIsNotSharedAcrossCopies(t *testing.T) {
	isolateViewerPrefs(t)
	m := dragAndRelease(t, perLevelModel(), 33)
	snapshot := m
	m.nav.Level = model.LevelResources
	_ = dragAndRelease(t, m, 40)
	assert.NotContains(t, snapshot.paneResize.levels, model.LevelResources)
}

func TestPanePerLevelUnknownLevelIsNotPersisted(t *testing.T) {
	path := isolateViewerPrefs(t)
	require.NoError(t, persistPaneWidthsPrefs(model.LevelResources, 100, 200))
	require.NoError(t, persistPaneWidthsPrefs(model.Level(99), 100, 200))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(raw), "left:"), string(raw))
	assert.Contains(t, readPrefsFile(t, path).ExplorerPaneWidths, "resources")
}

func TestPanePerLevelLevelChangeMidDragKeepsStartLevel(t *testing.T) {
	path := isolateViewerPrefs(t)
	m := perLevelModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m.nav.Level = model.LevelResources
	m = release(motion(m, 33, 5), 33, 5)

	assert.Contains(t, m.paneResize.levels, model.LevelResourceTypes)
	assert.NotContains(t, m.paneResize.levels, model.LevelResources)
	s := readPrefsFile(t, path)
	assert.Contains(t, s.ExplorerPaneWidths, "resource_types")
	assert.NotContains(t, s.ExplorerPaneWidths, "resources")
}

func TestPanePerLevelUndraggedLevelUsesDefault(t *testing.T) {
	isolateViewerPrefs(t)
	m := dragAndRelease(t, perLevelModel(), 33)
	for _, lvl := range []model.Level{model.LevelClusters, model.LevelOwned, model.LevelContainers} {
		m.nav.Level = lvl
		assert.Equal(t, defaultWidths(120), widthsOf(m), "level %d", lvl)
	}
}

func dragRightTo(t *testing.T, m Model, x int) Model {
	t.Helper()
	_, middleEnd := m.columnBoundaries()
	m = press(m, middleEnd-1, 5)
	return release(motion(m, x, 5), x, 5)
}

func TestPanePerLevelDragSavesOnlyDraggedSide(t *testing.T) {
	path := isolateViewerPrefs(t)
	m := dragAndRelease(t, perLevelModel(), 33)

	p := m.paneResize.levels[model.LevelResourceTypes]
	assert.NotZero(t, p[0])
	assert.Zero(t, p[1])
	w := readPrefsFile(t, path).ExplorerPaneWidths["resource_types"]
	assert.Equal(t, PaneWidthsPref{Left: p[0]}, w)

	m = dragRightTo(t, m, 70)
	p2 := m.paneResize.levels[model.LevelResourceTypes]
	assert.Equal(t, p[0], p2[0])
	assert.NotZero(t, p2[1])
	w = readPrefsFile(t, path).ExplorerPaneWidths["resource_types"]
	assert.Equal(t, PaneWidthsPref{Left: p2[0], Right: p2[1]}, w)
}

func TestPanePerLevelDoubleClickResetsOnlyClickedSide(t *testing.T) {
	path := isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := dragAndRelease(t, perLevelModel(), 33)
	m = dragRightTo(t, m, 70)
	p := m.paneResize.levels[model.LevelResourceTypes]
	require.NotZero(t, p[0])
	require.NotZero(t, p[1])
	*clock = clock.Add(5 * time.Second)

	leftEnd, _ := m.columnBoundaries()
	m = doubleClick(m, clock, leftEnd-1, 100*time.Millisecond)

	assert.Equal(t, [2]int{0, p[1]}, m.paneResize.levels[model.LevelResourceTypes])
	w := readPrefsFile(t, path).ExplorerPaneWidths["resource_types"]
	assert.Equal(t, PaneWidthsPref{Right: p[1]}, w)
}

func TestPanePerLevelResettingBothSidesDropsEntry(t *testing.T) {
	path := isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := dragAndRelease(t, perLevelModel(), 33)
	m = dragRightTo(t, m, 70)
	*clock = clock.Add(5 * time.Second)

	leftEnd, middleEnd := m.columnBoundaries()
	m = doubleClick(m, clock, leftEnd-1, 100*time.Millisecond)
	*clock = clock.Add(5 * time.Second)
	m = doubleClick(m, clock, middleEnd-1, 100*time.Millisecond)

	assert.NotContains(t, m.paneResize.levels, model.LevelResourceTypes)
	assert.NotContains(t, readPrefsFile(t, path).ExplorerPaneWidths, "resource_types")
	assert.Equal(t, defaultWidths(120), widthsOf(m))
}
