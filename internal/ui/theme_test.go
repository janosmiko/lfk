package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

// yamlUnmarshal is a test helper to parse YAML config data.
func yamlUnmarshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()
	assert.NotEmpty(t, theme.Primary)
	assert.NotEmpty(t, theme.Secondary)
	assert.NotEmpty(t, theme.Text)
	assert.NotEmpty(t, theme.SelectedFg)
	assert.NotEmpty(t, theme.SelectedBg)
	assert.NotEmpty(t, theme.Border)
	assert.NotEmpty(t, theme.Dimmed)
	assert.NotEmpty(t, theme.Error)
	assert.NotEmpty(t, theme.Warning)
	assert.NotEmpty(t, theme.Purple)
	assert.NotEmpty(t, theme.Base)
	assert.NotEmpty(t, theme.BarBg)
	assert.NotEmpty(t, theme.Surface)

	// Default should be tokyonight values.
	assert.Equal(t, "#7aa2f7", theme.Primary)
}

func TestDefaultKeybindings(t *testing.T) {
	kb := DefaultKeybindings()
	assert.NotEmpty(t, kb.Logs)
	assert.NotEmpty(t, kb.Refresh)
	assert.NotEmpty(t, kb.Restart)
	assert.NotEmpty(t, kb.Exec)
	assert.NotEmpty(t, kb.Describe)
	assert.NotEmpty(t, kb.Delete)
	assert.NotEmpty(t, kb.ForceDelete)
	assert.NotEmpty(t, kb.Scale)
}

func TestDefaultAbbreviations(t *testing.T) {
	abbr := DefaultAbbreviations()
	assert.NotEmpty(t, abbr)

	// Check some known abbreviations.
	assert.Equal(t, "deployment", abbr["deploy"])
	assert.Equal(t, "pod", abbr["po"])
	assert.Equal(t, "service", abbr["svc"])
	assert.Equal(t, "namespace", abbr["ns"])
	assert.Equal(t, "configmap", abbr["cm"])
	assert.Equal(t, "horizontalpodautoscaler", abbr["hpa"])
}

func TestMergeThemeOverrides(t *testing.T) {
	t.Run("overrides non-empty fields", func(t *testing.T) {
		base := DefaultTheme()
		overrides := Theme{
			Primary: "#ff0000",
			Error:   "#00ff00",
		}
		mergeThemeOverrides(&base, overrides)
		assert.Equal(t, "#ff0000", base.Primary)
		assert.Equal(t, "#00ff00", base.Error)
		// Other fields should remain default.
		assert.Equal(t, "#9ece6a", base.Secondary)
	})

	t.Run("empty overrides change nothing", func(t *testing.T) {
		base := DefaultTheme()
		original := base
		mergeThemeOverrides(&base, Theme{})
		assert.Equal(t, original, base)
	})

	t.Run("all fields overridden", func(t *testing.T) {
		base := DefaultTheme()
		overrides := Theme{
			Primary:    "#a",
			Secondary:  "#b",
			Text:       "#c",
			SelectedFg: "#d",
			SelectedBg: "#e",
			Border:     "#f",
			Dimmed:     "#g",
			Error:      "#h",
			Warning:    "#i",
			Purple:     "#j",
			Base:       "#k",
			BarBg:      "#l",
			Surface:    "#m",
		}
		mergeThemeOverrides(&base, overrides)
		assert.Equal(t, overrides, base)
	})
}

func TestApplyTheme(t *testing.T) {
	// Apply a theme and verify it doesn't panic.
	theme := DefaultTheme()
	ApplyTheme(theme)

	// Verify some global styles were set (they should render without panic).
	_ = SelectedStyle.Render("test")
	_ = NormalStyle.Render("test")
	_ = ErrorStyle.Render("test")
	_ = DimStyle.Render("test")
	_ = TitleBarStyle.Render("test")
	_ = OverlayStyle.Render("test")
}

func TestColumnsForKind(t *testing.T) {
	// Save and restore globals after test.
	origResource := ConfigResourceColumns
	t.Cleanup(func() {
		ConfigResourceColumns = origResource
	})

	t.Run("returns nil when nothing configured", func(t *testing.T) {
		ConfigResourceColumns = nil
		assert.Nil(t, ColumnsForKind("Pod", ""))
		assert.Nil(t, ColumnsForKind("", ""))
	})

	t.Run("returns nil for unconfigured kind", func(t *testing.T) {
		ConfigResourceColumns = map[string][]string{
			"pod": {"IP", "Node", "Image"},
		}
		assert.Nil(t, ColumnsForKind("Deployment", ""))
	})

	t.Run("returns columns for configured kind", func(t *testing.T) {
		ConfigResourceColumns = map[string][]string{
			"pod": {"IP", "Node", "Image"},
		}
		assert.Equal(t, []string{"IP", "Node", "Image"}, ColumnsForKind("Pod", ""))
	})

	t.Run("kind lookup is case-insensitive", func(t *testing.T) {
		ConfigResourceColumns = map[string][]string{
			"deployment": {"Replicas", "Available"},
		}
		assert.Equal(t, []string{"Replicas", "Available"}, ColumnsForKind("Deployment", ""))
		assert.Equal(t, []string{"Replicas", "Available"}, ColumnsForKind("deployment", ""))
		assert.Equal(t, []string{"Replicas", "Available"}, ColumnsForKind("DEPLOYMENT", ""))
	})

	t.Run("empty kind returns nil", func(t *testing.T) {
		ConfigResourceColumns = map[string][]string{
			"pod": {"IP"},
		}
		assert.Nil(t, ColumnsForKind("", ""))
	})

	t.Run("wildcard works", func(t *testing.T) {
		ConfigResourceColumns = map[string][]string{
			"pod": {"*"},
		}
		assert.Equal(t, []string{"*"}, ColumnsForKind("Pod", ""))
	})
}

