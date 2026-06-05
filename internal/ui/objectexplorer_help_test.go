package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestObjectExplorerHelpContextReachable guards against drift between the
// "Object Explorer" context string set in handleObjectExplorerKey and the help
// section's context field. If they diverge, the help overlay would show no
// Object Explorer section.
func TestObjectExplorerHelpContextReachable(t *testing.T) {
	lines := BuildHelpLines("", "Object Explorer", 100)
	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "Object Explorer")
	assert.Contains(t, joined, "Drill into object/array field")
}
