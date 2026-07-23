package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
)

func (m Model) checkRBAC() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	rt := m.actionCtx.resourceType
	client := m.client
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindRBACCheck,
		"RBAC check: "+rt.Kind,
		bgtaskTarget(ctx, ns),
		func(sctx context.Context) tea.Msg {
			results, err := client.CheckRBAC(sctx, ctx, ns, rt.APIGroup, rt.Resource)
			return rbacCheckMsg{results: results, kind: rt.Kind, resource: rt.Resource, err: err}
		},
	)
}

// loadRoleRules extracts rules from a Role/ClusterRole spec and loads
// them into the CanI view. The rules are rendered directly without
// looking up discovered API resources.
func (m Model) loadRoleRules(rules []k8s.AccessRule) tea.Cmd {
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindRBACCheck,
		"Role rules: "+m.actionCtx.name,
		bgtaskTarget(m.effectiveContext(), m.namespace),
		func(sctx context.Context) tea.Msg {
			return canILoadedMsg{rules: rules, roleRules: true}
		},
	)
}

func (m Model) loadCanIRules() tea.Cmd {
	client := m.client
	if m.isUnionSentinel() {
		return m.loadCanIRulesUnion(client)
	}
	ctx := m.effectiveContext()
	ns := m.namespace
	if m.allNamespaces || ns == "" {
		ns = "default"
	}
	subject := m.canISubject
	if subject != "" && strings.HasPrefix(subject, "system:serviceaccount:") {
		return m.scheduleK8sCall(
			scheduler.PriorityCritical,
			scheduler.KindRBACCheck,
			"CanI rules: "+subject,
			ctx,
			func(sctx context.Context) tea.Msg {
				rules, namespaces, err := client.GetSelfRulesMultiNS(sctx, ctx, subject)
				return canILoadedMsg{rules: rules, namespaces: namespaces, err: err}
			},
		)
	}
	if subject != "" {
		viewNS := ns
		return m.scheduleK8sCall(
			scheduler.PriorityCritical,
			scheduler.KindRBACCheck,
			"CanI rules: "+subject,
			bgtaskTarget(ctx, viewNS),
			func(sctx context.Context) tea.Msg {
				rules, err := client.GetSelfRulesAs(sctx, ctx, viewNS, subject)
				return canILoadedMsg{rules: rules, namespaces: []string{viewNS}, err: err}
			},
		)
	}
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindRBACCheck,
		"CanI rules (current user)",
		bgtaskTarget(ctx, ns),
		func(sctx context.Context) tea.Msg {
			rules, err := client.GetSelfRulesAs(sctx, ctx, ns, "")
			return canILoadedMsg{rules: rules, namespaces: []string{ns}, err: err}
		},
	)
}

func canIRulesForContext(
	ctx context.Context,
	client *k8s.Client,
	contextName, namespace, subject string,
) ([]k8s.AccessRule, []string, error) {
	if subject != "" && strings.HasPrefix(subject, "system:serviceaccount:") {
		return client.GetSelfRulesMultiNS(ctx, contextName, subject)
	}
	if subject != "" {
		rules, err := client.GetSelfRulesAs(ctx, contextName, namespace, subject)
		return rules, []string{namespace}, err
	}
	rules, err := client.GetSelfRulesAs(ctx, contextName, namespace, "")
	return rules, []string{namespace}, err
}

func (m Model) loadCanIRulesUnion(client *k8s.Client) tea.Cmd {
	contexts := append([]string(nil), m.unionContexts...)
	ns := m.namespace
	subject := m.canISubject
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindRBACCheck,
		"CanI rules (union)",
		bgtaskTarget(UnionContextSentinel, ns),
		func(sctx context.Context) tea.Msg {
			results := make([]canIContextRules, 0, len(contexts))
			for _, contextName := range contexts {
				rules, namespaces, err := canIRulesForContext(sctx, client, contextName, ns, subject)
				if err != nil {
					return canILoadedMsg{union: true, err: fmt.Errorf("%s: %w", contextName, err)}
				}
				results = append(results, canIContextRules{
					context:    contextName,
					rules:      rules,
					namespaces: namespaces,
				})
			}
			return canILoadedMsg{union: true, contextRules: results, namespaces: []string{ns}}
		},
	)
}

func (m Model) loadCanISAList() tea.Cmd {
	client := m.client
	ctx := m.effectiveContext()
	return m.scheduleK8sCall(
		scheduler.PriorityCritical,
		scheduler.KindRBACCheck,
		"List service accounts",
		ctx,
		func(sctx context.Context) tea.Msg {
			accounts, err := client.ListServiceAccounts(sctx, ctx, "")
			if err != nil {
				return canISAListMsg{err: err}
			}
			subjects, _ := client.ListRBACSubjects(sctx, ctx)
			return canISAListMsg{accounts: accounts, subjects: subjects}
		},
	)
}
