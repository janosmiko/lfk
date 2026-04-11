package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltInMetadata_EntriesAreWellFormed(t *testing.T) {
	require.NotEmpty(t, BuiltInMetadata)
	for key, meta := range BuiltInMetadata {
		assert.NotEmpty(t, meta.Category, "key=%s missing category", key)
		assert.NotEmpty(t, meta.DisplayName, "key=%s missing display name", key)
		assert.NotEmpty(t, meta.Icon, "key=%s missing icon", key)
		// key format: "group/resource" where group may be empty.
		parts := strings.SplitN(key, "/", 2)
		require.Len(t, parts, 2, "key=%s must contain exactly one '/'", key)
		assert.NotEmpty(t, parts[1], "key=%s missing resource part", key)
	}
}

func TestBuiltInMetadata_CoversCoreK8sResources(t *testing.T) {
	// Minimum set that must be present for the curated sidebar to work.
	required := []string{
		"/pods",
		"/services",
		"/configmaps",
		"/secrets",
		"/namespaces",
		"/nodes",
		"/persistentvolumes",
		"/persistentvolumeclaims",
		"/serviceaccounts",
		"/events",
		"apps/deployments",
		"apps/statefulsets",
		"apps/daemonsets",
		"apps/replicasets",
		"batch/jobs",
		"batch/cronjobs",
		"networking.k8s.io/ingresses",
		"networking.k8s.io/networkpolicies",
		"storage.k8s.io/storageclasses",
		"storage.k8s.io/csidrivers",
		"storage.k8s.io/csinodes",
		"rbac.authorization.k8s.io/roles",
		"rbac.authorization.k8s.io/rolebindings",
		"rbac.authorization.k8s.io/clusterroles",
		"rbac.authorization.k8s.io/clusterrolebindings",
	}
	for _, key := range required {
		_, ok := BuiltInMetadata[key]
		assert.True(t, ok, "BuiltInMetadata must contain %s", key)
	}
}

func TestSeedResources_CoversCoreNavigation(t *testing.T) {
	seed := SeedResources()
	require.NotEmpty(t, seed)

	// Every seed entry must resolve to a BuiltInMetadata entry so
	// BuildSidebarItems produces a visible, categorized sidebar even before
	// discovery completes.
	for _, e := range seed {
		key := e.APIGroup + "/" + e.Resource
		_, ok := BuiltInMetadata[key]
		assert.True(t, ok, "seed entry %s has no BuiltInMetadata", key)
	}

	// Minimum set: the sidebar must be usable for day-to-day navigation
	// before discovery finishes.
	kinds := make(map[string]bool, len(seed))
	for _, e := range seed {
		kinds[e.Kind] = true
	}
	for _, must := range []string{"Pod", "Deployment", "Service", "ConfigMap", "Secret", "Namespace", "Node"} {
		assert.True(t, kinds[must], "seed missing %s", must)
	}
}
