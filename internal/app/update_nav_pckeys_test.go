package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// PC-style navigation keys (Home/End/PgUp/PgDown) should behave the same as
// their vim equivalents (gg/G/Ctrl+F/Ctrl+B) across the various viewer and
// overlay handlers.
//
// Each context has up to 4 tests:
//   - TestPCKeys<Ctx>PgDown  — pgdown matches ctrl+f
//   - TestPCKeys<Ctx>PgUp    — pgup   matches ctrl+b
//   - TestPCKeys<Ctx>Home    — jumps to top (where the context has gg)
//   - TestPCKeys<Ctx>End     — jumps to bottom (where the context has G)
//
// Contexts that have no gg/G equivalent (Can-I Subject, Namespace, Rollback,
// Helm Rollback, Helm History) only get PgUp/PgDown tests.

// --- Help viewer (handleHelpKey) -------------------------------------------------

// helpModel returns a model primed for help-viewer key tests with a
// deterministic viewport height so page math is predictable.
func helpModel() Model {
	m := baseModelSearch()
	m.mode = modeHelp
	m.height = 40
	m.helpScroll = 0
	return m
}

func TestPCKeysHelpPgDownMatchesCtrlF(t *testing.T) {
	m := helpModel()
	resPg, _ := m.handleHelpKey(keyMsg("pgdown"))
	resCF, _ := m.handleHelpKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).helpScroll, resPg.(Model).helpScroll,
		"pgdown should scroll the same as ctrl+f in help viewer")
	assert.Greater(t, resPg.(Model).helpScroll, 0,
		"pgdown should advance the help scroll")
}

func TestPCKeysHelpPgUpMatchesCtrlB(t *testing.T) {
	m := helpModel()
	m.helpScroll = 100
	resPg, _ := m.handleHelpKey(keyMsg("pgup"))
	resCB, _ := m.handleHelpKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).helpScroll, resPg.(Model).helpScroll,
		"pgup should scroll the same as ctrl+b in help viewer")
	assert.Less(t, resPg.(Model).helpScroll, 100,
		"pgup should retract the help scroll")
}

func TestPCKeysHelpHomeJumpsToTop(t *testing.T) {
	m := helpModel()
	m.helpScroll = 100
	m.pendingG = true
	res, _ := m.handleHelpKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.helpScroll, "home should reset helpScroll to 0")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysHelpEndJumpsToBottom(t *testing.T) {
	m := helpModel()
	m.helpScroll = 0
	res, _ := m.handleHelpKey(keyMsg("end"))
	rm := res.(Model)
	// end (like G) clamps to the actual max so a follow-up ctrl+u
	// responds on the first press instead of undoing 9999 worth of
	// phantom scroll.
	assert.Less(t, rm.helpScroll, 9999, "end must clamp the sentinel, not park at 9999")
	assert.Greater(t, rm.helpScroll, 0, "end must scroll past the top")
}

// --- Describe viewer (handleDescribeKey) ----------------------------------------

// describeModelWithLines returns a model primed for describe-viewer tests with
// enough lines for paging to be meaningful.
func describeModelWithLines() Model {
	const n = 200
	lines := make([]string, n)
	for i := range n {
		lines[i] = "line" + string(rune('a'+i%26))
	}
	return Model{
		mode:            modeDescribe,
		describeContent: strings.Join(lines, "\n"),
		describeCursor:  0,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
}

func TestPCKeysDescribePgDownMatchesCtrlF(t *testing.T) {
	m := describeModelWithLines()
	m.describeCursor = 10
	resPg, _ := m.handleDescribeKey(keyMsg("pgdown"))
	resCF, _ := m.handleDescribeKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).describeCursor, resPg.(Model).describeCursor,
		"pgdown should move the describe cursor the same as ctrl+f")
	assert.Greater(t, resPg.(Model).describeCursor, 10,
		"pgdown should advance the describe cursor")
}

func TestPCKeysDescribePgUpMatchesCtrlB(t *testing.T) {
	m := describeModelWithLines()
	m.describeCursor = 100
	resPg, _ := m.handleDescribeKey(keyMsg("pgup"))
	resCB, _ := m.handleDescribeKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).describeCursor, resPg.(Model).describeCursor,
		"pgup should move the describe cursor the same as ctrl+b")
	assert.Less(t, resPg.(Model).describeCursor, 100,
		"pgup should retract the describe cursor")
}

func TestPCKeysDescribeHomeJumpsToTop(t *testing.T) {
	m := describeModelWithLines()
	m.describeCursor = 100
	m.describeLineInput = "15"
	m.pendingG = true
	res, _ := m.handleDescribeKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.describeCursor, "home should reset describeCursor to 0")
	assert.Empty(t, rm.describeLineInput, "home should clear describeLineInput")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysDescribeEndJumpsToBottom(t *testing.T) {
	m := describeModelWithLines()
	m.describeCursor = 0
	// Even with a line-number buffer, end must always go to the bottom.
	m.describeLineInput = "42"
	res, _ := m.handleDescribeKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 199, rm.describeCursor,
		"end should jump to the last line regardless of describeLineInput")
	assert.Empty(t, rm.describeLineInput, "end should clear describeLineInput")
}

