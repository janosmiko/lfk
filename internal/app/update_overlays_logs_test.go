package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

func TestCovBoost2AutoSyncKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayAutoSync
	result, _ := m.handleAutoSyncKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovBoost2AutoSyncKeyQ(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayAutoSync
	result, _ := m.handleAutoSyncKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovBoost2AutoSyncKeyJ(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	result, _ := m.handleAutoSyncKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyK(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 1
	result, _ := m.handleAutoSyncKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyDown(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	result, _ := m.handleAutoSyncKey(keyMsg("down"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyUp(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	result, _ := m.handleAutoSyncKey(keyMsg("up"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeyJMax(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	result, _ := m.handleAutoSyncKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.autoSyncCursor) // stays at max
}

func TestCovBoost2AutoSyncKeyKMin(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	result, _ := m.handleAutoSyncKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.autoSyncCursor)
}

func TestCovBoost2AutoSyncKeySpaceToggleEnabled(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 0
	m.autoSyncEnabled = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, rm.autoSyncEnabled)
}

func TestCovBoost2AutoSyncKeySpaceToggleSelfHeal(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 1
	m.autoSyncEnabled = true
	m.autoSyncSelfHeal = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, rm.autoSyncSelfHeal)
}

func TestCovBoost2AutoSyncKeySpaceTogglePrune(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	m.autoSyncEnabled = true
	m.autoSyncPrune = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, rm.autoSyncPrune)
}

func TestCovBoost2AutoSyncKeySpaceDisabledSelfHeal(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 1
	m.autoSyncEnabled = false
	m.autoSyncSelfHeal = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.False(t, rm.autoSyncSelfHeal) // not toggled because autoSync disabled
}

func TestCovBoost2AutoSyncKeySpaceDisabledPrune(t *testing.T) {
	m := baseModelBoost2()
	m.autoSyncCursor = 2
	m.autoSyncEnabled = false
	m.autoSyncPrune = false
	result, _ := m.handleAutoSyncKey(keyMsg(" "))
	rm := result.(Model)
	assert.False(t, rm.autoSyncPrune)
}

func TestCovBoost2AutoSyncKeyEnter(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = []model.Item{{Name: "app-1", Namespace: "default"}}
	result, cmd := m.handleAutoSyncKey(keyMsg("enter"))
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovBoost2AutoSyncKeyCtrlS(t *testing.T) {
	m := baseModelBoost2()
	m.middleItems = []model.Item{{Name: "app-1", Namespace: "default"}}
	result, cmd := m.handleAutoSyncKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	assert.NotNil(t, cmd)
	_ = result
}

func TestCovBoost2AutoSyncKeyUnknown(t *testing.T) {
	m := baseModelBoost2()
	result, cmd := m.handleAutoSyncKey(keyMsg("x"))
	assert.Nil(t, cmd)
	_ = result
}

func TestCovLogContainerSelectEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogContainerSelectEscWithFilter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainerFilterText = "main"
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.logContainerFilterText)
}

func TestCovLogContainerSelectSpaceAll(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 0
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.Nil(t, rm.logSelectedContainers)
}

func TestCovLogContainerSelectSpaceSelectOne(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 1
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.NotNil(t, rm.logSelectedContainers)
}

func TestCovLogContainerSelectSpaceDeselect(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.logSelectedContainers = []string{"main", "sidecar"}
	m.overlayCursor = 1 // Points to "main".
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.True(t, len(rm.logSelectedContainers) < 2 || rm.logSelectedContainers == nil)
}

func TestCovLogContainerSelectSpaceAddContainer(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar", "proxy"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.logSelectedContainers = []string{"main"}
	m.overlayCursor = 2 // Points to "sidecar".
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg(" "))
	rm := result.(Model)
	assert.Equal(t, 2, len(rm.logSelectedContainers))
}

func TestCovLogContainerSelectEnterNoModify(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 1 // Points to "main".
	result, cmd := m.handleLogContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogContainerSelectEnterAll(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 0 // Points to "All Containers".
	result, cmd := m.handleLogContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Nil(t, rm.logSelectedContainers)
	assert.NotNil(t, cmd)
}

func TestCovLogContainerSelectEnterModified(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.logContainerSelectionModified = true
	m.logSelectedContainers = []string{"main"}
	result, cmd := m.handleLogContainerSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogContainerSelectSlash(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logContainerFilterActive)
}

func TestCovLogContainerSelectJK(t *testing.T) {
	m := baseModelBoost2()
	m.logContainers = []string{"main", "sidecar"}
	m.overlayItems = logContainerOverlayItems(m.logContainers)
	m.overlayCursor = 0
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)

	rm.overlayItems = logContainerOverlayItems(m.logContainers)
	result2, _ := rm.handleLogContainerSelectOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestCovLogContainerSelectCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.logContainers = make([]string, 20)
	m.overlayItems = make([]model.Item, 21) // "All Containers" + 20
	m.overlayItems[0] = model.Item{Name: "All Containers", Status: "all"}
	for i := range m.logContainers {
		m.logContainers[i] = "c" + string(rune('a'+i))
		m.overlayItems[i+1] = model.Item{Name: m.logContainers[i]}
	}
	m.overlayCursor = 0
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor > 0)
}

func TestCovLogContainerSelectCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.logContainers = make([]string, 20)
	m.overlayItems = make([]model.Item, 21)
	m.overlayItems[0] = model.Item{Name: "All Containers", Status: "all"}
	for i := range m.logContainers {
		m.logContainers[i] = "c" + string(rune('a'+i))
		m.overlayItems[i+1] = model.Item{Name: m.logContainers[i]}
	}
	m.overlayCursor = 15
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor < 15)
}

func TestCovLogPodSelectEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogPodSelectEscWithFilter(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.logPodFilterText = "web"
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Empty(t, rm.logPodFilterText)
}

func TestCovLogPodSelectEscWithSavedPod(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.logSavedPodName = "old-pod"
	result, cmd := m.handleLogPodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogPodSelectEnterAllPods(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.logParentKind = "Deployment"
	m.logParentName = "my-deploy"
	m.overlayItems = []model.Item{{Name: "All Pods", Status: "all"}, {Name: "pod-1"}}
	m.overlayCursor = 0
	result, cmd := m.handleLogPodSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovLogPodSelectEnterSpecificPod(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.overlayItems = []model.Item{{Name: "All Pods", Status: "all"}, {Name: "pod-1", Namespace: "ns1"}}
	m.overlayCursor = 1
	result, cmd := m.handleLogPodSelectOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, "pod-1", rm.actionCtx.name)
}

// Filter narrows to a single pod: Enter must apply it and close the
// overlay so the user does not have to press Enter again on a one-row list.
func TestLogPodFilterModeEnterAutoSelectsSoleResult(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.overlayItems = []model.Item{
		{Name: "All Pods", Status: "all"},
		{Name: "kube-proxy-abc", Namespace: "kube-system"},
		{Name: "etcd-main", Namespace: "kube-system"},
	}
	m.logPodFilterActive = true
	m.logPodFilterText = "kube-proxy"
	result, cmd := m.handleLogPodFilterMode(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.logPodFilterActive)
	assert.Equal(t, overlayNone, rm.overlay, "overlay must close")
	assert.Equal(t, "kube-proxy-abc", rm.actionCtx.name, "sole filter match must be applied")
	assert.Empty(t, rm.logPodFilterText, "filter text must be cleared after commit")
	assert.NotNil(t, cmd, "startLogStream command must be returned")
}

// Two filtered pod results: Enter must keep the legacy behavior — exit
// filter mode and let the user pick. Auto-applying would be a guess.
func TestLogPodFilterModeEnterPreservesMultipleResults(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.overlayItems = []model.Item{
		{Name: "All Pods", Status: "all"},
		{Name: "kube-proxy-1"},
		{Name: "kube-proxy-2"},
	}
	m.logPodFilterActive = true
	m.logPodFilterText = "kube"
	result, cmd := m.handleLogPodFilterMode(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.logPodFilterActive)
	assert.Equal(t, overlayLogPodSelect, rm.overlay, "overlay must stay open")
	assert.Equal(t, "kube", rm.logPodFilterText, "filter text must be preserved")
	assert.Nil(t, cmd, "no command must be issued when more than one match")
}

// Filter narrows to the "All Pods" virtual row only: fast-path must apply
// the all-pods stream (kind/name from logParentKind/logParentName).
func TestLogPodFilterModeEnterAutoSelectsSoleAllPodsResult(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	m.logParentKind = "Deployment"
	m.logParentName = "my-deploy"
	m.overlayItems = []model.Item{
		{Name: "All Pods", Status: "all"},
		{Name: "kube-proxy-1"},
	}
	m.logPodFilterActive = true
	m.logPodFilterText = "All"
	result, cmd := m.handleLogPodFilterMode(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Equal(t, "Deployment", rm.actionCtx.kind)
	assert.Equal(t, "my-deploy", rm.actionCtx.name)
	assert.NotNil(t, cmd)
}

func TestCovLogPodSelectSlash(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.logPodFilterActive)
}

func TestCovLogPodSelectJK(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = []model.Item{{Name: "p1"}, {Name: "p2"}, {Name: "p3"}}
	m.overlayCursor = 0
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)

	result2, _ := rm.handleLogPodSelectOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.overlayCursor)
}

func TestCovLogPodSelectCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = make([]model.Item, 20)
	m.overlayCursor = 0
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor > 0)
}

func TestCovLogPodSelectCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.overlayItems = make([]model.Item, 20)
	m.overlayCursor = 15
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.True(t, rm.overlayCursor < 15)
}

func TestCovPortForwardKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	result, _ := m.handlePortForwardOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovPortForwardKeyJK(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}, {Port: "443"}}
	m.pfPortCursor = 0
	result, _ := m.handlePortForwardOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.pfPortCursor)

	result2, _ := rm.handlePortForwardOverlayKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.pfPortCursor)
}

func TestCovPortForwardKeyEnterWithPort(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}}
	m.pfPortCursor = 0
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterWithCustomLocal(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}}
	m.pfPortCursor = 0
	m.portForwardInput.Insert("9090")
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterManual(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfPortCursor = -1
	m.portForwardInput.Insert("8080:80")
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterManualSinglePort(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfPortCursor = -1
	m.portForwardInput.Insert("8080")
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyEnterEmpty(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.pfPortCursor = -1
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd) // scheduleStatusClear
	assert.True(t, rm.hasStatusMessage())
}

func TestCovPortForwardKeyEnterService(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPortForward
	m.actionCtx.kind = "Service"
	m.pfAvailablePorts = []ui.PortInfo{{Port: "80"}}
	m.pfPortCursor = 0
	result, cmd := m.handlePortForwardOverlayKey(keyMsg("enter"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.NotNil(t, cmd)
}

func TestCovPortForwardKeyBackspace(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "12", rm.portForwardInput.Value)
}

func TestCovPortForwardKeyBackspaceEmpty(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handlePortForwardOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Empty(t, rm.portForwardInput.Value)
}

func TestCovPortForwardKeyCtrlW(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123 456")
	result, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	rm := result.(Model)
	assert.True(t, len(rm.portForwardInput.Value) < len("123 456"))
}

func TestCovPortForwardKeyCtrlA(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	_ = result
}

func TestCovPortForwardKeyCtrlE(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlE})
	_ = result
}

func TestCovPortForwardKeyLeft(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(keyMsg("left"))
	_ = result
}

func TestCovPortForwardKeyRight(t *testing.T) {
	m := baseModelBoost2()
	m.portForwardInput.Insert("123")
	result, _ := m.handlePortForwardOverlayKey(keyMsg("right"))
	_ = result
}

func TestCovPortForwardKeyDigit(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handlePortForwardOverlayKey(keyMsg("8"))
	rm := result.(Model)
	assert.Equal(t, "8", rm.portForwardInput.Value)
}

func TestCovPortForwardKeyColon(t *testing.T) {
	m := baseModelBoost2()
	m.pfPortCursor = 0
	result, _ := m.handlePortForwardOverlayKey(keyMsg(":"))
	rm := result.(Model)
	assert.Equal(t, -1, rm.pfPortCursor)
}

func TestCovPortForwardKeyInvalidChar(t *testing.T) {
	m := baseModelBoost2()
	result, _ := m.handlePortForwardOverlayKey(keyMsg("a"))
	rm := result.(Model)
	assert.Empty(t, rm.portForwardInput.Value)
}

func TestCovPortForwardOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPortForward
	result, _ := m.handlePortForwardOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovPortForwardOverlayKeyDigit(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPortForward
	result, _ := m.handlePortForwardOverlayKey(keyMsg("8"))
	rm := result.(Model)
	assert.Contains(t, rm.portForwardInput.Value, "8")
}

func TestCovPortForwardOverlayKeyBackspace(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPortForward
	m.portForwardInput.Insert("8080")
	result, _ := m.handlePortForwardOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "808", rm.portForwardInput.Value)
}

func TestCovPortForwardOverlayKeyColon(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayPortForward
	result, _ := m.handlePortForwardOverlayKey(keyMsg(":"))
	rm := result.(Model)
	assert.Contains(t, rm.portForwardInput.Value, ":")
}

func TestCovLogPodSelectOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayLogPodSelect
	m.logMultiItems = []model.Item{{Name: "pod-1"}}
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogPodSelectOverlayKeyDown(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayLogPodSelect
	m.logMultiItems = []model.Item{{Name: "pod-1"}, {Name: "pod-2"}}
	m.overlayCursor = 0
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("j"))
	_ = result.(Model)
}

func TestCovLogPodSelectOverlayKeyUp(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayLogPodSelect
	m.logMultiItems = []model.Item{{Name: "pod-1"}, {Name: "pod-2"}}
	m.overlayCursor = 1
	result, _ := m.handleLogPodSelectOverlayKey(keyMsg("k"))
	_ = result.(Model)
}

func TestCovLogContainerSelectOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"c1", "c2"}
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogContainerSelectOverlayKeyDownNav(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"c1", "c2"}
	m.overlayCursor = 0
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("j"))
	_ = result.(Model)
}

func TestCovLogContainerSelectOverlayKeyUpNav(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"c1", "c2"}
	m.overlayCursor = 1
	result, _ := m.handleLogContainerSelectOverlayKey(keyMsg("k"))
	_ = result.(Model)
}

func TestCovAutoSyncKeyEsc2(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayAutoSync
	result, _ := m.handleAutoSyncKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}
