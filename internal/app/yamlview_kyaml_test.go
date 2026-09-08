package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

const kyamlTestSource = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: web\nspec:\n  replicas: 3\n"

func kyamlTestModel() Model {
	m := Model{width: 100, height: 40, mode: modeYAML}
	m.yamlView = yamlViewState{
		content:   kyamlTestSource,
		source:    kyamlTestSource,
		sections:  parseYAMLSections(kyamlTestSource),
		collapsed: map[string]bool{},
	}
	return m
}

// runKYAMLToggle presses the toggle and delivers the reply the command
// produces, which is what the event loop does across two ticks.
func runKYAMLToggle(t *testing.T, m Model) Model {
	t.Helper()
	mdl, cmd := m.toggleYAMLKYAML()
	require.NotNil(t, cmd, "conversion is off-thread, so the toggle must hand back a command")
	msg, ok := cmd().(yamlKYAMLRenderedMsg)
	require.True(t, ok, "the command must resolve to a yamlKYAMLRenderedMsg")
	out, _ := mdl.(Model).updateYAMLKYAMLRendered(msg)
	return out.(Model)
}

// The conversion is O(document) work. Doing it inline would freeze the render
// loop on a large CRD, which is why the initial load already runs off-thread.
func TestToggleYAMLKYAML_DefersConversionToACommand(t *testing.T) {
	m := kyamlTestModel()

	mdl, cmd := m.toggleYAMLKYAML()
	rm := mdl.(Model)

	require.NotNil(t, cmd)
	assert.Equal(t, kyamlTestSource, rm.yamlView.content,
		"content must not change until the reply lands")
	assert.False(t, rm.yamlView.kyaml, "the flag flips with the reply, not with the keypress")
	assert.Contains(t, rm.statusMessage, "Rendering KYAML")
}

func TestToggleYAMLKYAML_RendersConvertedContent(t *testing.T) {
	rm := runKYAMLToggle(t, kyamlTestModel())

	assert.True(t, rm.yamlView.kyaml)
	assert.Contains(t, rm.yamlView.content, `kind: "Pod",`)
	assert.Contains(t, rm.yamlView.content, "metadata: {")
	assert.Equal(t, kyamlTestSource, rm.yamlView.source, "the block YAML stays available to toggle back to")
}

func TestUpdateYAMLKYAMLRendered_IgnoresStaleReply(t *testing.T) {
	m := kyamlTestModel()
	mdl, cmd := m.toggleYAMLKYAML()
	rm := mdl.(Model)
	stale := cmd().(yamlKYAMLRenderedMsg)

	// The user toggles again before the first conversion comes back.
	mdl2, _ := rm.toggleYAMLKYAML()
	rm2 := mdl2.(Model)

	out, _ := rm2.updateYAMLKYAMLRendered(stale)
	got := out.(Model)

	assert.Equal(t, kyamlTestSource, got.yamlView.content,
		"a superseded conversion must not overwrite the document")
	assert.False(t, got.yamlView.kyaml)
}

func TestUpdateYAMLKYAMLRendered_ErrorKeepsFlagOff(t *testing.T) {
	m := kyamlTestModel()
	m.yamlView.kyamlReq = 7

	out, _ := m.updateYAMLKYAMLRendered(yamlKYAMLRenderedMsg{req: 7, err: assert.AnError})
	got := out.(Model)

	assert.False(t, got.yamlView.kyaml)
	assert.Equal(t, kyamlTestSource, got.yamlView.content)
	assert.True(t, got.statusMessageErr)
}

// A reply for the previous resource must not repaint the one now on screen.
func TestYamlLoadedMsg_SupersedesAnInFlightConversion(t *testing.T) {
	m := kyamlTestModel()
	toggled, cmd := m.toggleYAMLKYAML()
	stale := cmd().(yamlKYAMLRenderedMsg)

	// The event loop carries the toggled model forward, counter and all.
	other := "kind: Service\n"
	mdl, _ := toggled.(Model).updateYamlLoaded(yamlLoadedMsg{content: other, sections: parseYAMLSections(other)})
	rm := mdl.(Model)

	out, _ := rm.updateYAMLKYAMLRendered(stale)
	got := out.(Model)

	assert.Equal(t, other, got.yamlView.content, "the newly loaded document wins")
}

func TestToggleYAMLKYAML_TogglingBackRestoresSource(t *testing.T) {
	m := kyamlTestModel()

	rm := runKYAMLToggle(t, runKYAMLToggle(t, m))

	assert.False(t, rm.yamlView.kyaml)
	assert.Equal(t, kyamlTestSource, rm.yamlView.content)
	assert.Equal(t, parseYAMLSections(kyamlTestSource), rm.yamlView.sections,
		"folds come back with the block YAML they were parsed from")
}

