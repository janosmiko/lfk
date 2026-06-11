package security

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Collect paginates one list call to completion. Any error (typically
// Forbidden for read-only users) returns ok=false so the caller can skip
// dependent checks rather than misreport. Shared by the built-in sources
// (advisor, rbac).
func Collect[T any](fn func(opts metav1.ListOptions) ([]T, string, error)) ([]T, bool) {
	var out []T
	opts := metav1.ListOptions{Limit: 200}
	for {
		items, cont, err := fn(opts)
		if err != nil {
			return nil, false
		}
		out = append(out, items...)
		if cont == "" {
			return out, true
		}
		opts.Continue = cont
	}
}
