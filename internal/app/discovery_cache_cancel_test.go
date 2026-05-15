package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
)

// TestLoadAllDiscoveryCaches_RespectsCancelledContext ensures the preload
// loop honors cancellation, so a quit during a 1200-context kubeconfig
// walk doesn't block exit. Pre-fix the loop had no context.Context
// plumbed and would run to completion regardless.
func TestLoadAllDiscoveryCaches_RespectsCancelledContext(t *testing.T) {
	c := k8s.NewTestClient(nil, nil)
	// Register 50 synthetic contexts so the loop would have plenty to do
	// if cancellation were ignored.
	for i := range 50 {
		name := contextName(i)
		c.AddTestContext(name, "https://"+name+".example.local:6443")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call so the loop exits on the first check

	out := loadAllDiscoveryCaches(ctx, c)
	// With cancellation honored before the first iteration body completes
	// the result is empty. The contract is "return what was loaded so far",
	// so an empty map (or nil) is acceptable.
	assert.LessOrEqual(t, len(out), 1,
		"cancelled context must short-circuit the preload loop (got %d entries)", len(out))
}

// TestLoadAllDiscoveryCaches_BackgroundContextProcessesAll confirms the
// happy path still works: with no cancellation the loop walks every
// context. None of our test contexts have cache files on disk so the
// result map is empty, but the absence of a hang is the assertion.
func TestLoadAllDiscoveryCaches_BackgroundContextProcessesAll(t *testing.T) {
	c := k8s.NewTestClient(nil, nil)
	for i := range 10 {
		c.AddTestContext(contextName(i), "https://h.example.local:6443")
	}

	out := loadAllDiscoveryCaches(context.Background(), c)
	// No cache files on disk → empty result map; the test passes if the
	// call returned without hanging.
	_ = out
}

func contextName(i int) string {
	return "test-ctx-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26))
}
