package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestRenderFindingDetailsFullFields(t *testing.T) {
	item := model.Item{
		Name:      "CVE-2024-1234",
		Namespace: "prod",
		Columns: []model.KeyValue{
			{Key: "Severity", Value: "CRIT"},
			{Key: "Resource", Value: "deploy/api"},
			{Key: "Title", Value: "CVE-2024-1234"},
			{Key: "Category", Value: "vuln"},
			{Key: "Source", Value: "trivy-operator"},
			{Key: "Description", Value: "A flaw in openssl"},
			{Key: "References", Value: "https://example.com/cve"},
			{Key: "Cve", Value: "CVE-2024-1234"},
			{Key: "Package", Value: "openssl"},
		},
	}
	out := RenderFindingDetails(item, 80, 30)
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "deploy/api")
	assert.Contains(t, out, "prod")
	assert.Contains(t, out, "trivy-operator")
	assert.Contains(t, out, "vuln")
	assert.Contains(t, out, "A flaw in openssl")
	assert.Contains(t, out, "https://example.com/cve")
	assert.Contains(t, out, "openssl")
	assert.Contains(t, out, "[Enter] jump to resource")
}

func TestRenderFindingDetailsMinimal(t *testing.T) {
	item := model.Item{
		Columns: []model.KeyValue{
			{Key: "Severity", Value: "LOW"},
			{Key: "Title", Value: "latest tag"},
		},
	}
	out := RenderFindingDetails(item, 80, 20)
	assert.Contains(t, out, "LOW")
	assert.Contains(t, out, "latest tag")
	assert.NotContains(t, out, "Namespace:")
}

func TestRenderFindingDetailsNarrowWidth(t *testing.T) {
	item := model.Item{
		Columns: []model.KeyValue{
			{Key: "Severity", Value: "HIGH"},
			{Key: "Title", Value: "x"},
			{Key: "Description", Value: strings.Repeat("word ", 30)},
		},
	}
	out := RenderFindingDetails(item, 40, 20)
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "HIGH")
}

func TestRenderFindingDetailsExtraColumnsRendered(t *testing.T) {
	item := model.Item{
		Columns: []model.KeyValue{
			{Key: "Severity", Value: "MED"},
			{Key: "Title", Value: "t"},
			{Key: "FixedVersion", Value: "1.2.3"},
			{Key: "Installed", Value: "1.0.0"},
		},
	}
	out := RenderFindingDetails(item, 80, 20)
	assert.Contains(t, out, "FixedVersion")
	assert.Contains(t, out, "1.2.3")
	assert.Contains(t, out, "Installed")
	assert.Contains(t, out, "1.0.0")
}

func TestWrapLinesShortInput(t *testing.T) {
	lines := wrapLines("hello", 80)
	assert.Equal(t, []string{"hello"}, lines)
}

func TestWrapLinesLongInput(t *testing.T) {
	lines := wrapLines("one two three four five", 10)
	assert.Equal(t, []string{"one two", "three four", "five"}, lines)
}

func TestWrapLinesPreservesNewlines(t *testing.T) {
	lines := wrapLines("first line\nsecond line", 80)
	assert.Equal(t, []string{"first line", "second line"}, lines)
}

func TestWrapLinesEmptyParagraph(t *testing.T) {
	lines := wrapLines("a\n\nb", 80)
	assert.Equal(t, []string{"a", "", "b"}, lines)
}

func TestStyleSeverityBadgeUnknown(t *testing.T) {
	out := styleSeverityBadge("UNKNOWN")
	assert.Contains(t, out, "?")
}

func TestRenderFindingGroupDetails(t *testing.T) {
	group := model.Item{
		Name: "Privileged Container",
		Kind: "__security_finding_group__",
		Columns: []model.KeyValue{
			{Key: "Severity", Value: "CRIT"},
			{Key: "Affected", Value: "3"},
			{Key: "Category", Value: "misconfig"},
			{Key: "Source", Value: "heuristic"},
			{Key: "Description", Value: "Runs as privileged"},
		},
	}
	affected := []model.Item{
		{
			Name:      "pod/web-1",
			Namespace: "default",
			Kind:      "__security_affected_resource__",
			Columns: []model.KeyValue{
				{Key: "Severity", Value: "CRIT"},
				{Key: "ResourceKind", Value: "Pod"},
				{Key: "Namespace", Value: "default"},
			},
		},
		{
			Name:      "pod/web-2",
			Namespace: "default",
			Kind:      "__security_affected_resource__",
			Columns: []model.KeyValue{
				{Key: "Severity", Value: "CRIT"},
				{Key: "ResourceKind", Value: "Pod"},
				{Key: "Namespace", Value: "default"},
			},
		},
	}

	out := RenderFindingGroupDetails(group, affected, 80, 30)

	// Group header and summary fields.
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "Privileged Container")
	assert.Contains(t, out, "3 resources")
	assert.Contains(t, out, "heuristic")
	assert.Contains(t, out, "misconfig")
	assert.Contains(t, out, "Runs as privileged")

	// Affected resources are listed.
	assert.Contains(t, out, "pod/web-1")
	assert.Contains(t, out, "pod/web-2")

	// Hint bar.
	assert.Contains(t, out, "[Enter/l] affected resources")
}
