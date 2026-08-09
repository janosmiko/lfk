package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// fieldDocModel returns a model sitting on a Pod, which is what both viewers
// show when the footnote pane is opened.
func fieldDocModel() Model {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.fieldDoc.cache = newFieldDocCache()
	return m
}

func TestFieldDocKeyForPath(t *testing.T) {
	m := fieldDocModel()

	key, ok := m.fieldDocKeyForPath([]string{"spec", "containers", "[0]", "image"})

	require.True(t, ok)
	assert.Equal(t, "pods", key.resource)
	assert.Equal(t, "spec.containers.image", key.path, "array indices must be stripped")
	assert.Equal(t, m.effectiveContext(), key.context)
}

func TestFieldDocKeyForPathRejectsUnknownResource(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{}

	_, ok := m.fieldDocKeyForPath([]string{"spec"})

	assert.False(t, ok, "no resource type means no schema to ask for")
}

func TestFieldDocKeyForPathCarriesCRDAPIVersion(t *testing.T) {
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Cluster", APIGroup: "postgresql.cnpg.io", APIVersion: "v1",
		Resource: "clusters", Namespaced: true,
	}

	key, ok := m.fieldDocKeyForPath([]string{"spec", "instances"})

	require.True(t, ok)
	assert.Equal(t, "clusters", key.resource)
	assert.Equal(t, "postgresql.cnpg.io/v1", key.apiVersion, "a CRD resolves through its group/version")
	assert.Equal(t, "spec.instances", key.path)
}

func TestFieldDocLoadedStoresAndCaches(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	m.fieldDoc.loading = true
	m.fieldDoc.req = 7
	key := fieldDocKey{context: "test-ctx", apiVersion: "v1", resource: "pods", path: "spec.dnsPolicy"}
	m.fieldDoc.key = key

	got := m.updateFieldDocLoaded(fieldDocLoadedMsg{
		req:   7,
		key:   key,
		entry: fieldDocEntry{fieldType: "<string>", desc: "Set DNS policy for the pod."},
	})

	assert.False(t, got.fieldDoc.loading)
	assert.Equal(t, "Set DNS policy for the pod.", got.fieldDoc.entry.desc)
	assert.Equal(t, "<string>", got.fieldDoc.entry.fieldType)

	cached, ok := got.fieldDoc.cache.get(key)
	require.True(t, ok, "a loaded description must be cached")
	assert.Equal(t, "Set DNS policy for the pod.", cached.desc)
}

func TestFieldDocLoadedDropsStaleReply(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	m.fieldDoc.loading = true
	m.fieldDoc.req = 9
	m.fieldDoc.entry = fieldDocEntry{desc: "current field"}

	// A reply for request 8 arrives after the cursor already moved to 9.
	got := m.updateFieldDocLoaded(fieldDocLoadedMsg{
		req:   8,
		key:   fieldDocKey{path: "spec.old"},
		entry: fieldDocEntry{desc: "stale field"},
	})

	assert.Equal(t, "current field", got.fieldDoc.entry.desc, "a late reply must not overwrite the new field")
	assert.True(t, got.fieldDoc.loading, "the in-flight fetch must still be pending")
}

func TestFieldDocLoadedIgnoredWhenPaneClosed(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = false
	m.fieldDoc.req = 3

	got := m.updateFieldDocLoaded(fieldDocLoadedMsg{
		req:   3,
		key:   fieldDocKey{path: "spec.dnsPolicy"},
		entry: fieldDocEntry{desc: "arrived too late"},
	})

	assert.Empty(t, got.fieldDoc.entry.desc, "closing the pane wins over an in-flight reply")
}

func TestFieldDocLoadedRecordsError(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	m.fieldDoc.loading = true
	m.fieldDoc.req = 1

	got := m.updateFieldDocLoaded(fieldDocLoadedMsg{
		req: 1,
		key: fieldDocKey{path: "spec.nope"},
		err: errors.New("field \"nope\" does not exist"),
	})

	assert.False(t, got.fieldDoc.loading)
	assert.True(t, got.fieldDoc.on, "an error keeps the pane open so the message is readable")
	assert.Contains(t, got.fieldDoc.err, "does not exist")
}

// A cancelled fetch is the app shutting work down, not a schema problem, so it
// must not paint an error into the pane.
func TestFieldDocLoadedIgnoresCancellation(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	m.fieldDoc.loading = true
	m.fieldDoc.req = 1

	got := m.updateFieldDocLoaded(fieldDocLoadedMsg{
		req: 1, key: fieldDocKey{path: "spec"}, err: context.Canceled,
	})

	assert.Empty(t, got.fieldDoc.err)
	assert.False(t, got.fieldDoc.loading)
}

func TestFieldDocShowFieldUsesCacheWithoutFetching(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	key, ok := m.fieldDocKeyForPath([]string{"spec", "dnsPolicy"})
	require.True(t, ok)
	m.fieldDoc.cache.put(key, fieldDocEntry{fieldType: "<string>", desc: "cached text"})

	got, cmd := m.showFieldDoc([]string{"spec", "dnsPolicy"})

	assert.Nil(t, cmd, "a cache hit must not schedule a fetch")
	assert.False(t, got.fieldDoc.loading)
	assert.Equal(t, "cached text", got.fieldDoc.entry.desc)
}

