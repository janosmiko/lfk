package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKnativeActions_Revision locks in the Knative-specific verb on
// Revision: Activate. Activate patches the parent Service's
// spec.traffic to route 100% of traffic to the selected revision,
// which is the standard Knative rollback / promotion gesture.
func TestKnativeActions_Revision(t *testing.T) {
	items := ActionsForKind("Revision")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	assert.Contains(t, labels, "Activate",
		"Revision must offer Activate (patches parent Service to send 100% traffic to this revision)")
	// Standard actions still present.
	assert.Contains(t, labels, "Describe")
	assert.Contains(t, labels, "Edit")
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Events")
}

// TestKnativeActions_Service surfaces standard actions for now. The
// Edit Traffic Split overlay from the original plan is deferred to a
// follow-up — Activate on Revision covers the common "promote this
// revision" gesture in this PR.
func TestKnativeActions_Service(t *testing.T) {
	// "Knative Service" is the curated DisplayName; the dispatcher uses
	// the Kind value "Service". Knative collides with the core Service
	// Kind here — see comment in ActionsForKind for the resolution.
	items := ActionsForKind("Service")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	// Activate is for Revision only — it makes no sense on the parent
	// Service itself.
	assert.NotContains(t, labels, "Activate",
		"Service does not offer Activate — that verb only applies to a Revision")
}

// TestKnativeActions_Configuration / TestKnativeActions_Route surface
// only generic actions; Activate lives on Revision, traffic-split
// editing lives on Service (deferred).
func TestKnativeActions_Configuration(t *testing.T) {
	items := ActionsForKind("Configuration")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	assert.Contains(t, labels, "Describe")
	assert.Contains(t, labels, "Edit")
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Events")
	assert.NotContains(t, labels, "Activate")
}

func TestKnativeActions_Route(t *testing.T) {
	items := ActionsForKind("Route")
	labels := make([]string, 0, len(items))
	for _, it := range items {
		labels = append(labels, it.Label)
	}
	assert.Contains(t, labels, "Describe")
	assert.Contains(t, labels, "Edit")
	assert.Contains(t, labels, "Delete")
	assert.Contains(t, labels, "Events")
	assert.NotContains(t, labels, "Activate")
}
