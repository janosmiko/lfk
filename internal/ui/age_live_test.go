package ui

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

// LiveAge must compute Age from CreatedAt at call time so the rendered
// value reflects the current moment, not the precomputed string captured
// at load time. Watch ticks and shift+r refresh by re-rendering — the
// Age column has to follow.
func TestLiveAgeRecomputesFromCreatedAt(t *testing.T) {
	item := model.Item{
		Age:       "5m", // stale precomputed value
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	got := LiveAge(item)
	assert.Equal(t, "2h", got, "LiveAge must use CreatedAt, not the stored Age string")
}

func TestLiveAgeFallbackWhenCreatedAtZero(t *testing.T) {
	// Some items (synthetic rows like __port_forwards__, helm releases
	// served from helm.go merging) only set Age, not CreatedAt. Fall
	// back to the precomputed string instead of rendering "0s".
	item := model.Item{Age: "12d"}
	assert.Equal(t, "12d", LiveAge(item))
}

func TestLiveAgeEmptyWhenBothMissing(t *testing.T) {
	assert.Equal(t, "", LiveAge(model.Item{}))
}
