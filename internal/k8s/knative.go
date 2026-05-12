package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// knativeRevisionGVR / knativeServiceGVR are the v1 GroupVersionResources
// for Knative Serving. Knative promoted Serving to v1 in 0.16+; older
// API versions are out of scope.
var (
	knativeRevisionGVR = schema.GroupVersionResource{Group: "serving.knative.dev", Version: "v1", Resource: "revisions"}
	knativeServiceGVR  = schema.GroupVersionResource{Group: "serving.knative.dev", Version: "v1", Resource: "services"}
)

// knativeServiceLabel is the standard Knative label on every Revision
// (and the pods underneath) that names the parent Service. Knative's
// own controller sets it; missing the label means the Revision is
// orphaned (rare — usually a sign of a Service deletion in flight) and
// the Activate verb cannot resolve a parent to patch.
const knativeServiceLabel = "serving.knative.dev/service"

// ActivateKnativeRevision sends 100% of traffic to a specific Revision
// by patching the parent Knative Service's spec.traffic to a single
// entry pinned to that Revision. This is the canonical Knative
// rollback / promotion gesture; the controller propagates the new
// traffic split to the Service's downstream Route within a few seconds.
//
// Resolves the parent Service via the standard
// serving.knative.dev/service label on the Revision (Knative sets it on
// every Revision; missing means the Revision is orphaned and cannot be
// activated through the parent). Returns the parent Service name on
// success so the caller can render a clear status message.
//
// The patch replaces spec.traffic — JSON merge-patch treats arrays as
// atomic, which is the desired semantic here (Activate means "this
// revision gets all of it, drop everything else"). Existing tags on the
// previous traffic entries are intentionally dropped; users who need
// fine-grained traffic management edit the Service YAML directly until
// the dedicated traffic-split overlay lands (deferred follow-up).
func (c *Client) ActivateKnativeRevision(contextName, namespace, revisionName string) (parentService string, err error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return "", err
	}
	rev, err := dynClient.Resource(knativeRevisionGVR).Namespace(namespace).Get(context.Background(), revisionName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting Revision %s: %w", revisionName, err)
	}
	parent := rev.GetLabels()[knativeServiceLabel]
	if parent == "" {
		return "", fmt.Errorf("revision %s has no %s label — cannot resolve parent Knative Service", revisionName, knativeServiceLabel)
	}

	// Build the patch programmatically so the revisionName is safely
	// JSON-escaped (revision names are alphanumeric+hyphens today but
	// future Knative versions could relax that).
	patchBody, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"traffic": []map[string]any{
				{"revisionName": revisionName, "percent": 100, "latestRevision": false},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("building Activate patch for %s: %w", revisionName, err)
	}
	if _, err := dynClient.Resource(knativeServiceGVR).Namespace(namespace).Patch(
		context.Background(), parent, k8stypes.MergePatchType, patchBody, metav1.PatchOptions{},
	); err != nil {
		return "", fmt.Errorf("activating Revision %s on Service %s: %w", revisionName, parent, err)
	}
	return parent, nil
}
