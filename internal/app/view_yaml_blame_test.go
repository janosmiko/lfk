package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

func blameRenderCtx(blame []blameLine, inline bool) yamlRenderCtx {
	return yamlRenderCtx{
		blame:        blame,
		blameInline:  inline,
		mapping:      []int{0, 1},
		gutterWidth:  3,
		contentWidth: 80,
		maxLines:     10,
		yamlCursor:   0,
	}
}

func TestYamlBlameInline_ShowsManagerOperationAndAge(t *testing.T) {
	entry := blameLine{manager: "kubectl-client-side-apply"}
	entry.owner = k8s.FieldOwner{
		Manager:   "kubectl-client-side-apply",
		Operation: "Update",
		Time:      time.Now().Add(-4 * time.Minute),
	}
	ctx := blameRenderCtx([]blameLine{entry}, true)

	got := stripANSI(yamlBlameInline(ctx, 0, 10))

	assert.Contains(t, got, "kubectl-client-side-apply", "inline has room, so the name is not cut")
	assert.Contains(t, got, "Update")
	assert.Contains(t, got, "ago")
}

func TestYamlBlameInline_MarksAnInheritedOwner(t *testing.T) {
	ctx := blameRenderCtx([]blameLine{{manager: "helm", rolled: true}}, true)

	assert.Contains(t, stripANSI(yamlBlameInline(ctx, 0, 10)), "inherited")
}

func TestYamlBlameInline_EmptyWhenOffOrUnknown(t *testing.T) {
	entry := []blameLine{{manager: "kubectl"}}
	assert.Empty(t, yamlBlameInline(blameRenderCtx(entry, false), 0, 10), "toggle off")
	assert.Empty(t, yamlBlameInline(blameRenderCtx(entry, true), 42, 10), "line past the data")
	assert.Empty(t, yamlBlameInline(blameRenderCtx([]blameLine{{}}, true), 0, 10), "no manager")
}

func TestYamlBlameInline_SkippedInVisualMode(t *testing.T) {
	ctx := blameRenderCtx([]blameLine{{manager: "kubectl"}}, true)
	ctx.visualMode = true

	assert.Empty(t, yamlBlameInline(ctx, 0, 10), "a selection must not gain trailing text")
}

func TestYamlBlameInline_SkippedWhenTheLineFillsTheWidth(t *testing.T) {
	ctx := blameRenderCtx([]blameLine{{manager: "kubectl"}}, true)

	assert.Empty(t, yamlBlameInline(ctx, 0, ctx.contentWidth-2), "no room left to say anything useful")
}

func TestRenderYAMLViewportLines_InlineOnlyOnTheCursorLine(t *testing.T) {
	blame := []blameLine{{manager: "argocd"}, {manager: "kubectl"}}
	ctx := blameRenderCtx(blame, true)

	out := renderYAMLViewportLines([]string{"  spec:", "    replicas: 3"}, ctx)

	require.Len(t, out, 2)
	assert.Contains(t, stripANSI(out[0]), "argocd", "cursor line carries the note")
	assert.NotContains(t, stripANSI(out[1]), "kubectl", "other lines stay clean")
}

func TestRenderYAMLViewportLines_LineStartsWithTheLineNumberAgain(t *testing.T) {
	off := renderYAMLViewportLines([]string{"  spec:"}, blameRenderCtx(nil, false))
	on := renderYAMLViewportLines([]string{"  spec:"}, blameRenderCtx([]blameLine{{manager: "kubectl"}}, true))

	assert.True(t, strings.HasPrefix(stripANSI(on[0]), stripANSI(off[0])),
		"the blame trails the line instead of shifting it right")
}

func TestHandleYAMLToggleBlame_OnThenOff(t *testing.T) {
	m := Model{}
	m.yamlView.content = "spec:\n  replicas: 3\n"

	mdl, cmd := m.handleYAMLToggleBlame()
	on := mdl.(Model)
	assert.True(t, on.yamlView.blameOn)
	assert.True(t, on.yamlView.blameLoading)
	assert.NotNil(t, cmd, "turning the blame on has to fetch the managers")

	on.yamlView.blame = []blameLine{{manager: "kubectl"}}
	mdl, _ = on.handleYAMLToggleBlame()
	off := mdl.(Model)
	assert.False(t, off.yamlView.blameOn)
	assert.Nil(t, off.yamlView.blame, "hiding the blame drops the data with it")
}

