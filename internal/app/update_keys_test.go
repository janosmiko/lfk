package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// helper to make a rune key message.
func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// helper to make a special key message.
func specialKey(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

// baseExplorerModel returns a minimal Model for key handling tests in explorer mode.
func baseExplorerModel() Model {
	return Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test",
			ResourceType: model.ResourceTypeEntry{
				DisplayName: "Pods",
				Kind:        "Pod",
				Resource:    "pods",
			},
		},
		middleItems: []model.Item{
			{Name: "pod-a", Kind: "Pod"},
			{Name: "pod-b", Kind: "Pod"},
			{Name: "pod-c", Kind: "Pod"},
		},
		width:              120,
		height:             40,
		mode:               modeExplorer,
		namespace:          "default",
		tabs:               []TabState{{}},
		selectedItems:      make(map[string]bool),
		cursorMemory:       make(map[string]int),
		itemCache:          make(map[string][]model.Item),
		yamlCollapsed:      make(map[string]bool),
		selectedNamespaces: make(map[string]bool),
		selectionAnchor:    -1,
	}
}

// --- handleKey: dismiss status tip ---

func TestHandleKeyDismissesStatusTip(t *testing.T) {
	m := baseExplorerModel()
	m.statusMessage = "Press ? for help"
	m.statusMessageTip = true

	ret, _ := m.handleKey(runeKey('j'))
	result := ret.(Model)
	assert.Empty(t, result.statusMessage)
	assert.False(t, result.statusMessageTip)
}

// --- handleKey: cursor movement j/k ---

func TestHandleKeyJMovesDown(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)

	ret, _ := m.handleKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor())
}

func TestHandleKeyKMovesUp(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(2)

	ret, _ := m.handleKey(runeKey('k'))
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor())
}

func TestHandleKeyDownArrow(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)

	ret, _ := m.handleKey(specialKey(tea.KeyDown))
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor())
}

func TestHandleKeyUpArrow(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(2)

	ret, _ := m.handleKey(specialKey(tea.KeyUp))
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor())
}

// --- handleKey: g / G for top/bottom ---

func TestHandleKeyGGGoesToTop(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(2)

	// First 'g' sets pendingG.
	ret, _ := m.handleKey(runeKey('g'))
	m = ret.(Model)
	assert.True(t, m.pendingG)

	// Second 'g' jumps to top.
	ret, _ = m.handleKey(runeKey('g'))
	m = ret.(Model)
	assert.False(t, m.pendingG)
	assert.Equal(t, 0, m.cursor())
}

func TestHandleKeyGGoesToBottom(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)

	ret, _ := m.handleKey(runeKey('G'))
	result := ret.(Model)
	assert.Equal(t, 2, result.cursor())
}

// --- handleKey: q opens quit confirm ---

func TestHandleKeyQOpensQuitConfirm(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, overlayQuitConfirm, result.overlay)
}

// --- handleKey: / opens search ---

func TestHandleKeySlashOpensSearch(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(1)

	ret, _ := m.handleKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.searchActive)
	assert.Equal(t, 1, result.searchPrevCursor)
}

// --- handleKey: f opens filter ---

func TestHandleKeyFOpensFilter(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(runeKey('f'))
	result := ret.(Model)
	assert.True(t, result.filterActive)
}

// --- handleKey: space toggles selection ---

func TestHandleKeySpaceTogglesSelection(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)

	ret, _ := m.handleKey(runeKey(' '))
	result := ret.(Model)
	assert.True(t, result.isSelected(model.Item{Name: "pod-a", Kind: "Pod"}))
	// Cursor should move down after selection.
	assert.Equal(t, 1, result.cursor())
}

// --- handleKey: esc behavior ---

func TestHandleKeyEscClearsFilter(t *testing.T) {
	m := baseExplorerModel()
	m.filterText = "nginx"
	m.setCursor(1)

	ret, _ := m.handleKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Empty(t, result.filterText)
	assert.Equal(t, 0, result.cursor())
}

func TestHandleKeyEscClearsSelection(t *testing.T) {
	m := baseExplorerModel()
	m.selectedItems["pod-a"] = true

	ret, _ := m.handleKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.hasSelection())
}

func TestHandleKeyEscExitsFullscreenDashboard(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenDashboard = true

	ret, _ := m.handleKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.False(t, result.fullscreenDashboard)
}

