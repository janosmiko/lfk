package app

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	clientfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// newLoadResourcesTestModel builds a Model ready to execute loadResources
// against a fake K8s client whose dynamic side knows about the Pod GVR.
// Shared by both registry integration tests to keep them focused on the
// bgtasks instrumentation assertions.
func newLoadResourcesTestModel() Model {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      "test-ctx",
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", APIVersion: "v1", Namespaced: true},
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		execMu:              &sync.Mutex{},
		namespace:           "default",
		bgtasks:             bgtasks.New(0),
		reqCtx:              context.Background(),
	}
	m.client = k8s.NewTestClient(clientfake.NewClientset(), newFinalDynClient())
	return m
}

// TestLoadResourcesCapturesSilentFromSuppressFlag verifies that
// loadResources propagates m.suppressBgtasks into the resourcesLoadedMsg
// it builds, so the msg handler can suppress downstream preview/metrics
// cmds as well. Without this propagation, watch-mode refreshes would
// silently re-load the list (which we already handle), but then the
// msg arrival would trigger loadPreview/loadPodMetricsForList on a
// Model whose suppressBgtasks flag had already been cleared — and those
// downstream loads would flash through the title-bar indicator every 2
// seconds.
func TestLoadResourcesCapturesSilentFromSuppressFlag(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	m.suppressBgtasks = true

	cmd := m.loadResources(false)
	msg := cmd().(resourcesLoadedMsg)

	assert.True(t, msg.silent,
		"loadResources must carry suppressBgtasks into the msg so "+
			"downstream cmds also run suppressed")
}

// TestLoadResourcesSilentDefaultsFalse verifies that a normal load (not
// from watch-tick) produces a msg with silent=false, so downstream
// cmds run visible.
func TestLoadResourcesSilentDefaultsFalse(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	// suppressBgtasks is false by default

	cmd := m.loadResources(false)
	msg := cmd().(resourcesLoadedMsg)

	assert.False(t, msg.silent,
		"user-driven loads must not mark the msg as silent")
}

// TestSuppressBgtasksFlagDoesNotLeakAfterWatchTick verifies that
// updateWatchTick resets m.suppressBgtasks to false on its returned
// model. Without this reset, the flag persists into the next Update
// (e.g. a user navigating to Secrets right after a watch refresh) and
// that navigation's loader incorrectly calls StartUntracked, so the
// indicator never appears for the user action.
func TestSuppressBgtasksFlagDoesNotLeakAfterWatchTick(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()
	m.watchMode = true
	// Precondition: the flag starts cleared.
	assert.False(t, m.suppressBgtasks)

	result, _ := m.updateWatchTick(watchTickMsg{})
	updated := result.(Model)

	assert.False(t, updated.suppressBgtasks,
		"suppressBgtasks must be reset on the returned model so it "+
			"doesn't leak into subsequent user-driven Updates")
}

// TestLoadResourcesRegistersTaskSynchronously verifies that loadResources
// calls Registry.Start SYNCHRONOUSLY at cmd-construction time (while
// Update is still building its return value), not later from inside the
// goroutine that runs the tea.Cmd closure.
//
// This is load-bearing for the title-bar indicator: bubbletea renders
// View() between Update() and goroutine dispatch, so if Start ran inside
// the closure the render frame would miss the task entirely and fast
// loads (typical k8s API calls at <100ms) would never flash through
// the indicator.
func TestLoadResourcesRegistersTaskSynchronously(t *testing.T) {
	t.Parallel()
	m := newLoadResourcesTestModel()

	before := m.bgtasks.NextIDForTest()
	cmd := m.loadResources(false)
	if cmd == nil {
		t.Fatal("loadResources returned nil cmd")
	}
	// At THIS point — before executing the cmd — the registry must
	// already have the task, because trackBgTask calls Start
	// synchronously while building the Cmd.
	afterConstruction := m.bgtasks.NextIDForTest()
	assert.Greater(t, afterConstruction, before,
		"trackBgTask must call Registry.Start synchronously at cmd "+
			"construction, not inside the deferred closure, so View() "+
			"sees the task before the goroutine runs")

	// After the cmd runs, the registry should return to its starting
	// state thanks to the deferred Finish inside the closure.
	_ = cmd()
	assert.Equal(t, 0, m.bgtasks.Len(),
		"loadResources must Finish its registered task after completion")
}
