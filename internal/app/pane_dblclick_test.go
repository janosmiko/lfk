package app

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

func fakePaneClock(t *testing.T) *time.Time {
	t.Helper()
	now := time.Unix(1_700_000_000, 0)
	prev := paneClock
	paneClock = func() time.Time { return now }
	t.Cleanup(func() { paneClock = prev })
	return &now
}

func draggedModel(t *testing.T, clock *time.Time) Model {
	t.Helper()
	m := dragLeftTo(t, baseExplorerModel(), 33)
	m = release(m, 33, 5)
	*clock = clock.Add(5 * time.Second)
	return m
}

func doubleClick(m Model, clock *time.Time, x int, gap time.Duration) Model {
	m = press(m, x, 5)
	m = release(m, x, 5)
	*clock = clock.Add(gap)
	return press(m, x, 5)
}

func defaultWidths(width int) []int {
	d := baseExplorerModel()
	d.width = width
	l, mid, r := d.explorerColumnWidths()
	return []int{l, mid, r}
}

func widthsOf(m Model) []int {
	l, mid, r := m.explorerColumnWidths()
	return []int{l, mid, r}
}

func TestPaneDoubleClickResetsLayoutAfterDrag(t *testing.T) {
	isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := draggedModel(t, clock)
	require.NotEqual(t, defaultWidths(120), widthsOf(m))

	leftEnd, _ := m.columnBoundaries()
	m = doubleClick(m, clock, leftEnd-1, 100*time.Millisecond)

	assert.Equal(t, defaultWidths(120), widthsOf(m))
	assert.Equal(t, dividerNone, m.paneResize.drag)
	assert.Zero(t, m.paneResize.leftPermille)
	assert.Zero(t, m.paneResize.rightPermille)
	m.width = 200
	assert.Equal(t, defaultWidths(200), widthsOf(m))
}

func TestPaneDoubleClickRemovesPrefKeysKeepsOthers(t *testing.T) {
	path := isolateViewerPrefs(t)
	require.NoError(t, persistViewerPref(prefLogWrap, true))
	clock := fakePaneClock(t)
	m := draggedModel(t, clock)
	require.NotNil(t, readPrefsFile(t, path).ExplorerLeftPermille)

	leftEnd, _ := m.columnBoundaries()
	_ = doubleClick(m, clock, leftEnd-1, 100*time.Millisecond)

	s := readPrefsFile(t, path)
	assert.Nil(t, s.ExplorerLeftPermille)
	assert.Nil(t, s.ExplorerRightPermille)
	require.NotNil(t, s.LogWrap)
	assert.True(t, *s.LogWrap)
}

func TestPaneDoubleClickRestartGivesDefaultLayout(t *testing.T) {
	isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := draggedModel(t, clock)
	leftEnd, _ := m.columnBoundaries()
	_ = doubleClick(m, clock, leftEnd-1, 100*time.Millisecond)

	ApplyViewerPrefs()
	m2 := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	m2.width = 120
	assert.Equal(t, defaultWidths(120), widthsOf(m2))
}

func TestPaneDoubleClickWithoutOverrideDoesNotWrite(t *testing.T) {
	path := isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = doubleClick(m, clock, leftEnd-1, 100*time.Millisecond)
	assert.Equal(t, defaultWidths(120), widthsOf(m))
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestPaneClicksOnDifferentBordersDoNotReset(t *testing.T) {
	isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := draggedModel(t, clock)
	want := widthsOf(m)
	leftEnd, middleEnd := m.columnBoundaries()

	m = press(m, leftEnd-1, 5)
	m = release(m, leftEnd-1, 5)
	*clock = clock.Add(100 * time.Millisecond)
	m = press(m, middleEnd-1, 5)
	assert.Equal(t, want, widthsOf(m))
	assert.Equal(t, dividerRight, m.paneResize.drag)
}

func TestPaneClicksMoreThan400msApartDoNotReset(t *testing.T) {
	isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := draggedModel(t, clock)
	want := widthsOf(m)
	leftEnd, _ := m.columnBoundaries()
	m = doubleClick(m, clock, leftEnd-1, 500*time.Millisecond)
	assert.Equal(t, want, widthsOf(m))
	assert.Equal(t, dividerLeft, m.paneResize.drag)
}

func TestPaneClickBetweenOnNonBorderBreaksPair(t *testing.T) {
	isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := draggedModel(t, clock)
	want := widthsOf(m)
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m = release(m, leftEnd-1, 5)
	m = press(m, 60, 5)
	*clock = clock.Add(50 * time.Millisecond)
	m = press(m, leftEnd-1, 5)
	assert.Equal(t, want, widthsOf(m))
}

func TestPaneDragThenQuickPressDoesNotCountAsDoubleClick(t *testing.T) {
	isolateViewerPrefs(t)
	clock := fakePaneClock(t)
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m = motion(m, 33, 5)
	m = release(m, 33, 5)
	want := widthsOf(m)
	*clock = clock.Add(100 * time.Millisecond)
	nl, _ := m.columnBoundaries()
	m = press(m, nl-1, 5)
	assert.Equal(t, want, widthsOf(m))
	assert.Equal(t, dividerLeft, m.paneResize.drag)
}
