package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	metadatafake "k8s.io/client-go/metadata/fake"

	"github.com/janosmiko/lfk/internal/model"
)

// newFakeMetaClient builds a fake metadata client seeded with PartialObjectMetadata objects.
func newFakeMetaClient(objects ...*metav1.PartialObjectMetadata) *metadatafake.FakeMetadataClient {
	scheme := metadatafake.NewTestScheme()
	if err := metav1.AddMetaToScheme(scheme); err != nil {
		panic(fmt.Sprintf("AddMetaToScheme: %v", err))
	}
	objs := make([]runtime.Object, 0, len(objects))
	for _, o := range objects {
		objs = append(objs, o)
	}
	return metadatafake.NewSimpleMetadataClient(scheme, objs...)
}

// newClientWithMeta creates a Client that uses the provided fake metadata client
// and no dynamic client (Secrets go through meta path; others would need a dyn client).
// The metadata-only Secret path is enabled — callers testing the eager path
// should flip SetSecretLazyLoading(false) after construction.
func newClientWithMeta(mc *metadatafake.FakeMetadataClient, dc *dynamicfake.FakeDynamicClient) *Client {
	c := NewTestClient(nil, dc)
	c.testMetaClient = mc
	c.SetSecretLazyLoading(true)
	return c
}

// secretRT is the ResourceTypeEntry for namespaced Secrets.
var secretRT = model.ResourceTypeEntry{
	APIGroup:   "",
	APIVersion: "v1",
	Resource:   "secrets",
	Kind:       "Secret",
	Namespaced: true,
}

// secretRTCluster is a cluster-scoped variant (not realistic but tests the non-namespaced branch).
var secretRTCluster = model.ResourceTypeEntry{
	APIGroup:   "",
	APIVersion: "v1",
	Resource:   "secrets",
	Kind:       "Secret",
	Namespaced: false,
}

// --- helper ---

func partialSecret(name, namespace string, created time.Time) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(created),
		},
	}
}

func partialSecretWithDeletion(name, namespace string, created, deletion time.Time) *metav1.PartialObjectMetadata {
	s := partialSecret(name, namespace, created)
	ts := metav1.NewTime(deletion)
	s.DeletionTimestamp = &ts
	return s
}

func partialSecretWithOwner(name, namespace string, created time.Time, ownerKind, ownerName, ownerAPIVersion string, ownerUID types.UID) *metav1.PartialObjectMetadata {
	s := partialSecret(name, namespace, created)
	s.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: ownerAPIVersion,
			Kind:       ownerKind,
			Name:       ownerName,
			UID:        ownerUID,
		},
	}
	return s
}

// --- Tests ---

