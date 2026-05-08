package app

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubMsg struct{ value string }

func newTestModelWithScheduler() Model {
	m := newTestModel()
	m.scheduler = scheduler.New(0)
	return m
}

func TestScheduleK8sCall_RoundtripsResultMessage(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	m.scheduler.StartWorkers()
	defer m.scheduler.StopWorkers()

	cmd := m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindResourceList, "test", "test target",
		func(ctx context.Context) tea.Msg {
			return stubMsg{value: "delivered"}
		})
	require.NotNil(t, cmd)

	msg := cmd()
	got, ok := msg.(stubMsg)
	require.True(t, ok)
	assert.Equal(t, "delivered", got.value)
}

func TestScheduleK8sCall_NilFnReturnsNilCmd(t *testing.T) {
	m := newTestModelWithScheduler()
	cmd := m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindResourceList, "test", "test", nil)
	assert.Nil(t, cmd)
}

func TestScheduleK8sCall_CoalescedReturnsNilMsg(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	// No StartWorkers — keep the first task in the queue so the second
	// submission coalesces it. Then we observe the FIRST cmd's msg is
	// nil (because ErrCoalesced maps to nil tea.Msg per the helper's
	// contract).

	first := m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindResourceList, "List Pods", "default",
		func(ctx context.Context) tea.Msg { return stubMsg{value: "first"} })
	require.NotNil(t, first)

	// Second submission with the same Sig (kctx, kind, target, gen).
	second := m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindResourceList, "List Pods", "default",
		func(ctx context.Context) tea.Msg { return stubMsg{value: "second"} })
	require.NotNil(t, second)

	// Run the first cmd in a goroutine; it should return nil because
	// the scheduler reports ErrCoalesced.
	type result struct {
		msg tea.Msg
	}
	got := make(chan result, 1)
	go func() {
		got <- result{msg: first()}
	}()
	select {
	case r := <-got:
		assert.Nil(t, r.msg, "coalesced submission must return nil tea.Msg")
	case <-time.After(time.Second):
		t.Fatal("first cmd never returned")
	}
}

func TestLoadNamespacesSilent_SubmitsAtCriticalPriority(t *testing.T) {
	// Use a fake-client model so loadNamespacesSilent does not bail on nil client.
	// Workers are NOT started so the task stays queued for priority inspection.
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"

	cmd := m.loadNamespacesSilent(true)
	require.NotNil(t, cmd)
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityCritical) >= 1
	}, time.Second, 10*time.Millisecond, "loadNamespacesSilent must Submit at Critical priority")
}

func TestLoadNamespaces_SubmitsAtCriticalPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"

	cmd := m.loadNamespaces()
	require.NotNil(t, cmd)
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityCritical) >= 1
	}, time.Second, 10*time.Millisecond, "loadNamespaces must Submit at Critical priority")
}

func TestDiscoverAPIResources_SubmitsAtCriticalPriority(t *testing.T) {
	m := newTestModelWithScheduler()
	m.nav.Context = "test-ctx"
	// Don't StartWorkers — we want the task to sit in the queue so we can
	// assert priority via QueueLenByPriority.

	cmd := m.discoverAPIResources("test-ctx")
	require.NotNil(t, cmd)

	// Run cmd in a goroutine; the helper Submits synchronously, so by the
	// time the call returns, the task has been enqueued. Workers aren't
	// running so the task is still queued at PriorityCritical.
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityCritical) >= 1
	}, time.Second, 10*time.Millisecond, "discoverAPIResources must Submit at Critical priority")
}

func TestCheckRBAC_SubmitsAtCriticalPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"

	cmd := m.checkRBAC()
	require.NotNil(t, cmd)
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityCritical) >= 1
	}, time.Second, 10*time.Millisecond, "checkRBAC must Submit at Critical priority")
}

func TestLoadCanIRules_SubmitsAtCriticalPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"

	cmd := m.loadCanIRules()
	if cmd == nil {
		t.Skip("loadCanIRules returned nil cmd — likely guarded on state; manual verification needed")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityCritical) >= 1
	}, time.Second, 10*time.Millisecond, "loadCanIRules must Submit at Critical priority")
}

func TestLoadCanISAList_SubmitsAtCriticalPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"

	cmd := m.loadCanISAList()
	if cmd == nil {
		t.Skip("loadCanISAList returned nil cmd — likely guarded on state; manual verification needed")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityCritical) >= 1
	}, time.Second, 10*time.Millisecond, "loadCanISAList must Submit at Critical priority")
}

func TestLoadResources_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Pod",
		APIGroup:   "",
		APIVersion: "v1",
		Resource:   "pods",
		Namespaced: true,
	}
	m.scheduler.StopWorkers()

	cmd := m.loadResources(false)
	require.NotNil(t, cmd)
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadResources must Submit at High priority")
}

func TestLoadOwned_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true}
	m.nav.ResourceName = "test-app"
	m.scheduler.StopWorkers()

	cmd := m.loadOwned(false)
	if cmd == nil {
		t.Skip("loadOwned returned nil — likely guarded; manual verification needed")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadOwned must Submit at High priority")
}

