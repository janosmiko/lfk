package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// --- Commands ---

// loadContexts is the watch-friendly variant. It reads the in-memory
// context map without re-walking kubeconfig files — fast enough to call
// every 2 seconds from watch-mode at LevelClusters in setups with many
// kubeconfig files (~/.kube/config.d/).
func (m Model) loadContexts() tea.Cmd {
	return m.trackBgTask(
		scheduler.KindResourceList,
		"List contexts",
		"",
		func() tea.Msg {
			items, err := m.client.GetContexts()
			return contextsLoadedMsg{items: items, err: err}
		},
	)
}

// loadContextsReload re-walks every kubeconfig file from disk before
// listing. Use this only when you have reason to believe the kubeconfig
// changed externally (kind/k3d/minikube create or delete from inside
// lfk. Explicit user-initiated refresh). Watch-mode auto-refresh and
// startup use the cheaper loadContexts.
func (m Model) loadContextsReload() tea.Cmd {
	return m.trackBgTask(
		scheduler.KindResourceList,
		"List contexts (reload)",
		"",
		func() tea.Msg {
			if err := m.client.ReloadKubeconfig(); err != nil {
				return contextsLoadedMsg{err: err, reloaded: true}
			}
			items, err := m.client.GetContexts()
			return contextsLoadedMsg{items: items, err: err, reloaded: true}
		},
	)
}

func (m Model) loadResourceTypes() tea.Cmd {
	// effectiveContext resolves the union sentinel to unionContexts[0], which
	// is where m.discoveredResources entries land in union mode (the cache
	// key is the resolved cluster, not the sentinel). Passing the raw
	// nav.Context here would always miss the cache and emit the seed list.
	return m.loadResourceTypesFor(m.effectiveContext())
}

// loadResourceTypesFor emits a resourceTypesMsg for a specific context.
// Used by the LevelClusters preview path so the right pane can show the
// resource types for the *hovered* context, not the currently-active one
// (m.nav.Context is "" after back-navigation from LevelResourceTypes).
func (m Model) loadResourceTypesFor(kctx string) tea.Cmd {
	discovered := m.discoveredResources[kctx]
	var items []model.Item
	var seeded bool
	if len(discovered) > 0 {
		items = model.BuildSidebarItems(discovered)
	} else {
		// Discovery hasn't completed yet for this context. Emit the seed
		// list (Pods, Deployments, ...) so the right-pane preview at
		// LevelClusters has something to show while hovering a context.
		// The middle-pane (LevelResourceTypes) handler will *ignore*
		// seeded messages while it's still waiting for discovery — see
		// updateResourceTypes — so the loader there is preserved.
		items = model.BuildSidebarItems(model.SeedResources())
		seeded = true
	}
	silent := m.suppressBgtasks
	return func() tea.Msg {
		return resourceTypesMsg{items: items, seeded: seeded, silent: silent}
	}
}

// discoverAPIResources launches async API resource discovery for the given
// context via Client.DiscoverAPIResources. Uses scheduler dispatch with
// PriorityCritical so it has a reserved worker slot and a per-Kind timeout.
// The scheduler-provided context replaces context.Background() for timeout
// control. Results are delivered via apiResourceDiscoveryMsg.
func (m Model) discoverAPIResources(contextName string) tea.Cmd {
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindAPIDiscovery,
		"Discover API resources",
		contextName,
		func(ctx context.Context) tea.Msg {
			entries, err := client.DiscoverAPIResources(ctx, contextName)
			return apiResourceDiscoveryMsg{context: contextName, entries: entries, err: err}
		},
	)
}

// loadQuotas fetches ResourceQuota objects for the current namespace.
func (m Model) loadQuotas() tea.Cmd {
	client := m.client
	kctx := m.effectiveContext()
	ns := m.effectiveNamespace()
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindResourceList,
		"Quotas",
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			quotas, err := client.GetNamespaceQuotas(ctx, kctx, ns)
			return quotaLoadedMsg{quotas: quotas, err: err}
		},
	)
}

