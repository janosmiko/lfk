package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// MergedSecurityCounts must combine an item's own findings with its owner
// resources' findings (owner:N columns, value "APIVersion||Kind||Name"), the
// same aggregation the SEC row badge uses.
func TestMergedSecurityCounts_IncludesOwnerFindings(t *testing.T) {
	idx := security.BuildFindingIndex([]security.Finding{
		{ID: "1", Severity: security.SeverityCritical, Resource: security.ResourceRef{Namespace: "default", Kind: "Deployment", Name: "api"}},
		{ID: "2", Severity: security.SeverityHigh, Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api-xyz"}},
	})
	item := &model.Item{
		Kind: "Pod", Name: "api-xyz", Namespace: "default",
		Columns: []model.KeyValue{{Key: "owner:0", Value: "apps/v1||Deployment||api"}},
	}

	counts := MergedSecurityCounts(idx, item)
	assert.Equal(t, 1, counts.Critical, "owner (Deployment) critical merged in")
	assert.Equal(t, 1, counts.High, "pod's own high counted")
	assert.Equal(t, 2, counts.Total())
}

func TestMergedSecurityCounts_NilSafe(t *testing.T) {
	assert.Equal(t, 0, MergedSecurityCounts(nil, &model.Item{}).Total())
	assert.Equal(t, 0, MergedSecurityCounts(security.BuildFindingIndex(nil), nil).Total())
}

// SecurityRefsForItem must return the item's own ref plus one ref per owner:N
// column — the exact set MergedSecurityCounts aggregates over, so the
// per-resource findings list always matches the badge.
func TestSecurityRefsForItem_OwnAndOwnerRefs(t *testing.T) {
	item := &model.Item{
		Kind: "Pod", Name: "api-xyz", Namespace: "default",
		Columns: []model.KeyValue{
			{Key: "owner:0", Value: "apps/v1||ReplicaSet||api-5d"},
			{Key: "owner:1", Value: "apps/v1||Deployment||api"},
			{Key: "Status", Value: "Running"},
		},
	}

	refs := SecurityRefsForItem(item)
	assert.Equal(t, []security.ResourceRef{
		{Namespace: "default", Kind: "Pod", Name: "api-xyz"},
		{Namespace: "default", Kind: "ReplicaSet", Name: "api-5d"},
		{Namespace: "default", Kind: "Deployment", Name: "api"},
	}, refs)
}

func TestSecurityRefsForItem_NilAndMalformedOwner(t *testing.T) {
	assert.Nil(t, SecurityRefsForItem(nil))

	item := &model.Item{
		Kind: "Pod", Name: "web", Namespace: "ns",
		Columns: []model.KeyValue{{Key: "owner:0", Value: "malformed"}},
	}
	refs := SecurityRefsForItem(item)
	assert.Equal(t, []security.ResourceRef{{Namespace: "ns", Kind: "Pod", Name: "web"}}, refs,
		"malformed owner columns are skipped")
}