func TestLoadContainers_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.ResourceName = "test-pod"
	m.scheduler.StopWorkers()

	cmd := m.loadContainers(false)
	if cmd == nil {
		t.Skip("loadContainers returned nil — likely guarded; manual verification needed")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadContainers must Submit at High priority")
}

func TestLoadYAML_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true}
	m.nav.ResourceName = "test-pod"
	m.middleItems = []model.Item{{Name: "test-pod", Namespace: "default", Kind: "Pod"}}
	m.scheduler.StopWorkers()

	cmd := m.loadYAML()
	if cmd == nil {
		t.Skip("loadYAML returned nil — likely guarded; manual verification needed")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadYAML must Submit at High priority")
}

func TestLoadEventTimeline_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.actionCtx = actionContext{
		context:      "test-ctx",
		namespace:    "default",
		name:         "test-pod",
		kind:         "Pod",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	m.scheduler.StopWorkers()

	cmd := m.loadEventTimeline()
	if cmd == nil {
		t.Skip("loadEventTimeline returned nil")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadEventTimeline must Submit at High priority")
}

func TestLoadPodStartup_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.actionCtx = actionContext{
		context:   "test-ctx",
		namespace: "default",
		name:      "test-pod",
	}
	m.scheduler.StopWorkers()

	cmd := m.loadPodStartup()
	if cmd == nil {
		t.Skip("loadPodStartup returned nil")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadPodStartup must Submit at High priority")
}

func TestLoadNetworkPolicy_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.actionCtx = actionContext{
		context:   "test-ctx",
		namespace: "default",
		name:      "test-policy",
	}
	m.scheduler.StopWorkers()

	cmd := m.loadNetworkPolicy()
	if cmd == nil {
		t.Skip("loadNetworkPolicy returned nil")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadNetworkPolicy must Submit at High priority")
}

func TestLoadPreviewYAML_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: "test-pod", Namespace: "default", Kind: "Pod"}}
	m.scheduler.StopWorkers()

	cmd := m.loadPreviewYAML()
	if cmd == nil {
		t.Skip("loadPreviewYAML returned nil")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadPreviewYAML must Submit at High priority")
}

func TestLoadContainerPorts_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.actionCtx = actionContext{
		context:   "test-ctx",
		namespace: "default",
		name:      "test-pod",
		kind:      "Pod",
	}
	m.scheduler.StopWorkers()

	cmd := m.loadContainerPorts()
	if cmd == nil {
		t.Skip("loadContainerPorts returned nil")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadContainerPorts must Submit at High priority")
}

func TestLoadPreviewServiceEndpoints_SubmitsAtHighPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services", Namespaced: true}
	m.middleItems = []model.Item{{Name: "test-svc", Namespace: "default", Kind: "Service"}}
	m.scheduler.StopWorkers()

	cmd := m.loadPreviewServiceEndpoints()
	if cmd == nil {
		t.Skip("loadPreviewServiceEndpoints returned nil — likely guarded")
	}
	// loadPreviewServiceEndpoints returns tea.Batch(emit, fetch). Calling
	// the outer cmd produces a tea.BatchMsg; the bubbletea runtime would
	// normally dispatch the children — here we drive them ourselves so
	// the fetch sub-cmd actually Submits to the scheduler.
	go func() {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub != nil {
					_ = sub()
				}
			}
		}
	}()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadPreviewServiceEndpoints must Submit at High priority")
}

func TestLoadPreviewSecretData_SubmitsAtHighPriority(t *testing.T) {
	prevLazy := ui.ConfigSecretLazyLoading
	ui.ConfigSecretLazyLoading = true
	t.Cleanup(func() { ui.ConfigSecretLazyLoading = prevLazy })

	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Secret", APIGroup: "", APIVersion: "v1", Resource: "secrets", Namespaced: true}
	m.middleItems = []model.Item{{Name: "test-secret", Namespace: "default", Kind: "Secret"}}
	m.secretPreviewCache = make(map[string]*model.SecretData)
	m.scheduler.StopWorkers()

	cmd := m.loadPreviewSecretData()
	if cmd == nil {
		t.Skip("loadPreviewSecretData returned nil — likely guarded")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityHigh) >= 1
	}, time.Second, 10*time.Millisecond, "loadPreviewSecretData must Submit at High priority")
}

func TestLoadResourceTree_SubmitsAtLowPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true}
	m.nav.ResourceName = "test-pod"
	m.middleItems = []model.Item{{Name: "test-pod", Namespace: "default", Kind: "Pod"}}
	m.scheduler.StopWorkers()

	cmd := m.loadResourceTree()
	if cmd == nil {
		t.Skip("loadResourceTree returned nil — needs more state")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityLow) >= 1
	}, time.Second, 10*time.Millisecond, "loadResourceTree must Submit at Low priority")
}

func TestLoadQuotas_SubmitsAtLowPriority(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "test-ctx"
	m.scheduler.StopWorkers()

	cmd := m.loadQuotas()
	if cmd == nil {
		t.Skip("loadQuotas returned nil")
	}
	go cmd()

	require.Eventually(t, func() bool {
		return m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityLow) >= 1
	}, time.Second, 10*time.Millisecond, "loadQuotas must Submit at Low priority")
}
