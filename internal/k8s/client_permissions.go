package k8s

import (
	"context"
	"fmt"
	"sync"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PermissionQuery is one authorization question: may the current user run
// Verb on Resource (or on one of its subresources) in a namespace.
type PermissionQuery struct {
	Group       string
	Resource    string
	Subresource string
	Verb        string
}

// Key identifies a query in the map CheckPermissions returns, e.g.
// "create:pods/exec".
func (q PermissionQuery) Key() string {
	res := q.Resource
	if q.Subresource != "" {
		res += "/" + q.Subresource
	}
	return q.Verb + ":" + res
}

const (
	// maxPermissionQueries bounds one bulk pass. Each query is one API call,
	// so an unbounded caller could turn a namespace entry into a burst
	// against the API server.
	maxPermissionQueries = 32
	// permissionConcurrency bounds the reviews in flight at once.
	permissionConcurrency = 8
)

// CheckPermissions answers a set of queries in one pass and returns the
// verdict per PermissionQuery.Key. Duplicate queries cost one review.
//
// An error means no verdict is available for the whole set. Callers must fail
// open on it: a hidden action that would have worked is worse than an action
// that fails, and an aggregated or webhook authorizer can disagree with the
// review anyway.
func (c *Client) CheckPermissions(ctx context.Context, contextName, namespace string, queries []PermissionQuery) (map[string]bool, error) {
	unique := dedupePermissionQueries(queries)
	if len(unique) == 0 {
		return map[string]bool{}, nil
	}
	// A cancelled scope has nothing to answer for; skip the fan-out.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	var (
		mu       sync.Mutex
		firstErr error
	)
	results := make(map[string]bool, len(unique))
	sem := make(chan struct{}, permissionConcurrency)
	var wg sync.WaitGroup
	for _, q := range unique {
		wg.Add(1)
		go func(q PermissionQuery) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sar := &authorizationv1.SelfSubjectAccessReview{
				Spec: authorizationv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Namespace:   namespace,
						Verb:        q.Verb,
						Group:       q.Group,
						Resource:    q.Resource,
						Subresource: q.Subresource,
					},
				},
			}
			res, rErr := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})

			mu.Lock()
			defer mu.Unlock()
			if rErr != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("access review for %s: %w", q.Key(), rErr)
				}
				return
			}
			results[q.Key()] = res.Status.Allowed
		}(q)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// dedupePermissionQueries drops repeats and caps the set at
// maxPermissionQueries, keeping the first occurrences.
func dedupePermissionQueries(queries []PermissionQuery) []PermissionQuery {
	seen := make(map[string]struct{}, len(queries))
	unique := make([]PermissionQuery, 0, len(queries))
	for _, q := range queries {
		if q.Resource == "" || q.Verb == "" {
			continue
		}
		key := q.Key()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, q)
		if len(unique) == maxPermissionQueries {
			break
		}
	}
	return unique
}
