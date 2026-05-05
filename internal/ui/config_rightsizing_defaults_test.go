package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestApplyConfig_RightsizingDefaults verifies that a configFile with
// rightsizing_defaults populated propagates the parsed strategy +
// headroom into the model package-level vars used by
// executeActionRightsizing.
func TestApplyConfig_RightsizingDefaults(t *testing.T) {
	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
	})
	model.ConfigDefaultRightsizingStrategy = ""
	model.ConfigDefaultRightsizingHeadroom = 0

	cfg := configFile{
		RightsizingDefaults: &RightsizingDefaultsConfig{
			Strategy: "prom_max_1d",
			Headroom: 1.5,
		},
	}
	applyConfigOptions(cfg)

	assert.Equal(t, model.StrategyPromMax1D, model.ConfigDefaultRightsizingStrategy,
		"valid strategy must propagate into model.ConfigDefaultRightsizingStrategy")
	assert.InDelta(t, 1.5, model.ConfigDefaultRightsizingHeadroom, 1e-9,
		"valid headroom must propagate into model.ConfigDefaultRightsizingHeadroom")
}

// TestApplyConfig_RightsizingDefaults_AllStrategies covers each
// supported strategy literal so a future rename / addition gets
// caught at the boundary.
func TestApplyConfig_RightsizingDefaults_AllStrategies(t *testing.T) {
	cases := []struct {
		raw  string
		want model.RightsizingStrategy
	}{
		{"vpa", model.StrategyVPA},
		{"prom_max_1d", model.StrategyPromMax1D},
		{"prom_avg_1d", model.StrategyPromAvg1D},
		{"prom_p95_7d", model.StrategyPromP957D},
		{"snapshot", model.StrategySnapshot},
	}
	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
	})

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			model.ConfigDefaultRightsizingStrategy = ""
			cfg := configFile{
				RightsizingDefaults: &RightsizingDefaultsConfig{Strategy: tc.raw},
			}
			applyConfigOptions(cfg)
			assert.Equal(t, tc.want, model.ConfigDefaultRightsizingStrategy)
		})
	}
}

// TestApplyConfig_RightsizingDefaultsInvalidStrategyIgnored verifies
// that a bogus strategy literal leaves
// model.ConfigDefaultRightsizingStrategy empty so the runtime
// fallback chain (sticky -> built-in) takes over instead of carrying
// a junk value into the picker.
func TestApplyConfig_RightsizingDefaultsInvalidStrategyIgnored(t *testing.T) {
	prevStrategy := model.ConfigDefaultRightsizingStrategy
	t.Cleanup(func() { model.ConfigDefaultRightsizingStrategy = prevStrategy })
	model.ConfigDefaultRightsizingStrategy = ""

	cfg := configFile{
		RightsizingDefaults: &RightsizingDefaultsConfig{Strategy: "garbage"},
	}
	applyConfigOptions(cfg)

	assert.Empty(t, model.ConfigDefaultRightsizingStrategy,
		"invalid strategy literal must be ignored, leaving the var empty")
}

// TestApplyConfig_RightsizingDefaultsInvalidHeadroomIgnored covers
// the float-comparison path: a value not within 1e-9 of any preset
// in model.RightsizingHeadrooms is ignored.
func TestApplyConfig_RightsizingDefaultsInvalidHeadroomIgnored(t *testing.T) {
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() { model.ConfigDefaultRightsizingHeadroom = prevHeadroom })
	model.ConfigDefaultRightsizingHeadroom = 0

	cfg := configFile{
		RightsizingDefaults: &RightsizingDefaultsConfig{Headroom: 1.337},
	}
	applyConfigOptions(cfg)

	assert.InDelta(t, 0.0, model.ConfigDefaultRightsizingHeadroom, 1e-9,
		"non-preset headroom must be ignored (left at zero)")
}

// TestApplyConfig_RightsizingDefaults_AllValidHeadrooms covers each
// preset to make sure the float epsilon comparison catches the
// canonical values exactly.
func TestApplyConfig_RightsizingDefaults_AllValidHeadrooms(t *testing.T) {
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() { model.ConfigDefaultRightsizingHeadroom = prevHeadroom })

	for _, want := range model.RightsizingHeadrooms {
		t.Run("", func(t *testing.T) {
			model.ConfigDefaultRightsizingHeadroom = 0
			cfg := configFile{
				RightsizingDefaults: &RightsizingDefaultsConfig{Headroom: want},
			}
			applyConfigOptions(cfg)
			assert.InDelta(t, want, model.ConfigDefaultRightsizingHeadroom, 1e-9)
		})
	}
}

// TestApplyConfig_RightsizingDefaults_NilSectionLeavesDefaults verifies
// that omitting the rightsizing_defaults section entirely leaves the
// model vars untouched — matches the "unset" scenario where a user
// has no opinion and wants the runtime default.
func TestApplyConfig_RightsizingDefaults_NilSectionLeavesDefaults(t *testing.T) {
	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
	})
	model.ConfigDefaultRightsizingStrategy = model.StrategyVPA
	model.ConfigDefaultRightsizingHeadroom = 1.5

	cfg := configFile{} // RightsizingDefaults nil
	applyConfigOptions(cfg)

	// Pre-existing (non-zero) values must survive the apply pass when
	// the section is omitted — otherwise reloading config without the
	// section would silently drop user choices.
	assert.Equal(t, model.StrategyVPA, model.ConfigDefaultRightsizingStrategy,
		"omitted section must not clear an existing value")
	assert.InDelta(t, 1.5, model.ConfigDefaultRightsizingHeadroom, 1e-9,
		"omitted section must not clear an existing value")
}

// TestLoadConfig_RightsizingDefaultsFromYAML exercises the end-to-end
// path: YAML on disk → unmarshaled into configFile → applied via
// applyRightsizingDefaults. Catches snake_case / json-tag mismatches
// that pure struct-literal tests can't surface.
func TestLoadConfig_RightsizingDefaultsFromYAML(t *testing.T) {
	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
	})
	model.ConfigDefaultRightsizingStrategy = ""
	model.ConfigDefaultRightsizingHeadroom = 0

	yaml := "" +
		"rightsizing_defaults:\n" +
		"  strategy: prom_max_1d\n" +
		"  headroom: 1.5\n"
	path := writeConfigFile(t, yaml)
	LoadConfig(path)

	assert.Equal(t, model.StrategyPromMax1D, model.ConfigDefaultRightsizingStrategy,
		"YAML strategy must propagate through LoadConfig")
	assert.InDelta(t, 1.5, model.ConfigDefaultRightsizingHeadroom, 1e-9,
		"YAML headroom must propagate through LoadConfig")
}
