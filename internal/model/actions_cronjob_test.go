package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestActionsForKindCronJobToggleSuspend verifies the CronJob action menu
// exposes a single Suspend/Resume entry with its expected shortcut.
func TestActionsForKindCronJobToggleSuspend(t *testing.T) {
	items := ActionsForKind("CronJob")

	toggle, ok := findAction(items, "Suspend/Resume")
	assert.True(t, ok, "CronJob actions must include Suspend/Resume")
	assert.Equal(t, "S", toggle.Key)

	// The schedule toggle is one action, not a Suspend/Resume pair.
	_, hasSuspend := findAction(items, "Suspend CronJob")
	assert.False(t, hasSuspend, "CronJob actions must not carry a separate Suspend entry")
	_, hasResume := findAction(items, "Resume CronJob")
	assert.False(t, hasResume, "CronJob actions must not carry a separate Resume entry")
}
