package app

import (
	"os"
	"sync"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// capturePhase enumerates the traffic-capture overlay's life-cycle phases.
type capturePhase int

const (
	capturePhaseConfig capturePhase = iota
	capturePhaseEndpointPick
	capturePhaseLive
	capturePhaseStopped
)

// captureBackendAvailability is the per-backend probe result populated by
// loadCaptureBackends at overlay open.
type captureBackendAvailability struct {
	Backend   k8s.CaptureBackend
	Available bool
	Reason    string // empty when Available; otherwise an explanation shown grayed-out
}

// captureEndpoint is one Service-backing pod surfaced in the endpoint picker.
type captureEndpoint struct {
	PodName string
	IP      string
	Ready   bool
}

// captureOverlayState owns all state for the traffic-capture overlay.
//
// Lives as a single sub-struct on Model (mirrors whoCanState / crashInvState)
// so app.go stays under the 800-line file-length cap.
type captureOverlayState struct {
	// Target identification — captured at action open, refresh-resilient.
	targetKind string // "Pod" or "Service"
	targetNS   string
	targetName string

	// Detection results from loadCaptureBackends.
	backends  []captureBackendAvailability
	kubeshark *k8s.KubesharkInfo

	// Phase machine.
	phase capturePhase

	// Endpoint pick (Service targets only).
	endpoints      []captureEndpoint
	endpointCursor int
	resolvedPod    string // populated after Enter on the picker

	// Config phase.
	selectedBackend k8s.CaptureBackend
	iface           string
	snaplen         int
	filterValue     string
	presetCursor    int
	configFocus     captureConfigFocus // which Phase A field has keyboard focus

	// Live / Stopped phase.
	captureID      int
	liveBuf        *captureRing // ring buffer of last N PacketSummary
	showStatusOnly bool
	scrollOffset   int
	searchActive   bool

	// unsavedCaptureID is the ID of the most recent capture started in this
	// overlay session that the user has NOT marked saved by pressing Y.
	// Cleanup paths (overlay dismiss, restart) delete its pcap so abandoned
	// captures don't accumulate on disk. Y zeroes this out so the file is
	// preserved across the rest of the session.
	unsavedCaptureID int
}

// captureConfigFocus identifies which Phase A field has keyboard focus.
// Default is captureFocusFilter so every printable keypress goes to the
// filter input — the most common path. Tab cycles forward through the
// fields; Shift+Tab cycles backward.
type captureConfigFocus int

const (
	captureFocusFilter captureConfigFocus = iota
	captureFocusBackend
	captureFocusInterface
	captureFocusPreset
)

// captureFocusOrder is the cycle order used by Tab / Down / j. It matches the
// renderer's visual top-to-bottom row order so navigation feels predictable
// (Backend at top, Preset at bottom; the read-only Snaplen row is skipped).
// Default focus is captureFocusFilter so an unmodified keypress goes straight
// to the filter input.
var captureFocusOrder = []captureConfigFocus{
	captureFocusBackend,
	captureFocusInterface,
	captureFocusFilter,
	captureFocusPreset,
}

func (f captureConfigFocus) next() captureConfigFocus {
	for i, v := range captureFocusOrder {
		if v == f {
			return captureFocusOrder[(i+1)%len(captureFocusOrder)]
		}
	}
	return captureFocusFilter
}

func (f captureConfigFocus) prev() captureConfigFocus {
	for i, v := range captureFocusOrder {
		if v == f {
			return captureFocusOrder[(i-1+len(captureFocusOrder))%len(captureFocusOrder)]
		}
	}
	return captureFocusFilter
}

// captureRing is a mutex-protected ring buffer for live packet summaries.
type captureRing struct {
	mu       sync.Mutex
	buf      []k8s.PacketSummary
	capacity int // ring capacity; renamed from `cap` to avoid shadowing the builtin
	next     int
	n        int
}

func newCaptureRing(capacity int) *captureRing {
	return &captureRing{buf: make([]k8s.PacketSummary, capacity), capacity: capacity}
}

// Push appends a summary, evicting the oldest if the ring is full.
func (r *captureRing) Push(s k8s.PacketSummary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = s
	r.next = (r.next + 1) % r.capacity
	if r.n < r.capacity {
		r.n++
	}
}

// Snapshot returns a copy of the ring's contents in chronological order
// (oldest first).
func (r *captureRing) Snapshot() []k8s.PacketSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]k8s.PacketSummary, r.n)
	if r.n < r.capacity {
		copy(out, r.buf[:r.n])
		return out
	}
	// Full ring: r.next points to the oldest entry.
	copy(out, r.buf[r.next:])
	copy(out[r.capacity-r.next:], r.buf[:r.next])
	return out
}