// --- Bookmarks overlay (handleBookmarkOverlayKey) -------------------------------

func bookmarksModel() Model {
	const n = 50
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkSearchMode = bookmarkModeNormal
	m.bookmarks = make([]model.Bookmark, n)
	for i := range n {
		m.bookmarks[i] = model.Bookmark{Slot: string(rune('a' + i%26)), Name: "bm"}
	}
	return m
}

func TestPCKeysBookmarksPgDownMatchesCtrlF(t *testing.T) {
	m := bookmarksModel()
	m.overlayCursor = 0
	resPg, _ := m.handleBookmarkOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).overlayCursor, resPg.(Model).overlayCursor,
		"pgdown should match ctrl+f in bookmarks overlay")
	assert.Greater(t, resPg.(Model).overlayCursor, 0)
}

func TestPCKeysBookmarksPgUpMatchesCtrlB(t *testing.T) {
	m := bookmarksModel()
	m.overlayCursor = 40
	resPg, _ := m.handleBookmarkOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleBookmarkOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).overlayCursor, resPg.(Model).overlayCursor,
		"pgup should match ctrl+b in bookmarks overlay")
	assert.Less(t, resPg.(Model).overlayCursor, 40)
}

func TestPCKeysBookmarksHomeJumpsToTop(t *testing.T) {
	m := bookmarksModel()
	m.overlayCursor = 25
	m.pendingG = true
	res, _ := m.handleBookmarkOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.overlayCursor, "home should set overlayCursor to 0")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysBookmarksEndJumpsToBottom(t *testing.T) {
	m := bookmarksModel()
	m.overlayCursor = 0
	m.pendingG = true
	res, _ := m.handleBookmarkOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 49, rm.overlayCursor, "end should set overlayCursor to the last bookmark")
	assert.False(t, rm.pendingG, "end must clear pendingG")
}

// --- Can-I browser (handleCanIKey) ----------------------------------------------

func canIModel() Model {
	const n = 80
	m := baseModelCov()
	m.height = 40
	groups := make([]model.CanIGroup, n)
	for i := range n {
		groups[i] = model.CanIGroup{Name: "grp"}
	}
	m.canIGroups = groups
	return m
}

func TestPCKeysCanIPgDownMatchesCtrlF(t *testing.T) {
	m := canIModel()
	m.canIGroupCursor = 0
	resPg, _ := m.handleCanIKey(keyMsg("pgdown"))
	resCF, _ := m.handleCanIKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).canIGroupCursor, resPg.(Model).canIGroupCursor,
		"pgdown should match ctrl+f in Can-I browser")
	assert.Greater(t, resPg.(Model).canIGroupCursor, 0)
}

func TestPCKeysCanIPgUpMatchesCtrlB(t *testing.T) {
	m := canIModel()
	m.canIGroupCursor = 60
	m.canIGroupScroll = 40
	resPg, _ := m.handleCanIKey(keyMsg("pgup"))
	resCB, _ := m.handleCanIKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).canIGroupCursor, resPg.(Model).canIGroupCursor,
		"pgup should match ctrl+b in Can-I browser")
	assert.Less(t, resPg.(Model).canIGroupCursor, 60)
}

func TestPCKeysCanIHomeJumpsToTop(t *testing.T) {
	m := canIModel()
	m.canIGroupCursor = 50
	m.canIGroupScroll = 40
	m.canIResourceScroll = 5
	m.pendingG = true
	res, _ := m.handleCanIKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.canIGroupCursor, "home should set canIGroupCursor to 0")
	assert.Equal(t, 0, rm.canIGroupScroll, "home should set canIGroupScroll to 0")
	assert.Equal(t, 0, rm.canIResourceScroll, "home should reset canIResourceScroll")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysCanIEndJumpsToBottom(t *testing.T) {
	m := canIModel()
	m.canIGroupCursor = 0
	resG, _ := m.handleCanIKey(keyMsg("G"))
	resEnd, _ := m.handleCanIKey(keyMsg("end"))
	assert.Equal(t, resG.(Model).canIGroupCursor, resEnd.(Model).canIGroupCursor,
		"end should match G in Can-I browser")
	assert.Equal(t, 79, resEnd.(Model).canIGroupCursor, "end should jump to last group")
}

// --- Can-I subject overlay (handleCanISubjectOverlayKey) ------------------------
// This overlay has no gg/G handlers; only pgup/pgdown aliases are expected.

func caniSubjectModel() Model {
	const n = 40
	m := baseModelOverlay()
	m.overlay = overlayCanISubject
	items := make([]model.Item, n)
	for i := range n {
		items[i] = model.Item{Name: "sub"}
	}
	m.overlayItems = items
	return m
}