func (m Model) loadResources(forPreview bool) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	rt := m.nav.ResourceType
	gen := m.requestGen
	silent := m.suppressBgtasks

	// In union mode at LevelResources, resource type discovery lives under the
	// first union context rather than the UnionContextSentinel.
	discoveryCtx := kctx
	if m.unionMode && kctx == UnionContextSentinel && len(m.unionContexts) > 0 {
		discoveryCtx = m.unionContexts[0]
	}
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		if secRT, ok := securityResourceTypeForItem(sel); ok {
			rt = secRT
		} else {
			found, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredResources[discoveryCtx])
			if !ok {
				return nil
			}
			rt = found
		}
	}
	// Union fetch: fan out to all union contexts in parallel. Covers both
	// preview hovers and main-list loads — without this, the preview path
	// at LevelResourceTypes would fall through to the regular GetResources
	// call below and pass the literal "__union__" sentinel as the context,
	// which restConfigForContext rejects with `context "__union__" does
	// not exist` on every sidebar hover whose target rt isn't already in
	// itemCache.
	//
	// Preview hovers serve from cache when fresh. Main-list and watch
	// ticks always refetch so deleted pods don't linger and Age advances.
	// This mirrors the non-union policy below — see the fresh-cache
	// shortcut comment.
	if m.unionMode && kctx == UnionContextSentinel && rt.Resource != "" {
		if forPreview {
			cacheKey := kctx + "/" + rt.Resource
			if cached, ok := m.itemCache[cacheKey]; ok &&
				m.cacheFingerprints[cacheKey] == m.fetchFingerprint() {
				items := cached
				rtCopy := rt
				return func() tea.Msg {
					return resourcesLoadedMsg{items: items, forPreview: true, gen: gen, silent: silent, rt: rtCopy}
				}
			}
		}
		unionCtxs := m.unionContexts
		client := m.client
		// Main list is Critical, preview hovers stay High — see the
		// non-union path below for the rationale.
		unionListPriority := scheduler.PriorityHigh
		if !forPreview {
			unionListPriority = scheduler.PriorityCritical
		}
		return m.scheduleK8sCall(
			unionListPriority,
			scheduler.KindResourceList,
			"List "+model.DisplayNameFor(rt)+" (union)",
			strings.Join(unionCtxs, ", "),
			func(ctx context.Context) tea.Msg {
				var listOpts []k8s.ListOption
				if !forPreview {
					listOpts = append(listOpts, k8s.PreferCache())
				}
				items, err := client.GetResourcesUnion(ctx, unionCtxs, ns, rt, listOpts...)
				return resourcesLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent, rt: rt}
			},
		)
	}

	// Fresh-cache shortcut: serve preview hover-cycles between sibling
	// resource types without hitting the API. Restricted to forPreview
	// because main-list loads (drill-in, navigate-back, watch tick,
	// shift+r) must always re-fetch — without that, deleted pods linger
	// in the list and Age never moves forward. Navigation still feels
	// instant because update_navigation.go renders the cached entry
	// synchronously while this fetch runs in the background.
	if forPreview && rt.Resource != "" {
		cacheKey := kctx + "/" + rt.Resource
		if cached, ok := m.itemCache[cacheKey]; ok &&
			m.cacheFingerprints[cacheKey] == m.fetchFingerprint() {
			items := cached
			rtCopy := rt
			return func() tea.Msg {
				return resourcesLoadedMsg{items: items, forPreview: forPreview, gen: gen, silent: silent, rt: rtCopy}
			}
		}
	}
	// Per-resource findings view (the "Security Findings" action): filter the
	// shared scan to m.securityResourceFilter's refs across every source.
	// Must dispatch before the generic security branch below — the sentinel
	// Kind does not encode a source, so the per-source getter would resolve
	// a bogus source name and return an empty list.
	if rt.Kind == model.SecurityResourceFindingsKind && m.client != nil {
		refs := m.securityResourceFilter
		if items, ok, err := m.client.GetSecurityFindingsForResourceCached(kctx, ns, refs); ok {
			rtCopy := rt
			return func() tea.Msg {
				return resourcesLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent, rt: rtCopy}
			}
		}
		client := m.client
		rtCopy := rt
		return m.scheduleK8sCall(
			scheduler.PriorityHigh,
			scheduler.KindResourceList,
			"Security findings: "+model.DisplayNameFor(rt),
			bgtaskTarget(kctx, ns),
			func(ctx context.Context) tea.Msg {
				items, err := client.GetSecurityFindingsForResource(ctx, kctx, ns, refs)
				return resourcesLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent, rt: rtCopy}
			},
		)
	}
	// Security findings share one coalesced, cached cluster scan. When that
	// scan is already warm, render the source's findings synchronously off the
	// scheduler — instant even when the scheduler is congested, and without a
	// tracked task (so the explorer stops showing a per-source "scan" entry
	// that is really a cache-hit filter over the shared scan). A cold cache
	// falls through to the scan task below, which warms it.
	if rt.APIGroup == model.SecurityVirtualAPIGroup && m.client != nil {
		if items, ok, err := m.client.GetSecurityFindingsCached(kctx, ns, rt); ok {
			rtCopy := rt
			return func() tea.Msg {
				return resourcesLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent, rt: rtCopy}
			}
		}
	}
	client := m.client
	// The main resource list (drill-in / watch refresh) runs at High — the
	// same lane as preview hovers. It must NOT be Critical: Critical is the
	// reserved lane for foundational, one-shot gating work (API discovery,
	// RBAC, namespaces). Submitting the user-driven list there let rapid
	// navigation flood the single reserved slot with non-coalescing
	// (per-generation) list loads, and — because general workers drain
	// Critical first — starve the High preview work that renders pod details.
	// High keeps the list ahead of Low background (dashboard/metrics/events)
	// while sharing the general pool fairly with previews.
	// Security findings reaching here have a cold cache. Name the task so it
	// reads as the shared security scan (it triggers/awaits the one coalesced
	// FetchAll that covers every source), not a per-source object listing.
	listName := "List " + model.DisplayNameFor(rt)
	if rt.APIGroup == model.SecurityVirtualAPIGroup {
		listName = "Security findings: " + model.DisplayNameFor(rt)
	}
	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceList,
		listName,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			var listOpts []k8s.ListOption
			if !forPreview {
				listOpts = append(listOpts, k8s.PreferCache())
			}
			items, err := client.GetResources(ctx, kctx, ns, rt, listOpts...)
			return resourcesLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent, rt: rt}
		},
	)
}

