package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- explainJumpToMatch ---

func TestExplainJumpToMatch(t *testing.T) {
	fields := []model.ExplainField{
		{Name: "apiVersion"},
		{Name: "kind"},
		{Name: "metadata"},
		{Name: "spec"},
		{Name: "status"},
	}

	t.Run("forward finds match", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
			explainCursor: 0,
		}
		found := m.explainJumpToMatch("spec", 1, true)
		assert.True(t, found)
		assert.Equal(t, 3, m.explainCursor)
	})

	t.Run("forward wraps around", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
			explainCursor: 4,
		}
		found := m.explainJumpToMatch("api", 4, true)
		assert.True(t, found)
		assert.Equal(t, 0, m.explainCursor)
	})

	t.Run("backward finds match", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
			explainCursor: 4,
		}
		found := m.explainJumpToMatch("kind", 3, false)
		assert.True(t, found)
		assert.Equal(t, 1, m.explainCursor)
	})

	t.Run("backward wraps around", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
			explainCursor: 0,
		}
		found := m.explainJumpToMatch("status", 0, false)
		assert.True(t, found)
		assert.Equal(t, 4, m.explainCursor)
	})

	t.Run("empty query returns false", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
		}
		found := m.explainJumpToMatch("", 0, true)
		assert.False(t, found)
	})

	t.Run("no fields returns false", func(t *testing.T) {
		m := Model{height: 30}
		found := m.explainJumpToMatch("test", 0, true)
		assert.False(t, found)
	})

	t.Run("no match returns false", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
		}
		found := m.explainJumpToMatch("nonexistent", 0, true)
		assert.False(t, found)
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
		}
		found := m.explainJumpToMatch("SPEC", 0, true)
		assert.True(t, found)
		assert.Equal(t, 3, m.explainCursor)
	})

	t.Run("adjusts scroll when cursor above viewport", func(t *testing.T) {
		m := Model{
			height:        30,
			explainFields: fields,
			explainScroll: 4,
		}
		found := m.explainJumpToMatch("api", 4, true)
		assert.True(t, found)
		assert.Equal(t, 0, m.explainScroll)
	})
}

func TestCov80OpenExplainBrowserAtResources(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Deployment",
		Resource:   "deployments",
		APIGroup:   "apps",
		APIVersion: "v1",
		Namespaced: true,
	}
	result, cmd := m.openExplainBrowser()
	rm := result.(Model)
	assert.True(t, rm.loading)
	_ = cmd
}

func TestCov80OpenExplainBrowserNoSel(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = nil
	result, cmd := m.openExplainBrowser()
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCov80OpenExplainBrowserAtClusters(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelClusters
	result, cmd := m.openExplainBrowser()
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCov80OpenExplainBrowserEmptyResource(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{}
	result, cmd := m.openExplainBrowser()
	rm := result.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovHandleExplainKeyHelp(t *testing.T) {
	m := baseModelCov()
	m.mode = modeExplain
	result, cmd := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	assert.Equal(t, modeHelp, result.(Model).mode)
	assert.Nil(t, cmd)
}

func TestCovHandleExplainKeyQuit(t *testing.T) {
	m := baseModelCov()
	m.mode = modeExplain
	result, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	assert.Equal(t, modeExplorer, result.(Model).mode)
}

func TestCovHandleExplainKeyEscRoot(t *testing.T) {
	m := baseModelCov()
	m.mode = modeExplain
	m.explainPath = ""
	result, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, modeExplorer, result.(Model).mode)
}

func TestCovHandleExplainKeySlash(t *testing.T) {
	m := baseModelCov()
	result, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, result.(Model).explainSearchActive)
}

func TestCovHandleExplainKeyJK(t *testing.T) {
	m := baseModelCov()
	m.explainFields = []model.ExplainField{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.explainCursor = 0
	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).explainCursor)

	m.explainCursor = 2
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, r.(Model).explainCursor)
}

func TestCovHandleExplainKeyGG(t *testing.T) {
	m := baseModelCov()
	m.explainFields = []model.ExplainField{{Name: "a"}, {Name: "b"}}

	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, 1, r.(Model).explainCursor)

	m.explainCursor = 1
	m.pendingG = true
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, r.(Model).explainCursor)

	m.pendingG = false
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.True(t, r.(Model).pendingG)
}

func TestCovHandleExplainKeyDrillPrimitive(t *testing.T) {
	// Only test the non-drillable (primitive) branch; drillable calls execKubectlExplain
	// which dereferences m.client (nil in unit tests).
	m := baseModelCov()
	m.explainFields = []model.ExplainField{{Name: "name", Type: "string"}}
	r, cmd := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)

	// Also test enter/l with no fields (out-of-bounds cursor).
	m.explainFields = nil
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, r)
}

