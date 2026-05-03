package k8s

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetServiceEndpoints_AggregatesAcrossSlices(t *testing.T) {
	// Two EndpointSlices for the same Service. Real kube-proxy sees them as
	// one merged set; the rollup must do the same so a Service with > 100
	// endpoints (sliced by the EndpointSlice controller) doesn't render as
	// "no endpoints".
	slice1 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc-aaa",
			Namespace: "default",
			Labels:    map[string]string{"kubernetes.io/service-name": "my-svc"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: new(true)},
				NodeName:   new("node-a"),
				TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: "foo-1"},
			},
		},
	}
	slice2 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc-bbb",
			Namespace: "default",
			Labels:    map[string]string{"kubernetes.io/service-name": "my-svc"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{Ready: new(false)},
				NodeName:   new("node-b"),
				TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: "foo-2"},
			},
		},
	}
	cs := k8sfake.NewClientset(slice1, slice2)
	c := newFakeClient(cs, nil)

	out, err := c.GetServiceEndpoints(context.Background(), "", "default", "my-svc")
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, 1, out.Ready, "one ready endpoint across the two slices")
	assert.Equal(t, 1, out.NotReady, "one not-ready endpoint across the two slices")

	// Address ordering across slices isn't guaranteed by the API so accept
	// either order — the substring checks below cover both endpoints.
	assert.Contains(t, out.Block, "10.0.0.1 → pod/foo-1 on node-a")
	assert.Contains(t, out.Block, "10.0.0.2 → pod/foo-2 on node-b (NotReady)")
}

func TestGetServiceEndpoints_NoSlicesReturnsEmpty(t *testing.T) {
	// Service exists, no matching slices. Result must be a non-nil zero
	// rollup so callers can distinguish "fetch succeeded, no endpoints" from
	// "fetch in-flight" (which they already track via the cache miss).
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)

	out, err := c.GetServiceEndpoints(context.Background(), "", "default", "lonely")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, 0, out.Ready)
	assert.Equal(t, 0, out.NotReady)
	assert.Empty(t, out.Block, "no slices → no per-endpoint block")
}

func TestGetServiceEndpoints_FilterIgnoresOtherServices(t *testing.T) {
	// EndpointSlices are namespaced and labelled by service-name. The label
	// selector must keep slices for *other* services from leaking into the
	// rollup of the one we asked for — otherwise a busy namespace would
	// dump every endpoint into every Service preview.
	owned := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc-aaa",
			Namespace: "default",
			Labels:    map[string]string{"kubernetes.io/service-name": "my-svc"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: new(true)},
				TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: "foo"},
			},
		},
	}
	other := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-svc-aaa",
			Namespace: "default",
			Labels:    map[string]string{"kubernetes.io/service-name": "another-svc"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.99.99.99"},
				TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "intruder"},
			},
		},
	}
	cs := k8sfake.NewClientset(owned, other)
	c := newFakeClient(cs, nil)

	out, err := c.GetServiceEndpoints(context.Background(), "", "default", "my-svc")
	require.NoError(t, err)
	assert.Equal(t, 1, out.Ready)
	assert.NotContains(t, out.Block, "10.99.99.99",
		"another service's endpoints must not leak into this rollup")
	assert.NotContains(t, out.Block, "intruder")
}

func TestGetServiceEndpoints_MissingConditionsTreatedAsReady(t *testing.T) {
	// Same semantics as populateEndpointSlice: a nil Conditions.Ready means
	// "unknown" → treat as ready (per discovery.k8s.io/v1 spec). Locks the
	// behaviour so the rollup matches what kube-proxy actually does.
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc-aaa",
			Namespace: "default",
			Labels:    map[string]string{"kubernetes.io/service-name": "my-svc"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				// No Conditions at all → unknown → ready.
				TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "foo"},
			},
		},
	}
	cs := k8sfake.NewClientset(slice)
	c := newFakeClient(cs, nil)

	out, err := c.GetServiceEndpoints(context.Background(), "", "default", "my-svc")
	require.NoError(t, err)
	assert.Equal(t, 1, out.Ready, "missing conditions.ready counts as ready")
	assert.Equal(t, 0, out.NotReady)
	assert.False(t, strings.Contains(out.Block, "NotReady"),
		"missing conditions.ready row must not show (NotReady)")
}