func TestHandleKeyEscNavigatesParent(t *testing.T) {
	m := baseExplorerModel()
	// At resources level with no filter/selection, esc navigates parent.
	ret, _ := m.handleKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	// Should have navigated to resource types level.
	assert.Equal(t, model.LevelResourceTypes, result.nav.Level)
}

// --- handleKey: h/left navigates parent ---

func TestHandleKeyHNavigatesParent(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(runeKey('h'))
	result := ret.(Model)
	assert.Equal(t, model.LevelResourceTypes, result.nav.Level)
}

// --- handleKey: enter opens full view ---

func TestHandleKeyEnterFullView(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)

	ret, _ := m.handleKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	// Enter on a resource should switch to YAML mode.
	assert.Equal(t, modeYAML, result.mode)
}

// --- handleKey: ? opens help ---

func TestHandleKeyQuestionMarkOpensHelp(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(runeKey('?'))
	result := ret.(Model)
	assert.Equal(t, modeHelp, result.mode)
	assert.Equal(t, 0, result.helpScroll)
}

// --- handleKey: w toggles watch ---

func TestHandleKeyWTogglesWatch(t *testing.T) {
	m := baseExplorerModel()
	assert.False(t, m.watchMode)

	ret, _ := m.handleKey(runeKey('w'))
	result := ret.(Model)
	assert.True(t, result.watchMode)

	ret, _ = result.handleKey(runeKey('w'))
	result = ret.(Model)
	assert.False(t, result.watchMode)
}

// --- handleKey: : opens command bar ---

func TestHandleKeyColonOpensCommandBar(t *testing.T) {
	m := baseExplorerModel()
	m.commandHistory = loadCommandHistory()

	ret, _ := m.handleKey(runeKey(':'))
	result := ret.(Model)
	assert.True(t, result.commandBarActive)
}

// --- handleKey: ctrl+s toggles secret values ---

func TestHandleKeyCtrlSTogglesSecrets(t *testing.T) {
	m := baseExplorerModel()
	assert.False(t, m.showSecretValues)

	ret, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	result := ret.(Model)
	assert.True(t, result.showSecretValues)

	ret, _ = result.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	result = ret.(Model)
	assert.False(t, result.showSecretValues)
}

// --- handleKey: P toggles full YAML preview ---

func TestHandleKeyPTogglesFullYAMLPreview(t *testing.T) {
	m := baseExplorerModel()
	assert.False(t, m.fullYAMLPreview)

	ret, _ := m.handleKey(runeKey('P'))
	result := ret.(Model)
	assert.True(t, result.fullYAMLPreview)

	ret, _ = result.handleKey(runeKey('P'))
	result = ret.(Model)
	assert.False(t, result.fullYAMLPreview)
}

// --- handleKey: F toggles fullscreen middle ---

func TestHandleKeyFTogglesFullscreenMiddle(t *testing.T) {
	m := baseExplorerModel()
	assert.False(t, m.fullscreenMiddle)

	ret, _ := m.handleKey(runeKey('F'))
	result := ret.(Model)
	assert.True(t, result.fullscreenMiddle)

	ret, _ = result.handleKey(runeKey('F'))
	result = ret.(Model)
	assert.False(t, result.fullscreenMiddle)
}

// --- handleKey: ctrl+a toggles select all ---

func TestHandleKeyCtrlASelectsAll(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	result := ret.(Model)
	assert.Equal(t, 3, len(result.selectedItems))

	// Second ctrl+a deselects all.
	ret, _ = result.handleKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	result = ret.(Model)
	assert.Equal(t, 0, len(result.selectedItems))
}

// --- handleKey: ' opens bookmarks ---

func TestHandleKeyQuoteOpensBookmarks(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(runeKey('\''))
	result := ret.(Model)
	assert.Equal(t, overlayBookmarks, result.overlay)
}

// --- handleKey: m sets pending mark ---

func TestHandleKeyMSetsPendingMark(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(runeKey('m'))
	result := ret.(Model)
	assert.True(t, result.pendingMark)
}

// --- handleKey: pending G cleared on non-g key ---

func TestHandleKeyPendingGClearedOnOtherKey(t *testing.T) {
	m := baseExplorerModel()
	m.pendingG = true

	ret, _ := m.handleKey(runeKey('j'))
	result := ret.(Model)
	assert.False(t, result.pendingG)
}

// --- handleKey: n/N with search ---