func TestUpdateYamlBlameLoaded_ErrorClosesTheBlame(t *testing.T) {
	m := Model{}
	m.yamlView.blameOn = true
	m.yamlView.blameLoading = true

	mdl, _ := m.updateYamlBlameLoaded(yamlBlameLoadedMsg{err: errNoBlameTarget})

	got := mdl.(Model)
	assert.False(t, got.yamlView.blameOn)
	assert.False(t, got.yamlView.blameLoading)
	assert.Nil(t, got.yamlView.blame)
}

func TestYamlViewStateCopy_ClonesTheBlameSlice(t *testing.T) {
	s := yamlViewState{blame: []blameLine{{manager: "kubectl"}}}

	cp := s.copy()
	cp.blame[0].manager = "changed"

	assert.Equal(t, "kubectl", s.blame[0].manager, "a tab snapshot must not alias the live viewer")
}

func TestUpdateYamlBlameLoaded_DropsAReplyForOldContent(t *testing.T) {
	m := Model{}
	m.yamlView.content = "spec:\n  replicas: 3\n"
	m.yamlView.blameOn = true
	m.yamlView.blameLoading = true

	other := "something the viewer no longer shows"
	stale := yamlBlameLoadedMsg{
		blame:       []blameLine{{manager: "kubectl"}},
		contentHash: yamlContentHash(other),
		contentLen:  len(other),
	}
	mdl, _ := m.updateYamlBlameLoaded(stale)

	got := mdl.(Model)
	assert.Nil(t, got.yamlView.blame, "blame built for other content would sit on the wrong lines")
	assert.False(t, got.yamlView.blameOn)
}

func TestUpdateYamlBlameLoaded_AcceptsAReplyForTheCurrentContent(t *testing.T) {
	content := "spec:\n  replicas: 3\n"
	m := Model{}
	m.yamlView.content = content
	m.yamlView.blameOn = true

	mdl, _ := m.updateYamlBlameLoaded(yamlBlameLoadedMsg{
		blame:       []blameLine{{}, {manager: "kubectl"}},
		contentHash: yamlContentHash(content),
		contentLen:  len(content),
	})

	got := mdl.(Model)
	require.Len(t, got.yamlView.blame, 2)
	assert.Equal(t, "kubectl", got.yamlView.blame[1].manager)
	assert.True(t, got.yamlView.blameOn)
}

func TestUpdateYamlBlameLoaded_ToggleOffDuringTheFetchWins(t *testing.T) {
	content := "spec:\n  replicas: 3\n"
	m := Model{}
	m.yamlView.content = content
	m.yamlView.blameLoading = true
	m.yamlView.blameOn = false // the user pressed m again while the fetch ran

	mdl, _ := m.updateYamlBlameLoaded(yamlBlameLoadedMsg{
		blame:       []blameLine{{}, {manager: "kubectl"}},
		contentHash: yamlContentHash(content),
		contentLen:  len(content),
	})

	got := mdl.(Model)
	assert.False(t, got.yamlView.blameOn, "the reply must not reopen blame the user closed")
	assert.Nil(t, got.yamlView.blame)
	assert.False(t, got.yamlView.blameLoading)
}

func TestComputeYAMLBlame_StripsBidiOverrideFromTheManagerName(t *testing.T) {
	// U+202E reverses the text that follows it, so a name can read as another.
	owners := ownersFrom(t, "kube\u202ectl\u2066x", `{"f:spec":{"f:replicas":{}}}`)

	got := computeYAMLBlame("spec:\n  replicas: 3\n", owners)

	assert.Equal(t, "kubectlx", got[1].manager)
}

