package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// loadPreview loads the right column based on the current level and selection.
func (m Model) loadPreview() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	switch m.nav.Level {
	case model.LevelClusters:
		return m.loadPreviewClusters(sel)
	case model.LevelResourceTypes:
		return m.loadPreviewResourceTypes(sel)
	case model.LevelResources:
		return m.loadPreviewResources()
	case model.LevelOwned:
		return m.loadPreviewOwned(sel)
	case model.LevelContainers:
		return nil
	}
	return nil
}

// loadPreviewClusters handles preview loading at the cluster list.
//
// The right pane shows the resource types for the *hovered* context, not the
// currently-active context (m.nav.Context is empty here after back-nav from
// LevelResourceTypes). Behavior:
//
//   - Cached (discovery already completed for hoveredCtx): emit the real
//     resource-type list so the right pane updates on cursor move.
//   - Uncached: emit an empty list to clear any stale items from a
//     previously-hovered context, and kick off discovery (unless one is
//     already in flight). renderRightClusters will render the loader
//     because rightItems is empty and m.discoveringContexts[hoveredCtx]
//     is true. Once apiResourceDiscoveryMsg arrives,
//     updateAPIResourceDiscovery replaces rightItems with the discovered
//     list.
func (m Model) loadPreviewClusters(sel *model.Item) tea.Cmd {
	hoveredCtx := sel.Name
	if hoveredCtx == "" {
		return m.loadResourceTypes()
	}
	if discovered := m.discoveredResources[hoveredCtx]; len(discovered) > 0 {
		items := model.BuildSidebarItems(discovered)
		// If the discovered slice came from the disk cache (prefilled at
		// startup) and discovery hasn't run live yet this session, we
		// still want to kick one off behind the cached view — the user
		// gets instant paint plus an asynchronous refresh.
		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			return resourceTypesMsg{items: items}
		})
		if m.shouldFireDiscoveryFor(hoveredCtx) {
			m.markDiscoveryStarted(hoveredCtx)
			cmds = append(cmds, m.discoverAPIResources(hoveredCtx))
		}
		return tea.Batch(cmds...)
	}
	// Clear rightItems so renderRightClusters falls into its loader branch.
	cmds := []tea.Cmd{func() tea.Msg {
		return resourceTypesMsg{items: nil}
	}}
	if m.shouldFireDiscoveryFor(hoveredCtx) {
		m.markDiscoveryStarted(hoveredCtx)
		cmds = append(cmds, m.discoverAPIResources(hoveredCtx))
	}
	return tea.Batch(cmds...)
}

// loadPreviewResourceTypes handles preview loading at the resource types level.
func (m Model) loadPreviewResourceTypes(sel *model.Item) tea.Cmd {
	if sel.Extra == "__overview__" {
		if ui.ConfigDashboard {
			return m.loadDashboard()
		}
		return nil
	}
	if sel.Extra == "__monitoring__" {
		return m.loadMonitoringDashboard()
	}
	if sel.Kind == "__collapsed_group__" {
		return nil
	}
	if sel.Kind == "__port_forwards__" {
		items := m.portForwardItems()
		gen := m.requestGen
		return func() tea.Msg {
			return resourcesLoadedMsg{items: items, forPreview: true, gen: gen}
		}
	}
	// Virtual security source entries are not in discoveredResources, so
	// loadResources(true) cannot resolve them via FindResourceTypeIn.
	// Synthesize the RT here (same logic as navigateChildResourceType) so
	// the preview pane shows findings when the cursor rests on a source.
	if strings.HasPrefix(sel.Kind, "__security_") && sel.Kind != "__security_finding__" {
		sourceName := strings.TrimSuffix(strings.TrimPrefix(sel.Kind, "__security_"), "__")
		kctx := m.nav.Context
		ns := m.effectiveNamespace()
		gen := m.requestGen
		rt := model.ResourceTypeEntry{
			DisplayName: sel.Name,
			Kind:        sel.Kind,
			APIGroup:    model.SecurityVirtualAPIGroup,
			APIVersion:  "v1",
			Resource:    "findings-" + sourceName,
			Namespaced:  true,
		}
		return m.trackBgTask(
			bgtasks.KindResourceList,
			"List "+sel.Name,
			bgtaskTarget(kctx, ns),
			func() tea.Msg {
				items, err := m.client.GetResources(m.reqCtx, kctx, ns, rt)
				return resourcesLoadedMsg{items: items, err: err, forPreview: true, gen: gen}
			},
		)
	}
	return m.loadResources(true)
}

