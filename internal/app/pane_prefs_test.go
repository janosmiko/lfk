package app

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
)

func readPrefsFile(t *testing.T, path string) ViewerPrefsState {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var s ViewerPrefsState
	require.NoError(t, yaml.Unmarshal(data, &s))
	return s
}

func dragLeftTo(t *testing.T, m Model, x int) Model {
	t.Helper()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	return motion(m, x, 5)
}

func TestPanePrefsReleasePersistsBothValues(t *testing.T) {
	path := isolateViewerPrefs(t)
	m := dragLeftTo(t, baseExplorerModel(), 30)
	m = release(m, 30, 5)

	s := readPrefsFile(t, path)
	require.NotNil(t, s.ExplorerLeftPermille)
	require.NotNil(t, s.ExplorerRightPermille)
	assert.Equal(t, m.paneResize.leftPermille, *s.ExplorerLeftPermille)
	assert.Equal(t, m.paneResize.rightPermille, *s.ExplorerRightPermille)
}

func TestPanePrefsMotionDoesNotWrite(t *testing.T) {
	path := isolateViewerPrefs(t)
	_ = dragLeftTo(t, baseExplorerModel(), 30)
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestPanePrefsReleaseWithoutChangeDoesNotWrite(t *testing.T) {
	path := isolateViewerPrefs(t)
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	_ = release(m, leftEnd-1, 5)
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestPanePrefsPreserveOtherKeys(t *testing.T) {
	path := isolateViewerPrefs(t)
	persistViewerPref(prefLogWrap, true)
	m := dragLeftTo(t, baseExplorerModel(), 30)
	_ = release(m, 30, 5)

	s := readPrefsFile(t, path)
	require.NotNil(t, s.LogWrap)
	assert.True(t, *s.LogWrap)
	assert.NotNil(t, s.ExplorerLeftPermille)
}

func TestPanePrefsStartupLoadsValid(t *testing.T) {
	isolateViewerPrefs(t)
	t.Cleanup(func() { explorerPaneStartup = paneResizeState{} })
	l, r := 250, 300
	saveViewerPrefs(ViewerPrefsState{ExplorerLeftPermille: &l, ExplorerRightPermille: &r})
	ApplyViewerPrefs()
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	assert.Equal(t, 250, m.paneResize.leftPermille)
	assert.Equal(t, 300, m.paneResize.rightPermille)
}

func TestPanePrefsStartupRejectsInvalid(t *testing.T) {
	one, big, neg, zero := 1000, 1000, -5, 0
	half := 500
	cases := map[string]ViewerPrefsState{
		"left zero":      {ExplorerLeftPermille: &zero},
		"left negative":  {ExplorerLeftPermille: &neg},
		"right too big":  {ExplorerRightPermille: &big},
		"sum":            {ExplorerLeftPermille: &half, ExplorerRightPermille: &half},
		"left too big":   {ExplorerLeftPermille: &one},
		"both one valid": {ExplorerLeftPermille: &zero, ExplorerRightPermille: &big},
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			isolateViewerPrefs(t)
			saveViewerPrefs(s)
			ApplyViewerPrefs()
			assert.Equal(t, paneResizeState{}, explorerPaneStartup)
		})
	}
}

func TestPanePrefsStartupDropsOnlyInvalidSide(t *testing.T) {
	isolateViewerPrefs(t)
	bad, ok := -5, 300
	saveViewerPrefs(ViewerPrefsState{ExplorerLeftPermille: &bad, ExplorerRightPermille: &ok})
	ApplyViewerPrefs()
	assert.Equal(t, paneResizeState{rightPermille: 300}, explorerPaneStartup)
}

func TestPanePrefsStartupAcceptsSingleValue(t *testing.T) {
	isolateViewerPrefs(t)
	r := 300
	saveViewerPrefs(ViewerPrefsState{ExplorerRightPermille: &r})
	ApplyViewerPrefs()
	assert.Equal(t, paneResizeState{rightPermille: 300}, explorerPaneStartup)
}

func TestPanePrefsRoundTrip(t *testing.T) {
	isolateViewerPrefs(t)
	t.Cleanup(func() { explorerPaneStartup = paneResizeState{} })
	m := dragLeftTo(t, baseExplorerModel(), 33)
	m = release(m, 33, 5)
	l, mid, r := m.explorerColumnWidths()

	ApplyViewerPrefs()
	m2 := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	m2.width, m2.height = 120, 40
	l2, mid2, r2 := m2.explorerColumnWidths()
	assert.Equal(t, []int{l, mid, r}, []int{l2, mid2, r2})
}
