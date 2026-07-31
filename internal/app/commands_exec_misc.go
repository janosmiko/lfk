package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) deleteResource() tea.Cmd {
	if m.actionCtx.resourceType.APIGroup == "_helm" {
		return m.uninstallHelmRelease()
	}

	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	policy := m.deletePropagation()
	logger.Info("Deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx, "propagation", policy)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, fmt.Sprintf("Delete %s/%s", rt.Resource, name), bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		err := m.client.DeleteResource(ctx, ns, rt, name, policy)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Deleted %s/%s", rt.Resource, name)}
	})
}

func (m Model) forceDeleteResource() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	// Cascading() guarantees a value kubectl accepts; --cascade=none is not one.
	cascade := m.deletePropagation().Cascading()
	logger.Info("Force deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx, "cascade", cascade)

	deleteArgs := forceDeleteArgs(rt, name, m.kubectlContext(ctx), ns, cascade)

	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Force delete %s/%s", rt.Resource, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		cmd := exec.Command(kubectlPath, deleteArgs...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
		logExecCmd("Running kubectl command", cmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl force delete failed", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx, "error", err)
			return actionResultMsg{err: fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Force deleted %s/%s", rt.Resource, name)}
	})
}

func (m Model) removeFinalizers() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	logger.Info("Removing finalizers from resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)

	patchArgs := []string{
		"patch", kubectlResourceArg(rt), name, "--context", m.kubectlContext(ctx),
		"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`,
	}
	if rt.Namespaced {
		patchArgs = append(patchArgs, "-n", ns)
	}

	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Remove finalizers: %s/%s", rt.Resource, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		cmd := exec.Command(kubectlPath, patchArgs...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
		logExecCmd("Running kubectl command", cmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl patch failed", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx, "error", err)
			return actionResultMsg{err: fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Finalizers removed from %s/%s", rt.Resource, name)}
	})
}

func (m Model) vulnScanImage(image string) tea.Cmd {
	trivyPath, err := exec.LookPath("trivy")
	if err != nil {
		return func() tea.Msg {
			return describeLoadedMsg{
				title: "Vulnerability Scan",
				err:   fmt.Errorf("trivy not found in PATH: %w (install: https://aquasecurity.github.io/trivy)", err),
			}
		}
	}

	title := fmt.Sprintf("Vuln Scan: %s", image)
	return m.trackBgTask(scheduler.KindSubprocess, title, "", func() tea.Msg {
		args := []string{"image", "--scanners", "vuln", "--format", "table", "--no-progress", image}
		cmd := exec.Command(trivyPath, args...)
		cmd.Env = os.Environ()
		logExecCmd("Running trivy command", cmd)
		output, cmdErr := cmd.CombinedOutput()
		content := cleanANSI(strings.TrimSpace(string(output)))
		if cmdErr != nil {
			logger.Error("trivy scan failed", "image", image, "error", cmdErr)
			if content == "" {
				return describeLoadedMsg{title: title, err: fmt.Errorf("trivy scan failed: %w", cmdErr)}
			}
			return describeLoadedMsg{content: content, title: title}
		}
		if content == "" {
			content = "No vulnerabilities found."
		}
		return describeLoadedMsg{content: content, title: title}
	})
}

func (m Model) resizePVC(newSize string) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logger.Info("Resizing PVC", "name", name, "newSize", newSize, "namespace", ns, "context", ctx)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Resize PVC: "+name, bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		err := m.client.ResizePVC(ctx, ns, name, newSize)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resize requested for %s to %s", name, newSize)}
	})
}

func (m Model) scaleResource(replicas int32) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	logger.Info("Scaling resource", "kind", kind, "name", name, "replicas", replicas, "namespace", ns, "context", ctx)
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, fmt.Sprintf("Scale %s/%s → %d", kind, name, replicas), bgtaskTarget(ctx, ns), func(_ context.Context) tea.Msg {
		err := m.client.ScaleResource(ctx, ns, name, kind, replicas)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Scaled %s to %d replicas", name, replicas)}
	})
}

