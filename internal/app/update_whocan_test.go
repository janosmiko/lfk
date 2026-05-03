package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestEnterWhoCanMode_FlipsModeAndResetsState(t *testing.T) {
	m := Model{}
	m.canIMode = canIModeForward
	m.whoCan.subjectsScroll = 7 // stale value from prior session
	mdl, _ := m.enterWhoCanMode()
	result := mdl.(Model)
	assert.Equal(t, canIModeWhoCan, result.canIMode,
		"Tab from Can-I must flip the overlay into Who-Can mode")
	assert.Equal(t, 0, result.whoCan.subjectsScroll,
		"scroll resets so the user lands at the top of the new subjects list")
}

func TestHandleWhoCanKey_TabReturnsToForward(t *testing.T) {
	m := Model{canIMode: canIModeWhoCan}
	mdl, _ := m.handleWhoCanKey(keyMsg("tab"))
	result := mdl.(Model)
	assert.Equal(t, canIModeForward, result.canIMode,
		"Tab from Who-Can goes back to forward Can-I — the toggle is bidirectional")
}

func TestHandleWhoCanKey_LeftRightCyclesVerb(t *testing.T) {
	m := Model{canIMode: canIModeWhoCan}
	require.Equal(t, "get", ui.WhoCanVerbs[0])
	mdl, _ := m.handleWhoCanKey(keyMsg("right"))
	result := mdl.(Model)
	assert.Equal(t, 1, result.whoCan.verbCursor, "right key advances the verb cursor")

	mdl, _ = result.handleWhoCanKey(keyMsg("left"))
	result = mdl.(Model)
	assert.Equal(t, 0, result.whoCan.verbCursor, "left wraps back")
}

func TestHandleWhoCanKey_LeftAtZeroDoesNotUnderflow(t *testing.T) {
	m := Model{canIMode: canIModeWhoCan, whoCan: whoCanState{verbCursor: 0}}
	mdl, _ := m.handleWhoCanKey(keyMsg("left"))
	result := mdl.(Model)
	assert.Equal(t, 0, result.whoCan.verbCursor,
		"left at the first verb stays at 0 — guards against underflow")
}

func TestHandleWhoCanKey_SlashEntersFilterMode(t *testing.T) {
	m := Model{canIMode: canIModeWhoCan}
	mdl, _ := m.handleWhoCanKey(keyMsg("/"))
	result := mdl.(Model)
	assert.True(t, result.whoCan.resourceFilterActive,
		"/ flips the resource filter input into edit mode")
}

func TestUpdateWhoCanLoaded_StoresSubjects(t *testing.T) {
	m := Model{requestGen: 4, whoCan: whoCanState{loading: true}}
	subs := []k8s.WhoCanSubject{
		{Kind: "User", Name: "alice", Via: "ClusterRoleBinding/admins → ClusterRole/cluster-admin"},
	}
	result := m.updateWhoCanLoaded(whoCanLoadedMsg{gen: 4, subjects: subs})
	assert.False(t, result.whoCan.loading, "spinner clears once the fetch lands")
	assert.Len(t, result.whoCan.subjects, 1, "fetched subjects copied into Model state")
}

func TestUpdateWhoCanLoaded_StaleGenIgnored(t *testing.T) {
	m := Model{requestGen: 10, whoCan: whoCanState{loading: true}}
	result := m.updateWhoCanLoaded(whoCanLoadedMsg{gen: 1, subjects: []k8s.WhoCanSubject{{Kind: "User", Name: "x"}}})
	assert.True(t, result.whoCan.loading,
		"stale response leaves the loading flag armed so the next fresh response can clear it")
	assert.Empty(t, result.whoCan.subjects, "stale subjects must not overwrite Model state")
}
