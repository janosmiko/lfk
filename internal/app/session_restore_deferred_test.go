package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// deferredRestoreModel is a restore parked on CRD discovery: the saved type is
// not in the seed list, so the filter and cursor ride along on
// pendingSessionList until discovery answers.
func deferredRestoreModel() Model {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "ctx-a"
	m.restoringSession = true
	m.sessionResourceTypeAwaitingDiscovery = "argoproj.io/v1alpha1/workflowtemplates"
	m.sessionResourceNameAwaitingDiscovery = "backup-db"
	m.pendingSessionList = pendingSessionListState{
		armed: true, filter: "backup", cursorName: "backup-db", cursorNs: "ns-2",
	}
	return m
}

func TestDeferredRestore_DiscoveryErrorDropsTheParkedState(t *testing.T) {
	m := deferredRestoreModel()

	got, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "ctx-a",
		err:     errors.New("forbidden"),
	})

	assert.False(t, got.restoringSession)
	assert.Empty(t, got.sessionResourceTypeAwaitingDiscovery)
	assert.Empty(t, got.sessionResourceNameAwaitingDiscovery)
	assert.False(t, got.pendingSessionList.armed)
}

func TestDeferredRestore_FailedDiscoveryCannotResumeInAnotherContext(t *testing.T) {
	m := deferredRestoreModel()
	// The saved type exists in the next cluster the user opens. Without the
	// error path dropping the parked state, that cluster's discovery would
	// resume the dead restore and teleport the user into the list.
	m.sessionResourceTypeAwaitingDiscovery = "/v1/pods"

	got, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "ctx-a",
		err:     errors.New("forbidden"),
	})

	got.nav.Context = "ctx-b"
	got, _ = got.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "ctx-b",
		entries: []model.ResourceTypeEntry{
			{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
		},
	})

	assert.Equal(t, model.LevelResourceTypes, got.nav.Level,
		"switching clusters must not resume a restore that already failed")
	assert.Empty(t, got.filterText, "the dead restore's filter must not arrive either")
}

func TestDeferredRestore_UnresolvableTypeDropsTheParkedFilter(t *testing.T) {
	m := deferredRestoreModel()

	// Discovery answers, but this cluster has no such CRD.
	got, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "ctx-a",
		entries: []model.ResourceTypeEntry{
			{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
		},
	})

	assert.False(t, got.restoringSession)
	assert.Empty(t, got.sessionResourceTypeAwaitingDiscovery)
	assert.False(t, got.pendingSessionList.armed,
		"a filter that has nowhere to land must not wait for the next restore")
}

func TestDeferredRestore_ResolvableTypeStillAppliesTheParkedFilter(t *testing.T) {
	m := deferredRestoreModel()
	m.sessionResourceTypeAwaitingDiscovery = "/v1/pods"

	got, cmd := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "ctx-a",
		entries: []model.ResourceTypeEntry{
			{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
		},
	})

	assert.Equal(t, model.LevelResources, got.nav.Level)
	assert.Equal(t, "backup", got.filterText, "the parked filter still rides through")
	assert.Equal(t, "backup-db", got.pendingTarget)
	assert.Equal(t, "ns-2", got.pendingTargetNamespace,
		"without the namespace the cursor can land on a same-named row in another one")
	assert.NotNil(t, cmd)
}