// loadPreviewResources handles preview loading at the resources level.
func (m Model) loadPreviewResources() tea.Cmd {
	if m.nav.ResourceType.Kind == "__port_forwards__" {
		return nil
	}
	// Security finding groups: preview shows affected resources.
	if sel := m.selectedMiddleItem(); sel != nil && sel.Kind == "__security_finding_group__" {
		return m.loadSecurityAffectedResourcesPreview(sel.Extra)
	}
	var cmds []tea.Cmd
	switch {
	case m.mapView && m.resourceTypeHasChildren():
		cmds = append(cmds, m.loadResourceTree())
	case m.resourceTypeHasChildren():
		cmds = append(cmds, m.loadOwned(true))
	case m.nav.ResourceType.Kind == "Pod":
		cmds = append(cmds, m.loadContainers(true))
	}
	if m.fullYAMLPreview {
		cmds = append(cmds, m.loadPreviewYAML())
	}
	kind := m.nav.ResourceType.Kind
	if kind == "Pod" || kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
		if metricsCmd := m.loadMetrics(); metricsCmd != nil {
			cmds = append(cmds, metricsCmd)
		}
	}
	if eventsCmd := m.loadPreviewEvents(); eventsCmd != nil {
		cmds = append(cmds, eventsCmd)
	}
	// loadPreviewSecretData is itself gated on kind and the lazy-loading
	// config flag; call it unconditionally and let it no-op when not
	// applicable. Keeping the gate centralised there makes the contract
	// testable without reaching into tea.Batch internals here.
	if secretCmd := m.loadPreviewSecretData(); secretCmd != nil {
		cmds = append(cmds, secretCmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// loadPreviewOwned handles preview loading at the owned level.
func (m Model) loadPreviewOwned(sel *model.Item) tea.Cmd {
	// Security affected resources are virtual — no YAML to preview.
	if strings.HasPrefix(sel.Kind, "__security_") {
		return nil
	}
	if sel.Kind == "Pod" {
		var cmds []tea.Cmd
		cmds = append(cmds, m.loadContainers(true))
		if m.fullYAMLPreview {
			cmds = append(cmds, m.loadPreviewYAML())
		}
		if metricsCmd := m.loadMetrics(); metricsCmd != nil {
			cmds = append(cmds, metricsCmd)
		}
		if eventsCmd := m.loadPreviewEvents(); eventsCmd != nil {
			cmds = append(cmds, eventsCmd)
		}
		return tea.Batch(cmds...)
	}
	// PVC is listed in kindHasOwnedChildren so the right-pane preview at
	// LevelResources can lazily show which pods use it, but at LevelOwned
	// the existing UX is to show the PVC's YAML (e.g., user is drilled
	// into a Helm release's children and hovers a PVC). Preserve that by
	// letting PVC fall through to the YAML path here even though it
	// reports "has children".
	if kindHasOwnedChildren(sel.Kind) && sel.Kind != "PersistentVolumeClaim" {
		return nil
	}
	if m.fullYAMLPreview {
		return m.loadPreviewYAML()
	}
	name := sel.Name
	kctx := m.nav.Context
	// Fall back to nav.Namespace (set when drilling into a helm release or
	// argocd application) so children without a metadata.namespace — common
	// for helm manifests that rely on --namespace rather than templating
	// .Release.Namespace — are fetched from the parent's namespace instead
	// of the ambient namespace filter.
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	reqCtx := m.reqCtx
	rt, ok := m.resolveOwnedResourceType(sel)
	if !ok {
		return func() tea.Msg {
			return buildYAMLLoadedMsg("", fmt.Errorf("unknown resource type: %s", sel.Kind))
		}
	}
	return m.trackBgTask(
		bgtasks.KindYAMLFetch,
		"YAML: "+name,
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			content, err := m.client.GetResourceYAML(reqCtx, kctx, ns, rt, name)
			return buildYAMLLoadedMsg(content, err)
		},
	)
}

// loadPreviewYAML loads the YAML for the currently selected middle item into previewYAML.
func (m Model) loadPreviewYAML() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	gen := m.requestGen
	reqCtx := m.reqCtx

	switch m.nav.Level {
	case model.LevelResources:
		rt := m.nav.ResourceType
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		return m.trackBgTask(
			bgtasks.KindYAMLFetch,
			"Preview YAML: "+name,
			bgtaskTarget(kctx, itemNs),
			func() tea.Msg {
				content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
				return buildPreviewYAMLLoadedMsg(content, err, gen)
			},
		)
	case model.LevelOwned:
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		taskTarget := bgtaskTarget(kctx, itemNs)
		if sel.Kind == "Pod" {
			return m.trackBgTask(
				bgtasks.KindYAMLFetch,
				"Preview YAML: "+name,
				taskTarget,
				func() tea.Msg {
					content, err := m.client.GetPodYAML(reqCtx, kctx, itemNs, name)
					return buildPreviewYAMLLoadedMsg(content, err, gen)
				},
			)
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return buildPreviewYAMLLoadedMsg("", fmt.Errorf("unknown resource type: %s", sel.Kind), gen)
			}
		}
		return m.trackBgTask(
			bgtasks.KindYAMLFetch,
			"Preview YAML: "+name,
			taskTarget,
			func() tea.Msg {
				content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
				return buildPreviewYAMLLoadedMsg(content, err, gen)
			},
		)
	}
	return nil
}

