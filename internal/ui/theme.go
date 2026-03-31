package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color palette for the application.
type Theme struct {
	Primary    string `json:"primary"`
	Secondary  string `json:"secondary"`
	Text       string `json:"text"`
	SelectedFg string `json:"selected_fg"`
	SelectedBg string `json:"selected_bg"`
	Border     string `json:"border"`
	Dimmed     string `json:"dimmed"`
	Error      string `json:"error"`
	Warning    string `json:"warning"`
	Purple     string `json:"purple"`
	Base       string `json:"base"`
	BarBg      string `json:"bar_bg"`
	Surface    string `json:"surface"`
}

// BaseBg, BarBg, and SurfaceBg are exported TerminalColor values that inline
// styles can use to set their background, respecting the transparency setting.
// When ConfigTransparentBg is true they are NoColor{}; otherwise they hold
// the theme's Base, BarBg, and Surface colors respectively.
var (
	BaseBg    lipgloss.TerminalColor = lipgloss.NoColor{}
	BarBg     lipgloss.TerminalColor = lipgloss.NoColor{}
	SurfaceBg lipgloss.TerminalColor = lipgloss.NoColor{}
)

// DefaultTheme returns the default Tokyonight Storm theme.
func DefaultTheme() Theme {
	return Theme{
		Primary:    "#7aa2f7",
		Secondary:  "#9ece6a",
		Text:       "#c0caf5",
		SelectedFg: "#24283b",
		SelectedBg: "#7aa2f7",
		Border:     "#4e5575",
		Dimmed:     "#4e5575",
		Error:      "#f7768e",
		Warning:    "#e0af68",
		Purple:     "#bb9af7",
		Base:       "#24283b",
		BarBg:      "#313446",
		Surface:    "#2a2e40",
	}
}

