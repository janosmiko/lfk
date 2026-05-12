package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// karpenterNodeClaimGVR is the v1 GroupVersionResource for
// karpenter.sh/NodeClaim. Karpenter promoted the API to v1 in v0.32+;
// earlier deployments are out of scope for this client surface.
var karpenterNodeClaimGVR = schema.GroupVersionResource{
	Group:    "karpenter.sh",
	Version:  "v1",
	Resource: "nodeclaims",
}

// GetNodeClaimNodeName reads the underlying node name from a
// karpenter.sh/NodeClaim's status.nodeName. The TUI uses it to forward
// Cordon Node / Drain Node actions from the NodeClaim row to the
// kubectl helpers that expect a node name. Returns an empty string with
// no error when the NodeClaim exists but hasn't been bound to a node
// yet (Karpenter still provisioning) so callers can render a clean
// "node not yet bound" message instead of an error toast.
//
// NodeClaim is cluster-scoped (no namespace argument).
func (c *Client) GetNodeClaimNodeName(contextName, name string) (string, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return "", err
	}
	obj, err := dynClient.Resource(karpenterNodeClaimGVR).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting NodeClaim %s: %w", name, err)
	}
	nodeName, found, err := unstructured.NestedString(obj.Object, "status", "nodeName")
	if err != nil {
		return "", fmt.Errorf("reading NodeClaim %s status.nodeName: %w", name, err)
	}
	if !found {
		return "", nil
	}
	return nodeName, nil
}

// DisruptNodeClaim deletes the NodeClaim, which is the Karpenter-native
// way to terminate a single node (Karpenter's NodeClaim controller
// observes the deletion and tears down the underlying cloud instance
// plus the corresponding core Node object). Equivalent to
// `kubectl delete nodeclaim <name>` but routes through the dynamic
// client so callers don't pay shell-out cost. The TUI gates this behind
// a type-to-confirm overlay because the cluster loses capacity until
// Karpenter reprovisions.
func (c *Client) DisruptNodeClaim(contextName, name string) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}
	if err := dynClient.Resource(karpenterNodeClaimGVR).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("disrupting NodeClaim %s: %w", name, err)
	}
	return nil
}
