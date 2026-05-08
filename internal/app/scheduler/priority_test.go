package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriority_String(t *testing.T) {
	assert.Equal(t, "Critical", PriorityCritical.String())
	assert.Equal(t, "High", PriorityHigh.String())
	assert.Equal(t, "Low", PriorityLow.String())
}

func TestPriority_Order(t *testing.T) {
	// Higher priority = numerically lower so we can sort ascending.
	assert.Less(t, int(PriorityCritical), int(PriorityHigh))
	assert.Less(t, int(PriorityHigh), int(PriorityLow))
}

func TestDefaultPriorityFor_KnownKinds(t *testing.T) {
	cases := []struct {
		kind Kind
		want Priority
	}{
		{KindAPIDiscovery, PriorityCritical},
		{KindNamespaceList, PriorityCritical},
		{KindRBACCheck, PriorityCritical},
		{KindMutation, PriorityCritical},
		{KindResourceList, PriorityHigh},
		{KindContainers, PriorityHigh},
		{KindYAMLFetch, PriorityHigh},
		{KindMetrics, PriorityLow},
		{KindResourceTree, PriorityLow},
		{KindDashboard, PriorityLow},
	}
	for _, tc := range cases {
		t.Run(tc.kind.String(), func(t *testing.T) {
			assert.Equal(t, tc.want, DefaultPriorityFor(tc.kind))
		})
	}
}

func TestDefaultPriorityFor_UnknownKind(t *testing.T) {
	// Subprocess and any future Kind without an explicit mapping default to Low.
	assert.Equal(t, PriorityLow, DefaultPriorityFor(KindSubprocess))
	assert.Equal(t, PriorityLow, DefaultPriorityFor(Kind(9999)))
}
