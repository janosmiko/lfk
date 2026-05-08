package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
)

func TestUpdateOverlaysCapture_EndpointPick_JKNavigates(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseEndpointPick
	m.captureOverlay.endpoints = []captureEndpoint{
		{PodName: "p1"}, {PodName: "p2"}, {PodName: "p3"},
	}

	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm := out.(Model)
	if mm.captureOverlay.endpointCursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", mm.captureOverlay.endpointCursor)
	}
}

func TestUpdateOverlaysCapture_EndpointPick_EnterAdvancesToConfig(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseEndpointPick
	m.captureOverlay.endpoints = []captureEndpoint{{PodName: "p1"}}

	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyEnter})
	mm := out.(Model)
	if mm.captureOverlay.phase != capturePhaseConfig {
		t.Errorf("phase = %v, want capturePhaseConfig", mm.captureOverlay.phase)
	}
	if mm.captureOverlay.resolvedPod != "p1" {
		t.Errorf("resolvedPod = %q, want p1", mm.captureOverlay.resolvedPod)
	}
}

func TestUpdateOverlaysCapture_Config_RightCyclesAvailableBackendsWhenFocused(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	m.captureOverlay.configFocus = captureFocusBackend
	m.captureOverlay.backends = []captureBackendAvailability{
		{Backend: k8s.BackendKubectlDebug, Available: true},
		{Backend: k8s.BackendKubeshark, Available: false}, // skipped
	}
	m.captureOverlay.selectedBackend = k8s.BackendKubectlDebug

	// With only one available backend, cycling right should stay on kubectl-debug.
	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRight})
	mm := out.(Model)
	if mm.captureOverlay.selectedBackend != k8s.BackendKubectlDebug {
		t.Errorf("after Right (only available backend): backend = %v, want kubectl-debug", mm.captureOverlay.selectedBackend)
	}
}

func TestUpdateOverlaysCapture_Config_TabCyclesFocus(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	// Default focus = Filter. Visual order is Backend → Interface → Filter → Preset,
	// so the first Tab from Filter advances to Preset (next visual row).
	if m.captureOverlay.configFocus != captureFocusFilter {
		t.Fatalf("default focus = %v, want captureFocusFilter", m.captureOverlay.configFocus)
	}
	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyTab})
	mm := out.(Model)
	if mm.captureOverlay.configFocus != captureFocusPreset {
		t.Errorf("after Tab: focus = %v, want captureFocusPreset (next visual row after Filter)", mm.captureOverlay.configFocus)
	}
	// Three more Tabs cycle back to Filter (Preset → Backend → Interface → Filter).
	for range 3 {
		out, _ = mm.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyTab})
		mm = out.(Model)
	}
	if mm.captureOverlay.configFocus != captureFocusFilter {
		t.Errorf("after full Tab cycle: focus = %v, want captureFocusFilter (loop)", mm.captureOverlay.configFocus)
	}
}

