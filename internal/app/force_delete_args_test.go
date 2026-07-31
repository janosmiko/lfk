package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestForceDeleteArgs(t *testing.T) {
	pod := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}

	args := forceDeleteArgs(pod, "web-0", "prod-ctx", "default", model.DeletePropagationForeground)

	assert.Equal(t, "delete", args[0])
	assert.Contains(t, args, "--force")
	assert.Contains(t, args, "--grace-period=0")
	assert.Contains(t, args, "--cascade=foreground")
	assert.Contains(t, args, "web-0")
	assert.Contains(t, args, "--context")
	assert.Contains(t, args, "prod-ctx")
	assert.Contains(t, args, "-n")
	assert.Contains(t, args, "default")
}

func TestForceDeleteArgs_ClusterScopedOmitsNamespace(t *testing.T) {
	node := model.ResourceTypeEntry{Kind: "Node", Resource: "nodes", Namespaced: false}

	args := forceDeleteArgs(node, "node-1", "prod-ctx", "default", model.DeletePropagationBackground)

	assert.NotContains(t, args, "-n")
	assert.NotContains(t, args, "default")
}

// kubectl rejects --cascade=none, so no policy may ever produce it.
func TestForceDeleteArgs_NeverEmitsInvalidCascade(t *testing.T) {
	pod := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	valid := []string{"--cascade=background", "--cascade=foreground", "--cascade=orphan"}

	for _, policy := range []model.DeletePropagation{
		model.DeletePropagationBackground,
		model.DeletePropagationForeground,
		model.DeletePropagationOrphan,
		model.DeletePropagationNone,
		model.DeletePropagation(""),
		model.DeletePropagation("bogus"),
	} {
		args := forceDeleteArgs(pod, "web-0", "prod-ctx", "default", policy)

		var found string
		for _, a := range args {
			if strings.HasPrefix(a, "--cascade=") {
				found = a
			}
		}
		assert.True(t, slices.Contains(valid, found),
			"policy %q produced %q, which kubectl rejects", policy, found)
	}
}

func TestForceDeleteArgs_NoneFallsBackToBackground(t *testing.T) {
	pod := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}

	args := forceDeleteArgs(pod, "web-0", "prod-ctx", "default", model.DeletePropagationNone)

	assert.Contains(t, args, "--cascade=background")
}