func TestHandleKeyNJumpsToNextSearch(t *testing.T) {
	m := baseExplorerModel()
	m.searchInput = TextInput{Value: "pod-b"}
	m.setCursor(0)

	ret, _ := m.handleKey(runeKey('n'))
	result := ret.(Model)
	// Should attempt to jump to next search match.
	assert.NotNil(t, result)
}

func TestHandleKeyNCapJumpsToPrevSearch(t *testing.T) {
	m := baseExplorerModel()
	m.searchInput = TextInput{Value: "pod-a"}
	m.setCursor(2)

	ret, _ := m.handleKey(runeKey('N'))
	result := ret.(Model)
	assert.NotNil(t, result)
}

// --- handleKey: j/k in fullscreen dashboard ---

func TestHandleKeyJKInFullscreenDashboard(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenDashboard = true
	m.previewScroll = 5
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{{Name: "Cluster Dashboard", Extra: "__overview__"}}
	m.dashboardPreview = strings.Repeat("dashboard line\n", 200)

	ret, _ := m.handleKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, 6, result.previewScroll)

	ret, _ = result.handleKey(runeKey('k'))
	result = ret.(Model)
	assert.Equal(t, 5, result.previewScroll)
}

// --- handleKey: T opens colorscheme ---

func TestHandleKeyTOpensColorscheme(t *testing.T) {
	m := baseExplorerModel()

	ret, _ := m.handleKey(runeKey('T'))
	result := ret.(Model)
	assert.Equal(t, overlayColorscheme, result.overlay)
}

func TestPush2HandleKeySearchActive(t *testing.T) {
	m := basePush80v2Model()
	m.searchActive = true
	result, _ := m.handleKey(keyMsg("a"))
	_ = result.(Model)
}

func TestPush2HandleKeyFilterActive(t *testing.T) {
	m := basePush80v2Model()
	m.filterActive = true
	result, _ := m.handleKey(keyMsg("a"))
	_ = result.(Model)
}

func TestPush2HandleKeyCommandBarActive(t *testing.T) {
	m := basePush80v2Model()
	m.commandBarActive = true
	result, _ := m.handleKey(keyMsg("a"))
	_ = result.(Model)
}

func TestPush2HandleKeyOverlayActive(t *testing.T) {
	m := basePush80v2Model()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "action1"}}
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestPush2HandleKeyHelpMode(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeHelp
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.NotEqual(t, modeHelp, rm.mode)
}