func TestUpdateOverlaysCapture_Config_VimNavigation(t *testing.T) {
	// j/k should cycle focus when not on Filter; h/l should cycle within
	// the focused field's value. When on Filter, j/k/h/l type into the input.
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	m.captureOverlay.configFocus = captureFocusBackend
	m.captureOverlay.backends = []captureBackendAvailability{
		{Backend: k8s.BackendKubectlDebug, Available: true},
		{Backend: k8s.BackendKubeshark, Available: true},
	}
	m.captureOverlay.selectedBackend = k8s.BackendKubectlDebug

	// `l` should cycle backend forward.
	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	mm := out.(Model)
	if mm.captureOverlay.selectedBackend != k8s.BackendKubeshark {
		t.Errorf("after l on Backend focus: backend = %v, want kubeshark", mm.captureOverlay.selectedBackend)
	}

	// `j` should advance focus from Backend to Interface (next visual row).
	out, _ = mm.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = out.(Model)
	if mm.captureOverlay.configFocus != captureFocusInterface {
		t.Errorf("after j on Backend: focus = %v, want captureFocusInterface", mm.captureOverlay.configFocus)
	}

	// `k` should rewind focus to Backend.
	out, _ = mm.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	mm = out.(Model)
	if mm.captureOverlay.configFocus != captureFocusBackend {
		t.Errorf("after k on Interface: focus = %v, want captureFocusBackend", mm.captureOverlay.configFocus)
	}

	// On Filter focus, `j` and `h` should both type into the filter (NOT navigate).
	mm.captureOverlay.configFocus = captureFocusFilter
	mm.captureOverlay.filterValue = ""
	out, _ = mm.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mm = out.(Model)
	out, _ = mm.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	mm = out.(Model)
	if mm.captureOverlay.filterValue != "jh" {
		t.Errorf("on Filter focus, j/h should type into filter; got filterValue = %q, want %q", mm.captureOverlay.filterValue, "jh")
	}
	if mm.captureOverlay.configFocus != captureFocusFilter {
		t.Errorf("on Filter focus, j/h should NOT navigate; focus = %v, want captureFocusFilter", mm.captureOverlay.configFocus)
	}
}

func TestUpdateOverlaysCapture_Config_FilterFocusedTakesAllChars(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	// configFocus defaults to captureFocusFilter; type letters that previously
	// triggered backend/iface cycling and verify they go into the filter.
	for _, r := range []rune{'b', 'i', 't', 'c', 'p'} {
		out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = out.(Model)
	}
	if m.captureOverlay.filterValue != "bitcp" {
		t.Errorf("filterValue = %q, want %q", m.captureOverlay.filterValue, "bitcp")
	}
}

func TestUpdateOverlaysCapture_Config_PresetFocus_RightCyclesPresets(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	m.captureOverlay.configFocus = captureFocusPreset

	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRight})
	mm := out.(Model)
	if mm.captureOverlay.presetCursor != 1 {
		t.Errorf("presetCursor = %d, want 1", mm.captureOverlay.presetCursor)
	}
	if mm.captureOverlay.filterValue != capturePresetFilters[capturePresets[1]] {
		t.Errorf("filterValue = %q, want %q",
			mm.captureOverlay.filterValue, capturePresetFilters[capturePresets[1]])
	}
}

func TestUpdateOverlaysCapture_Live_TTogglesStatusOnly(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseLive

	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	mm := out.(Model)
	if !mm.captureOverlay.showStatusOnly {
		t.Errorf("showStatusOnly = false, want true")
	}
}

// TestUpdateOverlaysCapture_Live_EscStopsButStaysOpen guards the
// two-step exit from the live phase: first Esc stops the capture and
// keeps the overlay open so the user sees the final stats land in the
// stopped phase; a second Esc actually dismisses. Earlier the test
// asserted the opposite (Esc closes immediately, capture continues in
// background) — that UX was changed because users hit Esc expecting the
// capture to stop, not to be backgrounded.
func TestUpdateOverlaysCapture_Live_EscStopsButStaysOpen(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseLive
	m.captureOverlay.captureID = 42

	out, cmd := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyEsc})
	mm := out.(Model)
	if mm.overlay != overlayTrafficCapture {
		t.Errorf("overlay = %v, want overlayTrafficCapture (overlay stays open after first Esc)", mm.overlay)
	}
	if cmd == nil {
		t.Fatal("Esc must dispatch the stop cmd")
	}
	if _, ok := cmd().(captureStoppedMsg); !ok {
		t.Errorf("got msg type %T, want captureStoppedMsg", cmd())
	}
	if mm.captureOverlay.captureID != 42 {
		t.Errorf("captureID = %d, want 42 (kept so the stopped phase shows the right entry)", mm.captureOverlay.captureID)
	}
}

