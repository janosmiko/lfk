package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSecurityHookStateClonesAvailabilityMap guards the defensive-copy
// fix: setSecurityHookState must clone the caller's availability map so
// later mutations on the caller side (e.g., Model.maps.Copy after the
// next probe result) can't be observed by hook readers outside the
// lock — currentSecurityHookState must likewise return a clone.
func TestSecurityHookStateClonesAvailabilityMap(t *testing.T) {
	prev, prevAvail := currentSecurityHookState()
	t.Cleanup(func() { setSecurityHookState(prev, prevAvail) })

	caller := map[string]bool{"trivy": true, "falco": false}
	setSecurityHookState(nil, caller)

	caller["trivy"] = false // mutate after publishing

	_, snapshot := currentSecurityHookState()
	assert.True(t, snapshot["trivy"], "publisher mutation must not affect hook reads")

	snapshot["trivy"] = false // reader tamper
	_, second := currentSecurityHookState()
	assert.True(t, second["trivy"], "reader mutation must not affect later hook reads")
}