func TestToggleYAMLKYAML_ResetsViewState(t *testing.T) {
	m := kyamlTestModel()
	m.yamlView.cursor = 4
	m.yamlView.scroll = 2
	m.yamlView.collapsed["spec"] = true
	m.yamlView.matchLines = []int{1, 2}
	m.yamlView.matchIdx = 1
	m.yamlView.visualMode = true
	m.yamlView.visualStart = 1
	m.yamlView.lineInput = "12"

	rm := runKYAMLToggle(t, m)

	assert.Zero(t, rm.yamlView.cursor)
	assert.Zero(t, rm.yamlView.scroll)
	assert.Empty(t, rm.yamlView.collapsed, "line numbers moved, so old fold keys no longer address anything")
	assert.Nil(t, rm.yamlView.matchLines)
	assert.Zero(t, rm.yamlView.matchIdx)
	assert.False(t, rm.yamlView.visualMode)
	assert.Empty(t, rm.yamlView.lineInput)
	assert.Equal(t, yamlFoldPrefixLen, rm.yamlView.visualCurCol,
		"the cursor column resets past the fold gutter, as it does on view entry")
}

func TestToggleYAMLKYAML_DropsFoldSectionsWhileOn(t *testing.T) {
	rm := runKYAMLToggle(t, kyamlTestModel())

	assert.Nil(t, rm.yamlView.sections,
		"fold ranges are indexed against block-YAML line numbers, which KYAML does not share")
}

func TestToggleYAMLKYAML_ConversionErrorKeepsFlagOff(t *testing.T) {
	m := kyamlTestModel()
	broken := "a: [1, 2\nb: {"
	m.yamlView.content = broken
	m.yamlView.source = broken

	rm := runKYAMLToggle(t, m)

	assert.False(t, rm.yamlView.kyaml, "a failed conversion must not leave the viewer claiming KYAML")
	assert.Equal(t, broken, rm.yamlView.content, "content is untouched on failure")
	assert.True(t, rm.statusMessageErr)
	assert.Contains(t, rm.statusMessage, "KYAML")
}

func TestYAMLNormalKey_KTogglesKYAML(t *testing.T) {
	m := kyamlTestModel()

	mdl, cmd := m.handleYAMLNormalKey(tea.KeyPressMsg{Code: 'K', Text: "K"})
	require.NotNil(t, cmd, "K must dispatch the conversion, not run it inline")

	out, _ := mdl.(Model).updateYAMLKYAMLRendered(cmd().(yamlKYAMLRenderedMsg))
	rm := out.(Model)

	require.True(t, rm.yamlView.kyaml, "K toggles KYAML in the YAML viewer")
	assert.Contains(t, rm.yamlView.content, `kind: "Pod",`)
}

func TestYAMLNormalKey_BlameIsGatedWhileKYAMLIsOn(t *testing.T) {
	rm := runKYAMLToggle(t, kyamlTestModel())
	rm.statusMessage = ""

	mdl, _ := rm.handleYAMLNormalKey(tea.KeyPressMsg{Code: 'm', Text: "m"})
	rm = mdl.(Model)

	assert.False(t, rm.yamlView.blameOn, "blame maps to block-YAML lines, so it cannot run over KYAML")
	assert.False(t, rm.yamlView.blameLoading)
	assert.Equal(t, "Not available in KYAML mode", rm.statusMessage)
	assert.True(t, rm.statusMessageErr)
}

func TestYAMLNormalKey_ObjectExplorerIsGatedWhileKYAMLIsOn(t *testing.T) {
	rm := runKYAMLToggle(t, kyamlTestModel())
	rm.statusMessage = ""

	mdl, _ := rm.handleYAMLNormalKey(tea.KeyPressMsg{Code: 'O', Text: "O"})
	rm = mdl.(Model)

	assert.Equal(t, modeYAML, rm.mode, "O must not navigate away while KYAML is on")
	assert.Equal(t, "Not available in KYAML mode", rm.statusMessage)
	assert.True(t, rm.statusMessageErr)
}

func TestYAMLNormalKey_BlameAndObjectExplorerWorkWhenKYAMLIsOff(t *testing.T) {
	m := kyamlTestModel()

	mdl, _ := m.handleYAMLNormalKey(tea.KeyPressMsg{Code: 'm', Text: "m"})
	rm := mdl.(Model)

	assert.True(t, rm.yamlView.blameOn, "the gate must not leak into plain YAML mode")
	assert.NotEqual(t, "Not available in KYAML mode", rm.statusMessage)
}

