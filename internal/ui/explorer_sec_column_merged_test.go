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