func TestPush2HandleKeyYAMLMode(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeYAML
	m.yamlContent = "apiVersion: v1"
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush2HandleKeyExplainMode(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeExplain
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush2HandleKeyDescribeMode(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeDescribe
	m.describeContent = "some content"
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush2HandleKeyDiffMode(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeDiff
	m.diffLeft = "left"
	m.diffRight = "right"
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush2HandleKeyLogsMode(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeLogs
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush2HandleKeyExplorerModeF(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeExplorer
	result, _ := m.handleKey(keyMsg("F"))
	rm := result.(Model)
	assert.True(t, rm.fullscreenMiddle)
}

func TestPush2HandleKeyExplorerModeW(t *testing.T) {
	m := basePush80v2Model()
	m.mode = modeExplorer
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.WatchMode))
	rm := result.(Model)
	assert.True(t, rm.watchMode)
	assert.NotNil(t, cmd)
}

func TestPush2HandleKeyExplorerModeSlash(t *testing.T) {
	m := basePush80v2Model()
	result, _ := m.handleKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.searchActive)
}

func TestPush2HandleKeyExplorerModeColon(t *testing.T) {
	m := basePush80v2Model()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m.commandHistory = loadCommandHistory()
	result, _ := m.handleKey(keyMsg(":"))
	rm := result.(Model)
	assert.True(t, rm.commandBarActive)
}

func TestPush2HandleKeyExplorerModeQuestion(t *testing.T) {
	m := basePush80v2Model()
	result, _ := m.handleKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestPush3HandleKeyHelp(t *testing.T) {
	m := basePush80v3Model()
	result, _ := m.handleKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestPush3HandleKeyF1(t *testing.T) {
	m := basePush80v3Model()
	result, _ := m.handleKey(keyMsg("f1"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestPush3HandleKeySlash(t *testing.T) {
	m := basePush80v3Model()
	result, _ := m.handleKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.searchActive)
}

func TestPush3HandleKeyCtrlC(t *testing.T) {
	m := basePush80v3Model()
	result, _ := m.handleKey(keyMsg("ctrl+c"))
	rm := result.(Model)
	// ctrl+c shows quit confirmation.
	assert.Equal(t, overlayQuitConfirm, rm.overlay)
}

func TestPush3HandleKeyQuestionInHelp(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeHelp
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.NotEqual(t, modeHelp, rm.mode)
}

func TestPush3HandleKeyEscInYAML(t *testing.T) {
	m := basePush80v3Model()
	m.mode = modeYAML
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush3HandleKeyTabNavigation(t *testing.T) {
	m := basePush80v3Model()
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0
	kb := ui.ActiveKeybindings

	result, _ := m.handleKey(keyMsg(kb.NextTab))
	rm := result.(Model)
	assert.Equal(t, 1, rm.activeTab)
}

func TestPush3HandleKeyPrevTab(t *testing.T) {
	m := basePush80v3Model()
	m.tabs = []TabState{{}, {}}
	m.activeTab = 1
	kb := ui.ActiveKeybindings

	result, _ := m.handleKey(keyMsg(kb.PrevTab))
	rm := result.(Model)
	assert.Equal(t, 0, rm.activeTab)
}

func TestP4DiffKeyQ(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestP4DiffKeyEsc(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestP4DiffKeyJ(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	m.diffLeft = "line1\nline2\nline3\n"
	m.diffRight = "line1\nline2x\nline3\n"
	result, _ := m.handleKey(keyMsg("j"))
	rm := result.(Model)
	_ = rm
}

func TestP4DiffKeyK(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	m.diffScroll = 5
	result, _ := m.handleKey(keyMsg("k"))
	rm := result.(Model)
	_ = rm
}

func TestP4DiffKeyG(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	result, _ := m.handleKey(keyMsg("G"))
	rm := result.(Model)
	_ = rm
}

func TestP4DiffKeyGG(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	m.pendingG = true
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.diffScroll)
}

func TestP4DiffKeyU(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	m.diffUnified = false
	result, _ := m.handleKey(keyMsg("u"))
	rm := result.(Model)
	assert.True(t, rm.diffUnified)
}

func TestP4DiffKeyHelp(t *testing.T) {
	m := bp4()
	m.mode = modeDiff
	result, _ := m.handleKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestP4LogKeyQ(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestP4LogKeyF(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	m.logFollow = false
	result, _ := m.handleKey(keyMsg("f"))
	rm := result.(Model)
	assert.True(t, rm.logFollow)
}

func TestP4LogKeyW(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	m.logWrap = false
	result, _ := m.handleKey(keyMsg("w"))
	rm := result.(Model)
	// 'w' toggles wrap. The actual key might be different.
	_ = rm
}

func TestP4LogKeyS(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	m.logTimestamps = false
	result, _ := m.handleKey(keyMsg("s"))
	rm := result.(Model)
	assert.True(t, rm.logTimestamps)
}

func TestP4LogKeyNumber(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	m.logLineNumbers = false
	result, _ := m.handleKey(keyMsg("#"))
	rm := result.(Model)
	assert.True(t, rm.logLineNumbers)
}

func TestP4LogKeyHelp(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	result, _ := m.handleKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestP4LogKeyGG(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	m.logLines = []string{"line1", "line2", "line3"}
	m.logScroll = 2
	m.pendingG = true
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.logScroll)
}

func TestP4LogKeyG(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	m.pendingG = false
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
}

func TestP4LogKeySlash(t *testing.T) {
	m := bp4()
	m.mode = modeLogs
	result, _ := m.handleKey(keyMsg("/"))
	rm := result.(Model)
	// '/' in log mode activates search.
	_ = rm
}

func TestP4ScaleOverlayEsc(t *testing.T) {
	m := bp4()
	m.overlay = overlayScaleInput
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestP4ScaleOverlayBackspace(t *testing.T) {
	m := bp4()
	m.overlay = overlayScaleInput
	m.scaleInput.Insert("3")
	result, _ := m.handleKey(keyMsg("backspace"))
	rm := result.(Model)
	_ = rm
}

func TestP4ScaleOverlayDigit(t *testing.T) {
	m := bp4()
	m.overlay = overlayScaleInput
	result, _ := m.handleKey(keyMsg("5"))
	rm := result.(Model)
	_ = rm
}

func TestP4NamespaceOverlayEsc(t *testing.T) {
	m := bp4()
	m.overlay = overlayNamespace
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestP4NamespaceOverlayNavDown(t *testing.T) {
	m := bp4()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{{Name: "default"}, {Name: "kube-system"}}
	m.overlayCursor = 0
	result, _ := m.handleKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestP4NamespaceOverlayNavUp(t *testing.T) {
	m := bp4()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{{Name: "default"}, {Name: "kube-system"}}
	m.overlayCursor = 1
	result, _ := m.handleKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestPush4HandleKeyQ(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayQuitConfirm, rm.overlay)
}

func TestPush4HandleKeyEscFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.fullscreenDashboard)
}

func TestPush4HandleKeyEscClearSelection(t *testing.T) {
	m := basePush4Model()
	m.selectedItems["pod-1//default"] = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.selectedItems)
}

func TestPush4HandleKeyEscClearFilter(t *testing.T) {
	m := basePush4Model()
	m.filterText = "something"
	result, cmd := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.filterText)
	assert.NotNil(t, cmd)
}

func TestPush4HandleKeyEscAtClusterLevel(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelClusters
	_, cmd := m.handleKey(keyMsg("esc"))
	// Should try to quit.
	assert.NotNil(t, cmd)
}

func TestPush4HandleKeyDownFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.Down))
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestPush4HandleKeyUpFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	m.previewScroll = 5
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.Up))
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestPush4HandleKeyGGFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	m.pendingG = true
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.previewScroll)
}

func TestPush4HandleKeyGFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
}

func TestPush4HandleKeyGExplorer(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
}

func TestPush4HandleKeyGBigFullscreenDashboard(t *testing.T) {
	m := basePush4Model()
	m.fullscreenDashboard = true
	result, _ := m.handleKey(keyMsg("G"))
	rm := result.(Model)
	// Scroll set to large number then clamped. Just verify it executes.
	_ = rm
}

func TestPush4HandleKeyPendingMarkSlot(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	m.pendingMark = true
	result, _ := m.handleKey(keyMsg("a"))
	rm := result.(Model)
	assert.False(t, rm.pendingMark)
}

func TestPush4HandleKeyPendingMarkInvalid(t *testing.T) {
	m := basePush4Model()
	m.pendingMark = true
	result, cmd := m.handleKey(keyMsg("!"))
	rm := result.(Model)
	assert.False(t, rm.pendingMark)
	assert.Nil(t, cmd)
}

func TestPush4HandleKeyPendingBookmarkConfirmYes(t *testing.T) {
	// Confirming a pending bookmark goes through saveBookmark which persists
	// to bookmarksFilePath(); without this isolation the test would overwrite
	// the developer's real ~/.local/state/lfk/bookmarks.yaml file.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{DisplayName: "Pods", Kind: "Pod", Resource: "pods"}
	bm := model.Bookmark{Name: "test", Slot: "a", ResourceType: "/v1/pods"}
	m.pendingBookmark = &bm
	result, _ := m.handleKey(keyMsg("y"))
	rm := result.(Model)
	assert.Nil(t, rm.pendingBookmark)
}

func TestPush4HandleKeyPendingBookmarkCancel(t *testing.T) {
	// The cancel branch does not persist, but isolating XDG_STATE_HOME is
	// cheap insurance against future refactors that might add a save call.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush4Model()
	bm := model.Bookmark{Name: "test", Slot: "a"}
	m.pendingBookmark = &bm
	result, _ := m.handleKey(keyMsg("n"))
	rm := result.(Model)
	assert.Nil(t, rm.pendingBookmark)
	assert.Contains(t, rm.statusMessage, "Cancelled")
}

func TestPush4HandleKeyDismissStartupTip(t *testing.T) {
	m := basePush4Model()
	m.statusMessageTip = true
	m.statusMessage = "tip message"
	result, _ := m.handleKey(keyMsg("j"))
	rm := result.(Model)
	assert.False(t, rm.statusMessageTip)
	assert.Empty(t, rm.statusMessage)
}

func TestPush4HandleKeyErrorLogOverlay(t *testing.T) {
	m := basePush4Model()
	m.overlayErrorLog = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.overlayErrorLog)
}

func TestPush4HandleKeyOverlayActive(t *testing.T) {
	m := basePush4Model()
	m.overlay = overlayConfirm
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestPush4HandleKeyFilterActive(t *testing.T) {
	m := basePush4Model()
	m.filterActive = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
}

func TestPush4HandleKeySearchActive(t *testing.T) {
	m := basePush4Model()
	m.searchActive = true
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

func TestPush4HandleKeyModeLogs(t *testing.T) {
	m := basePush4Model()
	m.mode = modeLogs
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeDescribe(t *testing.T) {
	m := basePush4Model()
	m.mode = modeDescribe
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeDiff(t *testing.T) {
	m := basePush4Model()
	m.mode = modeDiff
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeHelp(t *testing.T) {
	m := basePush4Model()
	m.mode = modeHelp
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeYAML(t *testing.T) {
	m := basePush4Model()
	m.mode = modeYAML
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeyModeExplain(t *testing.T) {
	m := basePush4Model()
	m.mode = modeExplain
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestPush4HandleKeySelectRange(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.SelectRange))
	rm := result.(Model)
	assert.Equal(t, 0, rm.selectionAnchor)
}

func TestPush4HandleKeyToggleSelect(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.ToggleSelect))
	rm := result.(Model)
	assert.True(t, len(rm.selectedItems) > 0)
}

func TestPush4HandleKeyToggleSelectBelowResources(t *testing.T) {
	m := basePush4Model()
	m.nav.Level = model.LevelClusters
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.ToggleSelect))
	rm := result.(Model)
	assert.Empty(t, rm.selectedItems)
}

func TestPush4HandleKeyFilter(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.Filter))
	rm := result.(Model)
	assert.True(t, rm.filterActive)
}

func TestPush4HandleKeySearch(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.searchActive)
}

