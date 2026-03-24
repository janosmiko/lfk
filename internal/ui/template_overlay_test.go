package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestRenderTemplateOverlayShowsFilterInput(t *testing.T) {
	templates := []model.ResourceTemplate{
		{Name: "Deployment", Description: "Create a Deployment", Category: "Workloads"},
		{Name: "Service", Description: "Create a Service", Category: "Networking"},
	}
	result := RenderTemplateOverlay(templates, "dep", 0, true, 25)
	assert.Contains(t, result, "filter>")
	assert.Contains(t, result, "dep")
}

func TestRenderTemplateOverlayShowsFilterLabel(t *testing.T) {
	templates := []model.ResourceTemplate{
		{Name: "Deployment", Description: "Create a Deployment", Category: "Workloads"},
	}
	result := RenderTemplateOverlay(templates, "dep", 0, false, 25)
	assert.Contains(t, result, "filter:")
	assert.Contains(t, result, "dep")
}

func TestRenderTemplateOverlayNoFilterHintRemoved(t *testing.T) {
	// Hints now live in the main status bar, not inline.
	templates := []model.ResourceTemplate{
		{Name: "Deployment", Description: "Create a Deployment", Category: "Workloads"},
	}
	result := RenderTemplateOverlay(templates, "", 0, false, 25)
	assert.NotContains(t, result, "/: filter")
}

func TestRenderTemplateOverlayEmptyTemplates(t *testing.T) {
	result := RenderTemplateOverlay(nil, "", 0, false, 25)
	assert.Contains(t, result, "No templates available")
}

func TestRenderTemplateOverlayNoMatchingTemplates(t *testing.T) {
	templates := []model.ResourceTemplate{
		{Name: "Deployment", Description: "Create a Deployment", Category: "Workloads"},
	}
	// Pass empty slice (caller filters before passing).
	result := RenderTemplateOverlay(templates[:0], "xyz", 0, false, 25)
	assert.Contains(t, result, "No templates available")
}

func TestRenderTemplateOverlayShowsNameOnly(t *testing.T) {
	templates := []model.ResourceTemplate{
		{Name: "Deployment", Description: "Create a Deployment", Category: "Workloads"},
		{Name: "Service", Description: "Create a Service", Category: "Networking"},
	}
	result := RenderTemplateOverlay(templates, "", 0, false, 25)
	assert.Contains(t, result, "Deployment")
	assert.Contains(t, result, "Service")
	assert.NotContains(t, result, "Create a Deployment")
	assert.NotContains(t, result, "Create a Service")
}
