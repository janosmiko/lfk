package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestExportTemplateCmd_SecurityView_ReportsStatus guards TASK-891 part 3:
// exportTemplateCmd used to return a bare nil for a synthetic security row,
// leaving the user with no feedback at all when the "T" chip did nothing.
// The returned command is scheduleStatusClear's timer, not nil — the fix is
// the status message, not the absence of a command.
func TestExportTemplateCmd_SecurityView_ReportsStatus(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}

	got, cmd := m.exportTemplateCmd()

	assert.NotNil(t, cmd, "expected the status-clear timer command, not the bare nil this guards against")
	assert.True(t, got.statusMessageErr)
	assert.Equal(t, "Export Template: security findings have no manifest to export", got.statusMessage)
}

// TestExportTemplateCmd_UnresolvableSelection_ReportsStatus guards the other
// silent-failure path: resolveTemplateSource returning ok=false (nothing
// selected, or a level/kind combination with no manifest behind it) used to
// also return a bare nil.
func TestExportTemplateCmd_UnresolvableSelection_ReportsStatus(t *testing.T) {
	m := basePush80Model()
	// Clear the fixture's default selection so selectedMiddleItem() returns
	// nil and resolveTemplateSource reports ok=false.
	m.middleItems = nil

	got, cmd := m.exportTemplateCmd()

	assert.NotNil(t, cmd, "expected the status-clear timer command, not the bare nil this guards against")
	assert.True(t, got.statusMessageErr)
	assert.Equal(t, "Export Template: nothing selected to export", got.statusMessage)
}

// TestExportTemplateCmd_SecurityAndUnresolvable_DistinctMessages: the two
// silent-failure causes are different (a synthetic view vs. an unresolvable
// selection) and must not share a single generic message.
func TestExportTemplateCmd_SecurityAndUnresolvable_DistinctMessages(t *testing.T) {
	security := basePush80Model()
	security.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	securityResult, _ := security.exportTemplateCmd()

	unresolvable := basePush80Model()
	unresolvable.middleItems = nil
	unresolvableResult, _ := unresolvable.exportTemplateCmd()

	assert.NotEqual(t, securityResult.statusMessage, unresolvableResult.statusMessage)
}
