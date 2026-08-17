// Package security — pagination.go
// Paginated list helper for the dynamic-client list calls every CRD-based
// security source makes. Wrapping client-go's tools/pager keeps the
// per-page response size bounded (etcd doesn't have to materialise the
// whole list in memory). It also recovers from continue-token expiration on
// long lists. Without pagination, a single FetchAll on a busy cluster
// can produce 10s of MB of JSON per source. That is enough to stress small
// control planes (we've seen one freeze on a 746-PolicyReport list).
package security

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/pager"
)

// DefaultListPageSize is the per-page Limit used by ListPaginated. 200 is a
// compromise. Kubectl defaults to 500 for fast machines against fat API
// servers. Security-source responses are unusually large per object
// (Trivy's VulnerabilityReport.report carries the full vuln list, Kyverno's
// PolicyReport carries .results[]). So 200 keeps each page under a few MB.
const DefaultListPageSize = 200

// DynamicLister is the subset of k8s.io/client-go/dynamic/Resource
// (and Namespaceable) that ListPaginated needs. Defining it here means
// the helper doesn't import the dynamic package, and tests can supply
// any implementation (including fake.FakeDynamicClient's resource
// view).
type DynamicLister interface {
	List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error)
}

// ListPaginated walks every page of the given Lister and returns a
// single combined UnstructuredList of all items. We use pager's
// EachListItem rather than its List because List wraps results in a
// *metainternalversion.List (a runtime container that doesn't preserve
// the concrete UnstructuredList type). That is awkward to convert
// back. EachListItem hands us the page items one at a time and lets us
// build the UnstructuredList directly.
//
// Cancellation: client-go's pager honours the supplied context. The
// underlying dynamic-client List calls return promptly when ctx is
// cancelled, and the loop exits without performing additional pages.
//
// Error handling: pager auto-recovers from "continue token expired"
// (HTTP 410 Gone) by re-listing from the start with the same page
// size. Other errors propagate. Long-running fetches that span the
// API server's pagination cache window therefore complete rather than
// fail.
func ListPaginated(ctx context.Context, lister DynamicLister) (*unstructured.UnstructuredList, error) {
	listFn := func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
		return lister.List(ctx, opts)
	}
	p := pager.New(listFn)
	p.PageSize = DefaultListPageSize

	out := &unstructured.UnstructuredList{}
	err := p.EachListItem(ctx, metav1.ListOptions{}, func(obj runtime.Object) error {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("paginated list returned unexpected item type %T", obj)
		}
		out.Items = append(out.Items, *u)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