// buildCaptureOverlayEntry translates Model state into the UI presentation
// struct passed to ui.RenderTrafficCaptureOverlay.
func buildCaptureOverlayEntry(m Model) ui.CaptureOverlayEntry {
	e := ui.CaptureOverlayEntry{
		Title:              captureOverlayTitle(m.captureOverlay),
		Phase:              toUICapturePhase(m.captureOverlay.phase),
		ConfigFocus:        toUICaptureFocus(m.captureOverlay.configFocus),
		Interface:          m.captureOverlay.iface,
		SnapLen:            m.captureOverlay.snaplen,
		FilterInputValue:   m.captureOverlay.filterValue,
		FilterPresets:      capturePresets,
		FilterPresetCursor: m.captureOverlay.presetCursor,
		EndpointCursor:     m.captureOverlay.endpointCursor,
		// Propagate the j/k scroll position so the renderer can window
		// the live packet table. Without this, scrollOffset is updated
		// on the model but the view always sees zero — making j/k feel
		// like a no-op.
		ScrollOffset: m.captureOverlay.scrollOffset,
	}

	for _, b := range m.captureOverlay.backends {
		e.Backends = append(e.Backends, ui.CaptureBackendChip{
			Label:     string(b.Backend),
			Selected:  b.Backend == m.captureOverlay.selectedBackend,
			Available: b.Available,
			Reason:    b.Reason,
		})
	}
	for _, ep := range m.captureOverlay.endpoints {
		e.EndpointRows = append(e.EndpointRows, ui.CaptureEndpointRow{
			PodName: ep.PodName, IP: ep.IP, Ready: ep.Ready,
		})
	}

	if m.captureOverlay.phase == capturePhaseLive || m.captureOverlay.phase == capturePhaseStopped {
		// Look up the entry from captureMgr for live counts.
		if m.captureMgr != nil {
			for _, ent := range m.captureMgr.Entries() {
				if ent.ID == m.captureOverlay.captureID {
					e.PacketCount = ent.PacketCount
					e.ByteCount = ent.ByteCount
					e.OutputPath = ent.OutputPath
					e.HeaderHints = "iface=" + ent.Request.Interface + " · backend=" + string(ent.Request.Backend)
					if ent.Request.BPFFilter != "" {
						e.FilterText = "filter: " + ent.Request.BPFFilter
					}
					switch ent.Status {
					case k8s.CaptureFailed:
						e.StatusBadge = "✗ failed"
					case k8s.CaptureStopped:
						e.StatusBadge = "■ stopped"
					case k8s.CaptureRunning, k8s.CaptureStarting:
						elapsed := time.Since(ent.StartedAt).Truncate(time.Second)
						e.StatusBadge = "● capturing " + elapsed.String()
					}
					e.LastError = ent.LastError
					break
				}
			}
		}
		if m.captureOverlay.liveBuf != nil {
			snap := m.captureOverlay.liveBuf.Snapshot()
			for _, s := range snap {
				e.Packets = append(e.Packets, ui.PacketRow{
					Time:     s.Time.Format("15:04:05.000"),
					Protocol: s.Protocol,
					Src:      s.SrcIP + ":" + s.SrcPort,
					Dst:      s.DstIP + ":" + s.DstPort,
					Length:   s.Length,
					Flags:    s.Flags,
					Extra:    s.Extra,
				})
			}
		}
		e.ShowStatusOnly = m.captureOverlay.showStatusOnly
	}
	return e
}

// captureScrollHalfPage / captureScrollFullPage estimate how many packet
// table rows fit in the live overlay. Used by ctrl+u/d (half) and
// ctrl+b/f (full) to move the scroll window by visually-meaningful chunks
// rather than relying on the user mashing j/k. Mirrors view_overlays.go's
// `h := min(35, m.height-6)` overlay-sizing math, minus a fixed budget for
// the title / filter / status / divider / table-header rows above the
// packet rows. The estimate doesn't have to be exact — j/k can fine-tune.
func (m Model) captureScrollFullPage() int {
	overlayH := min(35, m.height-6)
	contentH := max(overlayH-4, 5)
	rows := contentH - 8
	if rows < 1 {
		return 1
	}
	return rows
}

func (m Model) captureScrollHalfPage() int {
	half := m.captureScrollFullPage() / 2
	if half < 1 {
		return 1
	}
	return half
}

// dismissCaptureOverlay closes the capture overlay and deletes any pcap
// from the current session that the user did NOT mark saved (no Y press).
// Called from every phase's Esc/q handler so the user doesn't accumulate
// orphan pcap files on disk by accidentally opening + closing the overlay.
func (m Model) dismissCaptureOverlay() Model {
	m.deleteUnsavedCapture()
	m.captureOverlay.unsavedCaptureID = 0
	m.overlay = overlayNone
	return m
}

// deleteUnsavedCapture removes the pcap file for captureOverlay.unsavedCaptureID
// if any. Best-effort: filesystem errors are swallowed because (1) the file
// may already have been deleted by the user, and (2) failing to delete is
// not worse than the pre-cleanup status quo of leaving the file on disk.
func (m Model) deleteUnsavedCapture() {
	id := m.captureOverlay.unsavedCaptureID
	if id == 0 || m.captureMgr == nil {
		return
	}
	for _, ent := range m.captureMgr.Entries() {
		if ent.ID == id && ent.OutputPath != "" {
			_ = os.Remove(ent.OutputPath)
			return
		}
	}
}

func captureOverlayTitle(s captureOverlayState) string {
	target := s.targetNS + "/" + s.targetName
	if s.targetKind == "Service" && s.resolvedPod != "" {
		target = s.targetNS + "/" + s.resolvedPod + " (svc " + s.targetName + ")"
	}
	return "Traffic Capture — " + target
}

func toUICaptureFocus(f captureConfigFocus) ui.CaptureConfigFocus {
	switch f {
	case captureFocusBackend:
		return ui.CaptureFocusBackend
	case captureFocusInterface:
		return ui.CaptureFocusInterface
	case captureFocusPreset:
		return ui.CaptureFocusPreset
	default:
		return ui.CaptureFocusFilter
	}
}

func toUICapturePhase(p capturePhase) ui.CaptureOverlayPhase {
	switch p {
	case capturePhaseEndpointPick:
		return ui.CapturePhaseEndpointPick
	case capturePhaseLive:
		return ui.CapturePhaseLive
	case capturePhaseStopped:
		return ui.CapturePhaseStopped
	default:
		return ui.CapturePhaseConfig
	}
}
