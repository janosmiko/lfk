package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
)

// --- RenderRollbackOverlay ---

func TestRenderRollbackOverlay(t *testing.T) {
	t.Run("empty revisions shows message", func(t *testing.T) {
		result := RenderRollbackOverlay(nil, 0, 100, 40)
		assert.Contains(t, result, "No revisions found")
	})

	t.Run("revisions rendered with columns", func(t *testing.T) {
		revisions := []k8s.DeploymentRevision{
			{
				Revision:  3,
				Name:      "nginx-abc123",
				Replicas:  2,
				Images:    []string{"nginx:1.21"},
				CreatedAt: time.Now().Add(-1 * time.Hour),
			},
			{
				Revision:  2,
				Name:      "nginx-def456",
				Replicas:  1,
				Images:    []string{"nginx:1.20"},
				CreatedAt: time.Now().Add(-24 * time.Hour),
			},
		}
		result := RenderRollbackOverlay(revisions, 0, 120, 40)
		assert.Contains(t, result, "Rollback Deployment")
		assert.Contains(t, result, "REV")
		assert.Contains(t, result, "REPLICASET")
		assert.Contains(t, result, "PODS")
		assert.Contains(t, result, "IMAGE")
		assert.Contains(t, result, "AGE")
		assert.Contains(t, result, "nginx-abc123")
		assert.Contains(t, result, "nginx:1.21")
		assert.Contains(t, result, "nginx-def456")
	})

	t.Run("cursor highlights selected revision", func(t *testing.T) {
		revisions := []k8s.DeploymentRevision{
			{Revision: 2, Name: "rs1", Images: []string{"img:1"}, CreatedAt: time.Now()},
			{Revision: 1, Name: "rs2", Images: []string{"img:0"}, CreatedAt: time.Now()},
		}
		result := RenderRollbackOverlay(revisions, 1, 100, 40)
		// Second revision should be rendered (it's the cursor position).
		assert.Contains(t, result, "rs2")
	})

	t.Run("multiple images shows plus indicator", func(t *testing.T) {
		revisions := []k8s.DeploymentRevision{
			{
				Revision: 1,
				Name:     "multi",
				Images:   []string{"img1:latest", "img2:latest", "img3:latest"},
			},
		}
		result := RenderRollbackOverlay(revisions, 0, 120, 40)
		assert.Contains(t, result, "img1:latest")
		assert.Contains(t, result, "+2")
	})

	t.Run("hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		revisions := []k8s.DeploymentRevision{
			{Revision: 1, Name: "rs1", Images: []string{"img:1"}},
		}
		result := RenderRollbackOverlay(revisions, 0, 100, 40)
		assert.Contains(t, result, "Rollback Deployment")
	})
}

// --- RenderHelmRollbackOverlay ---

func TestRenderHelmRollbackOverlay(t *testing.T) {
	t.Run("empty revisions shows message", func(t *testing.T) {
		result := RenderHelmRollbackOverlay(nil, 0, 100, 40)
		assert.Contains(t, result, "No revisions found")
	})

	t.Run("revisions rendered with columns", func(t *testing.T) {
		revisions := []HelmRevision{
			{
				Revision:    3,
				Status:      "deployed",
				Chart:       "nginx-1.2.3",
				AppVersion:  "1.21",
				Description: "Upgrade complete",
				Updated:     "2024-01-15 10:00:00",
			},
			{
				Revision:    2,
				Status:      "superseded",
				Chart:       "nginx-1.2.2",
				AppVersion:  "1.20",
				Description: "Install complete",
				Updated:     "2024-01-14 10:00:00",
			},
		}
		result := RenderHelmRollbackOverlay(revisions, 0, 120, 40)
		assert.Contains(t, result, "Rollback Helm Release")
		assert.Contains(t, result, "REV")
		assert.Contains(t, result, "STATUS")
		assert.Contains(t, result, "CHART")
		assert.Contains(t, result, "APP VER")
		assert.Contains(t, result, "DESCRIPTION")
		assert.Contains(t, result, "UPDATED")
		assert.Contains(t, result, "deployed")
		assert.Contains(t, result, "nginx-1.2.3")
		assert.Contains(t, result, "Upgrade complete")
	})

	t.Run("cursor highlights selected revision", func(t *testing.T) {
		revisions := []HelmRevision{
			{Revision: 2, Status: "deployed", Chart: "app-1.0"},
			{Revision: 1, Status: "superseded", Chart: "app-0.9"},
		}
		result := RenderHelmRollbackOverlay(revisions, 1, 120, 40)
		assert.Contains(t, result, "app-0.9")
	})

	t.Run("hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		revisions := []HelmRevision{
			{Revision: 1, Status: "deployed", Chart: "app-1.0"},
		}
		result := RenderHelmRollbackOverlay(revisions, 0, 100, 40)
		assert.Contains(t, result, "Rollback Helm Release")
	})
}
