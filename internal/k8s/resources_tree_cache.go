package k8s

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// treeCache shares namespace-wide LISTs and ref-existence checkers across one
// tree build: a Helm release with N workloads used to issue N sequential Pod
// LISTs. Not safe for concurrent use, like newRefExistsFn's own cache.
type treeCache struct {
	dyn    dynamic.Interface
	lists  map[treeListKey][]unstructured.Unstructured
	exists map[string]existsFn
}

type treeListKey struct {
	gvr       schema.GroupVersionResource
	namespace string
}

func newTreeCache(dyn dynamic.Interface) *treeCache {
	return &treeCache{
		dyn:    dyn,
		lists:  map[treeListKey][]unstructured.Unstructured{},
		exists: map[string]existsFn{},
	}
}

// list returns every object of gvr in namespace, fetching once per
// (gvr, namespace) pair. Failures are not cached, so a later caller retries.
func (t *treeCache) list(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]unstructured.Unstructured, error) {
	key := treeListKey{gvr: gvr, namespace: namespace}
	if items, ok := t.lists[key]; ok {
		return items, nil
	}
	list, err := t.dyn.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	t.lists[key] = list.Items
	return list.Items, nil
}

// existsFor returns the ref-existence checker for namespace, shared by every
// Pod in the tree so a Secret referenced by many workloads costs one GET. The
// checker keeps the ctx of the first caller, so pass one ctx per tree build.
func (t *treeCache) existsFor(ctx context.Context, namespace string) existsFn {
	if fn, ok := t.exists[namespace]; ok {
		return fn
	}
	fn := newRefExistsFn(ctx, t.dyn, namespace)
	t.exists[namespace] = fn
	return fn
}
