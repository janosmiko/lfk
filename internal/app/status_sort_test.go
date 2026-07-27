package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// statusesInOrder returns each item's status in the order they currently sit,
// so a sort assertion reads as the column the user sees.
func statusesInOrder(items []model.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Status
	}
	return out
}

// Sorting by Status must group rows carrying the same status string together.
// Before the fix, statuses missing from the app-local priority table
// (Terminating, Succeeded, NotReady, ...) all shared one catch-all bucket and
// tie-broke on Name, so a real pod list interleaved them alphabetically and
// looked unsorted.
func TestSortByStatusGroupsIdenticalStatuses(t *testing.T) {
	// Names deliberately interleave the statuses alphabetically — this is the
	// shape of a real cluster list (argocd-* Terminating sorting between
	// backup-* Succeeded rows).
	items := []model.Item{
		{Name: "argocd-application-controller-0", Status: "Terminating"},
		{Name: "backup-db-cronjob-8f7gx", Status: "Succeeded"},
		{Name: "instance-manager-9c5ff", Status: "Terminating"},
		{Name: "laravel-cron-7wcfl", Status: "Succeeded"},
		{Name: "logging-alloy-lf29t", Status: "NotReady"},
		{Name: "magento-cron-7wk4k", Status: "Succeeded"},
		{Name: "overprovisioning-jf2xb", Status: "Pending"},
		{Name: "varnish-1", Status: "Running"},
	}

	sortItemsByColumn(items, "Status", true, "Pod")

	assert.Equal(t, []string{
		"Running",
		// NotReady, Pending and Terminating share the in-progress bucket;
		// within it each status string still groups.
		"NotReady",
		"Pending",
		"Terminating", "Terminating",
		"Succeeded", "Succeeded", "Succeeded",
	}, statusesInOrder(items))
}

// Within one status string, rows still order by name, so a refresh can never
// reshuffle same-status rows.
func TestSortByStatusOrdersByNameWithinStatus(t *testing.T) {
	items := []model.Item{
		{Name: "zeta", Status: "Succeeded"},
		{Name: "alpha", Status: "Succeeded"},
		{Name: "mid", Status: "Succeeded"},
	}

	sortItemsByColumn(items, "Status", true, "Pod")

	assert.Equal(t, []string{"alpha", "mid", "zeta"}, itemNames(items))
}

// Descending Status flips the buckets and the status groups, but same-status
// rows keep their ascending name order — matching the direction-independent
// tiebreaker chain the other columns use.
func TestSortByStatusDescendingKeepsNameOrderWithinStatus(t *testing.T) {
	items := []model.Item{
		{Name: "beta", Status: "Running"},
		{Name: "alpha", Status: "Succeeded"},
		{Name: "gamma", Status: "Running"},
	}

	sortItemsByColumn(items, "Status", false, "Pod")

	assert.Equal(t, []string{"Succeeded", "Running", "Running"}, statusesInOrder(items))
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, itemNames(items))
}

// Free-form CRD statuses bucket through the same word fallback that colors
// them, so an operator phrase sorts with its severity peers instead of
// dropping into the no-signal bucket.
func TestSortByStatusBucketsFreeFormPhrases(t *testing.T) {
	items := []model.Item{
		{Name: "b", Status: "Succeeded"},
		{Name: "a", Status: "Cluster in healthy state"},
	}

	sortItemsByColumn(items, "Status", true, "Cluster")

	assert.Equal(t, []string{"Cluster in healthy state", "Succeeded"}, statusesInOrder(items))
}
