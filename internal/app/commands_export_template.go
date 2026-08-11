// Package app — commands_export_template.go
// Fetches the selected resource's manifest and strips it to a template. The
// result opens the destination picker; nothing is written to the cluster, so
// the action stays available in read-only mode.
package app

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// exportTemplateCmd builds the fetch-and-strip command for the row under the
// cursor. Returns nil when the current level has no manifest behind it.
func (m Model) exportTemplateCmd() tea.Cmd {
	// Synthetic security rows have no manifest.
	if onSecurityView(&m) {
		return nil
	}
	fetch, name, kind, ok := m.resolveTemplateSource()
	if !ok {
		return nil
	}
	kctx := m.effectiveContext()
	return m.scheduleK8sCall(scheduler.PriorityHigh, scheduler.KindYAMLFetch,
		"Export template: "+name, bgtaskTarget(kctx, m.resolveNamespace()),
		func(ctx context.Context) tea.Msg {
			doc, err := fetch(ctx)
			if err != nil {
				return exportTemplateReadyMsg{err: fmt.Errorf("fetching resource: %w", err)}
			}
			manifest, err := k8s.StripToTemplate(doc)
			if err != nil {
				return exportTemplateReadyMsg{err: fmt.Errorf("stripping manifest: %w", err)}
			}
			return exportTemplateReadyMsg{name: name, kind: kind, manifest: manifest}
		})
}

// resolveTemplateSource returns a YAML fetcher for the row under the cursor,
// plus its name and kind. A container row resolves to its parent Pod: a
// container has no manifest of its own.
func (m Model) resolveTemplateSource() (fetch func(context.Context) (string, error), name, kind string, ok bool) {
	kctx := m.effectiveContext()
	ns := m.resolveNamespace()

	if m.nav.Level == model.LevelContainers {
		podName := m.nav.OwnedName
		if podName == "" {
			return nil, "", "", false
		}
		return func(ctx context.Context) (string, error) {
			return m.client.GetPodYAML(ctx, kctx, ns, podName)
		}, podName, "Pod", true
	}

	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil, "", "", false
	}
	itemNs := ns
	if sel.Namespace != "" {
		itemNs = sel.Namespace
	}
	itemCtx := kctx
	if sel.ClusterName != "" {
		itemCtx = sel.ClusterName
	}

	switch m.nav.Level {
	case model.LevelResources:
		rt := m.nav.ResourceType
		return func(ctx context.Context) (string, error) {
			return m.client.GetResourceYAML(ctx, itemCtx, itemNs, rt, sel.Name)
		}, sel.Name, rt.Kind, true
	case model.LevelOwned:
		if sel.Kind == "Pod" {
			return func(ctx context.Context) (string, error) {
				return m.client.GetPodYAML(ctx, itemCtx, itemNs, sel.Name)
			}, sel.Name, "Pod", true
		}
		rt, resolved := m.resolveOwnedResourceType(sel)
		if !resolved {
			return nil, "", "", false
		}
		return func(ctx context.Context) (string, error) {
			return m.client.GetResourceYAML(ctx, itemCtx, itemNs, rt, sel.Name)
		}, sel.Name, rt.Kind, true
	}
	return nil, "", "", false
}

// updateExportTemplateReady opens the destination picker once the manifest has
// been fetched and stripped.
func (m Model) updateExportTemplateReady(msg exportTemplateReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Export template failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.openExportTemplatePicker(msg.name, msg.kind, msg.manifest)
	return m, nil
}
