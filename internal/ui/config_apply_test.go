package ui

import (
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/stretchr/testify/assert"
)

// snapshotSchedulerGlobals captures the current scheduler config globals so
// the test's defer can restore the exact prior state instead of clobbering
// to hardcoded defaults — which would leak state if a future test mutates
// the globals before this one runs.
func snapshotSchedulerGlobals() func() {
	origWorkers := scheduler.ConfigWorkersPerContext
	origCritical := scheduler.ConfigCriticalReserved
	origDefaultTimeout := scheduler.ConfigDefaultTimeout
	origTimeoutsByKind := scheduler.ConfigTimeoutsByKind
	return func() {
		scheduler.ConfigWorkersPerContext = origWorkers
		scheduler.ConfigCriticalReserved = origCritical
		scheduler.ConfigDefaultTimeout = origDefaultTimeout
		scheduler.ConfigTimeoutsByKind = origTimeoutsByKind
	}
}

func TestApplyConfig_SchedulerKnobs(t *testing.T) {
	defer snapshotSchedulerGlobals()()

	cfg := configFile{
		Scheduler: &SchedulerConfig{
			WorkersPerContext: 8,
			CriticalReserved:  2,
			DefaultTimeout:    "45s",
			TimeoutsByKind: map[string]string{
				"APIDiscovery": "90s",
				"Mutation":     "3m",
			},
		},
	}
	applyConfigOptions(cfg)

	assert.Equal(t, 8, scheduler.ConfigWorkersPerContext)
	assert.Equal(t, 2, scheduler.ConfigCriticalReserved)
	assert.Equal(t, 45*time.Second, scheduler.ConfigDefaultTimeout)
	assert.Equal(t, 90*time.Second, scheduler.ConfigTimeoutsByKind[scheduler.KindAPIDiscovery])
	assert.Equal(t, 3*time.Minute, scheduler.ConfigTimeoutsByKind[scheduler.KindMutation])
}

func TestApplyConfig_NilSchedulerSectionPreservesDefaults(t *testing.T) {
	defer snapshotSchedulerGlobals()()
	scheduler.ConfigWorkersPerContext = 99 // sentinel

	cfg := configFile{} // no Scheduler section
	applyConfigOptions(cfg)

	assert.Equal(t, 99, scheduler.ConfigWorkersPerContext, "no Scheduler section should leave globals untouched")
}

func TestApplyConfig_InvalidTimeoutStringIsIgnored(t *testing.T) {
	defer snapshotSchedulerGlobals()()

	// Pre-populate with a sentinel that is clearly NOT DefaultRequestTimeout
	// so the assertion proves applyConfigOptions left the global alone, not
	// that it happened to match the compiled-in default.
	const sentinel = 1234 * time.Millisecond
	scheduler.ConfigDefaultTimeout = sentinel

	cfg := configFile{
		Scheduler: &SchedulerConfig{
			DefaultTimeout: "not-a-duration",
		},
	}
	applyConfigOptions(cfg)

	assert.Equal(t, sentinel, scheduler.ConfigDefaultTimeout, "invalid duration must be ignored, prior value preserved")
}