func TestPCKeysCanISubjectPgDownMatchesCtrlF(t *testing.T) {
	m := caniSubjectModel()
	m.overlayCursor = 0
	resPg, _ := m.handleCanISubjectOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleCanISubjectOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).overlayCursor, resPg.(Model).overlayCursor,
		"pgdown should match ctrl+f in Can-I subject overlay")
	assert.Greater(t, resPg.(Model).overlayCursor, 0)
}

func TestPCKeysCanISubjectPgUpMatchesCtrlB(t *testing.T) {
	m := caniSubjectModel()
	m.overlayCursor = 30
	resPg, _ := m.handleCanISubjectOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleCanISubjectOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).overlayCursor, resPg.(Model).overlayCursor,
		"pgup should match ctrl+b in Can-I subject overlay")
	assert.Less(t, resPg.(Model).overlayCursor, 30)
}

// --- Namespace overlay (handleNamespaceOverlayKey) ------------------------------
// This overlay has no gg/G handlers; only pgup/pgdown aliases are expected.

func namespaceOverlayModel() Model {
	const n = 40
	m := baseModelOverlay()
	m.overlay = overlayNamespace
	items := make([]model.Item, n)
	for i := range n {
		items[i] = model.Item{Name: "ns"}
	}
	m.overlayItems = items
	return m
}

func TestPCKeysNamespacePgDownMatchesCtrlF(t *testing.T) {
	m := namespaceOverlayModel()
	m.overlayCursor = 0
	resPg, _ := m.handleNamespaceOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleNamespaceOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).overlayCursor, resPg.(Model).overlayCursor,
		"pgdown should match ctrl+f in namespace overlay")
	assert.Greater(t, resPg.(Model).overlayCursor, 0)
}

func TestPCKeysNamespacePgUpMatchesCtrlB(t *testing.T) {
	m := namespaceOverlayModel()
	m.overlayCursor = 30
	resPg, _ := m.handleNamespaceOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleNamespaceOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).overlayCursor, resPg.(Model).overlayCursor,
		"pgup should match ctrl+b in namespace overlay")
	assert.Less(t, resPg.(Model).overlayCursor, 30)
}

// --- Template overlay (handleTemplateOverlayKey) --------------------------------

func templateOverlayModel() Model {
	const n = 40
	m := baseModelOverlay()
	m.overlay = overlayTemplates
	items := make([]model.ResourceTemplate, n)
	for i := range n {
		items[i] = model.ResourceTemplate{Name: "tpl", Category: "misc"}
	}
	m.templateItems = items
	return m
}

func TestPCKeysTemplatePgDownMatchesCtrlF(t *testing.T) {
	m := templateOverlayModel()
	m.templateCursor = 0
	resPg, _ := m.handleTemplateOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleTemplateOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).templateCursor, resPg.(Model).templateCursor,
		"pgdown should match ctrl+f in template overlay")
	assert.Greater(t, resPg.(Model).templateCursor, 0)
}

func TestPCKeysTemplatePgUpMatchesCtrlB(t *testing.T) {
	m := templateOverlayModel()
	m.templateCursor = 30
	resPg, _ := m.handleTemplateOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleTemplateOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).templateCursor, resPg.(Model).templateCursor,
		"pgup should match ctrl+b in template overlay")
	assert.Less(t, resPg.(Model).templateCursor, 30)
}

func TestPCKeysTemplateHomeJumpsToTop(t *testing.T) {
	m := templateOverlayModel()
	m.templateCursor = 20
	m.pendingG = true
	res, _ := m.handleTemplateOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.templateCursor, "home should set templateCursor to 0")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysTemplateEndJumpsToBottom(t *testing.T) {
	m := templateOverlayModel()
	m.templateCursor = 0
	m.pendingG = true
	res, _ := m.handleTemplateOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.templateCursor, "end should set templateCursor to last item")
	assert.False(t, rm.pendingG, "end must clear pendingG")
}

// --- Rollback overlay (handleRollbackOverlayKey) --------------------------------
// No gg/G.

func rollbackModel() Model {
	const n = 40
	m := baseModelOverlay()
	m.overlay = overlayRollback
	revs := make([]k8s.DeploymentRevision, n)
	for i := range n {
		revs[i] = k8s.DeploymentRevision{Revision: int64(i + 1), Name: "rs"}
	}
	m.rollbackRevisions = revs
	return m
}

func TestPCKeysRollbackPgDownMatchesCtrlF(t *testing.T) {
	m := rollbackModel()
	m.rollbackCursor = 0
	resPg, _ := m.handleRollbackOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleRollbackOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).rollbackCursor, resPg.(Model).rollbackCursor,
		"pgdown should match ctrl+f in rollback overlay")
	assert.Greater(t, resPg.(Model).rollbackCursor, 0)
}

