package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultKeybindings_CriticalDefaults(t *testing.T) {
	kb := DefaultKeybindings()
	// Verify critical defaults
	assert.Equal(t, "h", kb.Left)
	assert.Equal(t, "l", kb.Right)
	assert.Equal(t, "j", kb.Down)
	assert.Equal(t, "k", kb.Up)
	assert.Equal(t, "L", kb.Logs)
	assert.Equal(t, "v", kb.Describe)
	assert.Equal(t, "D", kb.Delete)
	assert.Equal(t, "ctrl+g", kb.FinalizerSearch)
	assert.Equal(t, " ", kb.ToggleSelect)
	assert.Equal(t, "t", kb.NewTab)
	assert.Equal(t, "m", kb.SetMark)
}

func TestMergeKeybindings_OverridesNonEmpty(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{
		Logs:     "l",
		Describe: "d",
	}
	MergeKeybindings(&dst, &src)
	assert.Equal(t, "l", dst.Logs)     // overridden
	assert.Equal(t, "d", dst.Describe) // overridden
	assert.Equal(t, "h", dst.Left)     // unchanged (src empty)
	assert.Equal(t, "D", dst.Delete)   // unchanged (src empty)
}

func TestMergeKeybindings_EmptySourceNoChange(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{} // all empty
	MergeKeybindings(&dst, &src)
	assert.Equal(t, DefaultKeybindings(), dst) // unchanged
}

func TestMergeKeybindings_AllFieldsOverridable(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{
		Left: "a", Right: "b", Down: "c", Up: "d",
		Help: "F1", Filter: "ctrl+f",
		ActionMenu: "z", NewTab: "ctrl+t",
	}
	MergeKeybindings(&dst, &src)
	assert.Equal(t, "a", dst.Left)
	assert.Equal(t, "b", dst.Right)
	assert.Equal(t, "F1", dst.Help)
	assert.Equal(t, "ctrl+f", dst.Filter)
	assert.Equal(t, "z", dst.ActionMenu)
	assert.Equal(t, "ctrl+t", dst.NewTab)
}

func TestDefaultAbbreviations_KnownEntries(t *testing.T) {
	abbr := DefaultAbbreviations()
	assert.Equal(t, "persistentvolumeclaim", abbr["pvc"])
	assert.Equal(t, "deployment", abbr["deploy"])
	assert.Equal(t, "pod", abbr["po"])
	assert.NotEmpty(t, abbr)
}

func TestColumnsForKind_CaseInsensitive(t *testing.T) {
	orig := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = orig })

	ConfigResourceColumns = map[string][]string{
		"pod": {"name", "status"},
	}

	assert.Equal(t, []string{"name", "status"}, ColumnsForKind("Pod", ""))
	assert.Equal(t, []string{"name", "status"}, ColumnsForKind("pod", ""))
	assert.Nil(t, ColumnsForKind("Deployment", ""))
	assert.Nil(t, ColumnsForKind("", ""))
}

func TestDefaultKeybindingsIncludeSecurity(t *testing.T) {
	kb := DefaultKeybindings()
	assert.Equal(t, "#", kb.Security)
}

func TestDefaultSecurityConfig(t *testing.T) {
	def := DefaultSecurityConfig()
	assert.True(t, def.Enabled)
	assert.True(t, def.SecColumn)
	assert.True(t, def.Sources["heuristic"].Enabled)
	assert.NotEmpty(t, def.Sources["heuristic"].Checks)
	assert.True(t, def.Sources["trivy_operator"].Enabled)
	assert.True(t, def.Sources["policy_report"].Enabled)
}