func TestFieldDocShowFieldSchedulesOnMiss(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true

	got, cmd := m.showFieldDoc([]string{"spec", "dnsPolicy"})

	require.NotNil(t, cmd, "a cache miss must schedule the debounced fetch")
	assert.True(t, got.fieldDoc.loading)
	assert.Equal(t, "spec.dnsPolicy", got.fieldDoc.key.path)
}

// Moving the cursor bumps the request number, so the debounce timer started for
// the previous line finds itself stale and never spawns kubectl.
func TestFieldDocDebounceDropsSupersededRequest(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	m.fieldDoc.loading = true
	m.fieldDoc.req = 4

	_, cmd := m.updateFieldDocDebounce(fieldDocDebounceMsg{req: 3})

	assert.Nil(t, cmd, "a superseded debounce tick must not fetch")
}

func TestFieldDocDebounceIgnoredWhenPaneClosed(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = false
	m.fieldDoc.req = 2

	_, cmd := m.updateFieldDocDebounce(fieldDocDebounceMsg{req: 2})

	assert.Nil(t, cmd, "a closed pane must not fetch")
}

// Dropping a superseded reply is not enough: the kubectl process would keep a
// scheduler worker until its own deadline. Moving on has to cancel it.
func TestFieldDocRetargetCancelsTheFetchInFlight(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	ctx, cancel := context.WithCancel(t.Context())
	m.fieldDoc.cancel = cancel
	m.fieldDoc.key, _ = m.fieldDocKeyForPath([]string{"spec", "dnsPolicy"})
	m.fieldDoc.loading = true

	got, _ := m.showFieldDoc([]string{"spec", "restartPolicy"})

	require.Error(t, ctx.Err(), "the superseded fetch must be cancelled")
	assert.Nil(t, got.fieldDoc.cancel, "the spent cancel must not be kept")
}

func TestFieldDocCloseCancelsTheFetchInFlight(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	ctx, cancel := context.WithCancel(t.Context())
	m.fieldDoc.cancel = cancel

	mdl, _ := m.toggleFieldDoc([]string{"spec", "dnsPolicy"})

	require.Error(t, ctx.Err(), "closing the pane must cancel the fetch")
	assert.False(t, mdl.(Model).fieldDoc.on)
}

// A finished fetch releases its context rather than leaving it to the parent.
func TestFieldDocLoadedReleasesTheRequestContext(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	m.fieldDoc.loading = true
	m.fieldDoc.req = 2
	ctx, cancel := context.WithCancel(t.Context())
	m.fieldDoc.cancel = cancel
	key := fieldDocKey{context: "test-ctx", resource: "pods", path: "spec.dnsPolicy"}

	got := m.updateFieldDocLoaded(fieldDocLoadedMsg{req: 2, key: key, entry: fieldDocEntry{desc: "d"}})

	require.Error(t, ctx.Err(), "a completed fetch must release its context")
	assert.Nil(t, got.fieldDoc.cancel)
}

// A cache hit answers without a fetch, so anything still running is stale.
func TestFieldDocCacheHitCancelsTheFetchInFlight(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	ctx, cancel := context.WithCancel(t.Context())
	m.fieldDoc.cancel = cancel
	key, ok := m.fieldDocKeyForPath([]string{"spec", "dnsPolicy"})
	require.True(t, ok)
	m.fieldDoc.cache.put(key, fieldDocEntry{desc: "cached"})

	got, cmd := m.showFieldDoc([]string{"spec", "dnsPolicy"})

	assert.Nil(t, cmd)
	assert.Equal(t, "cached", got.fieldDoc.entry.desc)
	require.Error(t, ctx.Err(), "the stale fetch must be cancelled on a cache hit")
}

func TestFieldDocCancelInFlightIsSafeWhenIdle(t *testing.T) {
	s := &fieldDocState{}

	assert.NotPanics(t, func() {
		s.cancelInFlight()
		s.cancelInFlight()
	}, "cancelling with nothing running must be a no-op")
}

func TestFieldDocToggleClosesAndClears(t *testing.T) {
	m := fieldDocModel()
	m.fieldDoc.on = true
	m.fieldDoc.entry = fieldDocEntry{desc: "something"}
	m.fieldDoc.err = "boom"

	mdl, _ := m.toggleFieldDoc([]string{"spec", "dnsPolicy"})
	got := mdl.(Model)

	assert.False(t, got.fieldDoc.on)
	assert.Empty(t, got.fieldDoc.entry.desc)
	assert.Empty(t, got.fieldDoc.err)
}

func TestFieldDocToggleOpens(t *testing.T) {
	m := fieldDocModel()

	mdl, cmd := m.toggleFieldDoc([]string{"spec", "dnsPolicy"})
	got := mdl.(Model)

	assert.True(t, got.fieldDoc.on)
	assert.NotNil(t, cmd, "opening on a cache miss must schedule a fetch")
}