func TestRenderYAMLViewportLines_WrappedNoteDroppedWhenTheViewportCutsTheLine(t *testing.T) {
	ctx := blameRenderCtx([]blameLine{{manager: "kubectl"}}, true)
	ctx.wrap = true
	ctx.contentWidth = 40
	ctx.maxLines = 2

	long := "  key: " + strings.Repeat("value ", 30)
	out := renderYAMLViewportLines([]string{long}, ctx)

	require.Len(t, out, 2, "the viewport cut the line before its last piece")
	for _, line := range out {
		assert.NotContains(t, stripANSI(line), "kubectl",
			"the note belongs after the end of the line, which is off screen here")
	}
}

func TestRenderYAMLViewportLines_WrappedNoteOnTheLastSubLine(t *testing.T) {
	ctx := blameRenderCtx([]blameLine{{manager: "kubectl"}}, true)
	ctx.wrap = true
	ctx.contentWidth = 60
	ctx.maxLines = 20

	out := renderYAMLViewportLines([]string{"  key: " + strings.Repeat("v ", 30)}, ctx)

	require.Greater(t, len(out), 1, "the line has to wrap for this test to mean anything")
	// The last piece must be short enough to leave room for the note.
	assert.NotContains(t, stripANSI(out[0]), "kubectl", "not on the first piece")
	assert.Contains(t, stripANSI(out[len(out)-1]), "kubectl", "on the last piece")
}

func TestRenderYAMLViewportLines_WrapModeNotesOnlyTheCursorLine(t *testing.T) {
	ctx := blameRenderCtx([]blameLine{{manager: "argocd"}, {manager: "kubectl"}}, true)
	ctx.wrap = true
	ctx.contentWidth = 60
	ctx.maxLines = 20
	ctx.yamlCursor = 1

	out := renderYAMLViewportLines([]string{"  a: 1", "  b: 2"}, ctx)

	joined := stripANSI(strings.Join(out, "\n"))
	assert.NotContains(t, joined, "argocd", "a line that is not the cursor gets no note")
	assert.Contains(t, joined, "kubectl", "the cursor line gets one")
}

func TestResetBlame_ClearsEverything(t *testing.T) {
	s := yamlViewState{blameOn: true, blameLoading: true, blame: []blameLine{{manager: "kubectl"}}}

	s.resetBlame()

	assert.False(t, s.blameOn)
	assert.False(t, s.blameLoading)
	assert.Nil(t, s.blame)
}

func TestUpdateYamlBlameLoaded_DropsAnOlderRequestForTheSameContent(t *testing.T) {
	// Two toggles in a row put two fetches in flight over one document, so
	// the content hash cannot tell them apart. Only the newer owners count.
	content := "spec:\n  replicas: 3\n"
	m := Model{}
	m.yamlView.content = content
	m.yamlView.blameOn = true
	m.yamlView.blameReq = 2

	reply := func(req uint64, manager string) yamlBlameLoadedMsg {
		return yamlBlameLoadedMsg{
			blame:       []blameLine{{}, {manager: manager}},
			req:         req,
			contentHash: yamlContentHash(content),
			contentLen:  len(content),
		}
	}

	mdl, _ := m.updateYamlBlameLoaded(reply(2, "current"))
	mdl, _ = mdl.(Model).updateYamlBlameLoaded(reply(1, "stale"))

	got := mdl.(Model)
	require.Len(t, got.yamlView.blame, 2)
	assert.Equal(t, "current", got.yamlView.blame[1].manager,
		"a reply from the earlier fetch must not overwrite the later one")
	assert.True(t, got.yamlView.blameOn, "a dropped reply leaves the view as it is")
}

func TestHandleYAMLToggleBlame_EachRequestGetsANewNumber(t *testing.T) {
	m := Model{}
	m.yamlView.content = "spec:\n  replicas: 3\n"

	mdl, _ := m.handleYAMLToggleBlame()
	first := mdl.(Model).yamlView.blameReq
	mdl, _ = mdl.(Model).handleYAMLToggleBlame() // off
	mdl, _ = mdl.(Model).handleYAMLToggleBlame() // on again

	assert.Greater(t, mdl.(Model).yamlView.blameReq, first)
}
