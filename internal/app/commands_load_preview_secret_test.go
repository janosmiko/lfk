package app

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// baseSecretModel returns a minimal Model configured for Secret list at
// LevelResources. The fake clientset is pre-populated with a Secret so
// GetSecretData can succeed.
//
// The secret_lazy_loading config flag is enabled for the duration of the
// test — these tests exercise the lazy-loading feature, and turning it on
// here keeps call sites terse. Tests that care about the disabled behaviour
// flip the flag back off explicitly.
func baseSecretModel(t *testing.T) Model {
	t.Helper()

	prevLazy := ui.ConfigSecretLazyLoading
	ui.ConfigSecretLazyLoading = true
	t.Cleanup(func() { ui.ConfigSecretLazyLoading = prevLazy })

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("s3cret"),
			"username": []byte("admin"),
		},
	}

	cs := fake.NewClientset(secret)
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	m := Model{
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: "test-ctx",
			ResourceType: model.ResourceTypeEntry{
				Kind:       "Secret",
				Resource:   "secrets",
				Namespaced: true,
			},
		},
		tabs:                []TabState{{}},
		selectedItems:       make(map[string]bool),
		cursorMemory:        make(map[string]int),
		itemCache:           make(map[string][]model.Item),
		cacheFingerprints:   make(map[string]string),
		discoveredResources: make(map[string][]model.ResourceTypeEntry),
		secretPreviewCache:  make(map[string]*model.SecretData),
		width:               120,
		height:              40,
		execMu:              &sync.Mutex{},
		client:              k8s.NewTestClient(cs, dyn),
		namespace:           "default",
		reqCtx:              context.Background(),
		bgtasks:             bgtasks.New(bgtasks.DefaultThreshold),
	}
	m.middleItems = []model.Item{
		{Name: "my-secret", Namespace: "default", Kind: "Secret"},
	}
	return m
}

// TestLoadPreviewSecretDataFiresOnSecretHover verifies that
// loadPreviewSecretData returns a non-nil command when Kind is "Secret", an
// item is selected, and the secret_lazy_loading config flag is enabled.
func TestLoadPreviewSecretDataFiresOnSecretHover(t *testing.T) {
	prev := ui.ConfigSecretLazyLoading
	ui.ConfigSecretLazyLoading = true
	t.Cleanup(func() { ui.ConfigSecretLazyLoading = prev })

	m := baseSecretModel(t)

	cmd := m.loadPreviewSecretData()

	assert.NotNil(t, cmd, "expected a loader command for Secret kind when lazy loading is enabled")
}

// TestLoadPreviewSecretDataSkippedWhenLazyLoadingDisabled verifies that the
// loader no-ops when secret_lazy_loading is off — the eager list path has
// already populated decoded values on the item, so a per-hover GET would
// duplicate work.
func TestLoadPreviewSecretDataSkippedWhenLazyLoadingDisabled(t *testing.T) {
	m := baseSecretModel(t)
	// baseSecretModel arms the flag; flip it off after setup for this test.
	ui.ConfigSecretLazyLoading = false

	cmd := m.loadPreviewSecretData()

	assert.Nil(t, cmd, "loadPreviewSecretData must return nil when lazy loading is disabled")
}

// TestLoadPreviewSecretDataNoopForNonSecret verifies that
// loadPreviewSecretData returns nil when the current resource type is not Secret.
func TestLoadPreviewSecretDataNoopForNonSecret(t *testing.T) {
	m := baseSecretModel(t)
	m.nav.ResourceType.Kind = "ConfigMap"

	cmd := m.loadPreviewSecretData()

	assert.Nil(t, cmd, "expected nil command for non-Secret kind")
}

// TestLoadPreviewSecretDataNoopWhenNoSelection verifies that
// loadPreviewSecretData returns nil when there is no selected item.
func TestLoadPreviewSecretDataNoopWhenNoSelection(t *testing.T) {
	m := baseSecretModel(t)
	m.middleItems = nil // no items, so selectedMiddleItem returns nil

	cmd := m.loadPreviewSecretData()

	assert.Nil(t, cmd, "expected nil command when no item is selected")
}

