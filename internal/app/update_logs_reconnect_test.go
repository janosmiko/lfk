package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// When following all containers of a Pod, a stream ending (e.g. the currently
// running init container finishes) must schedule an automatic reconnect so
// the next container is picked up without manual action. No sentinel marker
// is appended to the log buffer — the transition happens silently.
func TestUpdateLogLine_DonePodAllContainersSchedulesReconnect(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:     ch,
			follow: true,
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Pod",
			name: "my-pod",
		},
	}
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	rm := result.(Model)
	assert.Empty(t, rm.logView.lines,
		"no sentinel markers on auto-reconnect — the reconnect is silent")
	assert.NotNil(t, cmd,
		"done must return a restart command (scheduled)")
	assert.Equal(t, 1, rm.logView.autoReconnectAttempt)
}

// If the user has scrolled up to read history (logFollow=false), they are
// not watching live — there's no point arming a background reconnect for
// them. The stream just ends.
func TestUpdateLogLine_DoneNotInFollowModeDoesNotReconnect(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:     ch,
			follow: false, // user has scrolled away from the tail
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Pod",
			name: "my-pod",
		},
	}
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	rm := result.(Model)
	assert.Empty(t, rm.logView.lines)
	assert.Nil(t, cmd,
		"not in follow mode: don't schedule a reconnect on behalf of a user who isn't watching")
	assert.Equal(t, 0, rm.logView.autoReconnectAttempt)
}

// When a specific container was selected (actionCtx.containerName set), the
// user opted into that one stream — don't auto-reconnect when it ends.
func TestUpdateLogLine_DoneSpecificContainerStreamEnds(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch: ch,
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind:          "Pod",
			name:          "my-pod",
			containerName: "init-1",
		},
	}
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	rm := result.(Model)
	assert.Empty(t, rm.logView.lines)
	assert.Nil(t, cmd)
}

// A specific container that is still ContainerCreating makes kubectl logs
// -c <name> exit immediately with a "waiting to start" error. The viewer must
// keep that container's stream alive via auto-reconnect so logs appear once
// the container starts — otherwise it's stuck on the error forever.
func TestUpdateLogLine_SpecificContainerWaitingToStartReconnects(t *testing.T) {
	ch := make(chan string, 1)
	transient := `error: container "external-dns" in pod "external-dns-5fccb74474-gj9sk" is waiting to start: ContainerCreating`
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:     ch,
			follow: true,
		},
		tabs:              []TabState{{}},
		logReaderInFlight: make(map[chan string]bool),
		actionCtx: actionContext{
			kind:          "Pod",
			name:          "external-dns-5fccb74474-gj9sk",
			containerName: "external-dns",
		},
	}

	// The "waiting to start" notice arrives once and flags a pending start.
	result, _ := m.Update(logLineMsg{line: transient, ch: ch})
	m = result.(Model)
	assert.True(t, m.logView.pendingContainerStart)
	assert.Equal(t, []string{transient}, m.logView.rawLines, "notice shown once")

	// The stream then ends (kubectl exited) — a reconnect must be scheduled
	// even though a specific container was selected.
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	m = result.(Model)
	assert.NotNil(t, cmd, "specific-container stream waiting to start must reconnect")
}

// Across reconnect polls the same "waiting to start" notice must not be
// appended repeatedly — show it once, not one copy per poll.
func TestUpdateLogLine_SpecificContainerTransientDeduped(t *testing.T) {
	ch := make(chan string, 1)
	transient := `error: container "external-dns" in pod "p" is waiting to start: ContainerCreating`
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:                    ch,
			follow:                true,
			rawLines:              []string{transient},
			pendingContainerStart: true,
		},
		tabs:              []TabState{{}},
		logReaderInFlight: make(map[chan string]bool),
		actionCtx:         actionContext{kind: "Pod", name: "p", containerName: "external-dns"},
	}
	result, cmd := m.Update(logLineMsg{line: transient, ch: ch})
	m = result.(Model)
	assert.Equal(t, []string{transient}, m.logView.rawLines, "duplicate notice suppressed")
	assert.True(t, m.logView.pendingContainerStart)
	assert.NotNil(t, cmd, "reader re-armed to keep draining")
}

