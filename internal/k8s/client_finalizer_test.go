package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- FinalizerMatch struct ---

func TestFinalizerMatch_StructFields(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		fm := FinalizerMatch{
			Name:       "my-resource",
			Namespace:  "default",
			Kind:       "ConfigMap",
			APIGroup:   "",
			APIVersion: "v1",
			Resource:   "configmaps",
			Namespaced: true,
			Matched:    "finalizer.example.com/cleanup",
			Age:        "5d",
		}

		assert.Equal(t, "my-resource", fm.Name)
		assert.Equal(t, "default", fm.Namespace)
		assert.Equal(t, "ConfigMap", fm.Kind)
		assert.Equal(t, "", fm.APIGroup)
		assert.Equal(t, "v1", fm.APIVersion)
		assert.Equal(t, "configmaps", fm.Resource)
		assert.True(t, fm.Namespaced)
		assert.Equal(t, "finalizer.example.com/cleanup", fm.Matched)
		assert.Equal(t, "5d", fm.Age)
	})

	t.Run("cluster-scoped resource", func(t *testing.T) {
		fm := FinalizerMatch{
			Name:       "my-namespace",
			Namespace:  "",
			Kind:       "Namespace",
			APIGroup:   "",
			APIVersion: "v1",
			Resource:   "namespaces",
			Namespaced: false,
			Matched:    "kubernetes",
			Age:        "30d",
		}

		assert.Equal(t, "my-namespace", fm.Name)
		assert.Empty(t, fm.Namespace)
		assert.False(t, fm.Namespaced)
		assert.Equal(t, "kubernetes", fm.Matched)
	})

	t.Run("zero value struct", func(t *testing.T) {
		fm := FinalizerMatch{}

		assert.Empty(t, fm.Name)
		assert.Empty(t, fm.Namespace)
		assert.Empty(t, fm.Kind)
		assert.Empty(t, fm.APIGroup)
		assert.Empty(t, fm.APIVersion)
		assert.Empty(t, fm.Resource)
		assert.False(t, fm.Namespaced)
		assert.Empty(t, fm.Matched)
		assert.Empty(t, fm.Age)
	})

	t.Run("CRD resource with multiple finalizers", func(t *testing.T) {
		fm := FinalizerMatch{
			Name:       "my-cluster",
			Namespace:  "databases",
			Kind:       "Cluster",
			APIGroup:   "cnpg.io",
			APIVersion: "v1",
			Resource:   "clusters",
			Namespaced: true,
			Matched:    "cnpg.io/finalizer",
			Age:        "1h",
		}

		assert.Equal(t, "cnpg.io", fm.APIGroup)
		assert.Equal(t, "cnpg.io/finalizer", fm.Matched)
	})
}
