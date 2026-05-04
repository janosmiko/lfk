package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestWhoCanCollectResources_DedupesAndSorts(t *testing.T) {
	groups := []model.CanIGroup{
		{Name: "apps", Resources: []model.CanIResource{
			{Resource: "deployments"}, {Resource: "statefulsets"},
		}},
		{Name: "", Resources: []model.CanIResource{
			{Resource: "pods"}, {Resource: "secrets"}, {Resource: "configmaps"},
		}},
		{Name: "extensions", Resources: []model.CanIResource{
			{Resource: "deployments"}, // duplicate across groups
		}},
	}
	got := whoCanCollectResources(groups)
	assert.Equal(t, []string{
		"configmaps", "deployments", "pods", "secrets", "statefulsets",
	}, got, "result must be sorted and de-duplicated")
}

func TestWhoCanCollectResources_EmptyInput(t *testing.T) {
	assert.Empty(t, whoCanCollectResources(nil))
	assert.Empty(t, whoCanCollectResources([]model.CanIGroup{}))
}

func TestWhoCanCollectResources_SkipsBlankNames(t *testing.T) {
	groups := []model.CanIGroup{
		{Resources: []model.CanIResource{{Resource: ""}, {Resource: "pods"}}},
	}
	assert.Equal(t, []string{"pods"}, whoCanCollectResources(groups),
		"blank resource names are skipped — the picker can't query an empty resource")
}

func TestWhoCanFilterResources_EmptyQueryReturnsAll(t *testing.T) {
	all := []string{"a", "b", "c"}
	assert.Equal(t, all, whoCanFilterResources(all, ""))
}

func TestWhoCanFilterResources_CaseInsensitiveSubstring(t *testing.T) {
	all := []string{"pods", "pods/exec", "secrets", "configmaps"}
	assert.Equal(t, []string{"pods", "pods/exec"}, whoCanFilterResources(all, "POD"),
		"filter is case-insensitive substring match so users can find pods/exec by typing exec")
	assert.Equal(t, []string{"pods/exec"}, whoCanFilterResources(all, "exec"))
}

func TestWhoCanFilterResources_NoMatchReturnsEmpty(t *testing.T) {
	assert.Empty(t, whoCanFilterResources([]string{"pods", "secrets"}, "xyz"))
}
