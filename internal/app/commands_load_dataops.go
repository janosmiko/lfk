package app

import (
	"context"
	"maps"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
)

// loadSecretData fetches secret data for the secret editor.
func (m Model) loadSecretData() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	client := m.client

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceList,
		"Secret data: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			data, err := client.GetSecretData(ctx, kctx, ns, name)
			return secretDataLoadedMsg{data: data, err: err}
		},
	)
}

// saveSecretData saves the modified secret data back to the cluster.
func (m Model) saveSecretData() tea.Cmd {
	if m.secretData == nil {
		return nil
	}

	ctx := m.nav.Context
	ns := m.resolveNamespace()
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	data := make(map[string]string, len(m.secretData.Data))
	maps.Copy(data, m.secretData.Data)
	client := m.client

	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindMutation,
		"Save secret: "+name,
		bgtaskTarget(ctx, ns),
		func(_ context.Context) tea.Msg {
			err := client.UpdateSecretData(ctx, ns, name, data)
			return secretSavedMsg{err: err}
		},
	)
}

// loadConfigMapData fetches configmap data for the configmap editor.
func (m Model) loadConfigMapData() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	client := m.client

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceList,
		"ConfigMap data: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			data, err := client.GetConfigMapData(ctx, kctx, ns, name)
			return configMapDataLoadedMsg{data: data, err: err}
		},
	)
}

// saveConfigMapData saves the modified configmap data back to the cluster.
func (m Model) saveConfigMapData() tea.Cmd {
	if m.configMapData == nil {
		return nil
	}

	ctx := m.nav.Context
	ns := m.resolveNamespace()
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	data := make(map[string]string, len(m.configMapData.Data))
	maps.Copy(data, m.configMapData.Data)
	client := m.client

	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindMutation,
		"Save configmap: "+name,
		bgtaskTarget(ctx, ns),
		func(_ context.Context) tea.Msg {
			err := client.UpdateConfigMapData(ctx, ns, name, data)
			return configMapSavedMsg{err: err}
		},
	)
}

// loadLabelData fetches labels and annotations for the selected resource.
func (m Model) loadLabelData() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	rt := m.labelResourceType
	client := m.client

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceList,
		"Labels: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			data, err := client.GetLabelAnnotationData(ctx, kctx, rt, ns, name)
			return labelDataLoadedMsg{data: data, err: err}
		},
	)
}

// saveLabelData saves modified labels and annotations.
func (m Model) saveLabelData() tea.Cmd {
	if m.labelData == nil {
		return nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	rt := m.labelResourceType
	labels := make(map[string]string, len(m.labelData.Labels))
	maps.Copy(labels, m.labelData.Labels)
	annotations := make(map[string]string, len(m.labelData.Annotations))
	maps.Copy(annotations, m.labelData.Annotations)
	client := m.client

	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindMutation,
		"Save labels: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			err := client.UpdateLabelAnnotationData(ctx, kctx, rt, ns, name, labels, annotations)
			return labelSavedMsg{err: err}
		},
	)
}