func TestPCKeysRollbackPgUpMatchesCtrlB(t *testing.T) {
	m := rollbackModel()
	m.rollbackCursor = 30
	resPg, _ := m.handleRollbackOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleRollbackOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).rollbackCursor, resPg.(Model).rollbackCursor,
		"pgup should match ctrl+b in rollback overlay")
	assert.Less(t, resPg.(Model).rollbackCursor, 30)
}

// --- Helm Rollback overlay (handleHelmRollbackOverlayKey) -----------------------
// No gg/G.

func helmRollbackModel() Model {
	const n = 40
	m := baseModelOverlay()
	m.overlay = overlayHelmRollback
	revs := make([]ui.HelmRevision, n)
	for i := range n {
		revs[i] = ui.HelmRevision{Revision: i + 1}
	}
	m.helmRollbackRevisions = revs
	return m
}

func TestPCKeysHelmRollbackPgDownMatchesCtrlF(t *testing.T) {
	m := helmRollbackModel()
	m.helmRollbackCursor = 0
	resPg, _ := m.handleHelmRollbackOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleHelmRollbackOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).helmRollbackCursor, resPg.(Model).helmRollbackCursor,
		"pgdown should match ctrl+f in helm rollback overlay")
	assert.Greater(t, resPg.(Model).helmRollbackCursor, 0)
}

func TestPCKeysHelmRollbackPgUpMatchesCtrlB(t *testing.T) {
	m := helmRollbackModel()
	m.helmRollbackCursor = 30
	resPg, _ := m.handleHelmRollbackOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleHelmRollbackOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).helmRollbackCursor, resPg.(Model).helmRollbackCursor,
		"pgup should match ctrl+b in helm rollback overlay")
	assert.Less(t, resPg.(Model).helmRollbackCursor, 30)
}

// --- Helm History overlay (handleHelmHistoryOverlayKey) -------------------------
// No gg/G.

func helmHistoryModel() Model {
	const n = 40
	m := baseModelOverlay()
	m.overlay = overlayHelmHistory
	revs := make([]ui.HelmRevision, n)
	for i := range n {
		revs[i] = ui.HelmRevision{Revision: i + 1}
	}
	m.helmHistoryRevisions = revs
	return m
}

func TestPCKeysHelmHistoryPgDownMatchesCtrlF(t *testing.T) {
	m := helmHistoryModel()
	m.helmHistoryCursor = 0
	resPg, _ := m.handleHelmHistoryOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleHelmHistoryOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).helmHistoryCursor, resPg.(Model).helmHistoryCursor,
		"pgdown should match ctrl+f in helm history overlay")
	assert.Greater(t, resPg.(Model).helmHistoryCursor, 0)
}

func TestPCKeysHelmHistoryPgUpMatchesCtrlB(t *testing.T) {
	m := helmHistoryModel()
	m.helmHistoryCursor = 30
	resPg, _ := m.handleHelmHistoryOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleHelmHistoryOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).helmHistoryCursor, resPg.(Model).helmHistoryCursor,
		"pgup should match ctrl+b in helm history overlay")
	assert.Less(t, resPg.(Model).helmHistoryCursor, 30)
}

// --- Colorscheme overlay (handleColorschemeOverlayKey) --------------------------

// colorschemeModel builds a model with n selectable scheme entries (no headers).
func colorschemeModel() Model {
	const n = 40
	m := baseModelOverlay()
	m.overlay = overlayColorscheme
	entries := make([]ui.SchemeEntry, n)
	for i := range n {
		entries[i] = ui.SchemeEntry{Name: "scheme", IsHeader: false}
	}
	m.schemeEntries = entries
	return m
}

func TestPCKeysColorschemePgDownMatchesCtrlF(t *testing.T) {
	m := colorschemeModel()
	m.schemeCursor = 0
	resPg, _ := m.handleColorschemeOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleColorschemeOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).schemeCursor, resPg.(Model).schemeCursor,
		"pgdown should match ctrl+f in colorscheme overlay")
	assert.Greater(t, resPg.(Model).schemeCursor, 0)
}

func TestPCKeysColorschemePgUpMatchesCtrlB(t *testing.T) {
	m := colorschemeModel()
	m.schemeCursor = 30
	resPg, _ := m.handleColorschemeOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleColorschemeOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).schemeCursor, resPg.(Model).schemeCursor,
		"pgup should match ctrl+b in colorscheme overlay")
	assert.Less(t, resPg.(Model).schemeCursor, 30)
}

func TestPCKeysColorschemeHomeJumpsToTop(t *testing.T) {
	m := colorschemeModel()
	m.schemeCursor = 20
	m.pendingG = true
	res, _ := m.handleColorschemeOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.schemeCursor, "home should set schemeCursor to 0")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysColorschemeEndJumpsToBottom(t *testing.T) {
	m := colorschemeModel()
	m.schemeCursor = 0
	m.pendingG = true
	res, _ := m.handleColorschemeOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.schemeCursor, "end should set schemeCursor to last item")
	assert.False(t, rm.pendingG, "end must clear pendingG")
}

// --- Event Timeline overlay (handleEventTimelineOverlayKey) ---------------------

