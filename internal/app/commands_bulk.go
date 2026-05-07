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
	"github.com/janosmiko/lfk/internal/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (m Model) bulkDeleteResources() tea.Cmd {
	refs := expandGroupedItems(m.bulkItems)
	actionCtx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()
	registry := m.scheduler
	total := len(refs)

	ctx, cancel := context.WithCancel(context.Background())
	taskName := fmt.Sprintf("Delete %s (%d)", rt.Resource, total)
	id := registry.StartCancellable(scheduler.KindMutation, taskName, bgtaskTarget(actionCtx, ns), cancel)

	return func() tea.Msg {
		defer registry.Finish(id)
		var succeeded, failed int
		var errors []string
		for i, ref := range refs {
			if ctx.Err() != nil {
				break
			}
			itemNs := ns
			if ref.Namespace != "" {
				itemNs = ref.Namespace
			}
			// In union mode each row carries its source cluster on
			// ref.ClusterName; route per-item so a mixed-cluster selection
			// hits the right apiserver for each row instead of fanning the
			// whole list at the first item's cluster. ClusterName is empty
			// in normal mode so this falls back to actionCtx.
			itemCtx := actionCtx
			if ref.ClusterName != "" {
				itemCtx = ref.ClusterName
			}
			logger.Info("Bulk deleting", "resource", rt.Resource, "name", ref.Name, "namespace", itemNs, "context", itemCtx)
			err := client.DeleteResource(itemCtx, itemNs, rt, ref.Name)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", ref.Name, err.Error()))
			} else {
				succeeded++
			}
			registry.UpdateProgress(id, i+1, total)
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

// expandGroupedItems flattens bulk items so that grouped rows (with
// GroupedRefs) are expanded into one entry per underlying resource.
// Non-grouped items pass through unchanged.
//
// ClusterName is carried through for union-mode routing. Event grouping stores
// each underlying ref's ClusterName; the parent item is only a fallback for
// older cached grouped rows or callers that build grouped refs manually.
func expandGroupedItems(items []model.Item) []model.GroupedRef {
	refs := make([]model.GroupedRef, 0, len(items))
	for _, item := range items {
		if len(item.GroupedRefs) > 0 {
			for _, ref := range item.GroupedRefs {
				if ref.ClusterName == "" {
					ref.ClusterName = item.ClusterName
				}
				refs = append(refs, ref)
			}
		} else {
			refs = append(refs, model.GroupedRef{
				Name:        item.Name,
				Namespace:   item.Namespace,
				ClusterName: item.ClusterName,
			})
		}
	}
	return refs
}

func (m Model) bulkForceDeleteResources() tea.Cmd {
	refs := expandGroupedItems(m.bulkItems)
	actionCtx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()
	registry := m.scheduler
	total := len(refs)

	ctx, cancel := context.WithCancel(context.Background())
	taskName := fmt.Sprintf("Force delete %s (%d)", rt.Resource, total)
	id := registry.StartCancellable(scheduler.KindMutation, taskName, bgtaskTarget(actionCtx, ns), cancel)

	return func() tea.Msg {
		defer registry.Finish(id)

		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			return bulkActionResultMsg{failed: total, errors: []string{"kubectl not found"}}
		}

		var succeeded, failed int
		var errors []string
		for i, ref := range refs {
			if ctx.Err() != nil {
				break
			}
			itemNs := ns
			if ref.Namespace != "" {
				itemNs = ref.Namespace
			}
			// Per-item cluster routing for union mode (see bulkDeleteResources).
			itemCtx := actionCtx
			if ref.ClusterName != "" {
				itemCtx = ref.ClusterName
			}
			logger.Info("Bulk force deleting", "resource", rt.Resource, "name", ref.Name, "namespace", itemNs, "context", itemCtx)

			// Resolve KUBECONFIG to just the file that defines itemCtx so
			// that overlapping cluster/user names across kubeconfigs do not
			// route this command to the wrong cluster — see issue #23. Also
			// translate the lfk display name back to the literal kubeconfig
			// context name for the --context flag.
			kubeconfigPath := client.KubeconfigPathForContext(itemCtx)
			kubectlCtx := client.OriginalContextName(itemCtx)

			// Remove finalizers first.
			patchArgs := []string{
				"patch", rt.Resource, ref.Name, "--context", kubectlCtx,
				"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`,
			}
			if rt.Namespaced {
				patchArgs = append(patchArgs, "-n", itemNs)
			}
			patchCmd := exec.CommandContext(ctx, kubectlPath, patchArgs...)
			patchCmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
			logExecCmd("Running kubectl command", patchCmd)
			patchCmd.Run() //nolint:errcheck

			// Force delete.
			deleteArgs := []string{
				"delete", rt.Resource, ref.Name, "--context", kubectlCtx,
				"--grace-period=0", "--force",
			}
			if rt.Namespaced {
				deleteArgs = append(deleteArgs, "-n", itemNs)
			}
			cmd := exec.CommandContext(ctx, kubectlPath, deleteArgs...)
			cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
			logExecCmd("Running kubectl command", cmd)
			if _, err := cmd.CombinedOutput(); err != nil {
				logger.Error("kubectl bulk force delete failed", "resource", rt.Resource, "name", ref.Name, "namespace", itemNs, "context", itemCtx, "error", err)
				failed++
				msg := strings.TrimSpace(err.Error())
				if msg == "" {
					msg = "force delete failed"
				}
				errors = append(errors, fmt.Sprintf("%s: %s", ref.Name, msg))
			} else {
				succeeded++
			}
			registry.UpdateProgress(id, i+1, total)
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkScaleResources(replicas int32) tea.Cmd {
	items := m.bulkItems
	actionCtx := m.actionCtx.context
	kind := m.actionCtx.kind
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()
	registry := m.scheduler
	total := len(items)

	ctx, cancel := context.WithCancel(context.Background())
	taskName := fmt.Sprintf("Scale %s (%d)", rt.Resource, total)
	id := registry.StartCancellable(scheduler.KindMutation, taskName, bgtaskTarget(actionCtx, ns), cancel)

	return func() tea.Msg {
		defer registry.Finish(id)
		var succeeded, failed int
		var errors []string
		for i, item := range items {
			if ctx.Err() != nil {
				break
			}
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk scaling", "name", item.Name, "replicas", replicas, "namespace", itemNs)
			err := client.ScaleResource(actionCtx, itemNs, item.Name, kind, replicas)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
			registry.UpdateProgress(id, i+1, total)
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkRestartResources() tea.Cmd {
	items := m.bulkItems
	actionCtx := m.actionCtx.context
	kind := m.actionCtx.kind
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()
	registry := m.scheduler
	total := len(items)

	ctx, cancel := context.WithCancel(context.Background())
	taskName := fmt.Sprintf("Restart %s (%d)", rt.Resource, total)
	id := registry.StartCancellable(scheduler.KindMutation, taskName, bgtaskTarget(actionCtx, ns), cancel)

	return func() tea.Msg {
		defer registry.Finish(id)
		var succeeded, failed int
		var errors []string
		for i, item := range items {
			if ctx.Err() != nil {
				break
			}
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			// Per-item cluster routing for union mode (see bulkDeleteResources).
			itemCtx := actionCtx
			if item.ClusterName != "" {
				itemCtx = item.ClusterName
			}
			logger.Info("Bulk restarting", "name", item.Name, "namespace", itemNs, "context", itemCtx)
			err := client.RestartResource(itemCtx, itemNs, item.Name, kind)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
			registry.UpdateProgress(id, i+1, total)
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) batchPatchLabels(key, value string, remove bool, isAnnotation bool) tea.Cmd {
	items := m.bulkItems
	actionCtx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	client := m.client
	ns := m.actionNamespace()
	registry := m.scheduler
	total := len(items)

	labelOrAnnotation := "labels"
	if isAnnotation {
		labelOrAnnotation = "annotations"
	}

	ctx, cancel := context.WithCancel(context.Background())
	taskName := fmt.Sprintf("Patch %s (%d)", labelOrAnnotation, total)
	id := registry.StartCancellable(scheduler.KindMutation, taskName, bgtaskTarget(actionCtx, ns), cancel)

	return func() tea.Msg {
		defer registry.Finish(id)
		var succeeded, failed int
		var errors []string
		for i, item := range items {
			if ctx.Err() != nil {
				break
			}
			var patch map[string]any
			if remove {
				patch = map[string]any{key: nil}
			} else {
				patch = map[string]any{key: value}
			}
			itemNs := item.Namespace
			if itemNs == "" {
				itemNs = ns
			}
			// Cluster-scoped resources must be patched at cluster scope —
			// supplying a namespace here triggers a "namespace specified for
			// non-namespaced resource" error from the apiserver.
			if !rt.Namespaced {
				itemNs = ""
			}
			var err error
			if isAnnotation {
				err = client.PatchAnnotations(ctx, actionCtx, itemNs, item.Name, gvr, patch)
			} else {
				err = client.PatchLabels(ctx, actionCtx, itemNs, item.Name, gvr, patch)
			}
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %v", item.Name, err))
			} else {
				succeeded++
			}
			registry.UpdateProgress(id, i+1, total)
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}
