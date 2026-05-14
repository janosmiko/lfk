// Package app — virtual_kinds.go
// Identifies synthetic Kind strings that don't map to a real Kubernetes
// resource so the direct-action dispatchers (Logs / Edit / Describe /
// Delete / Scale / Diff / YAML / copy / export) can no-op rather than
// hand kubectl a bogus type.
package app

import "strings"

// isVirtualResourceKind reports whether kind is a synthetic Kind that
// does not map to a real Kubernetes resource — port-forward rows,
// capture rows, and security finding rows. Direct actions must no-op
// for these to avoid dispatching kubectl with a bogus type.
func isVirtualResourceKind(kind string) bool {
	return kind == "" ||
		kind == "__port_forwards__" ||
		kind == "__port_forward_entry__" ||
		kind == "__captures__" ||
		strings.HasPrefix(kind, "__security_")
}