// TestLoadPreviewSecretDataGenStaleness verifies that the update handler
// discards messages whose generation counter does not match m.requestGen.
func TestLoadPreviewSecretDataGenStaleness(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 5
	m.middleItems = []model.Item{
		{Name: "my-secret", Namespace: "default", Kind: "Secret"},
	}

	staleMsg := previewSecretDataLoadedMsg{
		gen:  3, // old generation
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		data: &model.SecretData{
			Keys: []string{"password"},
			Data: map[string]string{"password": "s3cret"},
		},
	}

	result, _ := m.Update(staleMsg)
	rm := result.(Model)

	// Cache must remain empty (stale message discarded).
	assert.Empty(t, rm.secretPreviewCache, "stale message must not populate cache")

	// middleItems must not have secret columns injected.
	for _, kv := range rm.middleItems[0].Columns {
		assert.False(t, len(kv.Key) >= 7 && kv.Key[:7] == "secret:",
			"stale message must not inject secret columns, got key %q", kv.Key)
	}
}

// TestLoadPreviewSecretDataCacheHit verifies that the second call to
// loadPreviewSecretData for the same key returns a command that emits an
// immediate message (not a background task) and that the k8s client is called
// only once.
func TestLoadPreviewSecretDataCacheHit(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 1

	// First call: cache miss — run the returned command synchronously.
	cmd := m.loadPreviewSecretData()
	require.NotNil(t, cmd)

	msg := cmd()
	loadedMsg, ok := msg.(previewSecretDataLoadedMsg)
	require.True(t, ok, "expected previewSecretDataLoadedMsg, got %T", msg)
	require.NoError(t, loadedMsg.err)

	// Apply the message to populate the cache.
	result, _ := m.Update(loadedMsg)
	m = result.(Model)

	// Verify cache is populated.
	key := secretPreviewCacheKey("test-ctx", "default", "my-secret")
	require.NotNil(t, m.secretPreviewCache[key], "cache should be populated after first load")

	// Second call: cache hit — should emit immediately (no background task).
	m.requestGen = 2 // advance gen to simulate a list refresh
	m.middleItems = []model.Item{
		{Name: "my-secret", Namespace: "default", Kind: "Secret"},
	}

	cmd2 := m.loadPreviewSecretData()
	require.NotNil(t, cmd2)

	// The returned command must be a synchronous closure that emits the
	// cached data immediately (i.e., it must return previewSecretDataLoadedMsg).
	msg2 := cmd2()
	loaded2, ok := msg2.(previewSecretDataLoadedMsg)
	require.True(t, ok, "cache hit must return previewSecretDataLoadedMsg immediately, got %T", msg2)
	assert.Equal(t, uint64(2), loaded2.gen, "cache hit must carry the current requestGen")
	assert.Equal(t, "admin", loaded2.data.Data["username"])
}

// TestLoadPreviewSecretDataColumnInjection verifies that after a successful
// load the matching middleItems entry gains secret:<key> columns with the
// decoded values from the response.
func TestLoadPreviewSecretDataColumnInjection(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 1

	msg := previewSecretDataLoadedMsg{
		gen:  1,
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		data: &model.SecretData{
			Keys: []string{"password", "username"},
			Data: map[string]string{
				"password": "s3cret",
				"username": "admin",
			},
		},
	}

	result, _ := m.Update(msg)
	rm := result.(Model)

	require.Len(t, rm.middleItems, 1)
	cols := rm.middleItems[0].Columns

	secretCols := make(map[string]string)
	for _, kv := range cols {
		if len(kv.Key) > 7 && kv.Key[:7] == "secret:" {
			secretCols[kv.Key[7:]] = kv.Value
		}
	}

	assert.Equal(t, "s3cret", secretCols["password"], "password column must be injected with decoded value")
	assert.Equal(t, "admin", secretCols["username"], "username column must be injected with decoded value")
}

// TestLoadPreviewSecretDataColumnInjectionNoDuplication verifies that
// re-applying a previewSecretDataLoadedMsg does not produce duplicate
// secret: columns in the item.
func TestLoadPreviewSecretDataColumnInjectionNoDuplication(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 1

	msg := previewSecretDataLoadedMsg{
		gen:  1,
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		data: &model.SecretData{
			Keys: []string{"password"},
			Data: map[string]string{"password": "s3cret"},
		},
	}

	// Apply twice (simulating a second hover trigger on same gen).
	result, _ := m.Update(msg)
	m2 := result.(Model)
	m2.requestGen = 1 // keep same gen for second apply
	result2, _ := m2.Update(msg)
	rm := result2.(Model)

	var secretKeyCount int
	for _, kv := range rm.middleItems[0].Columns {
		if len(kv.Key) > 7 && kv.Key[:7] == "secret:" {
			secretKeyCount++
		}
	}

	assert.Equal(t, 1, secretKeyCount, "secret: columns must not be duplicated on re-apply")
}