func eventTimelineModel() Model {
	const n = 200
	events := make([]k8s.EventInfo, n)
	for i := range n {
		events[i] = k8s.EventInfo{
			Type:    "Normal",
			Reason:  "Scheduled",
			Message: "event message",
		}
	}
	m := Model{
		overlay:           overlayEventTimeline,
		eventTimelineData: events,
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}
	m.eventTimelineLines = m.buildEventTimelineLines()
	return m
}

func TestPCKeysEventTimelinePgDownMatchesCtrlF(t *testing.T) {
	m := eventTimelineModel()
	m.eventTimelineCursor = 10
	resPg, _ := m.handleEventTimelineOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleEventTimelineOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).eventTimelineCursor, resPg.(Model).eventTimelineCursor,
		"pgdown should match ctrl+f in event timeline overlay")
	assert.Greater(t, resPg.(Model).eventTimelineCursor, 10)
}

func TestPCKeysEventTimelinePgUpMatchesCtrlB(t *testing.T) {
	m := eventTimelineModel()
	m.eventTimelineCursor = 100
	resPg, _ := m.handleEventTimelineOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleEventTimelineOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).eventTimelineCursor, resPg.(Model).eventTimelineCursor,
		"pgup should match ctrl+b in event timeline overlay")
	assert.Less(t, resPg.(Model).eventTimelineCursor, 100)
}

func TestPCKeysEventTimelineHomeJumpsToTop(t *testing.T) {
	m := eventTimelineModel()
	m.eventTimelineCursor = 100
	m.eventTimelineLineInput = "15"
	res, _ := m.handleEventTimelineOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.eventTimelineCursor, "home should reset eventTimelineCursor to 0")
	assert.Empty(t, rm.eventTimelineLineInput, "home should clear eventTimelineLineInput")
}

func TestPCKeysEventTimelineEndJumpsToBottom(t *testing.T) {
	m := eventTimelineModel()
	m.eventTimelineCursor = 0
	// Even with a line-number buffer, end must always go to the bottom.
	m.eventTimelineLineInput = "42"
	res, _ := m.handleEventTimelineOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 199, rm.eventTimelineCursor,
		"end should jump to the last event regardless of eventTimelineLineInput")
	assert.Empty(t, rm.eventTimelineLineInput, "end should clear eventTimelineLineInput")
}

// ============================================================================
// Group A: Overlays that previously had only pgup/pgdown — now gain gg/G/Home/End.
// ============================================================================

// --- Can-I Subject Selector (handleCanISubjectOverlayKey) -----------------------

func TestPCKeysCanISubjectHomeJumpsToTop(t *testing.T) {
	m := caniSubjectModel()
	m.overlayCursor = 25
	m.pendingG = true
	res, _ := m.handleCanISubjectOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.overlayCursor, "home should set overlayCursor to 0 in can-i subject overlay")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysCanISubjectEndJumpsToBottom(t *testing.T) {
	m := caniSubjectModel()
	m.overlayCursor = 0
	res, _ := m.handleCanISubjectOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.overlayCursor, "end should set overlayCursor to last subject")
}