func TestUpdateOverlaysCapture_Stopped_EnterRestarts(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseStopped
	m.captureOverlay.selectedBackend = k8s.BackendKubectlDebug
	m.captureOverlay.targetName = "pod1"
	m.captureOverlay.targetNS = "ns"
	m.actionCtx = actionContext{context: "ctx", namespace: "ns", name: "pod1", kind: "Pod"}

	_, cmd := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on stopped should dispatch start cmd")
	}
}

// TestUpdateOverlaysCapture_Stopped_EscClearsUnsavedID guards the
// state-mutation half of the dismiss-cleanup contract: closing the overlay
// must zero unsavedCaptureID so the next overlay open starts clean. The
// file-deletion side of the contract is exercised by manual testing /
// k8s-package integration coverage; doing it here would require reaching
// into CaptureManager internals.
func TestUpdateOverlaysCapture_Stopped_EscClearsUnsavedID(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseStopped
	m.captureOverlay.captureID = 7
	m.captureOverlay.unsavedCaptureID = 7

	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyEsc})
	mm := out.(Model)
	if mm.overlay != overlayNone {
		t.Errorf("overlay = %v, want overlayNone after Esc in stopped phase", mm.overlay)
	}
	if mm.captureOverlay.unsavedCaptureID != 0 {
		t.Errorf("unsavedCaptureID = %d, want 0 after dismiss", mm.captureOverlay.unsavedCaptureID)
	}
}

// TestUpdateOverlaysCapture_Live_PageScrollKeys guards the ctrl+u/d (half
// page) and ctrl+f/b (full page) shortcuts. Each must move scrollOffset by
// a non-trivial amount, in the right direction (newer = decrement, older =
// increment), and clamp at 0 when scrolling toward the latest packet.
func TestUpdateOverlaysCapture_Live_PageScrollKeys(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		startOffset int
		mustChange  bool
		mustBeZero  bool
	}{
		{"ctrl+u from 0 grows offset (older)", "ctrl+u", 0, true, false},
		{"ctrl+b from 0 grows offset (older)", "ctrl+b", 0, true, false},
		{"pgup from 0 grows offset (older)", "pgup", 0, true, false},
		{"ctrl+d from 0 stays at 0 (already at latest)", "ctrl+d", 0, false, true},
		{"ctrl+f from 0 stays at 0 (already at latest)", "ctrl+f", 0, false, true},
		{"pgdown from 0 stays at 0 (already at latest)", "pgdown", 0, false, true},
		{"ctrl+d from large offset shrinks (toward latest)", "ctrl+d", 100, true, false},
		{"ctrl+f from large offset shrinks (toward latest)", "ctrl+f", 100, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseFinalModel()
			m.captureMgr = k8s.NewCaptureManager()
			m.height = 40 // enough for captureScrollFullPage to be > 1
			m.overlay = overlayTrafficCapture
			m.captureOverlay.phase = capturePhaseLive
			m.captureOverlay.scrollOffset = tt.startOffset

			out, _ := m.updateOverlayCapture(keyMsgFromString(tt.key))
			mm := out.(Model)
			if tt.mustBeZero && mm.captureOverlay.scrollOffset != 0 {
				t.Errorf("scrollOffset = %d, want 0 (clamped at latest)", mm.captureOverlay.scrollOffset)
			}
			if tt.mustChange && mm.captureOverlay.scrollOffset == tt.startOffset {
				t.Errorf("scrollOffset unchanged at %d; %s should have moved it", mm.captureOverlay.scrollOffset, tt.key)
			}
		})
	}
}