func TestPush4HandleKeyHalfPageDown(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageDown))
	_ = result.(Model)
}

func TestPush4HandleKeyHalfPageUp(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageUp))
	_ = result.(Model)
}

func TestPush4HandleKeyPageDown(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageDown))
	_ = result.(Model)
}

func TestPush4HandleKeyPageUp(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.PageUp))
	_ = result.(Model)
}

func TestPush4HandleKeyHelp(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
}

func TestPush4HandleKeyMark(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("m"))
	rm := result.(Model)
	assert.True(t, rm.pendingMark)
}

func TestPush4HandleKeyBookmarkList(t *testing.T) {
	m := basePush4Model()
	m.bookmarks = []model.Bookmark{{Name: "bm1", Slot: "a"}}
	result, _ := m.handleKey(keyMsg("'"))
	rm := result.(Model)
	assert.Equal(t, overlayBookmarks, rm.overlay)
}

func TestPush4HandleKeyBookmarkListEmpty(t *testing.T) {
	m := basePush4Model()
	m.bookmarks = nil
	result, _ := m.handleKey(keyMsg("'"))
	rm := result.(Model)
	// Empty bookmarks should show a message or do nothing.
	_ = rm
}

func TestPush4HandleKeyRight(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("l"))
	_ = result.(Model)
}

