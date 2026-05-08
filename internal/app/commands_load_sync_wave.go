package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
)

// syncWaveTimelineSkeletonTimeout bounds the FAST first-phase fetch
// (Application GET + parse only). The skeleton typically lands in
// ~100ms; 10s is a generous safety bound against a misbehaving
// apiserver. Kept tight so a stuck skeleton fails fast and the caller
// can surface an error to the user instead of hanging the overlay.
//
// syncWaveTimelineFullTimeout bounds the SLOW full timeline build
// (Application GET + concurrent per-resource annotation fan-out). On
// large applications the fan-out can fairly hit ~30s, and a few outlier
// apps with hundreds of managed resources have been seen to need more
// than that. 2 minutes is a comfortable upper bound that still keeps a
// truly stuck fetch from leaking the bgtask forever.
const (
	syncWaveTimelineSkeletonTimeout = 10 * time.Second
	syncWaveTimelineFullTimeout     = 2 * time.Minute
)

// loadSyncWaveTimeline kicks off a fetch for the action-context
// Application and emits syncWaveTimelineMsg with the result. The token
// argument is captured into the message so a stale fetch from a previous
// overlay session can be ignored on receipt.
func (m Model) loadSyncWaveTimeline(token uint64) tea.Cmd {
	client := m.client
	kctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return m.scheduleK8sCall(scheduler.PriorityLow, scheduler.KindResourceList, "Sync wave timeline: "+name, bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		fetchCtx, cancel := context.WithTimeout(ctx, syncWaveTimelineFullTimeout)
		defer cancel()
		info, err := client.GetSyncWaveTimeline(fetchCtx, kctx, ns, name)
		return syncWaveTimelineMsg{info: info, err: err, token: token}
	})
}

// loadSyncWaveTimelineSkeleton kicks off the FAST first-phase fetch:
// Application GET + parse, no per-resource annotation fan-out. Emits
// syncWaveTimelineMsg with info.Loading == true. The handler chains
// loadSyncWaveTimeline to fill in the wave numbers when the skeleton
// arrives. Same token rotation rules as the full loader.
func (m Model) loadSyncWaveTimelineSkeleton(token uint64) tea.Cmd {
	client := m.client
	kctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return m.scheduleK8sCall(scheduler.PriorityLow, scheduler.KindResourceList, "Sync wave skeleton: "+name, bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		fetchCtx, cancel := context.WithTimeout(ctx, syncWaveTimelineSkeletonTimeout)
		defer cancel()
		info, err := client.GetSyncWaveTimelineSkeleton(fetchCtx, kctx, ns, name)
		return syncWaveTimelineMsg{info: info, err: err, token: token}
	})
}