// TestGetResources_SecretMetadataPath verifies that Secrets are fetched via the
// metadata-only path, returning Name, Namespace, and Age without data columns.
func TestGetResources_SecretMetadataPath(t *testing.T) {
	now := time.Now().UTC()
	created := now.Add(-2 * time.Hour)

	mc := newFakeMetaClient(
		partialSecret("my-secret", "default", created),
	)
	c := newClientWithMeta(mc, nil)

	items, err := c.GetResources(context.Background(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "my-secret", item.Name)
	assert.Equal(t, "default", item.Namespace)
	assert.NotEmpty(t, item.Age, "Age must be populated from CreationTimestamp")

	// No secret data columns.
	for _, kv := range item.Columns {
		assert.False(t, len(kv.Key) >= 7 && kv.Key[:7] == "secret:", "unexpected secret data column: %s", kv.Key)
		assert.NotEqual(t, "Type", kv.Key, "Type column must not be present in metadata path")
	}
}

// TestGetResources_SecretNoDataColumns confirms that no "secret:<k>" or "Type" columns appear.
func TestGetResources_SecretNoDataColumns(t *testing.T) {
	now := time.Now().UTC()
	mc := newFakeMetaClient(
		partialSecret("alpha", "ns1", now.Add(-1*time.Hour)),
		partialSecret("beta", "ns1", now.Add(-30*time.Minute)),
	)
	c := newClientWithMeta(mc, nil)

	items, err := c.GetResources(context.Background(), "", "ns1", secretRT)
	require.NoError(t, err)
	require.Len(t, items, 2)

	for _, item := range items {
		for _, kv := range item.Columns {
			assert.False(t, len(kv.Key) >= 7 && kv.Key[:7] == "secret:", "unexpected data column %q on item %q", kv.Key, item.Name)
			assert.NotEqual(t, "Type", kv.Key, "Type column must not appear on item %q", item.Name)
		}
	}
}

// TestGetResources_SecretDeletionTimestamp ensures that a Secret with a
// DeletionTimestamp surfaces a "Deletion" column and sets Deleting=true.
func TestGetResources_SecretDeletionTimestamp(t *testing.T) {
	now := time.Now().UTC()
	deletion := now.Add(-5 * time.Minute)

	mc := newFakeMetaClient(
		partialSecretWithDeletion("doomed", "default", now.Add(-1*time.Hour), deletion),
	)
	c := newClientWithMeta(mc, nil)

	items, err := c.GetResources(context.Background(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.True(t, item.Deleting, "Deleting must be true when DeletionTimestamp is set")

	colMap := make(map[string]string, len(item.Columns))
	for _, kv := range item.Columns {
		colMap[kv.Key] = kv.Value
	}
	assert.Contains(t, colMap, "Deletion", "Deletion column must be present")
	assert.Contains(t, colMap["Deletion"], deletion.Format("2006"), "Deletion column must include the year")
}

// TestGetResources_SecretNamespacedPath_AllNamespaces passes namespace="" to
// confirm the query does not panic or error (list across all namespaces).
func TestGetResources_SecretNamespacedPath_AllNamespaces(t *testing.T) {
	now := time.Now().UTC()
	mc := newFakeMetaClient(
		partialSecret("a", "ns-a", now.Add(-1*time.Hour)),
		partialSecret("b", "ns-b", now.Add(-2*time.Hour)),
	)
	c := newClientWithMeta(mc, nil)

	// namespace="" means all namespaces for a namespaced resource.
	items, err := c.GetResources(context.Background(), "", "", secretRT)
	require.NoError(t, err)
	// Fake client returns all objects regardless of namespace filter when namespace is "".
	assert.GreaterOrEqual(t, len(items), 2)
}

// TestGetResources_SecretClusterScoped exercises the cluster-scoped branch of
// the metadata path (non-realistic for Secrets, but ensures the branch is safe).
func TestGetResources_SecretClusterScoped(t *testing.T) {
	now := time.Now().UTC()
	mc := newFakeMetaClient(
		partialSecret("cluster-secret", "", now.Add(-3*time.Hour)),
	)
	c := newClientWithMeta(mc, nil)

	items, err := c.GetResources(context.Background(), "", "", secretRTCluster)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "cluster-secret", items[0].Name)
}

// TestGetResources_SecretSortedAlphabetically confirms that Secret items are
// sorted alphabetically by name (not by LastSeen like Events).
func TestGetResources_SecretSortedAlphabetically(t *testing.T) {
	now := time.Now().UTC()
	mc := newFakeMetaClient(
		partialSecret("zebra", "default", now.Add(-1*time.Hour)),
		partialSecret("apple", "default", now.Add(-2*time.Hour)),
		partialSecret("mango", "default", now.Add(-30*time.Minute)),
	)
	c := newClientWithMeta(mc, nil)

	items, err := c.GetResources(context.Background(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items, 3)

	assert.Equal(t, "apple", items[0].Name)
	assert.Equal(t, "mango", items[1].Name)
	assert.Equal(t, "zebra", items[2].Name)
}

// TestGetResources_SecretWithOwnerReferences verifies that owner references are
// extracted from PartialObjectMetadata and appear as owner columns.
func TestGetResources_SecretWithOwnerReferences(t *testing.T) {
	now := time.Now().UTC()
	mc := newFakeMetaClient(
		partialSecretWithOwner("owned-secret", "default", now.Add(-1*time.Hour),
			"Deployment", "my-deploy", "apps/v1", types.UID("uid-123")),
	)
	c := newClientWithMeta(mc, nil)

	items, err := c.GetResources(context.Background(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items, 1)

	colMap := make(map[string]string, len(items[0].Columns))
	for _, kv := range items[0].Columns {
		colMap[kv.Key] = kv.Value
	}
	ownerVal, ok := colMap["owner:0"]
	assert.True(t, ok, "owner:0 column should be present")
	assert.Contains(t, ownerVal, "Deployment")
	assert.Contains(t, ownerVal, "my-deploy")
}

// --- Regression: non-Secret resources still use dynamic path ---

// TestGetResources_NonSecretUsesDynamicPath confirms that Pods (a non-Secret
// resource) still go through the dynamic client and get kind-specific columns.
func TestGetResources_NonSecretUsesDynamicPath(t *testing.T) {
	scheme := runtime.NewScheme()
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
		&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":              "my-pod",
					"namespace":         "default",
					"creationTimestamp": "2026-04-24T00:00:00Z",
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "app",
							"image": "nginx:latest",
						},
					},
				},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []any{
						map[string]any{
							"name":         "app",
							"ready":        true,
							"restartCount": int64(0),
						},
					},
				},
			},
		},
	)

	// No meta client: Pods must route through the dynamic path.
	c := newClientWithMeta(nil, dc)

	podRT := model.ResourceTypeEntry{
		APIGroup:   "",
		APIVersion: "v1",
		Resource:   "pods",
		Kind:       "Pod",
		Namespaced: true,
	}

	items, err := c.GetResources(context.Background(), "", "default", podRT)
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.Equal(t, "my-pod", items[0].Name)
	// Pods get Ready populated by populatePodDetails.
	assert.Equal(t, "1/1", items[0].Ready)
	assert.Equal(t, "Running", items[0].Status)
}

