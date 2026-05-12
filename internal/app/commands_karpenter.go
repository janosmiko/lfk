package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	case "Cordon Node":
		mdl, cmd := m.executeActionSimpleLoading("Cordoning node from NodeClaim", m.cordonNodeFromClaim)
		return mdl, cmd, true
	case "Uncordon Node":
		mdl, cmd := m.executeActionSimpleLoading("Uncordoning node from NodeClaim", m.uncordonNodeFromClaim)
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
	return m.trackBgTask(scheduler.KindMutation, "Disrupt NodeClaim: "+name, ctx, func() tea.Msg {
		if err := m.client.DisruptNodeClaim(ctx, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Disrupted NodeClaim %s (Karpenter will terminate the node)", name)}
	})
}

// cordonNodeFromClaim resolves the NodeClaim's status.nodeName and
// shells out `kubectl cordon <node>` against the resolved name. The
// NodeClaim row is the user's chosen surface; the underlying Node row
// would also offer a plain Cordon. Two errors short-circuit before the
// shell-out: the resolve fails (NodeClaim missing / RBAC), or the
// claim has no node bound yet (Karpenter still provisioning).
func (m Model) cordonNodeFromClaim() tea.Cmd {
	return m.kubectlNodeCmdFromClaim("cordon")
}

// uncordonNodeFromClaim mirrors cordonNodeFromClaim but runs
// `kubectl uncordon <node>` against the resolved name. Provided so
// users can re-mark a node as schedulable directly from the NodeClaim
// row without bouncing through the Node view.
func (m Model) uncordonNodeFromClaim() tea.Cmd {
	return m.kubectlNodeCmdFromClaim("uncordon")
}

// drainNodeFromClaim mirrors cordonNodeFromClaim but runs
// `kubectl drain <node> --ignore-daemonsets --delete-emptydir-data`,
// matching the flags the standalone executeActionDrain path uses on a
// plain Node row.
func (m Model) drainNodeFromClaim() tea.Cmd {
	return m.kubectlNodeCmdFromClaim("drain")
}

// kubectlNodeCmdFromClaim resolves the NodeClaim's status.nodeName and
// runs kubectl <subcmd> against the resolved node. Tracked as a
// mutation so the title-bar indicator surfaces the in-flight operation
// and the :tasks overlay shows it in the history.
//
// subcmd must be either "cordon" or "drain"; drain adds the standard
// eviction-safety flags. Anything else falls through to a plain
// kubectl invocation with no extra args.
func (m Model) kubectlNodeCmdFromClaim(subcmd string) tea.Cmd {
	ctx := m.actionCtx.context
	claimName := m.actionCtx.name
	kctxArg := m.kubectlContext(ctx)
	kubeconfigPath := m.client.KubeconfigPathForContext(ctx)
	client := m.client

	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("%s node (NodeClaim %s)", subcmd, claimName), ctx, func() tea.Msg {
		nodeName, err := client.GetNodeClaimNodeName(ctx, claimName)
		if err != nil {
			return actionResultMsg{err: err}
		}
		if nodeName == "" {
			return actionResultMsg{err: fmt.Errorf("NodeClaim %s has no node bound yet (status.nodeName empty)", claimName)}
		}

		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
		args := []string{subcmd, nodeName, "--context", kctxArg}
		if subcmd == "drain" {
			args = append(args, "--ignore-daemonsets", "--delete-emptydir-data")
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
		logExecCmd("Running kubectl command (resolved from NodeClaim)", cmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl node command failed", "subcmd", subcmd, "claim", claimName, "node", nodeName, "context", ctx, "error", err)
			// kubectl sometimes exits non-zero with no stderr (e.g. context
			// cancelled, kubeconfig path missing on disk). Fall back to the
			// exec error so the user always sees concrete failure details
			// instead of a trailing blank.
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = err.Error()
			}
			return actionResultMsg{err: fmt.Errorf("%s %s (from NodeClaim %s): %s", subcmd, nodeName, claimName, detail)}
		}
		return actionResultMsg{message: fmt.Sprintf("%s %s (from NodeClaim %s): %s", subcmd, nodeName, claimName, strings.TrimSpace(string(output)))}
	})
}
