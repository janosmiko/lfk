package app

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// errNoDependentTarget means the owner walk has no way to answer for this
// target: no client, a kind whose children it does not know, or an object with
// no UID to walk from. The dialog drops the row rather than printing zero.
var errNoDependentTarget = errors.New("dependent count is not available for this kind")

// dependentsLoadedMsg carries the result of the owner walk. req numbers the
// walk, because closing and reopening a confirm can leave an older reply in
// flight that would otherwise land on the new dialog.
type dependentsLoadedMsg struct {
	count *k8s.DependentCount
	req   uint64
	err   error
}

// beginDependents arms the row for a confirm that is about to open.
func (m *Model) beginDependents() {
	m.deps.count = nil
	m.deps.loading = true
	m.deps.req++
}

// loadDependents walks the owner graph below the pending delete's target. The
// walk runs in the background, so the dialog opens on a placeholder rather
// than waiting for the cluster.
func (m Model) loadDependents() tea.Cmd {
	req := m.deps.req
	client := m.client
	ctxName := m.actionCtx.context
	namespace := m.actionCtx.namespace
	uid := uidFrom(m.actionCtx.raw)
	kinds, known := k8s.DependentKindsFor(
		m.actionCtx.kind, m.actionCtx.name, ownerSelectorString(m.actionCtx.raw))
	if client == nil || !known || uid == "" || namespace == "" {
		// Still answer. beginDependents already turned the placeholder on, and
		// a silent return would leave it reading "counting..." forever.
		return func() tea.Msg { return dependentsLoadedMsg{req: req, err: errNoDependentTarget} }
	}

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Dependents: "+m.actionCtx.name,
		bgtaskTarget(ctxName, namespace),
		func(ctx context.Context) tea.Msg {
			refs, err := client.DependentRefsInNamespace(ctx, ctxName, namespace, kinds)
			if err != nil {
				return dependentsLoadedMsg{req: req, err: err}
			}
			count := k8s.CountDependents(refs, []string{uid})
			return dependentsLoadedMsg{count: &count, req: req}
		},
	)
}

// loadBulkDependents totals the whole selection as one figure. Cost is bounded
// by the number of distinct namespaces, not by how many rows are selected.
func (m Model) loadBulkDependents() tea.Cmd {
	req := m.deps.req
	client := m.client
	ctxName := m.effectiveContext()
	byNS, uncounted := bulkDependentTargets(m.bulkItems)
	if len(byNS) == 0 {
		// Nothing in the selection has children the walk can follow, so this
		// needs no cluster call at all.
		return func() tea.Msg {
			return dependentsLoadedMsg{
				count: &k8s.DependentCount{ByKind: map[string]int{}, Uncounted: uncounted},
				req:   req,
			}
		}
	}
	if client == nil {
		return func() tea.Msg { return dependentsLoadedMsg{req: req, err: errNoDependentTarget} }
	}

	return m.scheduleK8sCall(
		scheduler.PriorityHigh,
		scheduler.KindYAMLFetch,
		"Dependents: selection",
		bgtaskTarget(ctxName, ""),
		func(ctx context.Context) tea.Msg {
			count, err := bulkDependentCount(byNS, uncounted,
				func(ns string, kinds []k8s.DependentKind) ([]k8s.DependentRef, error) {
					return client.DependentRefsInNamespace(ctx, ctxName, ns, kinds)
				})
			if err != nil {
				return dependentsLoadedMsg{req: req, err: err}
			}
			return dependentsLoadedMsg{count: count, req: req}
		},
	)
}

// dependentTargets is what one namespace contributes to a bulk selection: the
// UIDs to walk from, and the owner kinds that decide which child kinds to list.
type dependentTargets struct {
	uids  []string
	kinds []string
}

// bulkDependentTargets splits a bulk selection by namespace, and counts the
// rows the walk cannot follow. A cluster-scoped row, or one of a kind with no
// known children, is reported as uncounted rather than quietly ignored.
func bulkDependentTargets(items []model.Item) (map[string]*dependentTargets, int) {
	byNS := make(map[string]*dependentTargets)
	uncounted := 0
	for _, it := range items {
		uid := uidFrom(it.Raw)
		if !k8s.HasDependentKinds(it.Kind) || uid == "" || it.Namespace == "" {
			uncounted++
			continue
		}
		t := byNS[it.Namespace]
		if t == nil {
			t = &dependentTargets{}
			byNS[it.Namespace] = t
		}
		t.uids = append(t.uids, uid)
		t.kinds = append(t.kinds, it.Kind)
	}
	return byNS, uncounted
}

// bulkDependentCount totals a selection namespace by namespace. Each namespace
// lists every child kind its targets need exactly once, so a selection of
// fifty Deployments in one namespace still costs two calls.
func bulkDependentCount(
	byNS map[string]*dependentTargets, uncounted int,
	refs func(string, []k8s.DependentKind) ([]k8s.DependentRef, error),
) (*k8s.DependentCount, error) {
	total := k8s.DependentCount{ByKind: make(map[string]int), Uncounted: uncounted}
	for namespace, want := range byNS {
		found, err := refs(namespace, k8s.MergeDependentKinds(want.kinds))
		if err != nil {
			return nil, err
		}
		// Walked per namespace, and namespaces cannot share an owner, so the
		// sums cannot double-count.
		count := k8s.CountDependents(found, want.uids)
		total.Total += count.Total
		for kind, n := range count.ByKind {
			total.ByKind[kind] += n
		}
	}
	return &total, nil
}

// updateDependentsLoaded stores the walk's answer. A failure leaves the dialog
// without the row rather than blocking the action.
//
//nolint:unparam // consistent message handler signature; the caller passes the Cmd on
func (m Model) updateDependentsLoaded(msg dependentsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.req != m.deps.req {
		return m, nil // an older dialog answering late
	}
	m.deps.loading = false
	if msg.err != nil {
		m.deps.count = nil
		return m, nil
	}
	m.deps.count = msg.count
	return m, nil
}

// ownerSelectorString renders the target's own spec.selector as the wire form
// a list call takes, so the walk asks the server for the children instead of
// the namespace. An object with no selector, or one the API server would
// reject, yields an empty string and the caller lists unnarrowed.
func ownerSelectorString(raw map[string]any) string {
	sel := workloadSelectorFrom(raw)
	if sel == nil {
		return ""
	}
	parsed, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return ""
	}
	return parsed.String()
}

// uidFrom reads metadata.uid off the object a row carries, which is what the
// owner walk matches ownerReferences against. Names are not enough: a deleted
// and recreated object keeps its name and gets a new UID, and the garbage
// collector follows the UID.
func uidFrom(raw map[string]any) string {
	meta, ok := raw["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	uid, _ := meta["uid"].(string)
	return uid
}
