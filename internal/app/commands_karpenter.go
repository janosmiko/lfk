package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
)

// executeActionKarpenter dispatches the Karpenter-specific action
// labels (Disrupt, Cordon / Uncordon / Drain Node) into the matching
// Tea command. Returns ok=true when the label is handled here so
// executeActionExtended can early-return without growing its switch.
// Disrupt opens the type-to-confirm overlay; the deferred mutation
// lives in update_overlays_confirm.go's "Disrupt" branch.
func (m Model) executeActionKarpenter(actionLabel string) (tea.Model, tea.Cmd, bool) {
	switch actionLabel {
	case "Disrupt":
		mdl, cmd := m.executeActionDisruptNodeClaim()
		return mdl, cmd, true
	case "Cordon/Uncordon Node":
		mdl, cmd := m.executeActionSimpleLoading("Toggling node scheduling from NodeClaim", m.toggleNodeScheduleFromClaim)
		return mdl, cmd, true
	case "Drain Node":
		mdl, cmd := m.executeActionSimpleLoading("Draining node from NodeClaim", m.drainNodeFromClaim)
		return mdl, cmd, true
	}
	return m, nil, false
}

// disruptNodeClaim deletes the NodeClaim under the cursor. Karpenter
// observes the deletion and terminates the underlying cloud instance
// plus the matching core Node, so this is the Karpenter-native way to
// take a single node out of service. Wrapped in trackBgTask so the
// task indicator renders during the brief delete window.
//
// Type-to-confirm gating is done one level up in the overlay handler
// (the action menu wires "Disrupt" through pendingAction so the user
// must type DELETE + Enter before this command runs).
func (m Model) disruptNodeClaim() tea.Cmd {
	ctx := m.actionCtx.context
	name := m.actionCtx.name
	logger.Info("Karpenter NodeClaim disrupt requested", "context", ctx, "name", name)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Disrupt NodeClaim: "+name, ctx, func(_ context.Context) tea.Msg {
		if err := m.client.DisruptNodeClaim(ctx, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Disrupted NodeClaim %s (Karpenter will terminate the node)", name)}
	})
}

// toggleNodeScheduleFromClaim resolves the NodeClaim's status.nodeName and
// flips the resolved node's spec.unschedulable via the API (cordon when
// schedulable, uncordon when cordoned). The NodeClaim row is the user's
// chosen surface; the underlying Node row offers the same toggle. Two errors
// short-circuit before the patch: the resolve fails (NodeClaim missing /
// RBAC), or the claim has no node bound yet (Karpenter still provisioning).
func (m Model) toggleNodeScheduleFromClaim() tea.Cmd {
	kctx := m.actionCtx.context
	claimName := m.actionCtx.name
	client := m.client

	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Toggle node scheduling (NodeClaim "+claimName+")", kctx, func(ctx context.Context) tea.Msg {
		nodeName, err := client.GetNodeClaimNodeName(kctx, claimName)
		if err != nil {
			return actionResultMsg{err: err}
		}
		if nodeName == "" {
			return actionResultMsg{err: fmt.Errorf("NodeClaim %s has no node bound yet (status.nodeName empty)", claimName)}
		}
		cordoned, err := client.ToggleNodeSchedulable(ctx, kctx, nodeName)
		if err != nil {
			return actionResultMsg{err: err}
		}
		verb := "Uncordoned"
		if cordoned {
			verb = "Cordoned"
		}
		return actionResultMsg{message: fmt.Sprintf("%s %s (from NodeClaim %s)", verb, nodeName, claimName)}
	})
}

// drainNodeFromClaim resolves the NodeClaim's status.nodeName and then drains
// it through the shared drainNodeCmd path (so the drain streams into the
// embedded terminal like the plain Node "Drain" action). The resolve runs off
// the main thread and hands the node name back via drainNodeResolvedMsg; two
// errors short-circuit before the drain: the resolve fails (NodeClaim missing /
// RBAC), or the claim has no node bound yet (Karpenter still provisioning).
func (m Model) drainNodeFromClaim() tea.Cmd {
	ctx := m.actionCtx.context
	claimName := m.actionCtx.name
	client := m.client

	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Resolve node for drain (NodeClaim "+claimName+")", ctx, func(_ context.Context) tea.Msg {
		nodeName, err := client.GetNodeClaimNodeName(ctx, claimName)
		if err != nil {
			return actionResultMsg{err: err}
		}
		if nodeName == "" {
			return actionResultMsg{err: fmt.Errorf("NodeClaim %s has no node bound yet (status.nodeName empty)", claimName)}
		}
		return drainNodeResolvedMsg{nodeName: nodeName}
	})
}