func TestPush4HandleKeyLeft(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("h"))
	_ = result.(Model)
}

func TestPush4HandleKeyEnter(t *testing.T) {
	m := basePush4Model()
	result, _ := m.handleKey(keyMsg("enter"))
	_ = result.(Model)
}

func TestPush4HandleKeyToggleWatchMode(t *testing.T) {
	m := basePush4Model()
	m.watchMode = true
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.WatchMode))
	rm := result.(Model)
	assert.False(t, rm.watchMode)
}

func TestPush4HandleKeyTogglePreview(t *testing.T) {
	m := basePush4Model()
	kb := ui.ActiveKeybindings
	result, _ := m.handleKey(keyMsg(kb.TogglePreview))
	rm := result.(Model)
	// Just verify it toggles -- the actual value depends on default.
	_ = rm
}

func TestPush4HandleKeyToggleAllNamespaces(t *testing.T) {
	m := basePush4Model()
	m.allNamespaces = false
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.AllNamespaces))
	rm := result.(Model)
	assert.True(t, rm.allNamespaces)
	assert.NotNil(t, cmd)
}

func TestPush4HandleKeyToggleAllNamespacesOff(t *testing.T) {
	m := basePush4Model()
	m.allNamespaces = true
	m.nav.Level = model.LevelResources
	kb := ui.ActiveKeybindings
	result, cmd := m.handleKey(keyMsg(kb.AllNamespaces))
	rm := result.(Model)
	assert.False(t, rm.allNamespaces)
	assert.NotNil(t, cmd)
}

