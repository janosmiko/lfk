package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestRenderTitleBarSingleLine verifies that renderTitleBar always produces
// a single-line output regardless of content width, watch mode, version label,
// namespace configuration, or long breadcrumb text.
//
// This is a regression test for a bug where the title bar appeared duplicated
// (2-3 visual lines) because the lipgloss style was missing MaxWidth and
// MaxHeight constraints.
func TestRenderTitleBarSingleLine(t *testing.T) {
	tests := []struct {
		name  string
		model Model
	}{
		{
			name: "normal width",
			model: Model{
				width:     120,
				height:    30,
				namespace: "default",
				nav: model.NavigationState{
					Context: "my-cluster",
					Level:   model.LevelResources,
					ResourceType: model.ResourceTypeEntry{
						DisplayName: "Pods",
					},
				},
				tabs: []TabState{{}},
			},
		},
		{
			name: "narrow width truncates breadcrumb",
			model: Model{
				width:     40,
				height:    30,
				namespace: "default",
				nav: model.NavigationState{
					Context: "my-cluster",
					Level:   model.LevelResources,
					ResourceType: model.ResourceTypeEntry{
						DisplayName: "Pods",
					},
				},
				tabs: []TabState{{}},
			},
		},
		{
			name: "with watch indicator",
			model: Model{
				width:     120,
				height:    30,
				namespace: "default",
				watchMode: true,
				nav: model.NavigationState{
					Context: "my-cluster",
					Level:   model.LevelResources,
					ResourceType: model.ResourceTypeEntry{
						DisplayName: "Pods",
					},
				},
				tabs: []TabState{{}},
			},
		},
		{
			name: "with version label",
			model: Model{
				width:     120,
				height:    30,
				namespace: "default",
				version:   "v1.0.0",
				nav: model.NavigationState{
					Context: "my-cluster",
					Level:   model.LevelResources,
					ResourceType: model.ResourceTypeEntry{
						DisplayName: "Pods",
					},
				},
				tabs: []TabState{{}},
			},
		},
		{
			name: "with all namespaces",
			model: Model{
				width:         120,
				height:        30,
				namespace:     "default",
				allNamespaces: true,
				nav: model.NavigationState{
					Context: "my-cluster",
					Level:   model.LevelResources,
					ResourceType: model.ResourceTypeEntry{
						DisplayName: "Pods",
					},
				},
				tabs: []TabState{{}},
			},
		},
		{
			name: "with multiple selected namespaces",
			model: Model{
				width:     120,
				height:    30,
				namespace: "default",
				selectedNamespaces: map[string]bool{
					"alpha":   true,
					"bravo":   true,
					"charlie": true,
					"delta":   true,
					"echo":    true,
				},
				nav: model.NavigationState{
					Context: "my-cluster",
					Level:   model.LevelResources,
					ResourceType: model.ResourceTypeEntry{
						DisplayName: "Pods",
					},
				},
				tabs: []TabState{{}},
			},
		},
		{
			name: "wide content that would wrap without MaxWidth",
			model: Model{
				width:     60,
				height:    30,
				namespace: "default",
				version:   "v1.0.0",
				watchMode: true,
				selectedNamespaces: map[string]bool{
					"very-long-namespace-alpha":   true,
					"very-long-namespace-bravo":   true,
					"very-long-namespace-charlie": true,
					"very-long-namespace-delta":   true,
					"very-long-namespace-echo":    true,
				},
				nav: model.NavigationState{
					Context: "extremely-long-kubernetes-cluster-context-name-that-exceeds-normal-width",
					Level:   model.LevelResources,
					ResourceType: model.ResourceTypeEntry{
						DisplayName: "CustomResourceDefinitions",
					},
					ResourceName: "my-very-long-custom-resource-name-that-also-exceeds-normal-width",
				},
				tabs: []TabState{{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.model.renderTitleBar()

			// The rendered output must not be empty.
			assert.NotEmpty(t, output, "renderTitleBar returned empty output")

			// Check visual height via lipgloss: the title bar must occupy exactly 1 line.
			visualHeight := lipgloss.Height(output)
			assert.Equal(t, 1, visualHeight,
				"title bar visual height should be 1, got %d", visualHeight)

			// Count embedded newlines in the raw output.
			// A properly constrained single-line render should have at most 1 trailing newline.
			newlineCount := strings.Count(output, "\n")
			assert.LessOrEqual(t, newlineCount, 1,
				"title bar should have at most 1 newline (trailing), got %d", newlineCount)

			// Verify the stripped output contains expected breadcrumb prefix.
			stripped := stripANSI(output)
			assert.Contains(t, stripped, "lfk",
				"title bar should contain the app name in the breadcrumb")
		})
	}
}
