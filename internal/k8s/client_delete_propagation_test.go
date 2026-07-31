package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/model"
)

// deleteOptionsFor runs a delete and returns the DeleteOptions the client sent.
func deleteOptionsFor(t *testing.T, policy model.DeletePropagation) metav1.DeleteOptions {
	t.Helper()

	obj := &unstructured.Unstructured{}
	obj.SetName("backup-nightly")
	obj.SetNamespace("default")
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"})

	dc := newFakeDynClient(obj)
	c := newFakeClient(nil, dc)

	err := c.DeleteResource("", "default", model.ResourceTypeEntry{
		APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true,
	}, "backup-nightly", policy)
	require.NoError(t, err)

	for _, action := range dc.Actions() {
		if del, ok := action.(k8stesting.DeleteActionImpl); ok {
			return del.DeleteOptions
		}
	}
	t.Fatal("no delete action recorded")
	return metav1.DeleteOptions{}
}

// The API server defaults batch/v1 Job deletes to OrphanDependents, so an
// empty DeleteOptions silently leaves the Job's pods running. Every delete
// must name its policy explicitly.
func TestDeleteResource_SendsExplicitPropagationPolicy(t *testing.T) {
	tests := []struct {
		policy model.DeletePropagation
		want   metav1.DeletionPropagation
	}{
		{model.DeletePropagationBackground, metav1.DeletePropagationBackground},
		{model.DeletePropagationForeground, metav1.DeletePropagationForeground},
		{model.DeletePropagationOrphan, metav1.DeletePropagationOrphan},
	}
	for _, tt := range tests {
		t.Run(string(tt.policy), func(t *testing.T) {
			opts := deleteOptionsFor(t, tt.policy)
			require.NotNil(t, opts.PropagationPolicy,
				"DeleteOptions.PropagationPolicy must be set, or the server-side default wins")
			assert.Equal(t, tt.want, *opts.PropagationPolicy)
		})
	}
}

// An unset or garbage policy must not fall through to the server default.
func TestDeleteResource_UnknownPolicyFallsBackToBackground(t *testing.T) {
	for _, policy := range []model.DeletePropagation{"", "bogus"} {
		opts := deleteOptionsFor(t, policy)
		require.NotNil(t, opts.PropagationPolicy)
		assert.Equal(t, metav1.DeletePropagationBackground, *opts.PropagationPolicy,
			"policy %q must fall back to Background", policy)
	}
}

// None is the deliberate escape hatch: it sends no policy so the API server
// applies its own per-resource default.
func TestDeleteResource_NoneSendsNoPolicy(t *testing.T) {
	opts := deleteOptionsFor(t, model.DeletePropagationNone)
	assert.Nil(t, opts.PropagationPolicy,
		"None must omit PropagationPolicy so the server decides")
}
