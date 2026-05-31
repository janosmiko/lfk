package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/security"
)

func TestExecuteActionSecurityFindingsLoading(t *testing.T) {
	m := Model{}
	m.actionCtx = actionContext{kind: "Pod", name: "nginx", namespace: "default"}

	updated := m.executeActionSecurityFindings()
	assert.Contains(t, updated.statusMessage, "still loading",
		"with no index built, the action surfaces a loading hint")
}

func TestExecuteActionSecurityFindingsNoFindings(t *testing.T) {
	m := Model{securityModelState: securityModelState{
		securityIndex: security.BuildFindingIndex(nil),
	}}
	m.actionCtx = actionContext{kind: "Pod", name: "nginx", namespace: "default"}

	updated := m.executeActionSecurityFindings()
	assert.Contains(t, updated.statusMessage, "No security findings",
		"empty index for the resource -> 'no findings' message")
	assert.Contains(t, updated.statusMessage, "Pod/nginx",
		"status message names the resource so the user knows what was checked")
}

func TestExecuteActionSecurityFindingsCounts(t *testing.T) {
	ref := security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"}
	idx := security.BuildFindingIndex([]security.Finding{
		{ID: "1", Title: "crit", Severity: security.SeverityCritical, Resource: ref},
		{ID: "2", Title: "high1", Severity: security.SeverityHigh, Resource: ref},
		{ID: "3", Title: "high2", Severity: security.SeverityHigh, Resource: ref},
	})
	m := Model{securityModelState: securityModelState{securityIndex: idx}}
	m.actionCtx = actionContext{kind: "Pod", name: "nginx", namespace: "default"}

	updated := m.executeActionSecurityFindings()
	msg := updated.statusMessage

	assert.Contains(t, msg, "3 security findings",
		"total count surfaces in the prefix")
	assert.Contains(t, msg, "1 critical",
		"per-severity breakdown lists critical count")
	assert.Contains(t, msg, "2 high",
		"per-severity breakdown lists high count")
	assert.Contains(t, msg, "Pod/nginx",
		"status message names the resource")
	assert.Contains(t, msg, "Security category",
		"hint points the user toward the drill-in path")
}
