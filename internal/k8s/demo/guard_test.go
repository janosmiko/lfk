package demo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestGuardListPanics_UnregisteredGVR_ReturnsErrorNotPanic is the Layer 2
// regression guard: a List against a GVR missing from ListKinds must
// degrade to an error, not unwind the goroutine that called it. This is
// independent of the Layer 1 registration fix — it holds even for a GVR
// nobody has registered yet, which is exactly the case a future missing
// registration would hit.
func TestGuardListPanics_UnregisteredGVR_ReturnsErrorNotPanic(t *testing.T) {
	dyn := GuardListPanics(NewDynamicClient())
	unregistered := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}

	assert.NotPanics(t, func() {
		_, err := dyn.Resource(unregistered).Namespace("default").List(t.Context(), metav1.ListOptions{})
		require.Error(t, err)
	})

	assert.NotPanics(t, func() {
		_, err := dyn.Resource(unregistered).List(t.Context(), metav1.ListOptions{})
		require.Error(t, err)
	})
}

// TestGuardListPanics_RegisteredGVR_PassesThrough proves the guard is
// transparent for the normal path: seeded, registered resources still list
// their data through the wrapper.
func TestGuardListPanics_RegisteredGVR_PassesThrough(t *testing.T) {
	dyn := GuardListPanics(NewDynamicClient())
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

	list, err := dyn.Resource(podGVR).Namespace("").List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, list.Items)
}
