package app

import (
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

const actionDeleteWithVolume = "Delete with volume"

// A direct PV delete leaves the cloud volume orphaned on csi-provisioner < v4.
// The provisioner only cleans up through the Released + Delete path.
const pvReclaimDeletePatch = `{"spec":{"persistentVolumeReclaimPolicy":"Delete"}}`

// releasedCSIVolume returns the CSI driver and volume handle of a Released
// CSI PersistentVolume. ok is false for any other phase or a non-CSI PV.
func releasedCSIVolume(raw map[string]any) (driver, handle string, ok bool) {
	if phase, _, _ := unstructured.NestedString(raw, "status", "phase"); phase != "Released" {
		return "", "", false
	}
	csi, found, err := unstructured.NestedMap(raw, "spec", "csi")
	if err != nil || !found {
		return "", "", false
	}
	driver, _ = csi["driver"].(string)
	handle, _ = csi["volumeHandle"].(string)
	return driver, handle, true
}

// appendDeleteWithVolume adds the menu entry for a Released CSI PersistentVolume.
func appendDeleteWithVolume(actions []model.ActionMenuItem, kind string, raw map[string]any) []model.ActionMenuItem {
	if _, _, ok := releasedCSIVolume(raw); !ok || kind != "PersistentVolume" {
		return actions
	}
	return append(actions, model.ActionMenuItem{
		Label:       actionDeleteWithVolume,
		Description: "Let the provisioner delete the cloud volume and this PV",
		Key:         "W",
	})
}

// executeActionDeleteWithVolume handles the "Delete with volume" action.
func (m Model) executeActionDeleteWithVolume() (tea.Model, tea.Cmd) { //nolint:unparam // consistent action handler signature
	driver, handle, ok := releasedCSIVolume(m.actionCtx.raw)
	if !ok {
		m.setStatusMessage("Delete with volume needs a Released CSI PersistentVolume", true)
		return m, scheduleStatusClear()
	}
	m.confirmAction = m.actionCtx.name
	m.confirmTitle = "Confirm Delete with volume"
	m.confirmQuestion = fmt.Sprintf("The cloud volume will be deleted permanently (driver %s, volumeHandle %s). Delete %s with its volume?",
		ui.SanitizeTerminalText(driver), ui.SanitizeTerminalText(handle), ui.SanitizeTerminalText(m.actionCtx.name))
	m.confirmTypeInput.Clear()
	m.overlay = overlayConfirmType
	m.pendingAction = actionDeleteWithVolume
	return m, nil
}

func pvReclaimPatchArgs(rt model.ResourceTypeEntry, name, kubectlCtx string) []string {
	return []string{
		"patch", kubectlResourceArg(rt), name, "--context", kubectlCtx,
		"--type", "merge", "-p", pvReclaimDeletePatch,
	}
}

// setPVReclaimDelete patches the PV reclaim policy to Delete. It never
// deletes the PV object itself.
func (m Model) setPVReclaimDelete() tea.Cmd {
	kubectlPath, err := k8s.KubectlPath()
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	patchArgs := pvReclaimPatchArgs(rt, name, m.kubectlContext(ctx))
	return m.trackBgTask(scheduler.KindMutation, fmt.Sprintf("Delete with volume: %s", name), bgtaskTarget(ctx, ""), func() tea.Msg {
		cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(patchArgs)...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
		logExecCmd("Running kubectl command", cmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl patch failed", "resource", rt.Resource, "name", name, "context", ctx, "error", err)
			return actionResultMsg{err: logger.RedactErr(err, output)}
		}
		return actionResultMsg{message: fmt.Sprintf("Reclaim policy set to Delete: the provisioner will delete the volume and PV %s", name)}
	})
}
