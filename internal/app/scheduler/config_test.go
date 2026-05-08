package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClampWorkers_BelowMin(t *testing.T) {
	assert.Equal(t, 1, ClampWorkers(0))
	assert.Equal(t, 1, ClampWorkers(-5))
}

func TestClampWorkers_AboveMax(t *testing.T) {
	assert.Equal(t, 16, ClampWorkers(17))
	assert.Equal(t, 16, ClampWorkers(1000))
}

func TestClampWorkers_InRange(t *testing.T) {
	for n := 1; n <= 16; n++ {
		assert.Equal(t, n, ClampWorkers(n))
	}
}

func TestClampCriticalReserved(t *testing.T) {
	// Reserved must not exceed half of total workers.
	assert.Equal(t, 0, ClampCriticalReserved(-1, 4))
	assert.Equal(t, 0, ClampCriticalReserved(0, 4))
	assert.Equal(t, 1, ClampCriticalReserved(1, 4))
	assert.Equal(t, 2, ClampCriticalReserved(2, 4))
	assert.Equal(t, 2, ClampCriticalReserved(3, 4)) // half of 4
	assert.Equal(t, 2, ClampCriticalReserved(99, 4))
	assert.Equal(t, 0, ClampCriticalReserved(1, 1)) // half of 1 = 0
}

func TestTimeoutFor_DefaultWhenUnset(t *testing.T) {
	c := &Config{Default: 30 * time.Second}
	assert.Equal(t, 30*time.Second, c.TimeoutFor(KindResourceList))
	assert.Equal(t, 30*time.Second, c.TimeoutFor(KindAPIDiscovery))
}

func TestTimeoutFor_KindOverride(t *testing.T) {
	c := &Config{
		Default: 30 * time.Second,
		ByKind: map[Kind]time.Duration{
			KindAPIDiscovery: 60 * time.Second,
			KindMutation:     2 * time.Minute,
		},
	}
	assert.Equal(t, 60*time.Second, c.TimeoutFor(KindAPIDiscovery))
	assert.Equal(t, 2*time.Minute, c.TimeoutFor(KindMutation))
	assert.Equal(t, 30*time.Second, c.TimeoutFor(KindResourceList)) // falls through to default
}

func TestTimeoutFor_NilConfigUsesFallback(t *testing.T) {
	var c *Config
	assert.Equal(t, DefaultRequestTimeout, c.TimeoutFor(KindResourceList))
}

func TestFromGlobals_SnapshotsConfig(t *testing.T) {
	// Save and restore globals so we don't pollute other tests.
	origWorkers := ConfigWorkersPerContext
	origReserved := ConfigCriticalReserved
	origTimeout := ConfigDefaultTimeout
	origByKind := ConfigTimeoutsByKind
	defer func() {
		ConfigWorkersPerContext = origWorkers
		ConfigCriticalReserved = origReserved
		ConfigDefaultTimeout = origTimeout
		ConfigTimeoutsByKind = origByKind
	}()

	ConfigWorkersPerContext = 8
	ConfigCriticalReserved = 2
	ConfigDefaultTimeout = 45 * time.Second
	ConfigTimeoutsByKind = map[Kind]time.Duration{KindMutation: 90 * time.Second}

	c := FromGlobals()
	assert.Equal(t, 8, c.WorkersPerContext)
	assert.Equal(t, 2, c.CriticalReserved)
	assert.Equal(t, 45*time.Second, c.Default)
	assert.Equal(t, 90*time.Second, c.ByKind[KindMutation])
	// Defensive: make sure the snapshot is a copy, not the same map.
	c.ByKind[KindMutation] = 1 * time.Second
	assert.Equal(t, 90*time.Second, ConfigTimeoutsByKind[KindMutation], "FromGlobals must deep-copy ByKind")
}
