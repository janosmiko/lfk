package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// capturePresets are the user-facing BPF filter presets cycled by Tab.
var capturePresets = []string{"all", "DNS", "HTTP/S", "no kube internals"}

// capturePresetFilters maps preset name to the corresponding BPF expression.
var capturePresetFilters = map[string]string{
	"all":               "",
	"DNS":               "port 53",
	"HTTP/S":            "port 80 or port 443",
	"no kube internals": "not port 6443 and not port 10250 and not arp",
}

// updateOverlayCapture is the entry point routed from update_overlays.go.
func (m Model) updateOverlayCapture(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.captureOverlay.phase {
	case capturePhaseEndpointPick:
		return m.updateOverlayCaptureEndpointPick(msg)
	case capturePhaseConfig:
		return m.updateOverlayCaptureConfig(msg)
	case capturePhaseLive:
		return m.updateOverlayCaptureLive(msg)
	case capturePhaseStopped:
		return m.updateOverlayCaptureStopped(msg)
	}
	return m, nil
}

func (m Model) updateOverlayCaptureEndpointPick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc, msg.String() == "q":
		return m.dismissCaptureOverlay(), nil
	case msg.Type == tea.KeyDown, msg.String() == "j":
		if m.captureOverlay.endpointCursor < len(m.captureOverlay.endpoints)-1 {
			m.captureOverlay.endpointCursor++
		}
		return m, nil
	case msg.Type == tea.KeyUp, msg.String() == "k":
		if m.captureOverlay.endpointCursor > 0 {
			m.captureOverlay.endpointCursor--
		}
		return m, nil
	case msg.Type == tea.KeyEnter:
		if m.captureOverlay.endpointCursor < len(m.captureOverlay.endpoints) {
			m.captureOverlay.resolvedPod = m.captureOverlay.endpoints[m.captureOverlay.endpointCursor].PodName
			m.captureOverlay.phase = capturePhaseConfig
		}
		return m, nil
	}
	return m, nil
}

// updateOverlayCaptureConfig is the focus-aware Phase A key handler.
//
// Default focus is captureFocusFilter so every printable key goes to the
// filter input — that's the most common path. Tab cycles focus following
// the renderer's visual top-to-bottom order (Backend → Interface → Filter
// → Preset → Backend); Shift+Tab reverses.
//
// When focus is NOT on Filter, vim-style navigation also works: j/↓ next
// field, k/↑ prev field, h/← prev value, l/→ next value. When focus IS on
// Filter, all printable characters (including hjkl) edit the filter input —
// only Tab/Shift+Tab navigate, so a BPF expression like `tcp and host
// 1.2.3.4` types correctly.
func (m Model) updateOverlayCaptureConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Always-active keys that work regardless of which field is focused.
	switch msg.Type {
	case tea.KeyEsc:
		return m.dismissCaptureOverlay(), nil
	case tea.KeyEnter:
		return m.startSelectedBackend()
	case tea.KeyTab:
		m.captureOverlay.configFocus = m.captureOverlay.configFocus.next()
		return m, nil
	case tea.KeyShiftTab:
		m.captureOverlay.configFocus = m.captureOverlay.configFocus.prev()
		return m, nil
	}

	// Filter focus: every other key edits the filter input. Vim-style hjkl
	// types literally so a user can type `tcp and host 1.2.3.4` without
	// triggering navigation.
	if m.captureOverlay.configFocus == captureFocusFilter {
		// Up/Down still navigate fields when on Filter — those keys aren't
		// printable and we don't want to trap the user inside the filter
		// with no way to reach Backend/Interface without Tab.
		switch msg.Type {
		case tea.KeyUp:
			m.captureOverlay.configFocus = m.captureOverlay.configFocus.prev()
			return m, nil
		case tea.KeyDown:
			m.captureOverlay.configFocus = m.captureOverlay.configFocus.next()
			return m, nil
		}
		return m.updateCaptureFilterInput(msg)
	}

	// Non-filter focus: vim navigation + arrow-key navigation.
	switch msg.Type {
	case tea.KeyUp:
		m.captureOverlay.configFocus = m.captureOverlay.configFocus.prev()
		return m, nil
	case tea.KeyDown:
		m.captureOverlay.configFocus = m.captureOverlay.configFocus.next()
		return m, nil
	case tea.KeyLeft:
		return m.cycleFocusedFieldValue(-1)
	case tea.KeyRight, tea.KeySpace:
		return m.cycleFocusedFieldValue(+1)
	}

	// Vim-style key bindings (only when NOT on the filter field).
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		switch msg.Runes[0] {
		case 'j':
			m.captureOverlay.configFocus = m.captureOverlay.configFocus.next()
			return m, nil
		case 'k':
			m.captureOverlay.configFocus = m.captureOverlay.configFocus.prev()
			return m, nil
		case 'h':
			return m.cycleFocusedFieldValue(-1)
		case 'l':
			return m.cycleFocusedFieldValue(+1)
		}
	}

	return m, nil
}