func TestSetCommand_KYAMLTogglesViewer(t *testing.T) {
	m := kyamlTestModel()

	mdl, cmd := m.executeSetCommand("kyaml")
	require.NotNil(t, cmd)
	out, _ := mdl.(Model).updateYAMLKYAMLRendered(cmd().(yamlKYAMLRenderedMsg))
	rm := out.(Model)

	assert.True(t, rm.yamlView.kyaml)
	assert.Contains(t, rm.yamlView.content, `kind: "Pod",`)

	mdl, cmd = rm.executeSetCommand("nokyaml")
	require.NotNil(t, cmd)
	out, _ = mdl.(Model).updateYAMLKYAMLRendered(cmd().(yamlKYAMLRenderedMsg))
	rm = out.(Model)

	assert.False(t, rm.yamlView.kyaml)
	assert.Equal(t, kyamlTestSource, rm.yamlView.content)
}

func TestSetOptions_IncludeKYAML(t *testing.T) {
	opts := setOptions()
	assert.Contains(t, opts, "kyaml")
	assert.Contains(t, opts, "nokyaml")
}

func TestYamlLoadedMsg_KeepsSourceAndConvertsWhenKYAMLIsOn(t *testing.T) {
	m := Model{width: 100, height: 40, mode: modeYAML}
	m.yamlView.kyaml = true

	mdl, cmd := m.updateYamlLoaded(yamlLoadedMsg{
		content:  kyamlTestSource,
		sections: parseYAMLSections(kyamlTestSource),
	})
	require.NotNil(t, cmd, "a load under KYAML converts off-thread too")
	rm := mdl.(Model)
	assert.Equal(t, kyamlTestSource, rm.yamlView.content,
		"the block YAML stays on screen until the conversion lands")

	out, _ := rm.updateYAMLKYAMLRendered(cmd().(yamlKYAMLRenderedMsg))
	rm = out.(Model)

	assert.Equal(t, kyamlTestSource, rm.yamlView.source)
	assert.Contains(t, rm.yamlView.content, `kind: "Pod",`)
	assert.Nil(t, rm.yamlView.sections, "a refresh under KYAML must not keep block-YAML fold ranges")
}

func TestYamlLoadedMsg_PlainModeIsUnchanged(t *testing.T) {
	m := Model{width: 100, height: 40, mode: modeYAML}
	want := parseYAMLSections(kyamlTestSource)

	mdl, _ := m.updateYamlLoaded(yamlLoadedMsg{content: kyamlTestSource, sections: want})
	rm := mdl.(Model)

	assert.Equal(t, kyamlTestSource, rm.yamlView.content)
	assert.Equal(t, kyamlTestSource, rm.yamlView.source)
	assert.Equal(t, want, rm.yamlView.sections)
}

func TestYAMLHintBar_ShowsKYAMLToggle(t *testing.T) {
	m := kyamlTestModel()
	assert.Contains(t, stripANSI(m.yamlHintBar(240)), "kyaml")
}

func TestEnterFullView_SeedsKYAMLFromConfig(t *testing.T) {
	orig := ui.ConfigYAMLViewerKYAML
	t.Cleanup(func() { ui.ConfigYAMLViewerKYAML = orig })

	ui.ConfigYAMLViewerKYAML = true
	m := basePush80Model()
	mdl, _ := m.enterFullView()
	assert.True(t, mdl.(Model).yamlView.kyaml, "the viewer opens with the configured default")

	ui.ConfigYAMLViewerKYAML = false
	m2 := basePush80Model()
	m2.yamlView.kyaml = true
	mdl, _ = m2.enterFullView()
	assert.False(t, mdl.(Model).yamlView.kyaml, "a fresh open re-reads the config rather than keeping the last toggle")
}

// The KYAML flag and its source belong to one tab. Without them on TabState a
// tab rendering KYAML leaks the flag into a tab that is showing block YAML.
func TestTabState_CarriesKYAMLPerTab(t *testing.T) {
	kyamlTab := runKYAMLToggle(t, kyamlTestModel())

	saved := kyamlTab.cloneCurrentTab()
	require.True(t, saved.yamlKYAML, "the snapshot records the flag")
	require.Equal(t, kyamlTestSource, saved.yamlSource)

	m2 := basePush80Model()
	m2.tabs = []TabState{kyamlTab.cloneCurrentTab(), m2.cloneCurrentTab()}
	m2.activeTab = 0
	_ = m2.loadTab(0)
	assert.True(t, m2.yamlView.kyaml, "the KYAML tab comes back rendering KYAML")
	assert.Equal(t, kyamlTestSource, m2.yamlView.source)

	_ = m2.loadTab(1)
	assert.False(t, m2.yamlView.kyaml, "switching to a block-YAML tab clears the flag")
}

