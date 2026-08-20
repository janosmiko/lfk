package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/k8s"
)

// mkDashboardEvent builds a minimal unstructured Event with the given type
// ("Warning" or "Normal") for fetchWarningEvents tests.
func mkDashboardEvent(name, eventType string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Event",
		"metadata": map[string]any{"name": name, "namespace": "default"},
		"type":     eventType,
		"reason":   "Test",
		"message":  "test event",
	}}
}

// TestFetchWarningEvents_SendsWarningFieldSelector asserts the Events list
// call carries a server-side type=Warning field selector, and that the
// returned events match what the old client-side-only filter produced.
func TestFetchWarningEvents_SendsWarningFieldSelector(t *testing.T) {
	scheme := runtime.NewScheme()
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "events"}: "EventList",
		},
		mkDashboardEvent("evt-warning", "Warning"),
		mkDashboardEvent("evt-normal", "Normal"),
	)

	var gotFieldSelector string
	dyn.PrependReactor("list", "events", func(action clienttesting.Action) (bool, runtime.Object, error) {
		gotFieldSelector = action.(clienttesting.ListAction).GetListRestrictions().Fields.String()
		return false, nil, nil
	})

	client := k8s.NewTestClient(fake.NewClientset(), dyn)

	limited, all := fetchWarningEvents(t.Context(), "test-ctx", client)

	assert.Equal(t, "type=Warning", gotFieldSelector,
		"Events list call must carry a server-side type=Warning field selector")

	require.Len(t, all, 1)
	assert.Equal(t, "evt-warning", all[0].Name)
	require.Len(t, limited, 1)
	assert.Equal(t, "evt-warning", limited[0].Name)
}