func TestCovHandleExplainKeyBackRoot(t *testing.T) {
	// Only test the root-level exit path; nested path calls execKubectlExplain
	// which dereferences m.client (nil in unit tests).
	m := baseModelCov()
	m.explainPath = ""
	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Equal(t, modeExplorer, r.(Model).mode)
}

func TestCovHandleExplainKeySearchNextPrev(t *testing.T) {
	m := baseModelCov()
	m.explainSearchQuery = "status"
	m.explainFields = []model.ExplainField{{Name: "spec"}, {Name: "status"}, {Name: "meta"}}

	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, 1, r.(Model).explainCursor)

	m2 := baseModelCov()
	m2.explainSearchQuery = "spec"
	m2.explainFields = m.explainFields
	m2.explainCursor = 2
	r, _ = m2.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	assert.Equal(t, 0, r.(Model).explainCursor)

	m3 := baseModelCov()
	m3.explainSearchQuery = "zzz"
	m3.explainFields = m.explainFields
	r, cmd := m3.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)

	r, cmd = m3.handleExplainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	assert.True(t, r.(Model).statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestCovHandleExplainKeyPages(t *testing.T) {
	fields := make([]model.ExplainField, 50)
	for i := range fields {
		fields[i] = model.ExplainField{Name: "f"}
	}
	m := baseModelCov()
	m.explainFields = fields

	r, _ := m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	assert.Greater(t, r.(Model).explainCursor, 0)

	m.explainCursor = 25
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Less(t, r.(Model).explainCursor, 25)

	m.explainCursor = 0
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	assert.Greater(t, r.(Model).explainCursor, 0)

	m.explainCursor = 40
	r, _ = m.handleExplainKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	assert.Less(t, r.(Model).explainCursor, 40)
}

func TestCovHandleExplainSearchKey(t *testing.T) {
	m := baseModelCov()
	m.explainSearchActive = true
	m.explainSearchInput = TextInput{Value: "spec"}
	r, _ := m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).explainSearchActive)
	assert.Equal(t, "spec", r.(Model).explainSearchQuery)

	m.explainSearchActive = true
	m.explainSearchPrevCursor = 3
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.False(t, r.(Model).explainSearchActive)
	assert.Equal(t, 3, r.(Model).explainCursor)

	m.explainSearchActive = true
	m.explainSearchInput = TextInput{Value: "sp", Cursor: 2}
	m.explainFields = []model.ExplainField{{Name: "spec"}}
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "s", r.(Model).explainSearchInput.Value)

	m.explainSearchInput = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.NotEqual(t, "hello", r.(Model).explainSearchInput.Value)

	m.explainSearchInput = TextInput{}
	m.explainFields = []model.ExplainField{{Name: "spec"}}
	r, _ = m.handleExplainSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, "s", r.(Model).explainSearchInput.Value)
}

func TestCovExplainSearchOverlayNormalNav(t *testing.T) {
	m := baseModelCov()
	m.explainRecursiveResults = []model.ExplainField{{Name: "a", Path: "a"}, {Name: "b", Path: "b"}}

	r, _ := m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, r.(Model).explainRecursiveCursor)

	m2 := r.(Model)
	r, _ = m2.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, r.(Model).explainRecursiveCursor)

	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Equal(t, overlayNone, r.(Model).overlay)

	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	assert.True(t, r.(Model).explainRecursiveFilterActive)

	m.pendingG = true
	m.explainRecursiveCursor = 1
	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	assert.Equal(t, 0, r.(Model).explainRecursiveCursor)

	m.explainRecursiveCursor = 0
	r, _ = m.handleExplainSearchOverlayNormalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	assert.Equal(t, 1, r.(Model).explainRecursiveCursor)
}

func TestCovExplainSearchOverlayFilterKey(t *testing.T) {
	m := baseModelCov()
	m.explainRecursiveFilterActive = true
	m.explainRecursiveFilter = TextInput{}

	r, _ := m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, "s", r.(Model).explainRecursiveFilter.Value)

	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, r.(Model).explainRecursiveFilterActive)

	m.explainRecursiveFilter = TextInput{Value: "test"}
	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyEscape})
	assert.Empty(t, r.(Model).explainRecursiveFilter.Value)

	m.explainRecursiveFilter = TextInput{Value: "ab", Cursor: 2}
	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", r.(Model).explainRecursiveFilter.Value)

	m.explainRecursiveFilter = TextInput{Value: "hello", Cursor: 5}
	r, _ = m.handleExplainSearchOverlayFilterKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	assert.Empty(t, r.(Model).explainRecursiveFilter.Value)
}