func TestDetectIconModeFromEnv(t *testing.T) {
	tests := []struct {
		name       string
		envLFK     string
		envTerm    string
		envProgram string
		want       string
	}{
		{"LFK_ICONS=nerdfont forces nerdfont", "nerdfont", "", "", "nerdfont"},
		{"LFK_ICONS=unicode forces unicode", "unicode", "", "", "unicode"},
		{"LFK_ICONS=simple forces simple", "simple", "", "", "simple"},
		{"LFK_ICONS=emoji forces emoji", "emoji", "", "", "emoji"},
		{"LFK_ICONS=none forces none", "none", "", "", "none"},
		{"LFK_ICONS=bogus falls through to TERM", "bogus", "xterm-ghostty", "", "nerdfont"},

		{"TERM=xterm-ghostty", "", "xterm-ghostty", "", "nerdfont"},
		{"TERM=xterm-ghostty-256color", "", "xterm-ghostty-256color", "", "nerdfont"},
		{"TERM=xterm-kitty", "", "xterm-kitty", "", "nerdfont"},
		{"TERM=screen.wezterm", "", "screen.wezterm", "", "nerdfont"},

		{"TERM=xterm-256color + TERM_PROGRAM=ghostty", "", "xterm-256color", "ghostty", "nerdfont"},
		{"TERM=xterm-256color + TERM_PROGRAM=WezTerm", "", "xterm-256color", "WezTerm", "nerdfont"},
		{"TERM=xterm-256color + TERM_PROGRAM=kitty", "", "xterm-256color", "kitty", "nerdfont"},
		{"TERM=xterm-256color + TERM_PROGRAM=iTerm.app", "", "xterm-256color", "iTerm.app", "unicode"},

		{"TERM and TERM_PROGRAM unset", "", "", "", "unicode"},
		{"TERM=dumb + LFK_ICONS=nerdfont (env override)", "nerdfont", "dumb", "", "nerdfont"},
		{"mixed case LFK_ICONS", "NERDFONT", "", "", "nerdfont"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LFK_ICONS", tc.envLFK)
			t.Setenv("TERM", tc.envTerm)
			t.Setenv("TERM_PROGRAM", tc.envProgram)
			if got := detectIconMode(); got != tc.want {
				t.Errorf("detectIconMode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyConfigOptionsIconResolution(t *testing.T) {
	tests := []struct {
		name     string
		cfgIcons string
		envLFK   string
		envTerm  string
		wantIcon string
	}{
		{"unset config + ghostty → nerdfont (auto)", "", "", "xterm-ghostty", "nerdfont"},
		{"unset config + no terminal hint → unicode (auto)", "", "", "", "unicode"},
		{"icons: auto explicit + ghostty → nerdfont", "auto", "", "xterm-ghostty", "nerdfont"},
		{"icons: unicode explicit + ghostty → unicode (override)", "unicode", "", "xterm-ghostty", "unicode"},
		{"icons: nerdfont explicit + non-nerd terminal → nerdfont (user's choice)", "nerdfont", "", "xterm-256color", "nerdfont"},
		{"icons: simple → simple", "simple", "", "", "simple"},
		{"icons: emoji → emoji", "emoji", "", "", "emoji"},
		{"icons: none → none", "none", "", "", "none"},
		{"unknown icons value → unicode fallback", "bogus", "", "", "unicode"},
		{"LFK_ICONS env overrides auto config", "auto", "unicode", "xterm-ghostty", "unicode"},
		{"LFK_ICONS env overrides explicit config", "simple", "nerdfont", "", "nerdfont"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LFK_ICONS", tc.envLFK)
			t.Setenv("TERM", tc.envTerm)
			t.Setenv("TERM_PROGRAM", "")

			prev := IconMode
			defer func() { IconMode = prev }()
			IconMode = "unicode" // reset

			cfg := configFile{Icons: tc.cfgIcons}
			applyConfigOptions(cfg)

			if IconMode != tc.wantIcon {
				t.Errorf("IconMode = %q, want %q", IconMode, tc.wantIcon)
			}
		})
	}
}

func TestResolveReadOnly_PrecedenceCLIOverConfig(t *testing.T) {
	prevGlobal := ConfigReadOnly
	prevCluster := ConfigClusterReadOnly
	t.Cleanup(func() {
		ConfigReadOnly = prevGlobal
		ConfigClusterReadOnly = prevCluster
	})

	ConfigReadOnly = false
	ConfigClusterReadOnly = map[string]bool{}

	// CLI flag wins regardless of config.
	assert.True(t, ResolveReadOnly("any", true))

	// No CLI, no config -> false.
	assert.False(t, ResolveReadOnly("any", false))

	// Global config only.
	ConfigReadOnly = true
	assert.True(t, ResolveReadOnly("any", false))
	assert.True(t, ResolveReadOnly("prod", false))

	// Per-context override beats global (false beats true).
	ConfigClusterReadOnly = map[string]bool{"dev": false}
	assert.False(t, ResolveReadOnly("dev", false), "dev override should disable read-only")
	assert.True(t, ResolveReadOnly("prod", false), "prod still inherits global")

	// Per-context override beats global (true beats false).
	ConfigReadOnly = false
	ConfigClusterReadOnly = map[string]bool{"prod": true}
	assert.True(t, ResolveReadOnly("prod", false))
	assert.False(t, ResolveReadOnly("dev", false))
}

func TestApplyConfigOptions_ReadOnlyGlobalAndPerCluster(t *testing.T) {
	prevGlobal := ConfigReadOnly
	prevCluster := ConfigClusterReadOnly
	t.Cleanup(func() {
		ConfigReadOnly = prevGlobal
		ConfigClusterReadOnly = prevCluster
	})
	ConfigReadOnly = false
	ConfigClusterReadOnly = map[string]bool{}

	tru := true
	fls := false
	cfg := configFile{
		ReadOnly: &tru,
		Clusters: map[string]clusterConfig{
			"prod": {ReadOnly: &tru},
			"dev":  {ReadOnly: &fls},
			"void": {}, // no override
		},
	}
	applyConfigOptions(cfg)
	applyConfigMaps(cfg, map[string]string{})

	assert.True(t, ConfigReadOnly, "global read_only must be applied")
	assert.True(t, ConfigClusterReadOnly["prod"])
	assert.False(t, ConfigClusterReadOnly["dev"])
	_, hasVoid := ConfigClusterReadOnly["void"]
	assert.False(t, hasVoid, "clusters without an explicit read_only must not register an entry")
}