func (m Model) restartResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	logger.Info("Restarting resource", "kind", kind, "name", name, "namespace", ns, "context", ctx)
	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Restart %s/%s", kind, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.RestartResource(ctx, ns, name, kind)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Restarting %s", name)}
	})
}

func (m Model) rollbackDeployment(revision int64) tea.Cmd {
	kctx := m.nav.Context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	client := m.client

	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, fmt.Sprintf("Rollback Deployment: %s@%d", name, revision), bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		err := client.RollbackDeployment(ctx, kctx, ns, name, revision)
		return rollbackDoneMsg{err: err}
	})
}

// toggleNodeSchedulable cordons or uncordons the node by flipping
// spec.unschedulable via the API (equivalent to kubectl cordon/uncordon).
func (m Model) toggleNodeSchedulable() tea.Cmd {
	kctx := m.actionCtx.context
	name := m.actionCtx.name
	client := m.client
	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Toggle node scheduling: "+name, bgtaskTarget(kctx, ""), func(ctx context.Context) tea.Msg {
		cordoned, err := client.ToggleNodeSchedulable(ctx, kctx, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		verb := "Uncordoned"
		if cordoned {
			verb = "Cordoned"
		}
		return actionResultMsg{message: fmt.Sprintf("%s %s", verb, name)}
	})
}

func (m Model) execKubectlDrain() tea.Cmd {
	return m.drainNodeCmd(m.actionCtx.name)
}

// drainNodeCmd runs `kubectl drain <nodeName>` against the action context's
// cluster. In PTY mode it streams the drain progress into lfk's embedded
// terminal so the eviction log stays visible and scrollable in-app instead of
// scrolling past during a host-terminal takeover; Exec mode keeps the legacy
// hand-over. Shared by the Node "Drain" action and the Karpenter
// "Drain Node" action (which resolves the bound node first).
func (m Model) drainNodeCmd(nodeName string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	kctx := m.actionCtx.context
	args := []string{
		"drain", nodeName, "--context", m.kubectlContext(kctx),
		"--ignore-daemonsets", "--delete-emptydir-data",
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(kctx))
	logExecCmd("Running kubectl command", cmd)

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		cols, rows := m.embeddedPTYSize()
		return startPTYExecCmd(cmd, fmtPTYTitle("kubectl drain "+nodeName), cols, rows)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl drain failed", "node", nodeName, "context", kctx, "error", err)
			return actionResultMsg{err: fmt.Errorf("drain %s: %w", nodeName, err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Drained %s", nodeName)}
	})
}

func (m Model) triggerCronJob() tea.Cmd {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	client := m.client

	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Trigger CronJob: "+name, bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		jobName, err := client.TriggerCronJob(ctx, kctx, ns, name)
		return triggerCronJobMsg{jobName: jobName, err: err}
	})
}

func (m Model) toggleCronJobSuspend() tea.Cmd {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	client := m.client

	return m.scheduleK8sCall(scheduler.PriorityCritical, scheduler.KindMutation, "Toggle CronJob suspend: "+name, bgtaskTarget(kctx, ns), func(ctx context.Context) tea.Msg {
		suspended, err := client.ToggleCronJobSuspend(ctx, kctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		verb := "Resumed"
		if suspended {
			verb = "Suspended"
		}
		return actionResultMsg{message: fmt.Sprintf("%s CronJob %s", verb, name)}
	})
}

func (m Model) execCustomAction(expandedCmd string) tea.Cmd {
	ctx := m.actionCtx.context
	if ctx == "" {
		ctx = m.nav.Context
	}
	cmd := exec.Command("sh", "-c", expandedCmd)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
	logExecCmd("Running custom action", cmd)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("Custom action failed", "context", ctx, "error", err)
			return actionResultMsg{err: fmt.Errorf("custom action failed: %w", err)}
		}
		return actionResultMsg{message: "Custom action completed"}
	})
}