func TestPCKeysCanISubjectGGJumpsToTop(t *testing.T) {
	m := caniSubjectModel()
	m.overlayCursor = 25
	// First g arms pendingG.
	r1, _ := m.handleCanISubjectOverlayKey(keyMsg("g"))
	rm1 := r1.(Model)
	assert.True(t, rm1.pendingG, "first g should arm pendingG")
	// Second g jumps to top.
	r2, _ := rm1.handleCanISubjectOverlayKey(keyMsg("g"))
	rm2 := r2.(Model)
	assert.False(t, rm2.pendingG)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestPCKeysCanISubjectGJumpsToBottom(t *testing.T) {
	m := caniSubjectModel()
	m.overlayCursor = 0
	res, _ := m.handleCanISubjectOverlayKey(keyMsg("G"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.overlayCursor, "G should jump to last subject")
}

// --- Namespace overlay (handleNamespaceOverlayKey) ------------------------------

func TestPCKeysNamespaceHomeJumpsToTop(t *testing.T) {
	m := namespaceOverlayModel()
	m.overlayCursor = 25
	m.pendingG = true
	res, _ := m.handleNamespaceOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.overlayCursor, "home should set overlayCursor to 0 in namespace overlay")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysNamespaceEndJumpsToBottom(t *testing.T) {
	m := namespaceOverlayModel()
	m.overlayCursor = 0
	res, _ := m.handleNamespaceOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.overlayCursor, "end should set overlayCursor to last namespace")
}

func TestPCKeysNamespaceGGJumpsToTop(t *testing.T) {
	m := namespaceOverlayModel()
	m.overlayCursor = 25
	r1, _ := m.handleNamespaceOverlayKey(keyMsg("g"))
	rm1 := r1.(Model)
	assert.True(t, rm1.pendingG, "first g should arm pendingG")
	r2, _ := rm1.handleNamespaceOverlayKey(keyMsg("g"))
	rm2 := r2.(Model)
	assert.False(t, rm2.pendingG)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestPCKeysNamespaceGJumpsToBottom(t *testing.T) {
	m := namespaceOverlayModel()
	m.overlayCursor = 0
	res, _ := m.handleNamespaceOverlayKey(keyMsg("G"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.overlayCursor, "G should jump to last namespace")
}

// --- Rollback overlay (handleRollbackOverlayKey) --------------------------------

func TestPCKeysRollbackHomeJumpsToTop(t *testing.T) {
	m := rollbackModel()
	m.rollbackCursor = 25
	m.pendingG = true
	res, _ := m.handleRollbackOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.rollbackCursor)
	assert.False(t, rm.pendingG)
}

func TestPCKeysRollbackEndJumpsToBottom(t *testing.T) {
	m := rollbackModel()
	m.rollbackCursor = 0
	res, _ := m.handleRollbackOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.rollbackCursor)
}

func TestPCKeysRollbackGGJumpsToTop(t *testing.T) {
	m := rollbackModel()
	m.rollbackCursor = 25
	r1, _ := m.handleRollbackOverlayKey(keyMsg("g"))
	rm1 := r1.(Model)
	assert.True(t, rm1.pendingG)
	r2, _ := rm1.handleRollbackOverlayKey(keyMsg("g"))
	rm2 := r2.(Model)
	assert.False(t, rm2.pendingG)
	assert.Equal(t, 0, rm2.rollbackCursor)
}

func TestPCKeysRollbackGJumpsToBottom(t *testing.T) {
	m := rollbackModel()
	m.rollbackCursor = 0
	res, _ := m.handleRollbackOverlayKey(keyMsg("G"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.rollbackCursor)
}

// --- Helm Rollback overlay (handleHelmRollbackOverlayKey) -----------------------

func TestPCKeysHelmRollbackHomeJumpsToTop(t *testing.T) {
	m := helmRollbackModel()
	m.helmRollbackCursor = 25
	m.pendingG = true
	res, _ := m.handleHelmRollbackOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.helmRollbackCursor)
	assert.False(t, rm.pendingG)
}

func TestPCKeysHelmRollbackEndJumpsToBottom(t *testing.T) {
	m := helmRollbackModel()
	m.helmRollbackCursor = 0
	res, _ := m.handleHelmRollbackOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.helmRollbackCursor)
}

func TestPCKeysHelmRollbackGGJumpsToTop(t *testing.T) {
	m := helmRollbackModel()
	m.helmRollbackCursor = 25
	r1, _ := m.handleHelmRollbackOverlayKey(keyMsg("g"))
	rm1 := r1.(Model)
	assert.True(t, rm1.pendingG)
	r2, _ := rm1.handleHelmRollbackOverlayKey(keyMsg("g"))
	rm2 := r2.(Model)
	assert.False(t, rm2.pendingG)
	assert.Equal(t, 0, rm2.helmRollbackCursor)
}

func TestPCKeysHelmRollbackGJumpsToBottom(t *testing.T) {
	m := helmRollbackModel()
	m.helmRollbackCursor = 0
	res, _ := m.handleHelmRollbackOverlayKey(keyMsg("G"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.helmRollbackCursor)
}

// --- Helm History overlay (handleHelmHistoryOverlayKey) -------------------------

func TestPCKeysHelmHistoryHomeJumpsToTop(t *testing.T) {
	m := helmHistoryModel()
	m.helmHistoryCursor = 25
	m.pendingG = true
	res, _ := m.handleHelmHistoryOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.helmHistoryCursor)
	assert.False(t, rm.pendingG)
}

func TestPCKeysHelmHistoryEndJumpsToBottom(t *testing.T) {
	m := helmHistoryModel()
	m.helmHistoryCursor = 0
	res, _ := m.handleHelmHistoryOverlayKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.helmHistoryCursor)
}

func TestPCKeysHelmHistoryGGJumpsToTop(t *testing.T) {
	m := helmHistoryModel()
	m.helmHistoryCursor = 25
	r1, _ := m.handleHelmHistoryOverlayKey(keyMsg("g"))
	rm1 := r1.(Model)
	assert.True(t, rm1.pendingG)
	r2, _ := rm1.handleHelmHistoryOverlayKey(keyMsg("g"))
	rm2 := r2.(Model)
	assert.False(t, rm2.pendingG)
	assert.Equal(t, 0, rm2.helmHistoryCursor)
}

func TestPCKeysHelmHistoryGJumpsToBottom(t *testing.T) {
	m := helmHistoryModel()
	m.helmHistoryCursor = 0
	res, _ := m.handleHelmHistoryOverlayKey(keyMsg("G"))
	rm := res.(Model)
	assert.Equal(t, 39, rm.helmHistoryCursor)
}

// ============================================================================
// Group B: Viewers/overlays that had gg/G — now gain Home/End/PgUp/PgDown.
// ============================================================================

