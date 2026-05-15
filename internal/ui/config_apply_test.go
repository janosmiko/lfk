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

// snapshotKubeconfigDirs captures and restores ConfigKubeconfigDirs so tests
// don't leak global state into each other.
func snapshotKubeconfigDirs() func() {
	orig := ConfigKubeconfigDirs
	return func() { ConfigKubeconfigDirs = orig }
}

func TestKubeconfigDirsSetting_UnmarshalJSON(t *testing.T) {
	t.Run("string form parsed into single-element slice", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`"/etc/k8s/configs"`)))
		assert.Equal(t, []string{"/etc/k8s/configs"}, s.paths)
		assert.False(t, s.invalid)
	})
	t.Run("string form trims whitespace", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`"  /etc/k8s  "`)))
		assert.Equal(t, []string{"/etc/k8s"}, s.paths)
	})
	t.Run("whitespace-only string yields nil paths", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`"   "`)))
		assert.Empty(t, s.paths)
		assert.False(t, s.invalid)
	})
	t.Run("list form parsed into multi-element slice", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`["/dir/one", "/dir/two"]`)))
		assert.Equal(t, []string{"/dir/one", "/dir/two"}, s.paths)
	})
	t.Run("list form trims and drops empty entries", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`["  /dir/one  ", "", "  ", "/dir/two"]`)))
		assert.Equal(t, []string{"/dir/one", "/dir/two"}, s.paths)
	})
	t.Run("empty list yields nil paths", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`[]`)))
		assert.Empty(t, s.paths)
	})
	t.Run("number is captured as invalid", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`42`)))
		assert.True(t, s.invalid, "number is not a valid kubeconfig_dir shape")
	})
	t.Run("object is captured as invalid", func(t *testing.T) {
		var s kubeconfigDirsSetting
		assert.NoError(t, s.UnmarshalJSON([]byte(`{"path": "/foo"}`)))
		assert.True(t, s.invalid, "object is not a valid kubeconfig_dir shape")
	})
}

func TestApplyKubeconfigDirsSetting(t *testing.T) {
	t.Run("plain value assigned", func(t *testing.T) {
		defer snapshotKubeconfigDirs()()
		ConfigKubeconfigDirs = nil
		applyKubeconfigDirsSetting(&kubeconfigDirsSetting{paths: []string{"/etc/k8s/configs"}})
		assert.Equal(t, []string{"/etc/k8s/configs"}, ConfigKubeconfigDirs)
	})
	t.Run("multi-value list assigned", func(t *testing.T) {
		defer snapshotKubeconfigDirs()()
		ConfigKubeconfigDirs = nil
		applyKubeconfigDirsSetting(&kubeconfigDirsSetting{paths: []string{"/dir/one", "/dir/two"}})
		assert.Equal(t, []string{"/dir/one", "/dir/two"}, ConfigKubeconfigDirs)
	})
	t.Run("nil setting is no-op", func(t *testing.T) {
		defer snapshotKubeconfigDirs()()
		ConfigKubeconfigDirs = []string{"/prior"}
		applyKubeconfigDirsSetting(nil)
		assert.Equal(t, []string{"/prior"}, ConfigKubeconfigDirs)
	})
	t.Run("empty paths setting is no-op", func(t *testing.T) {
		defer snapshotKubeconfigDirs()()
		ConfigKubeconfigDirs = []string{"/prior"}
		applyKubeconfigDirsSetting(&kubeconfigDirsSetting{paths: nil})
		assert.Equal(t, []string{"/prior"}, ConfigKubeconfigDirs,
			"empty/whitespace-only YAML must not silently overwrite a previously-set value")
	})
	t.Run("invalid setting logs warn and leaves prior value untouched", func(t *testing.T) {
		defer snapshotKubeconfigDirs()()
		ConfigKubeconfigDirs = []string{"/prior"}
		applyKubeconfigDirsSetting(&kubeconfigDirsSetting{invalid: true, raw: "42"})
		assert.Equal(t, []string{"/prior"}, ConfigKubeconfigDirs)
	})
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
