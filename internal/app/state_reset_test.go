package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- exitCanIView ---

func TestExitCanIView(t *testing.T) {
	m := Model{
		overlay:               overlayCanI,
		canIGroups:            []model.CanIGroup{{Name: "core"}},
		canIGroupCursor:       3,
		canIGroupScroll:       5,
		canIResourceScroll:    2,
		canISubject:           "system:serviceaccount:default:my-sa",
		canISubjectName:       "my-sa",
		canIServiceAccounts:   []string{"default/my-sa"},
		canISearchActive:      true,
		canISearchQuery:       "pods",
		canISubjectFilterMode: true,
		canIAllowedOnly:       true,
		canINamespaces:        []string{"default", "kube-system"},
	}

	m.exitCanIView()

	assert.Equal(t, overlayNone, m.overlay)
	assert.Nil(t, m.canIGroups)
	assert.Equal(t, 0, m.canIGroupCursor)
	assert.Equal(t, 0, m.canIGroupScroll)
	assert.Equal(t, 0, m.canIResourceScroll)
	assert.Empty(t, m.canISubject)
	assert.Empty(t, m.canISubjectName)
	assert.Nil(t, m.canIServiceAccounts)
	assert.False(t, m.canISearchActive)
	assert.Empty(t, m.canISearchQuery)
	assert.False(t, m.canISubjectFilterMode)
	assert.False(t, m.canIAllowedOnly)
	assert.Nil(t, m.canINamespaces)
}

// --- exitExplainView ---

func TestExitExplainView(t *testing.T) {
	m := Model{
		mode:                modeExplain,
		explainFields:       []model.ExplainField{{Name: "apiVersion"}},
		explainDesc:         "some desc",
		explainPath:         "spec.containers",
		explainResource:     "pods",
		explainAPIVersion:   "v1",
		explainTitle:        "pods.v1",
		explainCursor:       5,
		explainScroll:       10,
		explainSearchQuery:  "name",
		explainSearchActive: true,
	}

	m.exitExplainView()

	assert.Equal(t, modeExplorer, m.mode)
	assert.Nil(t, m.explainFields)
	assert.Empty(t, m.explainDesc)
	assert.Empty(t, m.explainPath)
	assert.Empty(t, m.explainResource)
	assert.Empty(t, m.explainAPIVersion)
	assert.Empty(t, m.explainTitle)
	assert.Equal(t, 0, m.explainCursor)
	assert.Equal(t, 0, m.explainScroll)
	assert.Empty(t, m.explainSearchQuery)
	assert.False(t, m.explainSearchActive)
}

// --- buildExplainResourceFromType ---

func TestBuildExplainResourceFromTypeEmpty(t *testing.T) {
	resource, apiVersion := buildExplainResourceFromType(model.ResourceTypeEntry{})
	assert.Empty(t, resource)
	assert.Empty(t, apiVersion)
}

func TestBuildExplainResourceFromTypeCoreResource(t *testing.T) {
	rt := model.ResourceTypeEntry{
		Resource:   "pods",
		APIGroup:   "",
		APIVersion: "v1",
	}
	resource, apiVersion := buildExplainResourceFromType(rt)
	assert.Equal(t, "pods", resource)
	assert.Empty(t, apiVersion) // no group, so empty
}

func TestBuildExplainResourceFromTypeCRD(t *testing.T) {
	rt := model.ResourceTypeEntry{
		Resource:   "applications",
		APIGroup:   "argoproj.io",
		APIVersion: "v1alpha1",
	}
	resource, apiVersion := buildExplainResourceFromType(rt)
	assert.Equal(t, "applications", resource)
	assert.Equal(t, "argoproj.io/v1alpha1", apiVersion)
}

// --- previewSchemeAtCursor ---

func TestPreviewSchemeAtCursorOutOfRange(t *testing.T) {
	m := Model{
		schemeCursor: 5,
	}
	// Should not panic with out-of-range cursor
	m.previewSchemeAtCursor([]string{"dark", "light"})
}

func TestPreviewSchemeAtCursorNegative(t *testing.T) {
	m := Model{
		schemeCursor: -1,
	}
	// Should not panic with negative cursor
	m.previewSchemeAtCursor([]string{"dark"})
}
