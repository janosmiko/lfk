package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// innerPanelStyle is used for the content panel inside the help overlay.
var innerPanelStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// helpEntry holds a single keybinding entry.
type helpEntry struct {
	key  string
	desc string
}

// helpSection groups keybindings under a section header.
type helpSection struct {
	title    string
	bindings []helpEntry
}

// helpSections returns all help sections with their keybindings.
func helpSections() []helpSection {
	return []helpSection{
		{
			title: "Navigation",
			bindings: []helpEntry{
				{"h / Left", "Go to parent"},
				{"l / Right", "Go to child"},
				{"j / Down", "Move down"},
				{"k / Up", "Move up"},
				{"gg", "Jump to top"},
				{"G", "Jump to bottom"},
				{"Enter", "Open YAML view / navigate into"},
				{"z", "Toggle expand/collapse all resource groups"},
				{"p", "Pin/unpin CRD group (at resource types level)"},
			},
		},
		{
			title: "Views",
			bindings: []helpEntry{
				{"?", "Toggle help screen"},
				{"f", "Filter items in current view"},
				{"/", "Search and jump to match"},
				{"n", "Next search match"},
				{"N", "Previous search match"},
				{"P", "Toggle between details and YAML preview"},
				{"", "Details pane shows labels, finalizers, annotations, and metadata"},
				{"", "Details view shows deletion timestamp for resources being deleted"},
				{"M", "Toggle resource relationship map"},
				{"F", "Toggle fullscreen (middle column or dashboard)"},
				{".", "Quick filter presets"},
				{"!", "Error log"},
				{"0/1/2", "Jump to clusters/types/resources level"},
				{"Ctrl+S", "Toggle secret value visibility"},
				{"I", "API Explorer (resource structure)"},
				{"U", "RBAC permissions browser (can-i)"},
			},
		},
		{
			title: "Multi-Selection",
			bindings: []helpEntry{
				{"Space", "Toggle selection on current item"},
				{"Ctrl+Space", "Select range from anchor to cursor"},
				{"Ctrl+A", "Select/deselect all visible items"},
				{"Esc", "Clear selection"},
				{"x", "Bulk action menu (delete, force delete, scale, restart, diff)"},
				{"d", "Diff: compare YAML of two selected resources"},
			},
		},
		{
			title: "Actions",
			bindings: []helpEntry{
				{"\\", "Select namespace"},
				{"A", "Toggle all-namespaces mode"},
				{"x", "Action menu (logs, exec, debug, debug pod, describe, edit, delete, scale, port-forward, events, startup analysis, RBAC permissions)"},
				{"L", "View logs for selected resource"},
				{"e", "Secret/ConfigMap editor (inline key-value editing)"},
				{"E", "Edit selected resource in $EDITOR"},
				{"i", "Edit labels/annotations for selected resource"},
				{"R", "Refresh current view"},
				{"v", "Describe selected resource"},
				{"D", "Delete resource (Force Finalize if already deleting)"},
				{"X", "Force delete (grace-period=0, Pod/Job only)"},
				{"o", "Jump to owner/controller of selected resource"},
				{"y", "Copy resource name to clipboard"},
				{"Ctrl+Y", "Copy resource YAML to clipboard"},
				{"Ctrl+P", "Apply resource from clipboard (kubectl apply)"},
				{",", "Cycle sort mode (name / age / status)"},
				{"w", "Toggle watch mode (auto-refresh)"},
				{"@", "Monitoring overview (active Prometheus alerts)"},
				{"Q", "Namespace resource quota dashboard"},
				{"J/K", "Scroll preview pane down/up"},
				{"W", "Save resource to file / toggle warnings-only (Events)"},
				{"", "Port forwarding: use action menu (x) on Pod/Service/Deployment/StatefulSet/DaemonSet"},
				{"", "Auto-navigates to Port Forwards list after creation; shows resolved local port"},
			},
		},
		{
			title: "Bookmarks",
			bindings: []helpEntry{
				{"m<key>", "Set mark at current location (a-z, 0-9)"},
				{"'", "Open bookmarks list"},
				{"a-z/0-9", "Jump to named mark (in overlay)"},
				{"j/k", "Navigate bookmarks (in overlay)"},
				{"/", "Filter bookmarks by name (in overlay)"},
				{"Enter", "Jump to selected bookmark (in overlay)"},
				{"D", "Delete selected bookmark (in overlay)"},
				{"Ctrl+X", "Delete all bookmarks (in overlay)"},
			},
		},
		{
			title: "YAML View",
			bindings: []helpEntry{
				{"j/k", "Scroll up/down"},
				{"h/l", "Move cursor column left/right"},
				{"0/$", "Move cursor to line start/end"},
				{"w/b", "Move cursor to next/previous word"},
				{"g/G", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B", "Page down/up (full page)"},
				{"/", "Search in YAML"},
				{"n", "Next search match"},
				{"N", "Previous search match"},
				{"v", "Character visual selection (from cursor column)"},
				{"V", "Visual line selection"},
				{"Ctrl+V", "Block (column) visual selection (from cursor column)"},
				{"h/l", "Move selection column (in visual mode)"},
				{"y", "Copy selected text (in visual mode)"},
				{"Tab/z", "Toggle fold on section under cursor"},
				{"Z", "Toggle all folds (collapse/expand)"},
				{"e", "Edit resource in editor"},
				{"q/Esc", "Back to explorer"},
			},
		},
		{
			title: "Diff View",
			bindings: []helpEntry{
				{"j/k", "Scroll up/down"},
				{"g/G", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B", "Page down/up (full page)"},
				{"#", "Toggle line numbers"},
				{"123G", "Jump to line number"},
				{"u", "Toggle unified/side-by-side view"},
				{"q/Esc", "Back to explorer"},
			},
		},
		{
			title: "API Explorer",
			bindings: []helpEntry{
				{"j/k", "Navigate fields"},
				{"l/Enter", "Drill into field (Object/array types)"},
				{"h/Backspace", "Go back one level"},
				{"/", "Search fields"},
				{"n", "Next match (auto-drills into children)"},
				{"N", "Previous match (searches parent)"},
				{"r", "Recursive field browser (browse all nested fields with filter)"},
				{"g/G", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B", "Page down/up (full page)"},
				{"q", "Close API explorer"},
				{"Esc", "Go back one level / close at root"},
			},
		},
		{
			title: "Can-I Browser",
			bindings: []helpEntry{
				{"j/k", "Navigate groups"},
				{"J/K", "Scroll resource list down/up"},
				{"/", "Search/filter groups by name"},
				{"s", "Switch user/ServiceAccount"},
				{"g/G", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B", "Page down/up (full page)"},
				{"q/Esc", "Clear search / close"},
			},
		},
		{
			title: "Can-I Subject Selector",
			bindings: []helpEntry{
				{"j/k", "Navigate subjects"},
				{"/", "Filter subjects by name"},
				{"g/G", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Enter", "Select subject"},
				{"Esc", "Clear filter / close"},
			},
		},
		{
			title: "Network Policy Visualizer",
			bindings: []helpEntry{
				{"j/k", "Scroll up/down"},
				{"g/G", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B", "Page down/up (full page)"},
				{"q/Esc", "Close visualizer"},
			},
		},
		{
			title: "Log Viewer",
			bindings: []helpEntry{
				{"j/k", "Move cursor up/down"},
				{"h/l/left/right", "Move cursor column left/right"},
				{"$", "Move cursor to line end"},
				{"e/b", "Move cursor to word end/previous word"},
				{"g/G", "Top/bottom"},
				{"Ctrl+D/U", "Half page down/up"},
				{"Ctrl+F/B", "Full page down/up"},
				{"f", "Toggle follow mode (auto-scroll)"},
				{"w", "Toggle line wrapping"},
				{"#", "Toggle line numbers"},
				{"s", "Toggle timestamps"},
				{"c", "Toggle previous container logs"},
				{"/", "Search in logs"},
				{"n", "Next search match"},
				{"N/p", "Previous search match"},
				{"123G", "Jump to line number"},
				{"W", "Save loaded logs to file"},
				{"Ctrl+S", "Save all logs to file"},
				{"v", "Character visual selection (from cursor column)"},
				{"V", "Visual line selection"},
				{"Ctrl+V", "Block (column) visual selection (from cursor column)"},
				{"h/l", "Move selection column (in visual mode)"},
				{"y", "Copy selected text (in visual mode)"},
				{"\\", "Switch pod / filter containers"},
				{"", "Loads last 1000 lines initially; scroll up to load older logs"},
				{"q/Esc", "Close log viewer"},
			},
		},
		{
			title: "Exec Mode (embedded terminal)",
			bindings: []helpEntry{
				{"Ctrl+]", "Prefix key (like tmux Ctrl+b)"},
				{"Ctrl+] → Ctrl+]", "Exit terminal and return to explorer"},
				{"Ctrl+] → ]", "Next tab (PTY keeps running in background)"},
				{"Ctrl+] → [", "Previous tab (PTY keeps running in background)"},
				{"Ctrl+] → t", "New tab (clone current context)"},
				{"", "All other keys are forwarded to the PTY"},
			},
		},
		{
			title: "Tabs",
			bindings: []helpEntry{
				{"t", "New tab (clone current)"},
				{"[", "Previous tab"},
				{"]", "Next tab"},
			},
		},
		{
			title: "Mouse",
			bindings: []helpEntry{
				{"Click", "Select item / navigate"},
				{"Scroll", "Navigate up/down"},
				{"Shift+Drag", "Select text (terminal native)"},
			},
		},
		{
			title: "General",
			bindings: []helpEntry{
				{"T", "Switch color scheme"},
				{"q", "Quit (with confirmation)"},
				{"Esc", "Go back / quit"},
				{"Ctrl+C", "Close tab (quit if last)"},
			},
		},
		{
			title: "Configuration",
			bindings: []helpEntry{
				{"", "Config: ~/.config/lfk/config.yaml (or $XDG_CONFIG_HOME/lfk/config.yaml)"},
				{"", "State:  ~/.local/state/lfk/ (bookmarks, session, history, pinned groups)"},
				{"", "Logs:   ~/.local/share/lfk/lfk.log"},
			},
		},
	}
}

// buildHelpLines builds the formatted help lines, optionally filtering by a query string.
func buildHelpLines(filter string) []string {
	sections := helpSections()
	lowerFilter := strings.ToLower(filter)

	lines := make([]string, 0, 64)
	keyW := 14
	for si, section := range sections {
		var sectionLines []string
		for _, b := range section.bindings {
			if filter != "" {
				lowerKey := strings.ToLower(b.key)
				lowerDesc := strings.ToLower(b.desc)
				if !strings.Contains(lowerKey, lowerFilter) && !strings.Contains(lowerDesc, lowerFilter) {
					continue
				}
			}
			keyPart := HelpKeyStyle.Render(fmt.Sprintf("%-*s", keyW, b.key))
			descPart := DimStyle.Render(b.desc)
			sectionLines = append(sectionLines, "    "+keyPart+"  "+descPart)
		}

		// Only include sections that have matching bindings.
		if len(sectionLines) == 0 {
			continue
		}

		if len(lines) > 0 || si > 0 {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
		}
		header := HeaderStyle.Render(section.title)
		lines = append(lines, "  "+header)
		lines = append(lines, sectionLines...)
	}

	if filter != "" && len(lines) == 0 {
		lines = append(lines, DimStyle.Render("  No matching keybindings"))
	}

	return lines
}

// HelpContentLineCount returns the total number of content lines for the help screen
// with the given filter. Used by the app model to calculate max scroll.
func HelpContentLineCount(filter string) int {
	return len(buildHelpLines(filter))
}

// RenderHelpScreen renders a full help overlay with all keybindings.
// It supports scrolling via the scroll parameter and filtering via the filter parameter.
func RenderHelpScreen(screenWidth, screenHeight, scroll int, filter string, searching bool, searchInput *textinput.Model) string {
	boxW := screenWidth * 70 / 100
	boxH := screenHeight * 80 / 100
	if boxW < 50 {
		boxW = 50
	}
	if boxH < 20 {
		boxH = 20
	}

	contentW := boxW - 6 // account for border + padding

	title := TitleStyle.Render("Keybindings")

	lines := buildHelpLines(filter)
	totalLines := len(lines)

	// Calculate visible area: title, borders, padding, help line.
	maxLines := max(boxH-6, 5)

	// Clamp scroll.
	maxScroll := max(totalLines-maxLines, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	// Determine scroll indicators.
	hasAbove := scroll > 0
	hasBelow := scroll+maxLines < totalLines

	// Reserve lines for scroll indicators if needed.
	visibleLines := maxLines
	if hasAbove {
		visibleLines--
	}
	if hasBelow {
		visibleLines--
	}

	// Slice visible portion.
	end := min(scroll+visibleLines, totalLines)
	visible := lines[scroll:end]

	// Build final lines with indicators.
	var displayLines []string
	if hasAbove {
		displayLines = append(displayLines, DimStyle.Render("  \u2191 more above"))
	}
	displayLines = append(displayLines, visible...)
	if hasBelow {
		displayLines = append(displayLines, DimStyle.Render("  \u2193 more below"))
	}

	content := strings.Join(displayLines, "\n")
	innerPanel := innerPanelStyle.
		Width(contentW).
		Render(content)

	// Build help/status line.
	var helpLine string
	switch {
	case searching:
		helpLine = DimStyle.Render("search: ") + searchInput.View()
	case filter != "":
		helpLine = DimStyle.Render("filter: ") +
			HelpKeyStyle.Render(filter) +
			DimStyle.Render("  ") +
			HelpKeyStyle.Render("/") + DimStyle.Render(" edit  ") +
			HelpKeyStyle.Render("Esc") + DimStyle.Render(" close")
	default:
		helpLine = HelpKeyStyle.Render("j/k") + DimStyle.Render(" scroll  ") +
			HelpKeyStyle.Render("^d/^u") + DimStyle.Render(" half-page  ") +
			HelpKeyStyle.Render("/") + DimStyle.Render(" search  ") +
			HelpKeyStyle.Render("Esc") + DimStyle.Render(" / ") +
			HelpKeyStyle.Render("?") + DimStyle.Render(" / ") +
			HelpKeyStyle.Render("q") + DimStyle.Render(" close")
	}

	body := title + "\n" + innerPanel + "\n" + helpLine

	return OverlayStyle.
		Width(boxW).
		Render(body)
}
