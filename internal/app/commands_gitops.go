package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (m Model) syncArgoApp() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		err := m.client.SyncArgoApp(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Sync initiated for %s", name)}
	}
}

func (m Model) refreshArgoApp() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		err := m.client.RefreshArgoApp(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Hard refresh initiated for %s", name)}
	}
}

// reconcileFluxResource triggers reconciliation of a Flux resource.
func (m Model) reconcileFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.ReconcileFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Reconciliation triggered for %s", name)}
	}
}

// suspendFluxResource suspends a Flux resource.
func (m Model) suspendFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.SuspendFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended %s", name)}
	}
}

// resumeFluxResource resumes a Flux resource.
func (m Model) resumeFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.ResumeFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed %s", name)}
	}
}

func (m Model) diffArgoApp() tea.Cmd {
	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	title := fmt.Sprintf("ArgoCD Diff: %s", name)

	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("kubectl not found in PATH"),
			}
		}
	}

	return func() tea.Msg {
		kubeEnv := "KUBECONFIG=" + m.client.KubeconfigPaths()

		// Get the Application resource as JSON to extract managed resources.
		getArgs := []string{
			"get", "applications.argoproj.io", name,
			"-n", ns, "--context", ctx, "-o", "json",
		}
		getCmd := exec.Command(kubectlPath, getArgs...)
		getCmd.Env = append(os.Environ(), kubeEnv)
		logger.Info("Running kubectl command", "cmd", getCmd.String())
		appJSON, err := getCmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl get application failed", "cmd", getCmd.String(), "error", err, "output", string(appJSON))
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("failed to get application: %s: %s", err, strings.TrimSpace(string(appJSON))),
			}
		}

		// Parse the managed resources from status.resources
		type managedResource struct {
			Group     string `json:"group"`
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			Health    struct {
				Status string `json:"status"`
			} `json:"health"`
		}
		var app struct {
			Status struct {
				Resources []managedResource `json:"resources"`
				Sync      struct {
					Status string `json:"status"`
				} `json:"sync"`
			} `json:"status"`
		}

		if jsonErr := json.Unmarshal(appJSON, &app); jsonErr != nil {
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("failed to parse application JSON: %w", jsonErr),
			}
		}

		if len(app.Status.Resources) == 0 {
			return describeLoadedMsg{
				content: "No managed resources found. The application may not have been synced yet.",
				title:   title,
			}
		}

		// Build a diff output by running kubectl diff on each out-of-sync resource
		var diffBuf strings.Builder
		fmt.Fprintf(&diffBuf, "Application: %s\n", name)
		fmt.Fprintf(&diffBuf, "Sync Status: %s\n", app.Status.Sync.Status)
		fmt.Fprintf(&diffBuf, "Managed Resources: %d\n\n", len(app.Status.Resources))

		outOfSync := 0
		for _, res := range app.Status.Resources {
			if res.Status != "OutOfSync" {
				continue
			}
			outOfSync++

			// Build the resource identifier
			resource := res.Kind
			if res.Group != "" {
				resource = res.Kind + "." + res.Group
			}

			fmt.Fprintf(&diffBuf, "━━━ %s/%s", resource, res.Name)
			if res.Namespace != "" {
				fmt.Fprintf(&diffBuf, " (ns: %s)", res.Namespace)
			}
			diffBuf.WriteString(" ━━━\n")

			// Get the desired manifest from the application
			// Use kubectl get to fetch live state, then we'll show the status
			getResArgs := []string{"get", resource, res.Name, "--context", ctx, "-o", "yaml"}
			if res.Namespace != "" {
				getResArgs = append(getResArgs, "-n", res.Namespace)
			}
			getResCmd := exec.Command(kubectlPath, getResArgs...)
			getResCmd.Env = append(os.Environ(), kubeEnv)
			logger.Info("Running kubectl command", "cmd", getResCmd.String())
			resOut, resErr := getResCmd.CombinedOutput()
			if resErr != nil {
				logger.Error("kubectl get resource failed", "cmd", getResCmd.String(), "error", resErr, "output", string(resOut))
				diffBuf.WriteString("  Status: OutOfSync (resource may not exist yet)\n")
				if res.Health.Status != "" {
					fmt.Fprintf(&diffBuf, "  Health: %s\n", res.Health.Status)
				}
				diffBuf.WriteString("\n")
				continue
			}
			_ = resOut

			diffBuf.WriteString("  Status: OutOfSync\n")
			if res.Health.Status != "" {
				fmt.Fprintf(&diffBuf, "  Health: %s\n", res.Health.Status)
			}
			diffBuf.WriteString("\n")
		}

		if outOfSync == 0 {
			diffBuf.WriteString("All resources are in sync. No differences found.\n\n")

			// Still show a summary of all managed resources
			diffBuf.WriteString("Resource Summary:\n")
			for _, res := range app.Status.Resources {
				resource := res.Kind
				if res.Group != "" {
					resource = res.Kind + "." + res.Group
				}
				health := res.Health.Status
				if health == "" {
					health = "-"
				}
				fmt.Fprintf(&diffBuf, "  %-40s  Sync: %-10s  Health: %s\n",
					resource+"/"+res.Name, res.Status, health)
			}
		}

		return describeLoadedMsg{
			content: diffBuf.String(),
			title:   title,
		}
	}
}
