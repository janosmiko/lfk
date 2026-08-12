package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// renderedStatus is what the user actually sees. The bug this file guards was
// an invisible failure, so asserting on the model field alone would leave the
// same gap: a message set but never painted still tells nobody.
func renderedStatus(t *testing.T, m Model) string {
	t.Helper()
	return stripANSI(m.View().Content)
}

// TestExportTemplateCmd_SecurityView_ReportsStatus guards TASK-891 part 3: the
// "T" chip used to do nothing at all on a synthetic security row. The command
// asserted on is scheduleStatusClear's timer, not the fix itself.
func TestExportTemplateCmd_SecurityView_ReportsStatus(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}

	got, cmd := m.exportTemplateCmd()

	assert.NotNil(t, cmd, "expected the status-clear timer command, not the bare nil this guards against")
	assert.True(t, got.statusMessageErr)
	assert.Equal(t, "Export Template: security findings have no manifest to export", got.statusMessage)
	assert.Contains(t, renderedStatus(t, got), "security findings have no manifest to export")
}

// TestExportTemplateCmd_UnresolvableSelection_ReportsStatus guards the other
// silent-failure path: resolveTemplateSource reporting ok=false.
func TestExportTemplateCmd_UnresolvableSelection_ReportsStatus(t *testing.T) {
	m := basePush80Model()
	// Clear the fixture's default selection so selectedMiddleItem() returns
	// nil and resolveTemplateSource reports ok=false.
	m.middleItems = nil

	got, cmd := m.exportTemplateCmd()

	assert.NotNil(t, cmd, "expected the status-clear timer command, not the bare nil this guards against")
	assert.True(t, got.statusMessageErr)
	assert.Equal(t, "Export Template: nothing selected to export", got.statusMessage)
	assert.Contains(t, renderedStatus(t, got), "nothing selected to export")
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
	assert.NotEqual(t, renderedStatus(t, securityResult), renderedStatus(t, unresolvableResult),
		"two different causes must read differently on screen, not just in the model")
}