func (m Model) loadOwned(forPreview bool) tea.Cmd {
	kctx := m.effectiveContext()
	ns := m.effectiveNamespace()
	kind := m.nav.ResourceType.Kind
	name := m.nav.ResourceName
	gen := m.requestGen
	silent := m.suppressBgtasks
	client := m.client
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	} else if ns == "" && m.nav.Namespace != "" {
		ns = m.nav.Namespace
	}
	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindResourceList,
		"List "+kind+" children",
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			items, err := client.GetOwnedResources(ctx, kctx, ns, kind, name)
			return ownedLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent}
		},
	)
}

func (m Model) loadResourceTree() tea.Cmd {
	kctx := m.effectiveContext()
	ns := m.effectiveNamespace()
	gen := m.requestGen

	var kind, name string
	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		kind = m.nav.ResourceType.Kind
		name = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		kind = sel.Kind
		name = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	case model.LevelContainers:
		// At the containers level, M means "show the parent Pod's tree".
		// The Container itself has no refs distinct from its Pod, and the
		// user's mental model of "the resource map for what I'm looking at"
		// is the surrounding Pod.
		name = m.nav.OwnedName
		if name == "" {
			return nil
		}
		kind = "Pod"
		// effectiveNamespace returns "" in all-namespaces mode. Fall back
		// to the navigation namespace so the typed Pod GET resolves —
		// same pattern as loadContainers above.
		if ns == "" && m.nav.Namespace != "" {
			ns = m.nav.Namespace
		}
	default:
		return nil
	}

	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindResourceTree,
		"Resource tree: "+kind+"/"+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			tree, err := m.client.GetResourceTree(ctx, kctx, ns, kind, name)
			return resourceTreeLoadedMsg{tree: tree, err: err, gen: gen}
		},
	)
}

func (m Model) loadContainers(forPreview bool) tea.Cmd {
	kctx := m.effectiveContext()
	ns := m.effectiveNamespace()
	gen := m.requestGen
	silent := m.suppressBgtasks
	client := m.client
	if ns == "" && m.nav.Namespace != "" {
		ns = m.nav.Namespace
	}
	podName := m.nav.OwnedName
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		podName = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	}
	taskTarget := bgtaskTarget(kctx, ns)
	if podName != "" {
		taskTarget = taskTarget + " / " + podName
	}
	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindContainers,
		"List containers",
		taskTarget,
		func(ctx context.Context) tea.Msg {
			items, err := client.GetContainers(ctx, kctx, ns, podName)
			return containersLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent}
		},
	)
}