// --- Diff view (handleDiffNormalKey) --------------------------------------------

func diffModelWithLines() Model {
	const n = 200
	lines := make([]string, n)
	for i := range n {
		lines[i] = "line" + string(rune('a'+i%26))
	}
	return Model{
		mode:     modeDiff,
		diffLeft: strings.Join(lines, "\n"),
		tabs:     []TabState{{}},
		width:    80,
		height:   40,
	}
}

func TestPCKeysDiffPgDownMatchesCtrlF(t *testing.T) {
	m := diffModelWithLines()
	m.diffCursor = 10
	resPg, _ := m.handleDiffKey(keyMsg("pgdown"))
	resCF, _ := m.handleDiffKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).diffCursor, resPg.(Model).diffCursor,
		"pgdown should match ctrl+f in diff view")
	assert.Greater(t, resPg.(Model).diffCursor, 10)
}

func TestPCKeysDiffPgUpMatchesCtrlB(t *testing.T) {
	m := diffModelWithLines()
	m.diffCursor = 100
	resPg, _ := m.handleDiffKey(keyMsg("pgup"))
	resCB, _ := m.handleDiffKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).diffCursor, resPg.(Model).diffCursor,
		"pgup should match ctrl+b in diff view")
	assert.Less(t, resPg.(Model).diffCursor, 100)
}

func TestPCKeysDiffHomeJumpsToTop(t *testing.T) {
	m := diffModelWithLines()
	m.diffCursor = 100
	m.diffScroll = 80
	m.diffLineInput = "15"
	m.pendingG = true
	res, _ := m.handleDiffKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.diffCursor, "home should reset diffCursor to 0")
	assert.Equal(t, 0, rm.diffScroll, "home should reset diffScroll to 0")
	assert.Empty(t, rm.diffLineInput, "home should clear diffLineInput")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysDiffEndJumpsToBottom(t *testing.T) {
	m := diffModelWithLines()
	m.diffCursor = 0
	// Even with a line-number buffer, end must always go to the bottom.
	m.diffLineInput = "42"
	res, _ := m.handleDiffKey(keyMsg("end"))
	rm := res.(Model)
	// With 200 lines, cursor should go to the last.
	assert.Equal(t, 199, rm.diffCursor,
		"end should jump to the last diff line regardless of diffLineInput")
	assert.Empty(t, rm.diffLineInput, "end should clear diffLineInput")
}

// --- API Explorer (handleExplainKey) --------------------------------------------

func explainModel() Model {
	const n = 200
	m := baseModelCov()
	m.mode = modeExplain
	fields := make([]model.ExplainField, n)
	for i := range n {
		fields[i] = model.ExplainField{Name: "field"}
	}
	m.explainFields = fields
	return m
}

func TestPCKeysExplainPgDownMatchesCtrlF(t *testing.T) {
	m := explainModel()
	m.explainCursor = 10
	resPg, _ := m.handleExplainKey(keyMsg("pgdown"))
	resCF, _ := m.handleExplainKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).explainCursor, resPg.(Model).explainCursor,
		"pgdown should match ctrl+f in api explorer")
	assert.Greater(t, resPg.(Model).explainCursor, 10)
}

func TestPCKeysExplainPgUpMatchesCtrlB(t *testing.T) {
	m := explainModel()
	m.explainCursor = 100
	resPg, _ := m.handleExplainKey(keyMsg("pgup"))
	resCB, _ := m.handleExplainKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).explainCursor, resPg.(Model).explainCursor,
		"pgup should match ctrl+b in api explorer")
	assert.Less(t, resPg.(Model).explainCursor, 100)
}

func TestPCKeysExplainHomeJumpsToTop(t *testing.T) {
	m := explainModel()
	m.explainCursor = 100
	m.explainScroll = 50
	m.explainLineInput = "15"
	m.pendingG = true
	res, _ := m.handleExplainKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.explainCursor, "home should reset explainCursor to 0")
	assert.Equal(t, 0, rm.explainScroll, "home should reset explainScroll to 0")
	assert.Empty(t, rm.explainLineInput, "home should clear explainLineInput")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysExplainEndJumpsToBottom(t *testing.T) {
	m := explainModel()
	m.explainCursor = 0
	// Even with a line-number buffer, end should always go to bottom.
	m.explainLineInput = "42"
	res, _ := m.handleExplainKey(keyMsg("end"))
	rm := res.(Model)
	assert.Equal(t, 199, rm.explainCursor,
		"end should jump to the last explain field regardless of explainLineInput")
	assert.Empty(t, rm.explainLineInput, "end should clear explainLineInput")
}

// --- Network Policy Visualizer (handleNetworkPolicyOverlayKey) ------------------

func netpolModel() Model {
	return Model{
		overlay:      overlayNetworkPolicy,
		netpolScroll: 0,
		tabs:         []TabState{{}},
		width:        80,
		height:       40,
	}
}