func TestCustomActionsConfigParsing(t *testing.T) {
	t.Run("custom actions parsed from YAML", func(t *testing.T) {
		yamlData := []byte(`
custom_actions:
  Pod:
    - label: "SSH to Node"
      command: "ssh {Node}"
      key: "S"
      description: "SSH into the pod's node"
    - label: "Copy Logs"
      command: "kubectl logs {name} -n {namespace}"
      key: "C"
      description: "Copy pod logs"
  Deployment:
    - label: "Image History"
      command: "kubectl rollout history deployment/{name}"
      key: "H"
      description: "Show rollout history"
`)
		var cfg configFile
		err := yamlUnmarshal(yamlData, &cfg)
		assert.NoError(t, err)
		assert.Len(t, cfg.CustomActions["Pod"], 2)
		assert.Len(t, cfg.CustomActions["Deployment"], 1)
		assert.Equal(t, "SSH to Node", cfg.CustomActions["Pod"][0].Label)
		assert.Equal(t, "ssh {Node}", cfg.CustomActions["Pod"][0].Command)
		assert.Equal(t, "S", cfg.CustomActions["Pod"][0].Key)
		assert.Equal(t, "SSH into the pod's node", cfg.CustomActions["Pod"][0].Description)
		assert.Equal(t, "Image History", cfg.CustomActions["Deployment"][0].Label)
	})

	t.Run("empty custom actions", func(t *testing.T) {
		yamlData := []byte(`colorscheme: tokyonight`)
		var cfg configFile
		err := yamlUnmarshal(yamlData, &cfg)
		assert.NoError(t, err)
		assert.Nil(t, cfg.CustomActions)
	})
}

func TestApplyThemeWithDifferentSchemes(t *testing.T) {
	// Apply each built-in scheme and verify no panics.
	for name, scheme := range BuiltinSchemes() {
		t.Run(name, func(t *testing.T) {
			ApplyTheme(scheme)
			_ = SelectedStyle.Render("test")
			_ = NormalStyle.Render("test")
		})
	}
	// Restore default.
	ApplyTheme(DefaultTheme())
}

func TestTransparentBgConfig(t *testing.T) {
	t.Run("parses transparent_background from YAML", func(t *testing.T) {
		data := []byte("transparent_background: true\n")
		var cfg configFile
		err := yamlUnmarshal(data, &cfg)
		assert.NoError(t, err)
		assert.NotNil(t, cfg.TransparentBg)
		assert.True(t, *cfg.TransparentBg)
	})

	t.Run("defaults to false when not set", func(t *testing.T) {
		data := []byte("colorscheme: tokyonight\n")
		var cfg configFile
		err := yamlUnmarshal(data, &cfg)
		assert.NoError(t, err)
		assert.Nil(t, cfg.TransparentBg)
	})

	t.Run("transparent mode skips bar backgrounds", func(t *testing.T) {
		orig := ConfigTransparentBg
		defer func() {
			ConfigTransparentBg = orig
			ApplyTheme(DefaultTheme())
		}()

		theme := DefaultTheme()

		// Opaque mode: bars and columns should have background set.
		ConfigTransparentBg = false
		ApplyTheme(theme)
		// lipgloss.NoColor{} is the zero type when no background is set.
		_, isNoColor := TitleBarStyle.GetBackground().(lipgloss.NoColor)
		assert.False(t, isNoColor, "opaque TitleBarStyle should have a background color")

		_, isNoColor = StatusBarBgStyle.GetBackground().(lipgloss.NoColor)
		assert.False(t, isNoColor, "opaque StatusBarBgStyle should have a background color")

		_, isNoColor = ActiveColumnStyle.GetBackground().(lipgloss.NoColor)
		assert.False(t, isNoColor, "opaque ActiveColumnStyle should have a background color")

		// Transparent mode: bars and columns should NOT have background set.
		ConfigTransparentBg = true
		ApplyTheme(theme)
		_, isNoColor = TitleBarStyle.GetBackground().(lipgloss.NoColor)
		assert.True(t, isNoColor, "transparent TitleBarStyle should have no background")

		_, isNoColor = StatusBarBgStyle.GetBackground().(lipgloss.NoColor)
		assert.True(t, isNoColor, "transparent StatusBarBgStyle should have no background")

		_, isNoColor = ActiveColumnStyle.GetBackground().(lipgloss.NoColor)
		assert.True(t, isNoColor, "transparent ActiveColumnStyle should have no background")
	})

	t.Run("transparent mode keeps selection backgrounds", func(t *testing.T) {
		orig := ConfigTransparentBg
		defer func() {
			ConfigTransparentBg = orig
			ApplyTheme(DefaultTheme())
		}()

		ConfigTransparentBg = true
		ApplyTheme(DefaultTheme())

		// SelectedStyle should still have a background even in transparent mode.
		_, isNoColor := SelectedStyle.GetBackground().(lipgloss.NoColor)
		assert.False(t, isNoColor, "SelectedStyle should keep background in transparent mode")
	})
}
