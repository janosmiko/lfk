package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDeprecation(t *testing.T) {
	t.Run("deprecated extensions/v1beta1 ingresses", func(t *testing.T) {
		info, found := CheckDeprecation("extensions", "v1beta1", "ingresses")
		assert.True(t, found)
		assert.Equal(t, "1.22", info.RemovedIn)
		assert.Equal(t, "networking.k8s.io/v1", info.Replacement)
		assert.Contains(t, info.Message, "networking.k8s.io/v1")
	})

	t.Run("deprecated batch/v1beta1 cronjobs", func(t *testing.T) {
		info, found := CheckDeprecation("batch", "v1beta1", "cronjobs")
		assert.True(t, found)
		assert.Equal(t, "1.25", info.RemovedIn)
		assert.Equal(t, "batch/v1", info.Replacement)
	})

	t.Run("deprecated RBAC v1beta1 roles", func(t *testing.T) {
		info, found := CheckDeprecation("rbac.authorization.k8s.io", "v1beta1", "roles")
		assert.True(t, found)
		assert.Equal(t, "1.22", info.RemovedIn)
	})

	t.Run("deprecated autoscaling/v2beta2 HPA", func(t *testing.T) {
		info, found := CheckDeprecation("autoscaling", "v2beta2", "horizontalpodautoscalers")
		assert.True(t, found)
		assert.Equal(t, "1.26", info.RemovedIn)
	})

	t.Run("current API version not deprecated", func(t *testing.T) {
		_, found := CheckDeprecation("apps", "v1", "deployments")
		assert.False(t, found)
	})

	t.Run("core v1 pods not deprecated", func(t *testing.T) {
		_, found := CheckDeprecation("", "v1", "pods")
		assert.False(t, found)
	})

	t.Run("unknown resource not deprecated", func(t *testing.T) {
		_, found := CheckDeprecation("custom.io", "v1", "widgets")
		assert.False(t, found)
	})

	t.Run("deprecated flowcontrol v1beta2", func(t *testing.T) {
		info, found := CheckDeprecation("flowcontrol.apiserver.k8s.io", "v1beta2", "flowschemas")
		assert.True(t, found)
		assert.Equal(t, "1.29", info.RemovedIn)
	})
}