// TestGetResources_SecretLazyLoadingDisabledUsesDynamicPath confirms the
// config gate: when secretLazyLoading is off (the default), Secret listing
// routes through the dynamic client and eagerly decodes data via
// populateSecretDetails, producing secret:<k> columns on every item.
func TestGetResources_SecretLazyLoadingDisabledUsesDynamicPath(t *testing.T) {
	scheme := runtime.NewScheme()
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			secretGVR: "SecretList",
		},
		&unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "dyn-secret",
					"namespace": "default",
				},
				"type": "Opaque",
				"data": map[string]any{
					// base64("hello") = "aGVsbG8="
					"greeting": "aGVsbG8=",
				},
			},
		},
	)

	c := NewTestClient(nil, dc) // lazy loading off by default
	require.False(t, c.secretLazyLoading, "precondition: flag must default to false")

	items, err := c.GetResources(context.Background(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// Eager path: populateSecretDetails decoded the value into a secret:* column.
	var decoded string
	for _, kv := range items[0].Columns {
		if kv.Key == "secret:greeting" {
			decoded = kv.Value
		}
	}
	assert.Equal(t, "hello", decoded,
		"eager path must decode and store secret:greeting so the detail pane can render without a hover fetch")
}

// TestGetResources_SecretLazyLoadingEnabledUsesMetadataPath is the companion
// to the test above — with the flag on, Secret data columns are absent on
// the list item because only metadata is fetched.
func TestGetResources_SecretLazyLoadingEnabledUsesMetadataPath(t *testing.T) {
	meta := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "meta-secret",
			Namespace:         "default",
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
	}
	mc := newFakeMetaClient(meta)
	dc := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	c := newClientWithMeta(mc, dc) // helper enables the flag

	items, err := c.GetResources(context.Background(), "", "default", secretRT)
	require.NoError(t, err)
	require.Len(t, items, 1)

	for _, kv := range items[0].Columns {
		assert.NotEqual(t, "secret:greeting", kv.Key,
			"metadata path must not decode secret data into columns")
	}
}
