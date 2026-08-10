package app

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
)

// podPermissionsMsg carries one bulk review back to the model. scope names the
// cluster and namespace the verdicts belong to, so a late reply cannot answer
// for a namespace the user has since left.
type podPermissionsMsg struct {
	scope   permScopeKey
	allowed map[string]bool
	err     error
}

// loadPodPermissions asks the cluster which Pod verbs the current user holds
// in the namespace on screen. One pass per context and namespace: the second
// visit reads the cache and makes no call.
//
// The pass runs through the scheduler like every other cluster call, so
// entering a namespace never waits on it.
//
// Two lists are left unreviewed rather than reviewed wrongly: a union list
// draws rows from several clusters, and an all-namespaces list from several
// namespaces, so neither has one scope a single pass could speak for. Both
// then keep every action visible, which is the same fail-open answer an
// unfinished review gives.
func (m *Model) loadPodPermissions() tea.Cmd {
	client := m.client
	if client == nil || m.isUnionSentinel() {
		return nil
	}
	// effectiveNamespace answers "" for an all-namespaces or multi-namespace
	// list, which permScopeFor refuses. m.namespace would still hold the last
	// single namespace and send the pass to a namespace the list has left.
	scope, ok := m.permScopeFor(m.effectiveContext(), m.effectiveNamespace())
	if !ok || !m.perms.begin(scope) {
		return nil
	}
	queries := podPermissionQueries()
	return m.scheduleK8sCall(
		scheduler.PriorityLow,
		scheduler.KindRBACCheck,
		"Permissions: pods in "+scope.namespace,
		bgtaskTarget(scope.context, scope.namespace),
		func(sctx context.Context) tea.Msg {
			allowed, err := client.CheckPermissions(sctx, scope.context, scope.namespace, queries)
			return podPermissionsMsg{scope: scope, allowed: allowed, err: err}
		},
	)
}

// updatePodPermissions stores the verdicts. A failed review is dropped without
// a status message: the user did not ask for it, and every action stays
// visible, so there is nothing for them to act on.
func (m Model) updatePodPermissions(msg podPermissionsMsg) tea.Model {
	if msg.err != nil {
		m.perms.fail(msg.scope)
		return m
	}
	m.perms.record(msg.scope, msg.allowed)
	return m
}
