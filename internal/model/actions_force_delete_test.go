package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForceDeleteHotkeyConsistency verifies that every "Force Delete" entry
// across the action-menu variants uses the same hotkey ("X") and a
// description that follows the same "Force delete..." prefix convention as
// the regular "Delete" entries. The point of this test is to prevent the
// re-introduction of inconsistent help texts (#issue: "Force delete hotkeys
// are showing different help texts").
func TestForceDeleteHotkeyConsistency(t *testing.T) {
	tests := []struct {
		name        string
		items       []ActionMenuItem
		wantPrefix  string
		wantNoFlags bool // descriptions must not leak kubectl flags
	}{
		{name: "Pod", items: ActionsForKind("Pod"), wantPrefix: "Force delete this pod"},
		{name: "Job", items: ActionsForKind("Job"), wantPrefix: "Force delete this job"},
		{name: "Bulk", items: ActionsForBulk(""), wantPrefix: "Force delete selected resources"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item, ok := findAction(tc.items, "Force Delete")
			require.True(t, ok, "%s actions must include Force Delete", tc.name)
			assert.Equal(t, "X", item.Key, "Force Delete must be on key X")
			assert.True(t,
				strings.HasPrefix(item.Description, tc.wantPrefix),
				"%s Force Delete description must start with %q, got %q",
				tc.name, tc.wantPrefix, item.Description)
			assert.NotContains(t, item.Description, "--force",
				"description must not leak kubectl flags")
			assert.NotContains(t, item.Description, "grace-period",
				"description must not leak kubectl flags")
		})
	}
}