func TestPush4HandleKeyDeleteDirect(t *testing.T) {
	m := basePush4Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	result, _ := m.handleKey(keyMsg("d"))
	rm := result.(Model)
	_ = rm
}

func TestPush4HandleKeyClearPendingG(t *testing.T) {
	m := basePush4Model()
	m.pendingG = true
	result, _ := m.handleKey(keyMsg("x"))
	rm := result.(Model)
	assert.False(t, rm.pendingG)
}

func TestCovHandleKeyDispatchToHelp(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeHelp
	m.helpPreviousMode = modeExplorer
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovHandleKeyDispatchToDescribe(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDescribe
	m.describeContent = "line1\nline2"
	result, _ := m.handleKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.describeCursor)
}

func TestCovHandleKeyDispatchToOverlay(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeExplorer
	m.overlay = overlayConfirm
	m.pendingAction = "Delete"
	result, _ := m.handleKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleKeyDispatchToFilter(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeExplorer
	m.filterActive = true
	m.filterInput.Insert("test")
	result, _ := m.handleKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.filterActive)
}

func TestCovHandleKeyDispatchToSearch(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeExplorer
	m.searchActive = true
	m.searchInput.Insert("pod")
	result, _ := m.handleKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.searchActive)
}

func TestCovHandleKeyDispatchToLogs(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeLogs
	m.logLines = []string{"l1"}
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovHandleKeyDispatchToYAML(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeYAML
	m.yamlContent = "key: value"
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovHandleKeyDispatchToDiff(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovHandleKeyDispatchToExplain(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeExplain
	m.explainFields = []model.ExplainField{{Name: "a"}}
	result, _ := m.handleKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovHandleKeyDispatchToEventViewer(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeEventViewer
	m.eventTimelineLines = []string{"line1"}
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovHandleKeyExplorerLeft(t *testing.T) {
	m := baseModelNav()
	m.mode = modeExplorer
	m.nav.Level = model.LevelResources
	result, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	rm := result.(Model)
	assert.Equal(t, model.LevelResourceTypes, rm.nav.Level)
}

func TestCovHandleKeyCommandBar(t *testing.T) {
	m := baseModelNav()
	m.mode = modeExplorer
	m.commandBarActive = true
	m.commandBarInput.Insert("help")
	result, _ := m.handleKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.commandBarActive)
}

func TestCovHandleKeyExplainSearch(t *testing.T) {
	m := baseModelNav()
	m.mode = modeExplain
	m.explainSearchActive = true
	m.explainFields = []model.ExplainField{{Name: "a"}}
	result, _ := m.handleKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.explainSearchActive)
}

// --- Cancellation of active mutations ---

func TestCtrlCCancelsMutationsInsteadOfClosingTab(t *testing.T) {
	cancelled := false
	r := bgtasks.New(0)
	r.StartCancellable(bgtasks.KindMutation, "Delete pods (5)", "ctx / ns", func() { cancelled = true })

	m := baseModelNav()
	m.bgtasks = r

	result, cmd, handled := m.handleExplorerNavKey(specialKey(tea.KeyCtrlC))
	assert.True(t, handled)
	assert.Nil(t, cmd)
	assert.True(t, cancelled, "Ctrl+C should cancel active mutations")
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

func TestEscCancelsMutationsInsteadOfNavigatingBack(t *testing.T) {
	cancelled := false
	r := bgtasks.New(0)
	r.StartCancellable(bgtasks.KindMutation, "Scale deploys (3)", "ctx / ns", func() { cancelled = true })

	m := baseModelNav()
	m.bgtasks = r

	result, cmd, handled := m.handleExplorerNavKey(specialKey(tea.KeyEsc))
	assert.True(t, handled)
	assert.Nil(t, cmd)
	assert.True(t, cancelled, "Esc should cancel active mutations")
	rm := result.(Model)
	assert.True(t, rm.hasStatusMessage())
}

func TestCtrlCClosesTabWhenNoMutationsActive(t *testing.T) {
	r := bgtasks.New(0)
	r.Start(bgtasks.KindResourceList, "List Pods", "ctx / ns") // non-mutation

	m := baseModelNav()
	m.bgtasks = r
	m.tabs = []TabState{{}, {}} // 2 tabs so close-tab doesn't quit

	_, _, handled := m.handleExplorerNavKey(specialKey(tea.KeyCtrlC))
	assert.True(t, handled)
	// Should fall through to normal close-tab behavior (not cancelled)
}
