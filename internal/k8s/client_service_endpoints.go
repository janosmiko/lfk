package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceEndpoints is the rollup payload returned by GetServiceEndpoints
// for a single Service. The right-pane preview renders the Block as a
// multi-line "Endpoints" KeyValue, and uses the Ready / NotReady counts
// to colour the summary line (red when Ready == 0).
type ServiceEndpoints struct {
	// Ready counts addresses across all matching EndpointSlices whose
	// conditions.ready resolves to true (treating nil as ready, per
	// discovery.k8s.io/v1's spec — same convention as
	// populateEndpointSlice).
	Ready int
	// NotReady counts addresses whose conditions.ready is explicitly false.
	NotReady int
	// Block is the newline-joined per-endpoint preview lines built by
	// formatEndpointLine. Empty when no slices match the service.
	Block string
}

// GetServiceEndpoints lists every EndpointSlice in `namespace` whose
// `kubernetes.io/service-name` label matches `svcName` and aggregates
// them into a single ServiceEndpoints rollup ready for the Service
// preview. Multiple slices are concatenated in API list order.
//
// Returns a non-nil zero rollup when no slices match — callers (the
// preview cache + injector) treat "fetch succeeded, no endpoints" as a
// distinct state from "fetch in flight" via the cache miss path, so a
// nil/empty distinction here would muddle the contract.
//
// Headless and ExternalName Services are NOT short-circuited here; the
// caller (loadPreviewServiceEndpoints) is the right place to skip those
// before paying the API roundtrip, and keeping this method dumb makes
// it usable for any service-by-label rollup in the future.
func (c *Client) GetServiceEndpoints(ctx context.Context, contextName, namespace, svcName string) (*ServiceEndpoints, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}
	list, err := cs.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + svcName,
	})
	if err != nil {
		return nil, fmt.Errorf("listing endpoint slices for service %s/%s: %w", namespace, svcName, err)
	}

	out := &ServiceEndpoints{}
	if len(list.Items) == 0 {
		return out, nil
	}

	var lines []string
	for _, slice := range list.Items {
		for _, ep := range slice.Endpoints {
			// Per discovery.k8s.io/v1 spec: nil conditions.ready means
			// "unknown" → treat as ready. Same convention as
			// populateEndpointSlice's missing-conditions branch.
			ready := true
			if ep.Conditions.Ready != nil {
				ready = *ep.Conditions.Ready
			}
			var nodeName string
			if ep.NodeName != nil {
				nodeName = *ep.NodeName
			}
			var targetKind, targetName string
			if ep.TargetRef != nil {
				targetKind = ep.TargetRef.Kind
				targetName = ep.TargetRef.Name
			}
			for _, addr := range ep.Addresses {
				if addr == "" {
					continue
				}
				lines = append(lines, formatEndpointLine(addr, targetKind, targetName, nodeName, ready))
				if ready {
					out.Ready++
				} else {
					out.NotReady++
				}
			}
		}
	}
	out.Block = strings.Join(lines, "\n")
	return out, nil
}