// resolveOwnedResourceType determines the correct ResourceTypeEntry for an
// owned item at LevelOwned. It uses the item's Kind to look up the type in
// both built-in resource types and discovered CRDs. If the kind is not found,
// it falls back to constructing a ResourceTypeEntry from the item's Extra
// metadata (which may contain "group/version" from ArgoCD status) and the
// Kind. Returns false if the type cannot be resolved.
//
// When sel.Extra carries a "group/version" hint (e.g. helm manifest items
// expose the rendered apiVersion there), the lookup is biased toward the
// matching APIGroup so that two CRDs sharing the same Kind name but living
// in different API groups (e.g. VaultDynamicSecret in secrets.hashicorp.com
// vs. generators.external-secrets.io) resolve to the right CRD instead of
// whichever one was iterated first.
func (m Model) resolveOwnedResourceType(sel *model.Item) (model.ResourceTypeEntry, bool) {
	if sel == nil {
		return model.ResourceTypeEntry{}, false
	}
	kctx := m.nav.Context
	crds := m.discoveredResources[kctx]

	// If Extra carries an apiVersion ("group/version"), prefer matching by
	// Kind+APIGroup so duplicate Kind names across API groups resolve to
	// the right CRD. Core types (Extra="v1") have no group component and
	// fall through to the Kind-only lookup below.
	if sel.Extra != "" && sel.Kind != "" {
		parts := strings.SplitN(sel.Extra, "/", 2)
		if len(parts) == 2 && parts[0] != "" {
			if rt, ok := model.FindResourceTypeByKindAndGroup(sel.Kind, parts[0], crds); ok {
				return rt, true
			}
		}
	}

	// Try to find by Kind in built-in types and discovered CRDs.
	if rt, ok := model.FindResourceTypeByKind(sel.Kind, crds); ok {
		return rt, true
	}

	// If Extra contains a full resource ref string ("group/version/resource"), use it.
	if sel.Extra != "" {
		if rt, ok := model.FindResourceTypeIn(sel.Extra, crds); ok {
			return rt, true
		}
	}

	return model.ResourceTypeEntry{}, false
}

func (m Model) loadYAML() tea.Cmd {
	// Synthetic security items have no YAML. Defense in depth alongside
	// the gate in enterFullView so any future caller stays safe.
	if onSecurityView(&m) {
		return nil
	}
	kctx := m.effectiveContext()
	ns := m.resolveNamespace()
	client := m.client

	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		rt := m.nav.ResourceType
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		itemCtx := kctx
		if sel.ClusterName != "" {
			itemCtx = sel.ClusterName
		}
		return m.scheduleK8sCall(
			scheduler.PriorityHigh,
			scheduler.KindYAMLFetch,
			"YAML: "+name,
			bgtaskTarget(itemCtx, itemNs),
			func(ctx context.Context) tea.Msg {
				content, err := client.GetResourceYAML(ctx, itemCtx, itemNs, rt, name)
				return buildYAMLLoadedMsg(content, err)
			},
		)
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		itemCtx := kctx
		if sel.ClusterName != "" {
			itemCtx = sel.ClusterName
		}
		taskTarget := bgtaskTarget(itemCtx, itemNs)
		if sel.Kind == "Pod" {
			return m.scheduleK8sCall(
				scheduler.PriorityHigh,
				scheduler.KindYAMLFetch,
				"YAML: "+name,
				taskTarget,
				func(ctx context.Context) tea.Msg {
					content, err := client.GetPodYAML(ctx, itemCtx, itemNs, name)
					return buildYAMLLoadedMsg(content, err)
				},
			)
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return buildYAMLLoadedMsg("", fmt.Errorf("unknown resource type: %s", sel.Kind))
			}
		}
		return m.scheduleK8sCall(
			scheduler.PriorityHigh,
			scheduler.KindYAMLFetch,
			"YAML: "+name,
			taskTarget,
			func(ctx context.Context) tea.Msg {
				content, err := client.GetResourceYAML(ctx, itemCtx, itemNs, rt, name)
				return buildYAMLLoadedMsg(content, err)
			},
		)
	case model.LevelContainers:
		podName := m.nav.OwnedName
		return m.scheduleK8sCall(
			scheduler.PriorityHigh,
			scheduler.KindYAMLFetch,
			"YAML: "+podName,
			bgtaskTarget(kctx, ns),
			func(ctx context.Context) tea.Msg {
				content, err := client.GetPodYAML(ctx, kctx, ns, podName)
				return buildYAMLLoadedMsg(content, err)
			},
		)
	}
	return nil
}