func TestPCKeysNetworkPolicyPgDownMatchesCtrlF(t *testing.T) {
	m := netpolModel()
	m.netpolScroll = 0
	rPg := m.handleNetworkPolicyOverlayKey(keyMsg("pgdown"))
	rCF := m.handleNetworkPolicyOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, rCF.netpolScroll, rPg.netpolScroll,
		"pgdown should match ctrl+f in network policy overlay")
	assert.Greater(t, rPg.netpolScroll, 0)
}

func TestPCKeysNetworkPolicyPgUpMatchesCtrlB(t *testing.T) {
	m := netpolModel()
	m.netpolScroll = 100
	rPg := m.handleNetworkPolicyOverlayKey(keyMsg("pgup"))
	rCB := m.handleNetworkPolicyOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, rCB.netpolScroll, rPg.netpolScroll,
		"pgup should match ctrl+b in network policy overlay")
	assert.Less(t, rPg.netpolScroll, 100)
}

func TestPCKeysNetworkPolicyHomeJumpsToTop(t *testing.T) {
	m := netpolModel()
	m.netpolScroll = 50
	m.netpolLineInput = "15"
	m.pendingG = true
	r := m.handleNetworkPolicyOverlayKey(keyMsg("home"))
	assert.Equal(t, 0, r.netpolScroll, "home should reset netpolScroll to 0")
	assert.Empty(t, r.netpolLineInput, "home should clear netpolLineInput")
	assert.False(t, r.pendingG, "home must clear pendingG")
}

func TestPCKeysNetworkPolicyEndJumpsToBottom(t *testing.T) {
	m := netpolModel()
	m.netpolScroll = 0
	m.netpolLineInput = "42"
	r := m.handleNetworkPolicyOverlayKey(keyMsg("end"))
	// G with no line input jumps to 9999 (sentinel clamped at render time).
	assert.Equal(t, 9999, r.netpolScroll,
		"end should match G (9999) regardless of netpolLineInput")
	assert.Empty(t, r.netpolLineInput, "end should clear netpolLineInput")
}

// --- Error Log overlay (handleErrorLogOverlayKey) -------------------------------

func errorLogModel() Model {
	const n = 200
	entries := make([]ui.ErrorLogEntry, n)
	for i := range n {
		entries[i] = ui.ErrorLogEntry{Level: "ERR", Message: "entry"}
	}
	return Model{
		overlayErrorLog: true,
		errorLog:        entries,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
}

func TestPCKeysErrorLogPgDownMatchesCtrlF(t *testing.T) {
	m := errorLogModel()
	m.errorLogCursorLine = 10
	resPg, _ := m.handleErrorLogOverlayKey(keyMsg("pgdown"))
	resCF, _ := m.handleErrorLogOverlayKey(keyMsg("ctrl+f"))
	assert.Equal(t, resCF.(Model).errorLogCursorLine, resPg.(Model).errorLogCursorLine,
		"pgdown should match ctrl+f in error log overlay")
	assert.Greater(t, resPg.(Model).errorLogCursorLine, 10)
}

func TestPCKeysErrorLogPgUpMatchesCtrlB(t *testing.T) {
	m := errorLogModel()
	m.errorLogCursorLine = 100
	resPg, _ := m.handleErrorLogOverlayKey(keyMsg("pgup"))
	resCB, _ := m.handleErrorLogOverlayKey(keyMsg("ctrl+b"))
	assert.Equal(t, resCB.(Model).errorLogCursorLine, resPg.(Model).errorLogCursorLine,
		"pgup should match ctrl+b in error log overlay")
	assert.Less(t, resPg.(Model).errorLogCursorLine, 100)
}

func TestPCKeysErrorLogHomeJumpsToTop(t *testing.T) {
	m := errorLogModel()
	m.errorLogCursorLine = 100
	m.errorLogScroll = 80
	m.errorLogLineInput = "15"
	m.pendingG = true
	res, _ := m.handleErrorLogOverlayKey(keyMsg("home"))
	rm := res.(Model)
	assert.Equal(t, 0, rm.errorLogCursorLine, "home should reset errorLogCursorLine to 0")
	assert.Equal(t, 0, rm.errorLogScroll, "home should reset errorLogScroll to 0")
	assert.Empty(t, rm.errorLogLineInput, "home should clear errorLogLineInput")
	assert.False(t, rm.pendingG, "home must clear pendingG")
}

func TestPCKeysErrorLogEndJumpsToBottom(t *testing.T) {
	m := errorLogModel()
	m.errorLogCursorLine = 0
	m.errorLogLineInput = "42"
	res, _ := m.handleErrorLogOverlayKey(keyMsg("end"))
	rm := res.(Model)
	// With 200 entries, cursor should jump to last.
	assert.Equal(t, 199, rm.errorLogCursorLine,
		"end should jump to last error log line regardless of errorLogLineInput")
	assert.Empty(t, rm.errorLogLineInput, "end should clear errorLogLineInput")
}
