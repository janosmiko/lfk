package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
)

// syncWaveTimelineTimeout bounds one full timeline build (Application GET
// + concurrent annotation fetches). 30s is generous; the inner GETs
// individually time out via the apiserver client defaults.
const syncWaveTimelineTimeout = 30 * time.Second

// loadSyncWaveTimeline kicks off a fetch for the action-context
// Application and emits syncWaveTimelineMsg with the result. The token
// argument is captured into the message so a stale fetch from a previous
// overlay session can be ignored on receipt.
func (m Model) loadSyncWaveTimeline(token uint64) tea.Cmd {
	client := m.client
	kctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindResourceList, "Sync wave timeline: "+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		fetchCtx, cancel := context.WithTimeout(context.Background(), syncWaveTimelineTimeout)
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
	return m.trackBgTask(bgtasks.KindResourceList, "Sync wave skeleton: "+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		// Same timeout as the full fetch — the skeleton itself is fast
		// but we keep the bound for safety against a misbehaving apiserver.
		fetchCtx, cancel := context.WithTimeout(context.Background(), syncWaveTimelineTimeout)
		defer cancel()
		info, err := client.GetSyncWaveTimelineSkeleton(fetchCtx, kctx, ns, name)
		return syncWaveTimelineMsg{info: info, err: err, token: token}
	})
}