// cycleFocusedFieldValue moves the focused field's selection by dir
// (-1 = previous, +1 = next). No-op when focused on Filter.
func (m Model) cycleFocusedFieldValue(dir int) (tea.Model, tea.Cmd) {
	switch m.captureOverlay.configFocus {
	case captureFocusBackend:
		if dir < 0 {
			m.captureOverlay.selectedBackend = prevAvailableBackend(m.captureOverlay.backends, m.captureOverlay.selectedBackend)
		} else {
			m.captureOverlay.selectedBackend = nextAvailableBackend(m.captureOverlay.backends, m.captureOverlay.selectedBackend)
		}
	case captureFocusInterface:
		if dir < 0 {
			m.captureOverlay.iface = prevInterface(m.captureOverlay.iface)
		} else {
			m.captureOverlay.iface = nextInterface(m.captureOverlay.iface)
		}
	case captureFocusPreset:
		n := len(capturePresets)
		if n > 0 {
			if dir < 0 {
				m.captureOverlay.presetCursor = (m.captureOverlay.presetCursor - 1 + n) % n
			} else {
				m.captureOverlay.presetCursor = (m.captureOverlay.presetCursor + 1) % n
			}
			m.captureOverlay.filterValue = capturePresetFilters[capturePresets[m.captureOverlay.presetCursor]]
		}
	}
	return m, nil
}

// startSelectedBackend dispatches the start-or-handoff command for the
// currently-selected backend. Read-only mode blocks kubectl-debug;
// kubeshark hand-off is always allowed (it's read-only on the cluster).
func (m Model) startSelectedBackend() (tea.Model, tea.Cmd) {
	switch m.captureOverlay.selectedBackend {
	case k8s.BackendKubeshark:
		target := model.Item{
			Name:      m.captureOverlay.targetName,
			Namespace: m.captureOverlay.targetNS,
			Kind:      m.captureOverlay.targetKind,
		}
		return m, m.launchKubeshark(target)
	default:
		if m.readOnly {
			m.setStatusMessage("kubectl-debug capture disabled by read-only — kubeshark hand-off available", true)
			return m, nil
		}
		pod := m.captureOverlay.targetName
		if m.captureOverlay.targetKind == "Service" {
			pod = m.captureOverlay.resolvedPod
		}
		req := k8s.CaptureRequest{
			Backend:   m.captureOverlay.selectedBackend,
			Context:   m.actionCtx.context,
			Namespace: m.captureOverlay.targetNS,
			PodName:   pod,
			Interface: m.captureOverlay.iface,
			SnapLen:   m.captureOverlay.snaplen,
			BPFFilter: m.captureOverlay.filterValue,
		}
		// Allocate the ring on `m` (not on a local copy inside startCapture).
		// startCapture's tea.Cmd closes over the same buffer; the renderer
		// reads from m.captureOverlay.liveBuf.
		liveBuf := newCaptureRing(500)
		m.captureOverlay.liveBuf = liveBuf
		return m, m.startCapture(req, liveBuf)
	}
}

func (m Model) updateCaptureFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyBackspace && len(m.captureOverlay.filterValue) > 0 {
		m.captureOverlay.filterValue = m.captureOverlay.filterValue[:len(m.captureOverlay.filterValue)-1]
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		m.captureOverlay.filterValue += string(msg.Runes)
	}
	return m, nil
}

func nextAvailableBackend(backends []captureBackendAvailability, current k8s.CaptureBackend) k8s.CaptureBackend {
	return cycleAvailableBackend(backends, current, +1)
}

func prevAvailableBackend(backends []captureBackendAvailability, current k8s.CaptureBackend) k8s.CaptureBackend {
	return cycleAvailableBackend(backends, current, -1)
}

func cycleAvailableBackend(backends []captureBackendAvailability, current k8s.CaptureBackend, dir int) k8s.CaptureBackend {
	if len(backends) == 0 {
		return current
	}
	idx := -1
	for i, b := range backends {
		if b.Backend == current {
			idx = i
			break
		}
	}
	n := len(backends)
	for offset := 1; offset <= n; offset++ {
		i := ((idx + dir*offset) % n)
		if i < 0 {
			i += n
		}
		if backends[i].Available {
			return backends[i].Backend
		}
	}
	return current
}

func nextInterface(current string) string {
	switch current {
	case "any":
		return "eth0"
	case "eth0":
		return "lo"
	default:
		return "any"
	}
}

func prevInterface(current string) string {
	switch current {
	case "any":
		return "lo"
	case "eth0":
		return "any"
	case "lo":
		return "eth0"
	default:
		return "any"
	}
}

