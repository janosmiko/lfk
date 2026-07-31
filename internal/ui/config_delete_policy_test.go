package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func snapshotDeletePolicyGlobal(t *testing.T) {
	t.Helper()
	original := ConfigDeletePropagationPolicy
	t.Cleanup(func() { ConfigDeletePropagationPolicy = original })
}

// An unknown delete_propagation_policy must fall back to the compiled default
// rather than leaving the policy unset, which would defer to the API server's
// per-resource default (OrphanDependents for batch/v1 Jobs).
func TestDeletePropagationPolicy_InvalidFallsBack(t *testing.T) {
	snapshotDeletePolicyGlobal(t)

	path := writeConfigFile(t, "delete_propagation_policy: cascade-please\n")
	LoadConfig(path)

	assert.Equal(t, model.DeletePropagationBackground, ConfigDeletePropagationPolicy)
}

func TestDeletePropagationPolicy_OmittedKeepsDefault(t *testing.T) {
	snapshotDeletePolicyGlobal(t)

	path := writeConfigFile(t, "confirm_on_exit: true\n")
	LoadConfig(path)

	assert.Equal(t, model.DeletePropagationBackground, ConfigDeletePropagationPolicy)
}

// applyDeletePropagationPolicy must not depend on the global's prior value: a
// second load with an invalid or omitted key has to reset to Background, or the
// warning would claim Background while a stale orphan stayed active.
func TestDeletePropagationPolicy_ResetsStaleValue(t *testing.T) {
	tests := map[string]string{
		"invalid": "delete_propagation_policy: cascade-please\n",
		"omitted": "confirm_on_exit: true\n",
		"empty":   "delete_propagation_policy: \"\"\n",
	}
	for name, yaml := range tests {
		t.Run(name, func(t *testing.T) {
			snapshotDeletePolicyGlobal(t)

			LoadConfig(writeConfigFile(t, "delete_propagation_policy: orphan\n"))
			require.Equal(t, model.DeletePropagationOrphan, ConfigDeletePropagationPolicy,
				"precondition: orphan must be active before the second load")

			LoadConfig(writeConfigFile(t, yaml))
			assert.Equal(t, model.DeletePropagationBackground, ConfigDeletePropagationPolicy)
		})
	}
}

func TestDeletePropagationPolicy_AcceptsEachPolicy(t *testing.T) {
	for raw, want := range map[string]model.DeletePropagation{
		"background": model.DeletePropagationBackground,
		"foreground": model.DeletePropagationForeground,
		"orphan":     model.DeletePropagationOrphan,
		"Orphan":     model.DeletePropagationOrphan,
		"none":       model.DeletePropagationNone,
	} {
		t.Run(raw, func(t *testing.T) {
			snapshotDeletePolicyGlobal(t)

			path := writeConfigFile(t, "delete_propagation_policy: "+raw+"\n")
			LoadConfig(path)

			assert.Equal(t, want, ConfigDeletePropagationPolicy)
		})
	}
}