// A conversion started on one tab must not repaint another. Per-view counters
// both start at zero, so tab A's first request would match tab B's.
func TestUpdateYAMLKYAMLRendered_ReplyFromAnotherTabIsDropped(t *testing.T) {
	const bDoc = "kind: Service\n"

	m := basePush80Model()
	m.mode = modeYAML
	m.yamlView = yamlViewState{content: kyamlTestSource, source: kyamlTestSource, collapsed: map[string]bool{}}

	bTab := m.cloneCurrentTab()
	bTab.yamlContent, bTab.yamlSource, bTab.yamlKYAML = bDoc, bDoc, false
	m.tabs = []TabState{m.cloneCurrentTab(), bTab}
	m.activeTab = 0

	mdl, cmd := m.toggleYAMLKYAML()
	require.NotNil(t, cmd)
	onA := mdl.(Model)
	replyForA := cmd().(yamlKYAMLRenderedMsg)

	onA.saveCurrentTab()
	_ = onA.loadTab(1)
	require.Equal(t, bDoc, onA.yamlView.content, "tab B is active now")

	out, _ := onA.updateYAMLKYAMLRendered(replyForA)
	got := out.(Model)

	assert.Equal(t, bDoc, got.yamlView.content, "tab A's conversion must not repaint tab B")
	assert.False(t, got.yamlView.kyaml, "nor flip tab B into KYAML")
	assert.Equal(t, bDoc, got.yamlView.source, "nor swap the source it would toggle back to")
}

// A cloned tab is a new document view. Inheriting the source tab's accepted id
// would let the conversion started on the original repaint the clone.
func TestCloneCurrentTab_DoesNotInheritAnInFlightKYAMLRequest(t *testing.T) {
	m := basePush80Model()
	m.mode = modeYAML
	m.yamlView = yamlViewState{content: kyamlTestSource, source: kyamlTestSource, collapsed: map[string]bool{}}

	mdl, cmd := m.toggleYAMLKYAML()
	require.NotNil(t, cmd)
	onA := mdl.(Model)
	replyForA := cmd().(yamlKYAMLRenderedMsg)

	clone := onA.cloneCurrentTab()
	assert.NotEqual(t, replyForA.req, clone.yamlKYAMLReq,
		"the clone must not accept the reply the original is waiting for")

	onA.tabs = []TabState{onA.cloneCurrentTab(), clone}
	onA.activeTab = 0
	onA.saveCurrentTab()
	_ = onA.loadTab(1) // switch to the clone

	out, _ := onA.updateYAMLKYAMLRendered(replyForA)
	got := out.(Model)

	assert.Equal(t, kyamlTestSource, got.yamlView.content, "the clone keeps its own block YAML")
	assert.False(t, got.yamlView.kyaml, "and is not flipped into KYAML by the original's reply")
}

// Switching back must not let a later request collide with the stored one.
func TestTabState_KYAMLRequestIDsAreUniqueAcrossTabs(t *testing.T) {
	m := basePush80Model()
	m.yamlView = yamlViewState{content: kyamlTestSource, source: kyamlTestSource, collapsed: map[string]bool{}}

	mdlA, cmdA := m.toggleYAMLKYAML()
	reqA := cmdA().(yamlKYAMLRenderedMsg).req
	onA := mdlA.(Model)

	other := basePush80Model()
	other.yamlView = yamlViewState{content: "kind: Service\n", source: "kind: Service\n", collapsed: map[string]bool{}}
	_, cmdB := other.toggleYAMLKYAML()
	reqB := cmdB().(yamlKYAMLRenderedMsg).req

	assert.NotEqual(t, reqA, reqB, "two tabs must never share a request id")

	// Leaving a tab keeps its pending id, so the reply still lands on return.
	// Cloning is the opposite case: a new tab must not inherit it.
	onA.saveCurrentTab()
	assert.Equal(t, reqA, onA.tabs[onA.activeTab].yamlKYAMLReq,
		"a tab switch keeps the tab's accepted request")
}

func TestToggleYAMLKYAML_MultiDocumentSourceStaysWhole(t *testing.T) {
	src := "kind: Pod\n---\nkind: Service\n"
	m := kyamlTestModel()
	m.yamlView.content = src
	m.yamlView.source = src

	rm := runKYAMLToggle(t, m)

	require.True(t, rm.yamlView.kyaml)
	assert.Equal(t, 2, strings.Count(rm.yamlView.content, "---\n"))
}
