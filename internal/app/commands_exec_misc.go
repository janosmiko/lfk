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
)

func (m Model) deleteResource() tea.Cmd {
	if m.actionCtx.resourceType.APIGroup == "_helm" {
		return m.uninstallHelmRelease()
	}

	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	logger.Info("Deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)
	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Delete %s/%s", rt.Resource, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.DeleteResource(ctx, ns, rt, name)
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
	logger.Info("Force deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)

	deleteArgs := []string{
		"delete", rt.Resource, name, "--context", m.kubectlContext(ctx),
		"--grace-period=0", "--force",
	}
	if rt.Namespaced {
		deleteArgs = append(deleteArgs, "-n", ns)
	}

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
		"patch", rt.Resource, name, "--context", m.kubectlContext(ctx),
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
	return m.trackBgTask(scheduler.KindMutation, "Resize PVC: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
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
	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Scale %s/%s → %d", kind, name, replicas), bgtaskTarget(ctx, ns), func() tea.Msg {
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

	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Rollback Deployment: %s@%d", name, revision), bgtaskTarget(kctx, ns), func() tea.Msg {
		err := client.RollbackDeployment(context.Background(), kctx, ns, name, revision)
		return rollbackDoneMsg{err: err}
	})
}

func (m Model) execKubectlCordon() tea.Cmd {
	return m.execKubectlNodeCmd("cordon")
}

func (m Model) execKubectlUncordon() tea.Cmd {
	return m.execKubectlNodeCmd("uncordon")
}

func (m Model) execKubectlDrain() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	name := m.actionCtx.name
	args := []string{
		"drain", name, "--context", m.kubectlContext(m.actionCtx.context),
		"--ignore-daemonsets", "--delete-emptydir-data",
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
	logExecCmd("Running kubectl command", cmd)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl drain failed", "node", name, "context", m.actionCtx.context, "error", err)
			return actionResultMsg{err: fmt.Errorf("drain %s: %w", name, err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Drained %s", name)}
	})
}

func (m Model) execKubectlNodeCmd(subcmd string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	name := m.actionCtx.name
	args := []string{subcmd, name, "--context", m.kubectlContext(m.actionCtx.context)}

	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("%s node: %s", subcmd, name), m.actionCtx.context, func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
		logExecCmd("Running kubectl command", cmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl node command failed", "subcmd", subcmd, "name", name, "context", m.actionCtx.context, "error", err)
			return actionResultMsg{err: fmt.Errorf("%s %s: %s", subcmd, name, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: strings.TrimSpace(string(output))}
	})
}

func (m Model) triggerCronJob() tea.Cmd {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	client := m.client

	return m.trackBgTask(scheduler.KindMutation, "Trigger CronJob: "+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		jobName, err := client.TriggerCronJob(context.Background(), kctx, ns, name)
		return triggerCronJobMsg{jobName: jobName, err: err}
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