func TestCovExplainKeyHelp(t *testing.T) {
	m := baseModelExplain()
	result, _ := m.handleExplainKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
	assert.Equal(t, "API Explorer", rm.helpContextMode)
}

func TestCovExplainKeyQuit(t *testing.T) {
	m := baseModelExplain()
	result, _ := m.handleExplainKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovExplainKeyEscAtRoot(t *testing.T) {
	m := baseModelExplain()
	m.explainPath = ""
	result, _ := m.handleExplainKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovExplainKeySlash(t *testing.T) {
	m := baseModelExplain()
	result, _ := m.handleExplainKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.explainSearchActive)
}

func TestCovExplainKeyDown(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 0
	result, _ := m.handleExplainKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.explainCursor)
}

func TestCovExplainKeyUp(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 3
	result, _ := m.handleExplainKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.explainCursor)
}

func TestCovExplainKeyGG(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 3
	result, _ := m.handleExplainKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleExplainKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.explainCursor)
}

func TestCovExplainKeyBigG(t *testing.T) {
	m := baseModelExplain()
	result, _ := m.handleExplainKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.explainCursor)
}

func TestCovExplainKeyBigGWithInput(t *testing.T) {
	m := baseModelExplain()
	m.explainLineInput = "3"
	result, _ := m.handleExplainKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.explainCursor) // 3-1=2
}

func TestCovExplainKeyDigits(t *testing.T) {
	m := baseModelExplain()
	result, _ := m.handleExplainKey(keyMsg("5"))
	rm := result.(Model)
	assert.Equal(t, "5", rm.explainLineInput)
}

func TestCovExplainKeyZeroWithInput(t *testing.T) {
	m := baseModelExplain()
	m.explainLineInput = "3"
	result, _ := m.handleExplainKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, "30", rm.explainLineInput)
}

func TestCovExplainKeyCtrlD(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 0
	result, _ := m.handleExplainKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.explainCursor, 0)
}

func TestCovExplainKeyCtrlU(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 4
	result, _ := m.handleExplainKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.explainCursor, 4)
}

func TestCovExplainKeyCtrlF(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 0
	result, _ := m.handleExplainKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Greater(t, rm.explainCursor, 0)
}

func TestCovExplainKeyCtrlB(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 4
	result, _ := m.handleExplainKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.LessOrEqual(t, rm.explainCursor, 0)
}