// loadDiff fetches YAML for two resources and returns a diffLoadedMsg.
func (m Model) loadDiff(rt model.ResourceTypeEntry, itemA, itemB model.Item) tea.Cmd {
	if isVirtualResourceKind(rt.Kind) {
		return func() tea.Msg {
			return diffLoadedMsg{err: fmt.Errorf("diff not available for %q", rt.Kind)}
		}
	}
	kctx := m.nav.Context

	resolveCtx := func(item model.Item) string {
		if item.ClusterName != "" {
			return item.ClusterName
		}
		return kctx
	}
	resolveNS := func(item model.Item) string {
		if item.Namespace != "" {
			return item.Namespace
		}
		return m.resolveNamespace()
	}

	ctxA := resolveCtx(itemA)
	ctxB := resolveCtx(itemB)
	nsA := resolveNS(itemA)
	nsB := resolveNS(itemB)

	nameA := itemA.Name
	nameB := itemB.Name

	leftLabel := nameA
	rightLabel := nameB
	if nsA != nsB {
		leftLabel = nsA + "/" + nameA
		rightLabel = nsB + "/" + nameB
	}
	if ctxA != ctxB {
		leftLabel = ctxA + "/" + leftLabel
		rightLabel = ctxB + "/" + rightLabel
	}

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Diff: "+nameA+" vs "+nameB,
		bgtaskTarget(kctx, nsA),
		func(ctx context.Context) tea.Msg {
			yamlA, errA := m.client.GetResourceYAML(ctx, ctxA, nsA, rt, nameA)
			if errA != nil {
				return diffLoadedMsg{err: fmt.Errorf("fetching %s: %w", nameA, errA)}
			}
			yamlB, errB := m.client.GetResourceYAML(ctx, ctxB, nsB, rt, nameB)
			if errB != nil {
				return diffLoadedMsg{err: fmt.Errorf("fetching %s: %w", nameB, errB)}
			}
			return diffLoadedMsg{
				left:      yamlA,
				right:     yamlB,
				leftName:  leftLabel,
				rightName: rightLabel,
			}
		},
	)
}

// resolveNamespace returns the namespace to use for get/describe operations.
func (m Model) resolveNamespace() string {
	if m.nav.Namespace != "" {
		return m.nav.Namespace
	}
	return m.namespace
}

// defaultNamespaceForContext returns the namespace to scope to when none is
// set: the active context's kubeconfig namespace when a client is available,
// falling back to "default". Guards against a nil client (test models).
func (m Model) defaultNamespaceForContext() string {
	if m.client != nil {
		return m.client.DefaultNamespace(m.nav.Context)
	}
	return "default"
}

// rescopeNamespaceForContext re-applies the namespace scope after a plain
// context switch, so each cluster opens in the namespace its kubeconfig
// context pins. An explicit all_namespaces in config still wins.
func (m *Model) rescopeNamespaceForContext(ctxName string) {
	if m.client == nil || m.unionMode {
		return
	}
	// Every namespace selection here belongs to the cluster we just left. Those
	// names rarely exist in the new one, so a later A-toggle or previous-namespace
	// jump would scope this cluster to a namespace the user never picked.
	m.selectedNamespaces = nil
	m.nsSelectionNegated = false
	m.savedSelectedNamespaces = nil
	m.savedNsSelectionNegated = false
	m.nsSelectionModified = false
	m.previousNsScope = nil
	if resolveStartupAllNamespaces(m.client, ctxName) {
		m.allNamespaces = true
		// The A-toggle only resolves the new context's default namespace when
		// this is empty, so a leftover value scopes the new cluster to the old
		// cluster's namespace.
		m.namespace = ""
		return
	}
	m.allNamespaces = false
	m.namespace = m.client.DefaultNamespace(ctxName)
}