func (m Model) updateOverlayCaptureLive(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc, msg.String() == "q":
		// Esc/q stops the active capture and stays in the overlay so the
		// final stats land in the stopped phase visibly — the user can
		// then copy the path with Y, edit the filter with `e`, restart
		// with Enter, or close with another Esc. If there is no capture
		// running (shouldn't happen in this phase, but defensively),
		// dismiss + delete-unsaved.
		if m.captureOverlay.captureID > 0 {
			return m, m.stopCapture(m.captureOverlay.captureID)
		}
		return m.dismissCaptureOverlay(), nil
	case msg.String() == "s":
		return m, m.stopCapture(m.captureOverlay.captureID)
	case msg.String() == "t":
		m.captureOverlay.showStatusOnly = !m.captureOverlay.showStatusOnly
		return m, nil
	// Scroll semantics: scrollOffset is "rows back from the latest packet".
	// 0 means tail -f mode (auto-follow latest); k scrolls up into history,
	// j scrolls back toward the latest. ctrl+u/d move by half a page;
	// ctrl+b/f and PgUp/PgDn move by a full page (matches the convention
	// used by the describe / NetworkPolicy / right-sizing overlays). g
	// jumps to the oldest packet, G returns to live (latest). The renderer
	// clamps so out-of-range values land on the bounds.
	case msg.String() == "j":
		if m.captureOverlay.scrollOffset > 0 {
			m.captureOverlay.scrollOffset--
		}
		return m, nil
	case msg.String() == "k":
		m.captureOverlay.scrollOffset++
		return m, nil
	case msg.String() == "ctrl+d":
		m.captureOverlay.scrollOffset = max(m.captureOverlay.scrollOffset-m.captureScrollHalfPage(), 0)
		return m, nil
	case msg.String() == "ctrl+u":
		m.captureOverlay.scrollOffset += m.captureScrollHalfPage()
		return m, nil
	case msg.String() == "ctrl+f", msg.String() == "pgdown":
		m.captureOverlay.scrollOffset = max(m.captureOverlay.scrollOffset-m.captureScrollFullPage(), 0)
		return m, nil
	case msg.String() == "ctrl+b", msg.String() == "pgup":
		m.captureOverlay.scrollOffset += m.captureScrollFullPage()
		return m, nil
	case msg.String() == "g":
		m.captureOverlay.scrollOffset = 1<<31 - 1 // renderer clamps to max (oldest)
		return m, nil
	case msg.String() == "G":
		m.captureOverlay.scrollOffset = 0 // back to live (latest)
		return m, nil
	case msg.String() == "Y":
		// Surface the pcap path, copy it to the system clipboard, AND mark
		// the capture as "saved" so dismiss/restart cleanup keeps the file
		// instead of deleting it. Without the unsavedCaptureID reset, the
		// next dismiss or `e`+Enter would silently rm the file the user
		// just asked us to save.
		path := captureOutputPath(&m, m.captureOverlay.captureID)
		if path == "" {
			m.setStatusMessage("no active capture file", true)
			return m, nil
		}
		if m.captureOverlay.unsavedCaptureID == m.captureOverlay.captureID {
			m.captureOverlay.unsavedCaptureID = 0
		}
		m.setStatusMessage("pcap path copied to clipboard: "+path, false)
		return m, tea.Batch(copyToSystemClipboard(path), scheduleStatusClear())
	case msg.String() == "/":
		m.captureOverlay.searchActive = true
		return m, nil
	}
	return m, nil
}

// captureOutputPath looks up the on-disk pcap path for the given capture ID.
// Returns "" if no entry matches.
func captureOutputPath(m *Model, id int) string {
	if m.captureMgr == nil {
		return ""
	}
	for _, e := range m.captureMgr.Entries() {
		if e.ID == id {
			return e.OutputPath
		}
	}
	return ""
}

func (m Model) updateOverlayCaptureStopped(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc, msg.String() == "q":
		return m.dismissCaptureOverlay(), nil
	case msg.Type == tea.KeyEnter:
		return m.startSelectedBackend()
	case msg.String() == "e":
		// Go back to the config phase so the user can tweak the filter
		// (or backend / interface) before restarting. The previous filter
		// value lives on captureOverlay.filterValue and is preserved
		// across phase transitions, so the input pre-fills with what the
		// last capture used.
		m.captureOverlay.phase = capturePhaseConfig
		m.captureOverlay.configFocus = captureFocusFilter
		return m, nil
	}
	// Allow scroll/save/search in stopped state too — but Esc is handled
	// above, so updateOverlayCaptureLive's Esc (which would stop the
	// already-stopped capture) is unreachable from here.
	return m.updateOverlayCaptureLive(msg)
}