// TestCovExplainKeyEnterDrillable is tested via the loading flag since execKubectlExplain needs a client.
func TestCovExplainKeyEnterDrillable(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 0 // "apiVersion" is string type -- won't call execKubectlExplain
	result, cmd := m.handleExplainKey(keyMsg("enter"))
	rm := result.(Model)
	// Non-drillable type shows a status message
	assert.True(t, rm.hasStatusMessage())
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovExplainKeyEnterPrimitive(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 0 // "apiVersion" is string type
	_, cmd := m.handleExplainKey(keyMsg("enter"))
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovExplainKeyBackAtRoot(t *testing.T) {
	m := baseModelExplain()
	m.explainPath = ""
	result, _ := m.handleExplainKey(keyMsg("h"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovExplainKeyBackAtSubpath(t *testing.T) {
	m := baseModelExplain()
	m.explainPath = ""
	// At root, pressing h exits explain view
	result, _ := m.handleExplainKey(keyMsg("h"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovExplainKeySearchN(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchQuery = "spec"
	m.explainCursor = 0
	result, _ := m.handleExplainKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.explainCursor)
}

func TestCovExplainKeySearchNBig(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchQuery = "spec"
	m.explainCursor = 4
	result, _ := m.handleExplainKey(keyMsg("N"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.explainCursor)
}

func TestCovExplainKeySearchNNoMatch(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchQuery = "nonexistent"
	m.explainCursor = 0
	_, cmd := m.handleExplainKey(keyMsg("n"))
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovExplainKeyDefault(t *testing.T) {
	m := baseModelExplain()
	m.explainLineInput = "123"
	result, _ := m.handleExplainKey(keyMsg("x"))
	rm := result.(Model)
	assert.Empty(t, rm.explainLineInput)
}

func TestCovExplainSearchKeyEnter(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchActive = true
	m.explainSearchInput.Insert("spec")
	result, _ := m.handleExplainSearchKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.explainSearchActive)
	assert.Equal(t, "spec", rm.explainSearchQuery)
}

func TestCovExplainSearchKeyEsc(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchActive = true
	m.explainSearchPrevCursor = 2
	result, _ := m.handleExplainSearchKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.explainSearchActive)
	assert.Equal(t, 2, rm.explainCursor)
}

func TestCovExplainSearchKeyBackspace(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchActive = true
	m.explainSearchInput.Insert("sp")
	result, _ := m.handleExplainSearchKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "s", rm.explainSearchInput.Value)
}

func TestCovExplainSearchKeyCtrlW(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchActive = true
	m.explainSearchInput.Insert("foo bar")
	result, _ := m.handleExplainSearchKey(keyMsg("ctrl+w"))
	rm := result.(Model)
	assert.NotEqual(t, "foo bar", rm.explainSearchInput.Value)
}

func TestCovExplainSearchKeyTyping(t *testing.T) {
	m := baseModelExplain()
	m.explainSearchActive = true
	result, _ := m.handleExplainSearchKey(keyMsg("s"))
	rm := result.(Model)
	assert.Equal(t, "s", rm.explainSearchInput.Value)
}

func TestCovExplainJumpToMatchForward(t *testing.T) {
	m := baseModelExplain()
	found := m.explainJumpToMatch("status", 0, true)
	assert.True(t, found)
	assert.Equal(t, 4, m.explainCursor)
}

func TestCovExplainJumpToMatchBackward(t *testing.T) {
	m := baseModelExplain()
	m.explainCursor = 4
	found := m.explainJumpToMatch("kind", 3, false)
	assert.True(t, found)
	assert.Equal(t, 1, m.explainCursor)
}

func TestCovExplainJumpToMatchEmpty(t *testing.T) {
	m := baseModelExplain()
	found := m.explainJumpToMatch("", 0, true)
	assert.False(t, found)
}

func TestCovExplainJumpToMatchNoFields(t *testing.T) {
	m := baseModelExplain()
	m.explainFields = nil
	found := m.explainJumpToMatch("spec", 0, true)
	assert.False(t, found)
}

func TestCovOpenExplainBrowserNoSelection(t *testing.T) {
	m := baseModelExplain()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = nil
	_, cmd := m.openExplainBrowser()
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovOpenExplainBrowserAtContexts(t *testing.T) {
	m := baseModelExplain()
	m.nav.Level = model.LevelClusters
	_, cmd := m.openExplainBrowser()
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestCovExplainSearchOverlayNormalSlash(t *testing.T) {
	m := baseModelExplain()
	m.explainRecursiveFilterActive = false
	m.overlay = overlayExplainSearch
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.explainRecursiveFilterActive)
}

func TestCovExplainSearchOverlayNormalDown(t *testing.T) {
	m := baseModelExplain()
	m.overlay = overlayExplainSearch
	m.explainRecursiveResults = []model.ExplainField{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.explainRecursiveCursor)
}

func TestCovExplainSearchOverlayNormalUp(t *testing.T) {
	m := baseModelExplain()
	m.overlay = overlayExplainSearch
	m.explainRecursiveCursor = 2
	m.explainRecursiveResults = []model.ExplainField{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.explainRecursiveCursor)
}

func TestCovExplainSearchOverlayNormalGG(t *testing.T) {
	m := baseModelExplain()
	m.overlay = overlayExplainSearch
	m.explainRecursiveCursor = 2
	m.explainRecursiveResults = []model.ExplainField{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleExplainSearchOverlayKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.explainRecursiveCursor)
}

func TestCovExplainSearchOverlayNormalBigG(t *testing.T) {
	m := baseModelExplain()
	m.overlay = overlayExplainSearch
	m.explainRecursiveResults = []model.ExplainField{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.explainRecursiveCursor)
}

func TestCovExplainSearchOverlayNormalEsc(t *testing.T) {
	m := baseModelExplain()
	m.overlay = overlayExplainSearch
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovExplainSearchOverlayNormalCtrlD(t *testing.T) {
	m := baseModelExplain()
	m.overlay = overlayExplainSearch
	m.explainRecursiveCursor = 0
	m.explainRecursiveResults = make([]model.ExplainField, 30)
	for i := range m.explainRecursiveResults {
		m.explainRecursiveResults[i] = model.ExplainField{Name: "field"}
	}
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Greater(t, rm.explainRecursiveCursor, 0)
}

func TestCovExplainSearchOverlayNormalCtrlU(t *testing.T) {
	m := baseModelExplain()
	m.overlay = overlayExplainSearch
	m.explainRecursiveCursor = 20
	m.explainRecursiveResults = make([]model.ExplainField, 30)
	for i := range m.explainRecursiveResults {
		m.explainRecursiveResults[i] = model.ExplainField{Name: "field"}
	}
	result, _ := m.handleExplainSearchOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Less(t, rm.explainRecursiveCursor, 20)
}
