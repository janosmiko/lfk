package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// errNoQuarantineTarget means there is no client to ask, so QuarantineTargets
// cannot run.
var errNoQuarantineTarget = errors.New("quarantine targets are not available for this action")

// quarantineTargetsMsg carries the Services and label keys a Quarantine
// would touch. req, context, namespace and name together identify the
// fetch a reply belongs to, so a stale or mismatched pod reply is dropped.
type quarantineTargetsMsg struct {
	req       uint64
	context   string
	namespace string
	name      string
	services  []string
	keys      []string
	err       error
}

// loadQuarantineTargets fetches which Services route to the target pod.
// Runs before the confirm overlay opens: "no Services match" is answered
// with a status message instead of a confirm asking about nothing.
func (m Model) loadQuarantineTargets() tea.Cmd {
	req := m.quarantine.req
	client := m.client
	ctxName := m.actionCtx.context
	namespace := m.actionCtx.namespace
	name := m.actionCtx.name
	podLabels, _, _ := unstructured.NestedStringMap(m.actionCtx.raw, "metadata", "labels")
	if client == nil {
		return func() tea.Msg {
			return quarantineTargetsMsg{req: req, context: ctxName, namespace: namespace, name: name, err: errNoQuarantineTarget}
		}
	}
	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Quarantine targets: "+name,
		bgtaskTarget(ctxName, namespace),
		func(ctx context.Context) tea.Msg {
			services, keys, err := client.QuarantineTargets(ctx, ctxName, namespace, podLabels)
			return quarantineTargetsMsg{
				req: req, context: ctxName, namespace: namespace, name: name,
				services: services, keys: keys, err: err,
			}
		},
	)
}

// executeActionQuarantine handles the "Quarantine" action: it fetches the
// affected Services first and opens the confirm overlay only once the reply
// names them (updateQuarantineTargets).
func (m Model) executeActionQuarantine() (tea.Model, tea.Cmd) {
	m.quarantine.reset()
	return m, m.loadQuarantineTargets()
}

// executeActionRestore handles the "Restore" action. Unlike Quarantine, the
// confirm question is static and needs no cluster fetch: the pairs to put
// back are already on the pod's annotation (D5).
func (m Model) executeActionRestore() (tea.Model, tea.Cmd) {
	m.quarantine.reset()
	restored, ok := quarantinedLabels(m.actionCtx.raw)
	if !ok {
		m.setStatusMessage("This pod is not quarantined", false)
		return m, scheduleStatusClear()
	}
	m.quarantine.restore = restored
	name := ui.SanitizeTerminalText(m.actionCtx.name)
	m.confirmAction = m.actionCtx.name
	m.confirmTitle = "Confirm Restore"
	m.confirmQuestion = fmt.Sprintf("Restore %s?", name)
	m.overlay = overlayConfirm
	m.pendingAction = model.ActionLabelRestore
	return m, nil
}

// updateQuarantineTargets stores the fetch's reply and opens the confirm
// overlay, unless a pod with no matching Service selector has nothing to
// quarantine, in which case a status message replaces the dialog.
func (m Model) updateQuarantineTargets(msg quarantineTargetsMsg) (tea.Model, tea.Cmd) {
	if msg.req != m.quarantine.req {
		return m, nil // an older fetch answering late
	}
	if msg.context != m.actionCtx.context || msg.namespace != m.actionCtx.namespace || msg.name != m.actionCtx.name {
		return m, nil // a reply for a pod the menu no longer targets
	}
	if msg.err != nil {
		m.setErrorFromErr("Quarantine: ", msg.err)
		return m, scheduleStatusClear()
	}
	if len(msg.services) == 0 {
		name := ui.SanitizeTerminalText(m.actionCtx.name)
		m.setStatusMessage("Nothing to quarantine: no Service selector matches "+name, false)
		return m, scheduleStatusClear()
	}

	m.quarantine.services = msg.services
	m.quarantine.keys = msg.keys
	name := ui.SanitizeTerminalText(m.actionCtx.name)
	m.confirmAction = m.actionCtx.name
	m.confirmTitle = "Confirm Quarantine"
	m.confirmQuestion = fmt.Sprintf("Quarantine %s?", name)
	m.overlay = overlayConfirm
	m.pendingAction = model.ActionLabelQuarantine
	return m, nil
}

// quarantineTargetsChangedMsg means the Services routing to the pod (or
// their selector keys) changed between the confirm box opening and Enter,
// so the confirmed keys no longer match what a quarantine would touch.
type quarantineTargetsChangedMsg struct{}

// updateQuarantineTargetsChanged aborts a stale Quarantine commit: nothing
// was patched, so only the loading/confirm state needs unwinding.
func (m Model) updateQuarantineTargetsChanged() (tea.Model, tea.Cmd) {
	m.loading = false
	m.quarantine.reset()
	m.setStatusMessage("Services changed since the confirm box, run Quarantine again", true)
	return m, scheduleStatusClear()
}

// quarantinePodCmd commits the Quarantine confirm. keys is passed
// explicitly: the caller resets m.quarantine before this cmd's closure
// runs. It re-runs QuarantineTargets first, refusing to patch on drift.
func (m Model) quarantinePodCmd(keys []string) tea.Cmd {
	client := m.client
	ctxName := m.actionCtx.context
	namespace := m.actionCtx.namespace
	name := m.actionCtx.name
	podLabels, _, _ := unstructured.NestedStringMap(m.actionCtx.raw, "metadata", "labels")
	safeName := ui.SanitizeTerminalText(name)
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindMutation,
		"Quarantine: "+name,
		bgtaskTarget(ctxName, namespace),
		func(ctx context.Context) tea.Msg {
			_, freshKeys, err := client.QuarantineTargets(ctx, ctxName, namespace, podLabels)
			if err != nil {
				return actionResultMsg{err: err}
			}
			if !slices.Equal(freshKeys, keys) {
				return quarantineTargetsChangedMsg{}
			}
			if _, err := client.QuarantinePod(ctx, ctxName, namespace, name, keys); err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{message: "Quarantined " + safeName}
		},
	)
}

// restorePodCmd commits the Restore confirm: puts back the label pairs
// QuarantinePod recorded and clears the annotation.
func (m Model) restorePodCmd() tea.Cmd {
	client := m.client
	ctxName := m.actionCtx.context
	namespace := m.actionCtx.namespace
	name := m.actionCtx.name
	safeName := ui.SanitizeTerminalText(name)
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindMutation,
		"Restore: "+name,
		bgtaskTarget(ctxName, namespace),
		func(ctx context.Context) tea.Msg {
			if _, err := client.RestorePod(ctx, ctxName, namespace, name); err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{message: "Restored " + safeName}
		},
	)
}

// quarantineNullLabelsJSON renders the label keys a Quarantine patch nulls
// out, for the debug log line only — the patch itself is built with
// json.Marshal in the k8s package.
func quarantineNullLabelsJSON(keys []string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%q:null", k)
	}
	return strings.Join(parts, ",")
}