// loadEventTimeline fetches events correlated with the current action target resource.
func (m Model) loadEventTimeline() tea.Cmd {
	client := m.client
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	return m.trackBgTask(bgtasks.KindResourceList, "Event timeline: "+kind+"/"+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		events, err := client.GetResourceEvents(context.Background(), ctx, ns, name, kind)
		return eventTimelineMsg{events: events, err: err}
	})
}

func (m Model) checkRBAC() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	rt := m.actionCtx.resourceType
	return m.trackBgTask(bgtasks.KindResourceList, "RBAC check: "+rt.Kind, bgtaskTarget(ctx, ns), func() tea.Msg {
		results, err := m.client.CheckRBAC(context.Background(), ctx, ns, rt.APIGroup, rt.Resource)
		return rbacCheckMsg{results: results, kind: rt.Kind, resource: rt.Resource, err: err}
	})
}

func (m Model) loadCanIRules() tea.Cmd {
	client := m.client
	ctx := m.nav.Context
	ns := m.namespace
	if m.allNamespaces || ns == "" {
		ns = "default"
	}
	subject := m.canISubject

	// When checking a specific SA, discover all namespaces where it has
	// RoleBindings and query permissions across all of them.
	if subject != "" && strings.HasPrefix(subject, "system:serviceaccount:") {
		return m.trackBgTask(bgtasks.KindResourceList, "CanI rules: "+subject, ctx, func() tea.Msg {
			rules, namespaces, err := client.GetSelfRulesMultiNS(context.Background(), ctx, subject)
			return canILoadedMsg{rules: rules, namespaces: namespaces, err: err}
		})
	}

	// User or Group impersonation: query in the current namespace.
	// GetSelfRulesAs handles the "group:" prefix internally.
	if subject != "" {
		viewNS := ns
		return m.trackBgTask(bgtasks.KindResourceList, "CanI rules: "+subject, bgtaskTarget(ctx, viewNS), func() tea.Msg {
			rules, err := client.GetSelfRulesAs(context.Background(), ctx, viewNS, subject)
			return canILoadedMsg{rules: rules, namespaces: []string{viewNS}, err: err}
		})
	}

	// Current user: use the active namespace only.
	return m.trackBgTask(bgtasks.KindResourceList, "CanI rules (current user)", bgtaskTarget(ctx, ns), func() tea.Msg {
		rules, err := client.GetSelfRulesAs(context.Background(), ctx, ns, "")
		return canILoadedMsg{rules: rules, namespaces: []string{ns}, err: err}
	})
}

func (m Model) loadCanISAList() tea.Cmd {
	client := m.client
	ctx := m.nav.Context
	// Always list SAs across all namespaces so the user can check
	// permissions for any service account regardless of the current view.
	// Also discover Users and Groups from RBAC bindings.
	return m.trackBgTask(bgtasks.KindResourceList, "List service accounts", ctx, func() tea.Msg {
		accounts, err := client.ListServiceAccounts(context.Background(), ctx, "")
		if err != nil {
			return canISAListMsg{err: err}
		}
		subjects, _ := client.ListRBACSubjects(context.Background(), ctx)
		return canISAListMsg{accounts: accounts, subjects: subjects}
	})
}

func (m Model) loadPodStartup() tea.Cmd {
	client := m.client
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindResourceList, "Pod startup analysis: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		info, err := client.GetPodStartupAnalysis(context.Background(), ctx, ns, name)
		return podStartupMsg{info: info, err: err}
	})
}

func (m Model) loadAlerts() tea.Cmd {
	kubeCtx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	return m.trackBgTask(bgtasks.KindDashboard, "Alerts: "+kind+"/"+name, bgtaskTarget(kubeCtx, ns), func() tea.Msg {
		alerts, err := m.client.GetActiveAlerts(context.Background(), kubeCtx, ns, name, kind)
		return alertsLoadedMsg{alerts: alerts, err: err}
	})
}

// loadNetworkPolicy fetches and parses a NetworkPolicy for visualization.
func (m Model) loadNetworkPolicy() tea.Cmd {
	client := m.client
	kctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return m.trackBgTask(bgtasks.KindResourceList, "NetworkPolicy: "+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		info, err := client.GetNetworkPolicyInfo(context.Background(), kctx, ns, name)
		return netpolLoadedMsg{info: info, err: err}
	})
}