// ApplyTheme updates all style variables with the given theme colors.
func ApplyTheme(t Theme) {
	// baseBg is applied to all column/content text styles so the theme
	// background shows behind text (ANSI resets from inner styled content
	// would otherwise clear the container background). NoColor when transparent.
	var baseBg lipgloss.TerminalColor = lipgloss.NoColor{}
	if !ConfigTransparentBg {
		baseBg = lipgloss.Color(t.Base)
	}

	ActiveColumnStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Primary)).
		BorderBackground(baseBg).
		Background(baseBg)

	InactiveColumnStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Border)).
		BorderBackground(baseBg).
		Background(baseBg)

	SelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.SelectedBg))

	NormalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(baseBg)

	DimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(baseBg)

	CategoryStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Bold(true).
		Italic(true).
		Background(baseBg)

	IconStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Background(baseBg)

	StatusRunning = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Secondary)).Background(baseBg)
	StatusProgressing = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Primary)).Background(baseBg)
	StatusFailed = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Background(baseBg)
	StatusOther = lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dimmed)).Background(baseBg)

	var barBg lipgloss.TerminalColor = lipgloss.NoColor{}
	if !ConfigTransparentBg {
		barBg = baseBg
	}

	BarDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(barBg)

	BarNormalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(barBg)

	TitleBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(barBg).
		Padding(0, 1)

	TitleBreadcrumbStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Background(barBg)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Background(barBg).
		Padding(0, 1)

	NamespaceBadgeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.Primary)).
		Bold(true).
		Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Underline(true).
		Background(baseBg)

	HeaderIconStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Background(baseBg)

	NamespaceStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Warning)).
		Bold(true).
		Background(barBg).
		Padding(0, 1)

	YamlViewStyle = lipgloss.NewStyle().
		Background(baseBg).
		Padding(1, 2)

	YamlKeyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Bold(true).
		Background(baseBg)

	YamlValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(baseBg)

	YamlPunctuationStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(baseBg)

	YamlCommentStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Italic(true).
		Background(baseBg)

	YamlStringStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Background(baseBg)

	YamlNumberStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorOrange)).
		Background(baseBg)

	YamlBoolStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorOrange)).
		Bold(true).
		Background(baseBg)

	YamlNullStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Purple)).
		Italic(true).
		Background(baseBg)

	YamlAnchorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorCyan)).
		Bold(true).
		Background(baseBg)

	YamlTagStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Purple)).
		Background(baseBg)

	YamlBlockScalarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Purple)).
		Bold(true).
		Background(baseBg)

	StatusBarBgStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(barBg).
		Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(barBg).
		Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Bold(true).
		Background(barBg)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Error)).
		Background(baseBg).
		Bold(true)

	CurrentMarkerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Background(baseBg).
		Bold(true)

	var surfaceBg lipgloss.TerminalColor = lipgloss.NoColor{}
	if !ConfigTransparentBg {
		surfaceBg = lipgloss.Color(t.Surface)
	}

	// Export background colors so inline styles across the codebase can use them.
	BaseBg = baseBg
	BarBg = barBg
	SurfaceBg = surfaceBg

	OverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Primary)).
		BorderBackground(surfaceBg).
		Background(surfaceBg).
		Padding(1, 2)

	innerPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.Border)).
		BorderBackground(surfaceBg).
		Background(surfaceBg).
		Padding(0, 1)

	OverlayTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Primary)).
		Padding(0, 0, 1, 0).
		Background(surfaceBg)

	OverlaySelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.SelectedBg))

	OverlayNormalStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Background(surfaceBg)

	OverlayFilterStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Bold(true).
		Background(surfaceBg)

	OverlayDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Dimmed)).
		Background(surfaceBg)

	OverlayWarningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Error)).
		Bold(true).
		Background(surfaceBg)

	OverlayInputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Text)).
		Bold(true).
		Underline(true).
		Background(surfaceBg)

	ParentHighlightStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(t.Text)).
		Background(lipgloss.Color(t.Border))

	StatusMessageOkStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Background(barBg).
		Bold(true)

	StatusMessageErrStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Error)).
		Background(barBg).
		Bold(true)

	SearchHighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.Warning)).
		Foreground(lipgloss.Color(t.Base)).
		Bold(true)

	SelectedSearchHighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.Warning)).
		Foreground(lipgloss.Color(t.SelectedBg)).
		Bold(true).
		Underline(true)

	SelectionMarkerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Secondary)).
		Background(baseBg).
		Bold(true)

	SelectionCountStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.SelectedFg)).
		Background(lipgloss.Color(t.Secondary)).
		Bold(true)

	DeprecationStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Warning)).
		Background(baseBg)

	YamlCursorIndicatorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.Primary)).
		Background(baseBg)
}

// mergeThemeOverrides applies non-empty fields from overrides onto base.
func mergeThemeOverrides(base *Theme, overrides Theme) {
	if overrides.Primary != "" {
		base.Primary = overrides.Primary
	}
	if overrides.Secondary != "" {
		base.Secondary = overrides.Secondary
	}
	if overrides.Text != "" {
		base.Text = overrides.Text
	}
	if overrides.SelectedFg != "" {
		base.SelectedFg = overrides.SelectedFg
	}
	if overrides.SelectedBg != "" {
		base.SelectedBg = overrides.SelectedBg
	}
	if overrides.Border != "" {
		base.Border = overrides.Border
	}
	if overrides.Dimmed != "" {
		base.Dimmed = overrides.Dimmed
	}
	if overrides.Error != "" {
		base.Error = overrides.Error
	}
	if overrides.Warning != "" {
		base.Warning = overrides.Warning
	}
	if overrides.Purple != "" {
		base.Purple = overrides.Purple
	}
	if overrides.Base != "" {
		base.Base = overrides.Base
	}
	if overrides.BarBg != "" {
		base.BarBg = overrides.BarBg
	}
	if overrides.Surface != "" {
		base.Surface = overrides.Surface
	}
}