// Once real output arrives the pending flag clears, and a subsequent
// stream-end for that specific container no longer reconnects.
func TestUpdateLogLine_SpecificContainerRealOutputClearsPending(t *testing.T) {
	ch := make(chan string, 1)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:                    ch,
			follow:                true,
			pendingContainerStart: true,
		},
		tabs:              []TabState{{}},
		logReaderInFlight: make(map[chan string]bool),
		actionCtx:         actionContext{kind: "Pod", name: "p", containerName: "external-dns"},
	}
	result, _ := m.Update(logLineMsg{line: "2026-06-17T10:00:00Z serving metrics on :7979", ch: ch})
	m = result.(Model)
	assert.False(t, m.logView.pendingContainerStart, "real output means the container started")

	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	m = result.(Model)
	assert.Nil(t, cmd, "a started specific container that ends normally must not reconnect")
}

// Deployment/StatefulSet/etc. use --max-log-requests with a selector — a
// single "done" doesn't reliably mean a container transition, so we don't
// auto-reconnect. No sentinel marker is written either.
func TestUpdateLogLine_DoneDeploymentStreamEnds(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch: ch,
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Deployment",
			name: "my-deploy",
		},
	}
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	rm := result.(Model)
	assert.Empty(t, rm.logView.lines)
	assert.Nil(t, cmd)
}

// Multi-log streams merge multiple kubectl processes — don't auto-reconnect
// the merged channel.
func TestUpdateLogLine_DoneMultiStreamEnds(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:      ch,
			isMulti: true,
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Pod",
			name: "my-pod",
		},
	}
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	rm := result.(Model)
	assert.Empty(t, rm.logView.lines)
	assert.Nil(t, cmd)
}

// --previous mode shows a finite backlog — auto-reconnect doesn't make sense.
func TestUpdateLogLine_DonePreviousStreamEnds(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:       ch,
			previous: true,
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Pod",
			name: "my-pod",
		},
	}
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	rm := result.(Model)
	assert.Empty(t, rm.logView.lines)
	assert.Nil(t, cmd)
}

// After many consecutive empty reconnects (pod terminated), stop retrying.
// No restart command is returned. No sentinel marker is written — the log
// stream simply stops producing lines.
func TestUpdateLogLine_DoneGivesUpAfterMaxAttempts(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:                   ch,
			autoReconnectAttempt: logAutoReconnectMaxAttempts,
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Pod",
			name: "my-pod",
		},
	}
	result, cmd := m.Update(logLineMsg{done: true, ch: ch})
	rm := result.(Model)
	assert.Empty(t, rm.logView.lines)
	assert.Nil(t, cmd)
}

// A log line arriving resets the reconnect-attempt counter so a subsequent
// stream-end is treated as a fresh transition, not as "N-th consecutive
// dead stream."
func TestUpdateLogLine_LineReceivedResetsAttemptCounter(t *testing.T) {
	ch := make(chan string, 1)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch:                   ch,
			autoReconnectAttempt: 3,
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Pod",
			name: "my-pod",
		},
	}
	result, _ := m.Update(logLineMsg{line: "new line", ch: ch})
	rm := result.(Model)
	assert.Equal(t, 0, rm.logView.autoReconnectAttempt,
		"incoming line means the stream is producing output; reset attempt counter")
}

// A restart msg whose channel no longer matches m.logView.ch (user switched pods
// or exited logs mode) must be ignored.
func TestUpdateLogStreamRestart_StaleChannelIgnored(t *testing.T) {
	oldCh := make(chan string)
	newCh := make(chan string)
	m := Model{
		mode: modeLogs,
		logView: logViewState{
			ch: newCh, // current stream is a different channel
		},
		tabs: []TabState{{}},
		actionCtx: actionContext{
			kind: "Pod",
			name: "my-pod",
		},
	}
	result, cmd := m.Update(logStreamRestartMsg{ch: oldCh})
	rm := result.(Model)
	assert.Nil(t, cmd, "stale restart (different channel) must be a no-op")
	_ = rm
}

// If the user exited logs mode before the restart fires, do nothing.
func TestUpdateLogStreamRestart_NotInLogsModeIgnored(t *testing.T) {
	ch := make(chan string)
	m := Model{
		mode: modeExplorer,
		logView: logViewState{
			ch: ch,
		},
		tabs: []TabState{{}},
	}
	result, cmd := m.Update(logStreamRestartMsg{ch: ch})
	rm := result.(Model)
	assert.Nil(t, cmd)
	_ = rm
}