// loadHelmValues runs `helm get values` and returns the output as a message.
// If allValues is true, the --all flag is included to show computed defaults too.
func (m Model) loadHelmValues(allValues bool) tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmValuesLoadedMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)

	args := []string{"get", "values", name, "-n", ns, "--kube-context", m.kubectlContext(ctx), "-o", "yaml"}
	titleSuffix := "User Values"
	if allValues {
		args = append(args, "--all")
		titleSuffix = "All Values"
	}

	title := fmt.Sprintf("Helm %s: %s", titleSuffix, name)

	return m.trackBgTask(bgtasks.KindSubprocess, title, bgtaskTarget(ctx, ns), func() tea.Msg {
		cmd := exec.Command(helmPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logExecCmd("Running helm command", cmd)
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			// Helm chart values commonly embed secrets/passwords; redact
			// the captured output before persisting it to lfk.log.
			logger.Error("helm get values failed", "cmd", cmd.String(), "error", cmdErr, "output", logger.Redact(string(output)))
			return helmValuesLoadedMsg{
				title: title,
				err:   fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output))),
			}
		}
		content := strings.TrimSpace(string(output))
		if content == "" || content == "null" {
			content = "# No user-supplied values"
		}
		return helmValuesLoadedMsg{
			content: content,
			title:   title,
		}
	})
}

// loadContainerPorts loads the available ports for the action context resource.
func (m Model) loadContainerPorts() tea.Cmd {
	client := m.client
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind

	return m.trackBgTask(bgtasks.KindContainers, "List ports: "+kind+"/"+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		var ports []k8s.ContainerPort
		var err error
		switch kind {
		case "Pod":
			ports, err = client.GetContainerPorts(context.Background(), kctx, ns, name)
		case "Service":
			ports, err = client.GetServicePorts(context.Background(), kctx, ns, name)
		case "Deployment":
			ports, err = client.GetDeploymentPorts(context.Background(), kctx, ns, name)
		case "StatefulSet":
			ports, err = client.GetStatefulSetPorts(context.Background(), kctx, ns, name)
		case "DaemonSet":
			ports, err = client.GetDaemonSetPorts(context.Background(), kctx, ns, name)
		default:
			err = fmt.Errorf("unsupported kind for port discovery: %s", kind)
		}
		return containerPortsLoadedMsg{ports: ports, err: err}
	})
}

// secretPreviewCacheKey returns the cache key for secret preview data.
// Format: "ctx/namespace/name".
func secretPreviewCacheKey(ctx, ns, name string) string {
	return ctx + "/" + ns + "/" + name
}

// secretDataCachedFor reports whether the lazily-fetched data for the given
// item is already in the preview cache. Used by the right-pane renderer to
// distinguish "fetch still in flight" (show spinner) from "fetch completed,
// just no data rows to show" (render the metadata summary anyway) — needed
// because Secret items come from the metadata-only list path and have empty
// Columns until/unless the hover fetch injects data.
func (m Model) secretDataCachedFor(sel *model.Item) bool {
	if sel == nil || m.nav.ResourceType.Kind != "Secret" {
		return false
	}
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	_, ok := m.secretPreviewCache[secretPreviewCacheKey(m.nav.Context, ns, sel.Name)]
	return ok
}

// loadPreviewSecretData lazily fetches decoded secret data for the currently
// hovered secret at LevelResources. On cache hit it synthesizes an immediate
// message so the update handler can inject columns into freshly-rebuilt items
// after a list refresh. On cache miss it dispatches a background task.
//
// Returns nil when:
//   - the current resource type is not Secret,
//   - secret_lazy_loading is off in config (the list path already eagerly
//     decoded values into the item, so a hover GET would be redundant), or
//   - no middle item is selected.
func (m Model) loadPreviewSecretData() tea.Cmd {
	if m.nav.ResourceType.Kind != "Secret" || !ui.ConfigSecretLazyLoading {
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
	gen := m.requestGen

	key := secretPreviewCacheKey(kctx, ns, name)
	if cached := m.secretPreviewCache[key]; cached != nil {
		// Cache hit: emit immediately so the handler can inject columns into
		// items rebuilt after a list refresh, without touching the network.
		return func() tea.Msg {
			return previewSecretDataLoadedMsg{
				gen:  gen,
				ctx:  kctx,
				ns:   ns,
				name: name,
				data: cached,
			}
		}
	}

	// Cache miss: fetch in the background.
	reqCtx := m.reqCtx
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"Secret data: "+name,
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			data, err := m.client.GetSecretData(reqCtx, kctx, ns, name)
			return previewSecretDataLoadedMsg{
				gen:  gen,
				ctx:  kctx,
				ns:   ns,
				name: name,
				data: data,
				err:  err,
			}
		},
	)
}

// waitForStderr listens for captured stderr output and returns it as a message.
func (m Model) waitForStderr() tea.Cmd {
	if m.stderrChan == nil {
		return nil
	}
	ch := m.stderrChan
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return stderrCapturedMsg{message: msg}
	}
}
