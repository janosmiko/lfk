package k8s

import (
	"context"
	"slices"

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
	exists map[string]*refChecker
}

type treeListKey struct {
	gvr       schema.GroupVersionResource
	namespace string
}

func newTreeCache(dyn dynamic.Interface) *treeCache {
	return &treeCache{
		dyn:    dyn,
		lists:  map[treeListKey][]unstructured.Unstructured{},
		exists: map[string]*refChecker{},
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

// podsOwnedBy keeps the pods with an owner reference match accepts. Callers
// narrow a namespace-wide pod list to the tree's own pods before warming the
// ref checker, so an unrelated workload costs no lookups.
func podsOwnedBy(pods []unstructured.Unstructured, match func(metav1.OwnerReference) bool) []unstructured.Unstructured {
	owned := make([]unstructured.Unstructured, 0, len(pods))
	for _, pod := range pods {
		if slices.ContainsFunc(pod.GetOwnerReferences(), match) {
			owned = append(owned, pod)
		}
	}

	return owned
}

// existsFor returns the ref checker for namespace, shared by every Pod in the
// tree so a Secret referenced by many workloads costs one GET. It keeps the
// ctx of the first caller, so pass one ctx per tree build.
func (t *treeCache) existsFor(ctx context.Context, namespace string, pods []unstructured.Unstructured) existsFn {
	rc, ok := t.exists[namespace]
	if !ok {
		rc = newRefChecker(ctx, t.dyn, namespace)
		t.exists[namespace] = rc
	}
	rc.warm(pods)

	return rc.exists
}
