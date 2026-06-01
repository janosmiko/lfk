package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/security"
)

// stubIgnoreChecker mirrors the production modelIgnoreChecker semantics
// (resource ignored when its group, namespace, OR exact resource matches) so
// groupFindings is exercised through the real checker != nil branch — the
// integration the app-layer predicate tests never reach.
type stubIgnoreChecker struct {
	groups     map[string]bool // group fully ignored cluster-wide
	namespaces map[string]bool // group ignored within a namespace
	resources  map[string]bool // specific resource ignored
}

func (s stubIgnoreChecker) IsGroupIgnored(_, groupKey string) bool {
	return s.groups[groupKey]
}

func (s stubIgnoreChecker) IsResourceIgnored(_, groupKey, resourceKey string) bool {
	if s.groups[groupKey] || s.resources[resourceKey] {
		return true
	}
	ns, _, _ := strings.Cut(resourceKey, "/")
	return s.namespaces[ns]
}

// privileged: pod-a, pod-b (default), pod-d (kube-system). host_pid: pod-c.
func ignoreTestFindings() []security.Finding {
	mk := func(id, check, ns, name string, sev security.Severity) security.Finding {
		return security.Finding{
			ID: id, Source: "heuristic", Title: check, Severity: sev,
			Category: security.CategoryMisconfig,
			Labels:   map[string]string{"check": check},
			Resource: security.ResourceRef{Namespace: ns, Kind: "Pod", Name: name},
		}
	}
	return []security.Finding{
		mk("1", "privileged", "default", "pod-a", security.SeverityCritical),
		mk("2", "privileged", "default", "pod-b", security.SeverityCritical),
		mk("3", "host_pid", "default", "pod-c", security.SeverityHigh),
		mk("4", "privileged", "kube-system", "pod-d", security.SeverityCritical),
	}
}

func findGroup(groups []findingGroup, key string) *findingGroup {
	for i := range groups {
		if groups[i].Key == key {
			return &groups[i]
		}
	}
	return nil
}

// A cluster-wide group ignore removes the entire group from the list.
func TestGroupFindings_Checker_GroupIgnoreOmitsWholeGroup(t *testing.T) {
	checker := stubIgnoreChecker{groups: map[string]bool{"privileged": true}}
	groups := groupFindings(ignoreTestFindings(), "heuristic", checker, false)

	require.Len(t, groups, 1)
	assert.Equal(t, "host_pid", groups[0].Key, "the ignored group must be gone")
}

// A namespace-scoped ignore keeps the group but drops resources in that namespace.
func TestGroupFindings_Checker_NamespaceScopeReducesCount(t *testing.T) {
	checker := stubIgnoreChecker{namespaces: map[string]bool{"kube-system": true}}
	groups := groupFindings(ignoreTestFindings(), "heuristic", checker, false)

	g := findGroup(groups, "privileged")
	require.NotNil(t, g, "group stays visible under a namespace-scoped ignore")
	assert.Equal(t, 2, g.Count, "kube-system/pod-d excluded; default pod-a, pod-b remain")
	assert.False(t, g.Ignored, "a namespace-scoped ignore must not tag the whole group")
}

// A resource-scoped ignore drops only that resource.
func TestGroupFindings_Checker_ResourceScopeDropsOneResource(t *testing.T) {
	checker := stubIgnoreChecker{resources: map[string]bool{"default/Pod/pod-a": true}}
	groups := groupFindings(ignoreTestFindings(), "heuristic", checker, false)

	g := findGroup(groups, "privileged")
	require.NotNil(t, g)
	assert.Equal(t, 2, g.Count, "pod-a excluded; pod-b and pod-d remain")
}

// show-ignored mode reveals an ignored group AND tags it, which findingGroupToItem
// surfaces as the __ignored__ column the view layer reads.
func TestGroupFindings_Checker_ShowIgnoredTagsAndRevealsGroup(t *testing.T) {
	checker := stubIgnoreChecker{groups: map[string]bool{"privileged": true}}
	groups := groupFindings(ignoreTestFindings(), "heuristic", checker, true)

	require.Len(t, groups, 2, "showIgnored keeps the ignored group in the list")
	priv := findGroup(groups, "privileged")
	require.NotNil(t, priv)
	assert.True(t, priv.Ignored, "the cluster-wide-ignored group is tagged Ignored")
	assert.Equal(t, 3, priv.Count, "all resources shown in show-ignored mode")

	host := findGroup(groups, "host_pid")
	require.NotNil(t, host)
	assert.False(t, host.Ignored)

	// The tag must surface on the rendered item via the __ignored__ column.
	item := findingGroupToItem(*priv)
	assert.Equal(t, "true", item.ColumnValue("__ignored__"))
	assert.Empty(t, findingGroupToItem(*host).ColumnValue("__ignored__"))
}
