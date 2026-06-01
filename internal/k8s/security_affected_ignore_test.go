package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func affectedIgnoreClient() *Client {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr: "heuristic", Available: true,
		Findings: []security.Finding{
			{
				Source: "heuristic", Title: "privileged", Severity: security.SeverityCritical,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "web"},
				Labels:   map[string]string{"check": "privileged"},
			},
			{
				Source: "heuristic", Title: "privileged", Severity: security.SeverityHigh,
				Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "api"},
				Labels:   map[string]string{"check": "privileged"},
			},
		},
	})
	c := &Client{}
	c.SetSecurityManager(mgr)
	return c
}

// With show-ignored off, the drill-in must filter ignored resources so it
// matches the group's filtered affected count (previously it showed them all).
func TestGetSecurityAffectedResources_HidesIgnoredWhenToggleOff(t *testing.T) {
	c := affectedIgnoreClient()
	c.SetIgnoreChecker(stubIgnoreChecker{resources: map[string]bool{"ns/Pod/api": true}})
	c.SetShowIgnored(false)

	items, err := c.GetSecurityAffectedResources(context.Background(), "kctx", "",
		model.ResourceTypeEntry{Kind: "__security_heuristic__"}, "privileged")
	require.NoError(t, err)
	require.Len(t, items, 1, "the ignored resource must be filtered out")
	assert.Equal(t, "pod/web", items[0].Name)
	assert.Empty(t, items[0].ColumnValue("__ignored__"))
}

// With show-ignored on, both resources appear and the ignored one is tagged so
// the renderer can mark it.
func TestGetSecurityAffectedResources_TagsIgnoredWhenToggleOn(t *testing.T) {
	c := affectedIgnoreClient()
	c.SetIgnoreChecker(stubIgnoreChecker{resources: map[string]bool{"ns/Pod/api": true}})
	c.SetShowIgnored(true)

	items, err := c.GetSecurityAffectedResources(context.Background(), "kctx", "",
		model.ResourceTypeEntry{Kind: "__security_heuristic__"}, "privileged")
	require.NoError(t, err)
	require.Len(t, items, 2, "both resources shown in show-ignored mode")

	byName := make(map[string]model.Item, len(items))
	for _, it := range items {
		byName[it.Name] = it
	}
	assert.Equal(t, "true", byName["pod/api"].ColumnValue("__ignored__"), "ignored resource is tagged")
	assert.Empty(t, byName["pod/web"].ColumnValue("__ignored__"), "active resource is not tagged")
}