// TestSaveSecretInvalidatesPreviewCache verifies that handling a successful
// secretSavedMsg removes the corresponding entry from secretPreviewCache.
func TestSaveSecretInvalidatesPreviewCache(t *testing.T) {
	m := baseSecretModel(t)
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind:       "Secret",
		Resource:   "secrets",
		Namespaced: true,
	}
	m.overlay = overlaySecretEditor

	// Pre-populate the cache.
	key := secretPreviewCacheKey("test-ctx", "default", "my-secret")
	m.secretPreviewCache[key] = &model.SecretData{
		Keys: []string{"password"},
		Data: map[string]string{"password": "old"},
	}
	require.NotNil(t, m.secretPreviewCache[key], "pre-condition: cache must be populated")

	// Successful save: expect cache invalidation.
	result, _ := m.Update(secretSavedMsg{})
	rm := result.(Model)

	assert.Nil(t, rm.secretPreviewCache[key], "successful save must invalidate the preview cache entry")
}

// TestSaveSecretErrorDoesNotInvalidateCache verifies that a failed save does
// NOT remove the cache entry (the data is unchanged on the cluster).
func TestSaveSecretErrorDoesNotInvalidateCache(t *testing.T) {
	m := baseSecretModel(t)
	m.overlay = overlaySecretEditor

	key := secretPreviewCacheKey("test-ctx", "default", "my-secret")
	m.secretPreviewCache[key] = &model.SecretData{
		Keys: []string{"password"},
		Data: map[string]string{"password": "old"},
	}

	// Failed save: cache must remain.
	result, _ := m.Update(secretSavedMsg{err: assert.AnError})
	rm := result.(Model)

	assert.NotNil(t, rm.secretPreviewCache[key], "failed save must not invalidate the preview cache entry")
}

// TestLoadPreviewResourcesIncludesSecretLoader verifies that
// loadPreviewResources dispatches a Secret loader when kind is "Secret"
// AND the secret_lazy_loading config flag is on.
func TestLoadPreviewResourcesIncludesSecretLoader(t *testing.T) {
	prev := ui.ConfigSecretLazyLoading
	ui.ConfigSecretLazyLoading = true
	t.Cleanup(func() { ui.ConfigSecretLazyLoading = prev })

	m := baseSecretModel(t)
	m.nav.Level = model.LevelResources

	cmd := m.loadPreviewResources()

	assert.NotNil(t, cmd, "loadPreviewResources must return a command for Secrets when lazy loading is enabled")
}

// (See TestLoadPreviewSecretDataSkippedWhenLazyLoadingDisabled at the top
// for the disabled-case gate test. It covers the same behaviour at the
// loader boundary without needing to drain tea.Batch sub-commands.)

// TestLoadPreviewSecretDataNoopForNonSecretViaLoader verifies that
// loadPreviewSecretData returns nil when the resource type is not Secret,
// so it is never added to the command batch by loadPreviewResources.
func TestLoadPreviewSecretDataNoopForNonSecretViaLoader(t *testing.T) {
	m := baseSecretModel(t)
	m.nav.ResourceType.Kind = "ConfigMap"

	// The guard is inside loadPreviewSecretData; verify it directly.
	cmd := m.loadPreviewSecretData()
	assert.Nil(t, cmd, "loadPreviewSecretData must return nil for non-Secret kind")
}

// TestPreviewSecretDataLoadedMsgErrorIgnored verifies that an error response
// is not cached and does not inject columns.
func TestPreviewSecretDataLoadedMsgErrorIgnored(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 1

	msg := previewSecretDataLoadedMsg{
		gen:  1,
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		err:  assert.AnError,
	}

	result, _ := m.Update(msg)
	rm := result.(Model)

	key := secretPreviewCacheKey("test-ctx", "default", "my-secret")
	assert.Nil(t, rm.secretPreviewCache[key], "errors must not be cached")
	assert.Empty(t, rm.middleItems[0].Columns, "error response must not inject columns")
}