// loadRevisions fetches the revision history for a deployment.
func (m Model) loadRevisions() tea.Cmd {
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
		"Revisions: "+name,
		bgtaskTarget(kctx, ns),
		func(ctx context.Context) tea.Msg {
			revs, err := client.GetDeploymentRevisions(ctx, kctx, ns, name)
			return revisionListMsg{revisions: revs, err: err}
		},
	)
}

// bgtaskTarget formats a context+namespace pair for the :tasks overlay's
// Target column. Falls back gracefully when either part is empty.
func bgtaskTarget(kctx, ns string) string {
	switch {
	case kctx != "" && ns != "":
		return kctx + " / " + ns
	case kctx != "":
		return kctx
	case ns != "":
		return ns
	default:
		return ""
	}
}

// trackBgTask wraps a loader's inner closure with bgtasks Start/Finish.
// It calls Start SYNCHRONOUSLY (while Update is still building the Cmd
// return value), so the very next View() frame already sees the task in
// the registry. If Start were inside the returned closure instead, it
// would only run after the goroutine that executes the Cmd begins —
// which is after View() has already rendered that frame, so the user
// would never see the indicator for sub-frame loads.
//
// The deferred Finish still runs inside the goroutine, so it correctly
// fires on success, error, panic, or context cancellation.
//
// Pass a nil inner to skip tracking entirely (for loaders whose early
// return paths don't dispatch any work).
func (m Model) trackBgTask(kind scheduler.Kind, name, target string, inner func() tea.Msg) tea.Cmd {
	if inner == nil {
		return nil
	}
	registry := m.scheduler
	var id uint64
	if m.suppressBgtasks {
		id = registry.StartUntracked()
	} else {
		id = registry.StartOwned(m.currentTabUID(), kind, name, target)
	}
	return func() tea.Msg {
		defer registry.Finish(id)
		return inner()
	}
}

// scheduleK8sCall queues a K8s call with the given priority via the
// scheduler. Replaces trackBgTask for code paths that benefit from
// priority-based dispatch and coalescing. trackBgTask remains for
// non-K8s subprocess work (helm, trivy, kubectl describe).
//
// Submit is called synchronously (while Update is still building the Cmd
// return value) so coalescing and preemption take effect immediately —
// the same pattern trackBgTask uses for Start. The returned tea.Cmd
// blocks (in its goroutine) on the Future until the scheduler delivers a
// Result, then returns the Fn's tea.Msg or nil for ErrCoalesced /
// ErrContextSwitched.
func (m Model) scheduleK8sCall(prio scheduler.Priority, kind scheduler.Kind, name, target string, fn func(ctx context.Context) tea.Msg) tea.Cmd { //nolint:unparam // prio will receive varying priorities once call sites migrate from trackBgTask in subsequent tasks
	if fn == nil {
		return nil
	}
	future := m.scheduler.Submit(scheduler.SubmitReq{
		KubeContext: m.nav.Context,
		Kind:        kind,
		Priority:    prio,
		Name:        name,
		Target:      target,
		Gen:         m.requestGen,
		SilentTrack: m.suppressBgtasks,
		Owner:       m.currentTabUID(),
		Fn: func(ctx context.Context) (any, error) {
			return fn(ctx), nil
		},
	})
	return func() tea.Msg {
		res := <-future
		// All three sentinels mean "this submission was intentionally
		// abandoned" — coalesced by a newer one, context dropped, or
		// superseded by a newer requestGen via CancelStaleByGen. Return nil
		// so the UI ignores it. The surviving submission carries the result.
		if errors.Is(res.Err, scheduler.ErrCoalesced) ||
			errors.Is(res.Err, scheduler.ErrContextSwitched) ||
			errors.Is(res.Err, scheduler.ErrSuperseded) {
			return nil
		}
		// Today the inner Fn wrapper above always passes nil as the
		// scheduler-side error (K8s/timeout errors travel inside the
		// returned tea.Msg's own err field). If a future change starts
		// surfacing errors via res.Err, log them so they don't vanish
		// silently — the typed-msg path is still the primary signal,
		// but this defends against a regression.
		if res.Err != nil {
			logger.Info("scheduleK8sCall: unexpected non-sentinel err", "kind", kind.String(), "name", name, "err", res.Err.Error())
		}
		if msg, ok := res.Value.(tea.Msg); ok {
			return msg
		}
		return nil
	}
}