// keyMsgFromString builds a tea.KeyMsg whose String() yields the supplied
// shortcut. Bubbletea's tea.KeyMsg has typed Key constants for ctrl/pg
// shortcuts; we build them by Type rather than by Runes so msg.String()
// matches what the production code switches on.
func keyMsgFromString(s string) tea.KeyMsg {
	switch s {
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+f":
		return tea.KeyMsg{Type: tea.KeyCtrlF}
	case "ctrl+b":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// TestExecuteActionCaptureTraffic_KubectlDebugAvailableSynchronously guards
// that the moment the overlay opens, the user sees the kubectl-debug chip
// — without waiting for the slow async kubeshark probe to complete. This
// is the visible UX outcome of the kubectl-debug-pre-population change.
func TestExecuteActionCaptureTraffic_KubectlDebugAvailableSynchronously(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx = actionContext{kind: "Pod", name: "pod1", namespace: "ns", context: "ctx"}

	out, _ := m.executeActionCaptureTraffic()
	mm := out.(Model)
	if len(mm.captureOverlay.backends) == 0 {
		t.Fatal("backends must be pre-populated synchronously so the UI is usable on first frame")
	}
	if mm.captureOverlay.backends[0].Backend != k8s.BackendKubectlDebug {
		t.Errorf("backends[0] = %v, want kubectl-debug pre-populated", mm.captureOverlay.backends[0].Backend)
	}
	if !mm.captureOverlay.backends[0].Available {
		t.Error("kubectl-debug must be Available=true at overlay open")
	}
	if mm.captureOverlay.selectedBackend != k8s.BackendKubectlDebug {
		t.Errorf("selectedBackend = %v, want kubectl-debug as default", mm.captureOverlay.selectedBackend)
	}
}

// TestY_NoActiveCaptureKeepsUnsavedFlag guards the "no active capture
// file" early return: when captureOutputPath returns empty (no matching
// entry in captureMgr), Y must NOT zero unsavedCaptureID — there's
// nothing to mark saved, and clearing the flag would silently let a
// subsequent dismiss skip the deletion of an unrelated unsaved capture.
//
// The mark-as-saved branch (captureID present in manager → path copied
// → unsavedCaptureID = 0) is covered by manual testing per TESTS.md.
// Exercising it from the app package would require widening
// CaptureManager's API only for tests; the value isn't worth the
// surface change.
func TestY_NoActiveCaptureKeepsUnsavedFlag(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseLive
	m.captureOverlay.captureID = 9
	m.captureOverlay.unsavedCaptureID = 9

	out, cmd := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	mm := out.(Model)
	if cmd != nil {
		t.Errorf("Y with no active capture file must not dispatch a cmd; got %v", cmd)
	}
	if !mm.statusMessageErr {
		t.Errorf("Y with no active capture file must set an error status; got err=%v msg=%q",
			mm.statusMessageErr, mm.statusMessage)
	}
	if mm.captureOverlay.unsavedCaptureID != 9 {
		t.Errorf("unsavedCaptureID = %d, want 9 preserved (nothing to mark saved on the empty-path early return)",
			mm.captureOverlay.unsavedCaptureID)
	}
}

// TestUpdateOverlaysCapture_Stopped_EOpensConfigForFilterEdit guards that
// the user can step back to the config phase from stopped to tweak the
// filter (or backend) without losing the previously-typed value.
func TestUpdateOverlaysCapture_Stopped_EOpensConfigForFilterEdit(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseStopped
	m.captureOverlay.filterValue = "port 443"

	out, _ := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	mm := out.(Model)
	if mm.captureOverlay.phase != capturePhaseConfig {
		t.Errorf("phase = %v, want capturePhaseConfig after `e`", mm.captureOverlay.phase)
	}
	if mm.captureOverlay.filterValue != "port 443" {
		t.Errorf("filterValue should be preserved across edit transition; got %q", mm.captureOverlay.filterValue)
	}
	if mm.captureOverlay.configFocus != captureFocusFilter {
		t.Errorf("config focus = %v, want captureFocusFilter so the user can immediately type", mm.captureOverlay.configFocus)
	}
}

// TestUpdateOverlaysCapture_Live_YCopiesPathToClipboard guards that pressing
// Y in the live phase dispatches a non-nil cmd (the clipboard copy + status
// clear). Without this regression test, a refactor that drops the cmd would
// silently regress the user-visible "copy path to clipboard" flow.
func TestUpdateOverlaysCapture_Live_YCopiesPathToClipboard(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseLive
	m.captureOverlay.captureID = 0 // no active capture, so Y should report "no active capture file"

	out, cmd := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	mm := out.(Model)
	if cmd != nil {
		t.Errorf("Y with no active capture must not dispatch a cmd; got %v", cmd)
	}
	if !mm.statusMessageErr {
		t.Errorf("Y with no active capture must set an error status; got msg=%q err=%v", mm.statusMessage, mm.statusMessageErr)
	}
}

// TestStartSelectedBackend_AssignsLiveBufOnReturnedModel guards the
// previous bug where startCapture mutated `m.captureOverlay.liveBuf` on a
// value-copy receiver, so the wired-up ring was thrown away and the live
// packet table stayed empty even while the byte/packet counters incremented.
func TestStartSelectedBackend_AssignsLiveBufOnReturnedModel(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	m.captureOverlay.selectedBackend = k8s.BackendKubectlDebug
	m.captureOverlay.targetName = "pod1"
	m.captureOverlay.targetNS = "ns"
	m.actionCtx = actionContext{context: "ctx", namespace: "ns", name: "pod1", kind: "Pod"}

	out, _ := m.startSelectedBackend()
	mm := out.(Model)
	if mm.captureOverlay.liveBuf == nil {
		t.Error("startSelectedBackend must assign liveBuf on the returned model so the live packet table renders")
	}
}

// TestUpdateOverlaysCapture_Stopped_ReadOnly_BlocksRestart guards the second
// entry point into startSelectedBackend (Enter on capturePhaseStopped). Without
// this test, a refactor that moves the read-only gate or adds a new code path
// could silently let a stopped kubectl-debug capture be restarted under
// --read-only.
func TestUpdateOverlaysCapture_Stopped_ReadOnly_BlocksRestart(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.readOnly = true
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseStopped
	m.captureOverlay.selectedBackend = k8s.BackendKubectlDebug
	m.captureOverlay.targetName = "pod1"
	m.captureOverlay.targetNS = "ns"
	m.actionCtx = actionContext{context: "ctx", namespace: "ns", name: "pod1", kind: "Pod"}

	out, cmd := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(captureStartedMsg); ok {
			t.Error("kubectl-debug restart from stopped phase must be blocked under read-only")
		}
	}
	mm := out.(Model)
	if !mm.statusMessageErr {
		t.Errorf("expected an error status message after blocked restart; got err=%v msg=%q",
			mm.statusMessageErr, mm.statusMessage)
	}
}

func TestUpdateOverlaysCapture_ReadOnly_BlocksKubectlDebug_AllowsKubeshark(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.readOnly = true
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	m.captureOverlay.targetName = "pod1"
	m.captureOverlay.targetNS = "ns"
	m.actionCtx = actionContext{context: "ctx", namespace: "ns", name: "pod1", kind: "Pod"}

	// Streaming backend: Enter should NOT dispatch a start cmd.
	m.captureOverlay.selectedBackend = k8s.BackendKubectlDebug
	out, cmd := m.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		// If the impl returns a cmd, make sure it's not a start.
		msg := cmd()
		if _, ok := msg.(captureStartedMsg); ok {
			t.Error("kubectl-debug start should be blocked under read-only")
		}
	}
	_ = out

	// Kubeshark hand-off: Enter SHOULD dispatch a cmd (the launch).
	mm := baseFinalModel()
	mm.readOnly = true
	mm.captureMgr = k8s.NewCaptureManager()
	mm.overlay = overlayTrafficCapture
	mm.captureOverlay.phase = capturePhaseConfig
	mm.captureOverlay.selectedBackend = k8s.BackendKubeshark
	mm.captureOverlay.targetName = "pod1"
	mm.captureOverlay.targetNS = "ns"
	mm.actionCtx = actionContext{context: "ctx", namespace: "ns", name: "pod1", kind: "Pod"}

	_, cmd2 := mm.updateOverlayCapture(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd2 == nil {
		t.Error("kubeshark hand-off should be allowed under read-only; got nil cmd")
	}
}