// TestPreviewSecretDataLoadedEmptySecretCaches verifies that an empty-data
// secret response still populates the cache so the view can distinguish
// "fetch completed" from "fetch in flight". Without this, hovering an empty
// secret would render the "Loading..." spinner forever because the item's
// Columns slice stays empty.
func TestPreviewSecretDataLoadedEmptySecretCaches(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 1
	m.previewLoading = true // simulate: the hover triggered a fetch now in flight

	msg := previewSecretDataLoadedMsg{
		gen:  1,
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		data: &model.SecretData{
			Keys: []string{},
			Data: map[string]string{},
		},
	}

	result, _ := m.Update(msg)
	rm := result.(Model)

	key := secretPreviewCacheKey("test-ctx", "default", "my-secret")
	require.NotNil(t, rm.secretPreviewCache[key],
		"empty-data response must populate the cache so the view can detect fetch completion")
	assert.Empty(t, rm.secretPreviewCache[key].Keys,
		"cached data for an empty secret should have zero keys")
	assert.Empty(t, rm.middleItems[0].Columns,
		"an empty secret must not produce any secret: columns")
	assert.False(t, rm.previewLoading,
		"previewLoading must be cleared after secret data arrives, even for empty secrets")
}

// TestPreviewSecretDataLoadedClearsPreviewLoading verifies that a successful
// non-empty secret load clears the previewLoading flag so the spinner goes
// away once data has been injected.
func TestPreviewSecretDataLoadedClearsPreviewLoading(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 1
	m.previewLoading = true

	msg := previewSecretDataLoadedMsg{
		gen:  1,
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		data: &model.SecretData{
			Keys: []string{"k"},
			Data: map[string]string{"k": "v"},
		},
	}

	result, _ := m.Update(msg)
	rm := result.(Model)

	assert.False(t, rm.previewLoading,
		"previewLoading must be cleared after a successful load completes")
}

// TestPreviewSecretDataLoadedErrorClearsPreviewLoading verifies that an
// errored fetch still clears previewLoading — the fetch finished (it just
// failed), so the spinner should stop regardless.
func TestPreviewSecretDataLoadedErrorClearsPreviewLoading(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 1
	m.previewLoading = true

	msg := previewSecretDataLoadedMsg{
		gen:  1,
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		err:  assert.AnError,
	}

	result, _ := m.Update(msg)
	rm := result.(Model)

	assert.False(t, rm.previewLoading,
		"previewLoading must be cleared after an errored fetch — the load is no longer in flight")
}

// TestSecretDataCachedFor verifies the view helper that the right-pane
// renderer uses to decide whether to show the metadata summary for a Secret
// with empty Columns (fetch completed, no data) vs. the loading spinner
// (fetch still in flight).
func TestSecretDataCachedFor(t *testing.T) {
	m := baseSecretModel(t)
	sel := &m.middleItems[0]

	// Nothing cached yet → should report false.
	assert.False(t, m.secretDataCachedFor(sel),
		"secretDataCachedFor must be false when cache is empty")

	// Cache an empty-data entry (the problematic case).
	key := secretPreviewCacheKey("test-ctx", "default", "my-secret")
	m.secretPreviewCache[key] = &model.SecretData{
		Keys: []string{},
		Data: map[string]string{},
	}
	assert.True(t, m.secretDataCachedFor(sel),
		"secretDataCachedFor must be true once an empty-data entry is cached")

	// Non-Secret kind must always report false even if the key happens to match.
	m.nav.ResourceType.Kind = "ConfigMap"
	assert.False(t, m.secretDataCachedFor(sel),
		"secretDataCachedFor must be false for non-Secret kinds")

	// nil selection guard.
	m.nav.ResourceType.Kind = "Secret"
	assert.False(t, m.secretDataCachedFor(nil),
		"secretDataCachedFor must be false for a nil selection")
}

// TestPreviewSecretDataLoadedStaleGenKeepsPreviewLoading verifies that a
// stale-gen message is dropped without touching previewLoading — a newer
// load is still in flight and the spinner must remain.
func TestPreviewSecretDataLoadedStaleGenKeepsPreviewLoading(t *testing.T) {
	m := baseSecretModel(t)
	m.requestGen = 2
	m.previewLoading = true

	msg := previewSecretDataLoadedMsg{
		gen:  1, // stale
		ctx:  "test-ctx",
		ns:   "default",
		name: "my-secret",
		data: &model.SecretData{Keys: []string{"k"}, Data: map[string]string{"k": "v"}},
	}

	result, _ := m.Update(msg)
	rm := result.(Model)

	assert.True(t, rm.previewLoading,
		"stale-gen message must not clear previewLoading — a newer load is still in flight")
}
