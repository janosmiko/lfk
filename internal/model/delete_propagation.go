package model

import (
	"slices"
	"strings"
)

// DeletePropagation selects how a delete cascades to dependent objects.
// Values match the lowercase config spelling. The k8s layer maps them to
// metav1.DeletionPropagation.
type DeletePropagation string

const (
	// DeletePropagationBackground deletes the object immediately. The garbage
	// collector removes dependents afterwards. This is kubectl's default.
	DeletePropagationBackground DeletePropagation = "background"
	// DeletePropagationForeground keeps the object visible until every
	// dependent is gone.
	DeletePropagationForeground DeletePropagation = "foreground"
	// DeletePropagationOrphan leaves dependents running.
	DeletePropagationOrphan DeletePropagation = "orphan"
	// DeletePropagationNone sends no policy at all, leaving the decision to
	// the API server's per-resource default. That default is Background for
	// most kinds but Orphan for jobs.v1.batch and replicationcontrollers.v1,
	// so this is an escape hatch, not a safe choice.
	DeletePropagationNone DeletePropagation = "none"
)

// deletePropagationOrder is the Tab cycle order, mildest blast radius first.
var deletePropagationOrder = []DeletePropagation{
	DeletePropagationBackground,
	DeletePropagationForeground,
	DeletePropagationOrphan,
	DeletePropagationNone,
}

// deletePropagationCascading is the Tab cycle for deletes that run through
// kubectl. It omits None because kubectl cannot express "send no policy":
// leaving --cascade off makes kubectl send background anyway, and
// --cascade=none is not a value it accepts.
//
// Spelled out rather than sliced from deletePropagationOrder so that reordering
// or extending that list cannot silently change which policies are offered
// here, and so an append can never write into its backing array.
var deletePropagationCascading = []DeletePropagation{
	DeletePropagationBackground,
	DeletePropagationForeground,
	DeletePropagationOrphan,
}

// ParseDeletePropagation resolves a config string to a policy. The bool
// reports whether the input was recognized. Callers that want to warn on a
// typo check it, everyone else can use the Background fallback directly.
func ParseDeletePropagation(raw string) (DeletePropagation, bool) {
	switch DeletePropagation(strings.ToLower(strings.TrimSpace(raw))) {
	case DeletePropagationBackground:
		return DeletePropagationBackground, true
	case DeletePropagationForeground:
		return DeletePropagationForeground, true
	case DeletePropagationOrphan:
		return DeletePropagationOrphan, true
	case DeletePropagationNone:
		return DeletePropagationNone, true
	}
	return DeletePropagationBackground, false
}

// Cycle returns the next policy in the Tab order, wrapping at the end. An
// unrecognized value restarts the cycle rather than sticking.
func (p DeletePropagation) Cycle() DeletePropagation {
	return p.cycleIn(deletePropagationOrder)
}

// CycleCascading is Cycle restricted to the policies kubectl can express, so
// None is skipped rather than offered where it cannot be honored.
func (p DeletePropagation) CycleCascading() DeletePropagation {
	return p.cycleIn(deletePropagationCascading)
}

func (p DeletePropagation) cycleIn(order []DeletePropagation) DeletePropagation {
	for i, known := range order {
		if p == known {
			return order[(i+1)%len(order)]
		}
	}
	return DeletePropagationBackground
}

// Cascading resolves the policy to one kubectl accepts, mapping None (and any
// unrecognized value) to Background.
func (p DeletePropagation) Cascading() DeletePropagation {
	if slices.Contains(deletePropagationCascading, p) {
		return p
	}
	return DeletePropagationBackground
}

// KubectlCascade returns the value for `kubectl delete --cascade=`. It is
// always one of background, foreground, or orphan — kubectl rejects anything
// else, so None and unrecognized values resolve to background.
func (p DeletePropagation) KubectlCascade() string {
	return string(p.Cascading())
}

// Label returns the display name for the confirm overlay.
func (p DeletePropagation) Label() string {
	switch p {
	case DeletePropagationForeground:
		return "Foreground"
	case DeletePropagationOrphan:
		return "Orphan"
	case DeletePropagationNone:
		return "None"
	default:
		return "Background"
	}
}

// OrphansDependents reports whether this policy leaves dependents behind.
func (p DeletePropagation) OrphansDependents() bool {
	return p == DeletePropagationOrphan
}

// DefersToServer reports whether the delete sends no policy, letting the API
// server pick. Callers warn on it because the server's choice varies by
// resource and orphans Job and ReplicationController dependents.
func (p DeletePropagation) DefersToServer() bool {
	return p == DeletePropagationNone
}

// NeedsWarning reports whether a selection can leave dependents running and so
// should not look interchangeable with a cascading policy.
func (p DeletePropagation) NeedsWarning() bool {
	return p.OrphansDependents() || p.DefersToServer()
}
