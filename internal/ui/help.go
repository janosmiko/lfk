package ui

import (
	"fmt"
	"strings"

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
// context identifies which view this section belongs to.
// Empty context means the explorer (main) view.
type helpSection struct {
	title    string
	context  string // e.g. "YAML View", "Log Viewer", "" for explorer
	bindings []helpEntry
}

// helpKeyDisplay formats a keybinding value for display in the help screen.
// It capitalizes "ctrl+" prefixes for readability.
func helpKeyDisplay(key string) string {
	if strings.HasPrefix(key, "ctrl+") {
		return "Ctrl+" + strings.ToUpper(key[5:])
	}
	return key
}

// helpSections returns all help sections with their keybindings.
func helpSections() []helpSection {
	kb := ActiveKeybindings
	return []helpSection{
		{
			title: "Navigation",
			bindings: []helpEntry{
				{kb.Left + " / Left", "Go to parent"},
				{kb.Right + " / Right", "Go to child"},
				{kb.Down + " / Down", "Move down"},
				{kb.Up + " / Up", "Move up"},
				{kb.JumpTop + kb.JumpTop + " / Home", "Jump to top"},
				{kb.JumpBottom + " / End", "Jump to bottom"},
				{helpKeyDisplay(kb.PageDown) + " / " + helpKeyDisplay(kb.PageUp), "Half-page scroll down / up"},
				{helpKeyDisplay(kb.PageForward) + " / " + helpKeyDisplay(kb.PageBack) + " / PgDn / PgUp", "Full-page scroll down / up"},
				{kb.Enter, "Open YAML view / navigate into"},
				{kb.ExpandCollapse, "Toggle expand/collapse all resource groups / toggle event grouping (Events)"},
				{kb.PinGroup, "Pin/unpin CRD group (at resource types level)"},
				{kb.ToggleRare, "Toggle rarely used resource types (CSI, webhooks, advanced) in the sidebar"},
			},
		},
		{
			title: "Views",
			bindings: []helpEntry{
				{kb.Help + " / F1", "Toggle help screen"},
				{kb.Filter, "Filter items (~prefix: fuzzy, regex auto-detected, \\prefix: literal)"},
				{kb.Search, "Search and jump to match (~fuzzy, regex auto, \\literal)"},
				{"", "Up/Down inside filter or search recalls previous queries (shared, persistent across sessions)."},
				{"", "Paste from clipboard: Cmd+V (macOS) / Ctrl+Shift+V (Linux). Multiline asks to confirm."},
				{kb.NextMatch, "Next search match"},
				{kb.PrevMatch, "Previous search match"},
				{kb.TogglePreview, "Toggle between details and YAML preview"},
				{"", "Details pane shows labels, finalizers, annotations, and metadata"},
				{"", "Details view shows deletion timestamp for resources being deleted"},
				{kb.ResourceMap, "Toggle resource relationship map"},
				{kb.Fullscreen, "Toggle fullscreen (middle column or dashboard)"},
				{kb.FilterPresets, "Quick filter presets"},
				{kb.ErrorLog, "Error log"},
				{kb.LevelCluster + "/" + kb.LevelTypes + "/" + kb.LevelResources, "Jump to clusters/types/resources level"},
				{helpKeyDisplay(kb.SecretToggle), "Toggle secret value visibility (details pane only)"},
				{kb.APIExplorer, "API Explorer (resource structure)"},
				{kb.RBACBrowser, "RBAC permissions browser (can-i)"},
				{kb.ColumnToggle, "Column visibility toggle (show/hide and reorder columns)"},
			},
		},
		{
			title: "Multi-Selection",
			bindings: []helpEntry{
				{"Space", "Toggle selection on current item"},
				{"Ctrl+Space", "Select range from anchor to cursor"},
				{helpKeyDisplay(kb.SelectAll), "Select/deselect all visible items"},
				{"Esc", "Clear selection"},
				{kb.ActionMenu, "Bulk action menu (delete, force delete, scale, restart, diff, ArgoCD sync/refresh)"},
				{kb.Diff, "Diff: compare YAML of two selected resources"},
			},
		},
		{
			title: "Actions",
			bindings: []helpEntry{
				{kb.NamespaceSelector, "Select namespace"},
				{kb.AllNamespaces, "Toggle all-namespaces mode"},
				{kb.ActionMenu, "Action menu: l=tail logs (last 10 lines + follow), L=full logs, exec, debug, debug pod, describe, edit, delete, scale, port-forward, events, startup analysis, RBAC permissions"},
				{kb.Logs, "View full logs for selected resource"},
				{kb.SecretEditor, "Secret/ConfigMap editor (inline key-value editing)"},
				{kb.Edit, "Edit selected resource in $EDITOR"},
				{kb.LabelEditor, "Edit labels/annotations for selected resource"},
				{kb.Refresh, "Refresh current view"},
				{kb.Describe, "Describe selected resource"},
				{kb.Delete, "Delete (force delete Pod/Job if already deleting, force finalize others)"},
				{kb.ForceDelete, "Force delete (Pod/Job only)"},
				{helpKeyDisplay(kb.FinalizerSearch), "Finalizer search and remove (scan, select, remove finalizers)"},
				{kb.JumpOwner, "Jump to owner/controller of selected resource"},
				{kb.CopyName, "Copy resource name to clipboard"},
				{helpKeyDisplay(kb.CopyYAML), "Copy resource YAML to clipboard"},
				{helpKeyDisplay(kb.PasteApply), "Apply resource from clipboard (kubectl apply)"},
				{kb.SortNext, "Sort by next column"},
				{kb.SortPrev, "Sort by previous column"},
				{kb.SortFlip, "Toggle sort direction (ascending/descending)"},
				{kb.SortReset, "Reset sort to default (Name ascending)"},
				{kb.WatchMode, "Toggle watch mode (auto-refresh)"},
				{helpKeyDisplay(kb.ReadOnlyToggle), "Toggle read-only mode (cluster picker: highlighted row's [RO] marker; inside a context: current tab)"},
				{kb.Monitoring, "Monitoring overview (active Prometheus alerts)"},
				{kb.QuotaDashboard, "Namespace resource quota dashboard"},
				{kb.TasksOverlay, "Background tasks overlay (Tab toggles running/completed history)"},
				{kb.PreviewDown + "/" + kb.PreviewUp, "Scroll preview pane down/up"},
				{kb.SaveResource, "Save resource to file / toggle warnings-only (Events)"},
				{helpKeyDisplay(kb.TerminalToggle), "Cycle terminal mode (pty / exec / mux — mux skipped if no tmux/zellij detected)"},
				{"", "Port forwarding: use action menu (" + kb.ActionMenu + ") on Pod/Service/Deployment/StatefulSet/DaemonSet"},
				{"", "Auto-navigates to Port Forwards list after creation; shows resolved local port"},
			},
		},
		{
			title: "Command Bar (:)",
			bindings: []helpEntry{
				{kb.CommandBar, "Open command bar"},
				{"", "Resource jump: :pod, :dep, :svc (navigate to resource type)"},
				{"", "  With namespace: :pod kube-system (jump + filter namespace)"},
				{"", "Built-in: :ns (navigate to NS or filter), :ctx <name>, :set <opt>, :sort <col>, :export <fmt>"},
				{"", "Kubectl: :k get pod, :kubectl describe pod (requires k/kubectl prefix)"},
				{"", "Shell: :! <command> (run arbitrary shell command)"},
				{"", ""},
				{"Tab", "Cycle suggestions forward (auto-fill when 1 match)"},
				{"Shift+Tab", "Cycle suggestions backward"},
				{"Ctrl+N / Down", "Cycle suggestions forward"},
				{"Ctrl+P / Up", "Cycle suggestions backward"},
				{"Ctrl+D", "Scroll suggestions down (half page)"},
				{"Ctrl+U", "Scroll suggestions up (half page) / delete line when closed"},
				{"Ctrl+F / Ctrl+B", "Scroll suggestions (full page)"},
				{"Ctrl+Space", "Open/refresh suggestions"},
				{"Space / Right", "Accept ghost text preview"},
				{"Enter", "Accept suggestion (if visible) or execute command"},
				{"Esc", "Close suggestions, or close command bar"},
				{"Up / Down", "Browse command history (when no suggestions)"},
				{"Ctrl+W", "Delete word backwards"},
				{"Ctrl+A / Ctrl+E", "Home / End"},
			},
		},
		{
			title: "Bookmarks",
			bindings: []helpEntry{
				{kb.SetMark + "<a-z/0-9>", "Set context-aware mark (switches cluster on jump)"},
				{kb.SetMark + "<A-Z>", "Set context-free mark (stays in current cluster on jump)"},
				{kb.OpenMarks, "Open bookmarks list"},
				{"a-z/A-Z/0-9", "Jump to named mark (in overlay)"},
				{"j/k", "Navigate bookmarks (in overlay)"},
				{"/", "Filter bookmarks by name (in overlay)"},
				{"Enter", "Jump to selected bookmark (in overlay)"},
				{"Ctrl+X", "Delete selected bookmark with confirmation (in overlay)"},
				{"Alt+X", "Delete all bookmarks with confirmation (in overlay)"},
			},
		},
		{
			title: "Error Log (" + kb.ErrorLog + ")", context: "Error Log",
			bindings: []helpEntry{
				{"j/k", "Scroll up/down"},
				{"gg/G / Home/End", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"V", "Enter line visual selection mode"},
				{"v", "Enter character visual selection mode"},
				{"y", "Copy selected lines (visual) or all entries (normal)"},
				{"f", "Toggle fullscreen / overlay mode"},
				{"d", "Toggle debug log visibility"},
				{"Esc", "Cancel visual selection, or close overlay"},
				{"q", "Close overlay"},
			},
		},
		{
			title: "YAML View", context: "YAML View",
			bindings: []helpEntry{
				{"j/k", "Scroll up/down"},
				{"123j/123k", "Move cursor down/up N visible lines (count-prefixed; folds skipped)"},
				{"h/l", "Move cursor column left/right"},
				{"0/$", "Move cursor to line start/end"},
				{"^", "Move cursor to first non-whitespace"},
				{"w/b", "Move cursor to next/previous word start"},
				{"W/B", "Move cursor to next/previous WORD start"},
				{"e", "Move cursor to end of word"},
				{"E", "Move cursor to end of WORD"},
				{"gg/G / Home/End", "Top/bottom"},
				{"123G", "Jump to line number"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"/", "Search in YAML"},
				{"n", "Next search match"},
				{"N", "Previous search match"},
				{"v", "Character visual selection (from cursor column)"},
				{"V", "Visual line selection"},
				{"Ctrl+V", "Block (column) visual selection (from cursor column)"},
				{"h/l", "Move selection column (in visual mode)"},
				{"y", "Copy line (or selection in visual mode)"},
				{"123y", "Copy number of lines from cursor (count-prefixed yank, folds skipped)"},
				{"z", "Toggle fold on section under cursor"},
				{"Z", "Toggle all folds (collapse/expand)"},
				{"Ctrl+W / >", "Toggle line wrapping"},
				{"Ctrl+E", "Edit resource in editor"},
				{"q/Esc", "Back to explorer"},
			},
		},
		{
			title: "Describe View", context: "Describe View",
			bindings: []helpEntry{
				{"j/k", "Move cursor up/down"},
				{"123j/123k", "Move cursor down/up N lines (count-prefixed)"},
				{"h/l", "Move cursor column left/right"},
				{"0/$", "Move cursor to line start/end"},
				{"^", "Move cursor to first non-whitespace"},
				{"w/b", "Move cursor to next/previous word start"},
				{"W/B", "Move cursor to next/previous WORD start"},
				{"e", "Move cursor to end of word"},
				{"E", "Move cursor to end of WORD"},
				{"gg/G / Home/End", "Top/bottom"},
				{"123G", "Jump to line number"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"/", "Search in content"},
				{"n/N", "Next/previous match"},
				{"v", "Character visual selection"},
				{"V", "Visual line selection"},
				{"Ctrl+V", "Block (column) visual selection"},
				{"y", "Copy line (or selection in visual mode)"},
				{"123y", "Copy number of lines from cursor (count-prefixed yank)"},
				{"Ctrl+W / >", "Toggle line wrapping"},
				{"q/Esc", "Back to explorer"},
			},
		},
		{
			title: "Diff View", context: "Diff View",
			bindings: []helpEntry{
				{"j/k", "Move cursor up/down"},
				{"123j/123k", "Move cursor down/up N lines (count-prefixed)"},
				{"h/l", "Move cursor column left/right"},
				{"0/$", "Move cursor to line start/end"},
				{"^", "Move cursor to first non-whitespace"},
				{"w/b", "Move cursor to next/previous word start"},
				{"W/B", "Move cursor to next/previous WORD start"},
				{"e", "Move cursor to end of word"},
				{"E", "Move cursor to end of WORD"},
				{"Tab", "Switch cursor side (side-by-side mode)"},
				{"gg/G / Home/End", "Top/bottom"},
				{"123G", "Jump to line number"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"/", "Search in diff"},
				{"n/N", "Next/previous match"},
				{"v", "Character visual selection"},
				{"V", "Visual line selection"},
				{"Ctrl+V", "Block (column) visual selection"},
				{"h/l", "Move selection column (in visual mode)"},
				{"y", "Copy line (or selection in visual mode)"},
				{"123y", "Copy number of lines from cursor (count-prefixed yank, empty rows skipped)"},
				{"z", "Toggle fold unchanged section at cursor"},
				{"Z", "Toggle all folds"},
				{"#", "Toggle line numbers"},
				{"Ctrl+W / >", "Toggle line wrapping"},
				{"u", "Toggle unified/side-by-side view"},
				{"q/Esc", "Back to explorer"},
			},
		},
		{
			title: "API Explorer", context: "API Explorer",
			bindings: []helpEntry{
				{"j/k", "Navigate fields"},
				{"l/Enter", "Drill into field (Object/array types)"},
				{"h/Backspace", "Go back one level"},
				{"/", "Search fields"},
				{"n", "Next match (auto-drills into children)"},
				{"N", "Previous match (searches parent)"},
				{"r", "Recursive field browser (browse all nested fields with filter)"},
				{"gg/G / Home/End", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"q", "Close API explorer"},
				{"Esc", "Go back one level / close at root"},
			},
		},
		{
			title: "Can-I Browser", context: "Can-I Browser",
			bindings: []helpEntry{
				{"j/k", "Navigate groups"},
				{"J/K", "Scroll resource list down/up"},
				{"/", "Search/filter groups by name"},
				{"a", "Toggle all/allowed-only permissions"},
				{"s", "Switch subject (User/Group/SA)"},
				{"", "Title shows namespace scope (ns:...) for permission context"},
				{"gg/G / Home/End", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"q/Esc", "Clear search / close"},
			},
		},
		{
			title: "Can-I Subject Selector", context: "Can-I Browser",
			bindings: []helpEntry{
				{"j/k", "Navigate subjects"},
				{"/", "Filter subjects by name"},
				{"gg/G / Home/End", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"Enter", "Select subject"},
				{"Esc", "Clear filter / close"},
			},
		},
		{
			title: "Network Policy Visualizer", context: "Network Policy",
			bindings: []helpEntry{
				{"j/k", "Scroll up/down"},
				{"gg/G / Home/End", "Top/bottom"},
				{"Ctrl+D/U", "Page down/up (half page)"},
				{"Ctrl+F/B / PgDn/PgUp", "Page down/up (full page)"},
				{"q/Esc", "Close visualizer"},
			},
		},
		{
			title: "Log Viewer", context: "Log Viewer",
			bindings: []helpEntry{
				{"j/k", "Move cursor up/down"},
				{"123j/123k", "Move cursor down/up N lines (count-prefixed)"},
				{"h/l/left/right", "Move cursor column left/right"},
				{"0/$", "Move cursor to line start/end"},
				{"^", "Move cursor to first non-whitespace"},
				{"w/b", "Move cursor to next/previous word start"},
				{"W/B", "Move cursor to next/previous WORD start"},
				{"e", "Move cursor to end of word"},
				{"E", "Move cursor to end of WORD"},
				{"gg/G / Home/End", "Top/bottom"},
				{"Ctrl+D/U", "Half page down/up"},
				{"Ctrl+F/B / PgDn/PgUp", "Full page down/up"},
				{"f", "Toggle follow mode (auto-scroll)"},
				{"Tab/z/>", "Toggle line wrapping"},
				{"#", "Toggle line numbers"},
				{"s", "Toggle timestamps"},
				{"p", "Toggle pod/container prefixes"},
				{"P", "Toggle structured preview side panel (JSON / logfmt / klog / zap / nginx / envoy / java / postgres / text)"},
				{"J/K", "Scroll preview side panel down/up (when visible)"},
				{"c", "Toggle previous container logs"},
				{"/", "Search in logs"},
				{"n", "Next search match"},
				{"N", "Previous search match"},
				{"123G", "Jump to line number"},
				{"S", "Save loaded logs to file (path copied to clipboard)"},
				{"Ctrl+S", "Save all logs to file (path copied to clipboard)"},
				{"v", "Character visual selection (from cursor column)"},
				{"V", "Visual line selection"},
				{"Ctrl+V", "Block (column) visual selection (from cursor column)"},
				{"h/l", "Move selection column (in visual mode)"},
				{"y", "Copy line (or selection in visual mode)"},
				{"123y", "Copy number of lines from cursor (count-prefixed yank)"},
				{"\\", "Switch pod / filter containers"},
				{"", "Full logs load last 1000 lines initially (log_tail_lines); tail logs load last 10 (log_tail_lines_short). Scroll up for older history."},
				{"q/Esc", "Close log viewer"},
			},
		},
		{
			title: "Exec Mode (embedded terminal)", context: "Exec Mode",
			bindings: []helpEntry{
				{"Ctrl+]", "Prefix key (like tmux Ctrl+b)"},
				{"Ctrl+] -> Ctrl+]", "Exit terminal and return to explorer"},
				{"Ctrl+] -> " + kb.NextTab, "Next tab (PTY keeps running in background)"},
				{"Ctrl+] -> " + kb.PrevTab, "Previous tab (PTY keeps running in background)"},
				{"Ctrl+] -> " + kb.NewTab, "New tab (clone current context)"},
				{"Ctrl+] -> Ctrl+U / Ctrl+D", "Scroll back / forward by half a viewport"},
				{"Ctrl+] -> Ctrl+B / Ctrl+F", "Scroll back / forward by a full viewport"},
				{"Ctrl+] -> g / G", "Jump to oldest captured line / back to live"},
				{"Mouse Scroll", "Scroll the PTY scrollback (1 line per tick)"},
				{"Shift+Drag", "Select text (host terminal); Shift+Option+Drag for block select on macOS"},
				{"", "Typing any key forwards to the PTY and snaps back to the live shell"},
				{"", "Scrollback is captured from the PTY byte stream (newline-split, ANSI-stripped, ~5000 lines). Curses programs (vim, less, htop) that paint absolute screen positions will produce noisy scrollback while running."},
				{"", "Cycle terminal mode with Ctrl+T: pty (embedded) -> exec (host terminal) -> mux (new tmux/zellij window or pane). Mux is skipped when no multiplexer is detected. Set terminal: pty|exec|mux in config to pick the default."},
			},
		},
		{
			title: "Tabs",
			bindings: []helpEntry{
				{kb.NewTab, "New tab (clone current)"},
				{kb.PrevTab, "Previous tab"},
				{kb.NextTab, "Next tab"},
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
			title: "Help View",
			bindings: []helpEntry{
				{"/", "Search — highlights matches inline without filtering"},
				{"Ctrl+N / Ctrl+P", "Next / previous match while typing the search"},
				{"Enter", "Apply search (keep highlights and arm n/N)"},
				{"n / N", "Jump to next / previous search match (after Enter)"},
				{"f", "Filter — narrows the visible list to matching lines"},
				{"Esc", "Cascades: clear search → clear filter → close help"},
			},
		},
		{
			title: "General",
			bindings: []helpEntry{
				{kb.ThemeSelector, "Switch color scheme (" + kb.NewTab + ": toggle transparent bg)"},
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

// BuildHelpLines builds the formatted help lines, optionally filtering
// by a query string. contextMode limits sections to those matching the
// current view (empty = explorer). Exported so the app layer can run
// the same line-building pipeline to compute search match indices for
// n/N navigation.
//
// Returns plain (un-styled) text in the same row order RenderHelpScreen
// will display. Plain text is what app-layer search routines need:
// running MatchLine / strings.Contains over a styled line lets a
// digit query match bytes that live inside an SGR escape (e.g. the
// "1" in "\x1b[33;1m"), inflating match counts and pointing n/N at
// rows with no visible match.
func BuildHelpLines(filter, contextMode string) []string {
	specs := buildHelpSpecs(filter, contextMode)
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = helpSpecPlain(s)
	}
	return out
}

// HelpVisibleLines returns the number of help-content rows that fit
// inside the overlay box for a given screen height. Mirrors the same
// boxH / maxLines / visibleLines arithmetic RenderHelpScreen uses, so
// callers (clamp helpers, scroll-to-match positioning) compute the
// same maxScroll the renderer enforces.
func HelpVisibleLines(screenHeight int) int {
	boxH := max(screenHeight*80/100, 20)
	maxLines := max(boxH-6, 5)
	visibleLines := max(maxLines-2, 1)
	return visibleLines
}

// helpLineKind labels each logical row in the help screen so the
// renderer can pick the correct outer style for that row's plain text.
type helpLineKind int

const (
	helpLineBlank helpLineKind = iota
	helpLineSectionHeader
	helpLineEntry
	helpLineMessage
)

// helpLineSpec is the structural form of a help row, kept un-styled
// so the renderer can splice the search highlight into plain text
// before applying the outer styles. The pre-style highlight path
// avoids the bug where a "/" search query containing digits matched
// bytes inside an SGR escape sequence on the already-styled line —
// terminals rendered the leftover sequence fragments as literal
// "[33;" / ";1m" text on screen.
type helpLineSpec struct {
	kind helpLineKind
	// text is the plain content for header and message rows.
	text string
	// key is the padded plain key column for entry rows.
	key string
	// desc is the plain description column for entry rows.
	desc string
}

// helpKeyColumnWidth is the fixed-width key column so descriptions
// align vertically across every entry row.
const helpKeyColumnWidth = 14

// buildHelpSpecs walks the help sections and produces structural
// specs (un-styled) in the exact display order. Used by both
// BuildHelpLines and RenderHelpScreen so the plain match indices
// computed by the app layer line up 1:1 with the styled rows on
// screen.
func buildHelpSpecs(filter, contextMode string) []helpLineSpec {
	sections := helpSections()
	specs := make([]helpLineSpec, 0, 64)
	for _, section := range sections {
		// Context filtering: when a context is active, show only sections
		// that match that context. When no context (explorer), show only
		// sections with empty context (explorer sections).
		if contextMode == "" || contextMode == "Navigation" || contextMode == "Bookmarks" {
			if section.context != "" {
				continue
			}
		} else {
			if section.context != contextMode {
				continue
			}
		}

		var entries []helpLineSpec
		for _, b := range section.bindings {
			if filter != "" {
				if !MatchLine(b.key, filter) && !MatchLine(b.desc, filter) {
					continue
				}
			}
			entries = append(entries, helpLineSpec{
				kind: helpLineEntry,
				key:  fmt.Sprintf("%-*s", helpKeyColumnWidth, b.key),
				desc: b.desc,
			})
		}

		// Only include sections that have matching bindings.
		if len(entries) == 0 {
			continue
		}

		if len(specs) > 0 {
			specs = append(specs, helpLineSpec{kind: helpLineBlank})
		}
		specs = append(specs, helpLineSpec{kind: helpLineSectionHeader, text: section.title})
		specs = append(specs, entries...)
	}

	if filter != "" && len(specs) == 0 {
		specs = append(specs, helpLineSpec{kind: helpLineMessage, text: "No matching keybindings"})
	}

	return specs
}

// helpSpecPlain returns the un-styled visible form of a help-line
// spec. The plain form is what the app-layer match counter sees, so
// substring/regex/fuzzy queries match the same characters the user
// reads on screen — never bytes hidden inside an ANSI SGR sequence.
func helpSpecPlain(s helpLineSpec) string {
	switch s.kind {
	case helpLineBlank:
		return ""
	case helpLineSectionHeader:
		return "  " + s.text
	case helpLineEntry:
		return "    " + s.key + "  " + s.desc
	case helpLineMessage:
		return "  " + s.text
	}
	return ""
}

// helpSpecStyled renders a help-line spec to its final styled form.
// When search is non-empty the inline highlight is applied to plain
// key/desc/text first via HighlightMatchStyledOver, then wrapped with
// the segment's outer style via RenderOverPrestyled. Highlighting on
// plain segments keeps the match-finder away from any ANSI bytes —
// fixing the "/ search of a digit prints raw [33;1m" report.
//
// isCurrent flips the row's highlight to SelectedSearchHighlightStyle
// so n/N navigation can mark the active match distinctly.
func helpSpecStyled(s helpLineSpec, search string, isCurrent bool) string {
	hl := SearchHighlightStyle
	if isCurrent {
		hl = SelectedSearchHighlightStyle
	}
	switch s.kind {
	case helpLineBlank:
		return ""
	case helpLineSectionHeader:
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary)).Underline(true).Background(SurfaceBg)
		inner := HighlightMatchStyledOver(s.text, search, hl, headerStyle)
		return "  " + RenderOverPrestyled(inner, headerStyle)
	case helpLineEntry:
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(SurfaceBg)
		descStyle := OverlayDimStyle
		keyInner := HighlightMatchStyledOver(s.key, search, hl, keyStyle)
		descInner := HighlightMatchStyledOver(s.desc, search, hl, descStyle)
		return "    " + RenderOverPrestyled(keyInner, keyStyle) + "  " + RenderOverPrestyled(descInner, descStyle)
	case helpLineMessage:
		inner := HighlightMatchStyledOver(s.text, search, hl, OverlayDimStyle)
		return "  " + RenderOverPrestyled(inner, OverlayDimStyle)
	}
	return ""
}

// RenderHelpScreen renders a full help overlay with all keybindings.
// filter narrows the visible lines (f key). search highlights matches
// in the visible lines without removing them (/ key). currentMatchLine
// is the index (in the post-filter line list) of the line under the
// n/N navigation cursor — that line gets a distinct "selected match"
// style so the user can see which match is current. Pass -1 when
// there's no active navigation. contextMode limits sections to the
// current view (empty = explorer).
func RenderHelpScreen(screenWidth, screenHeight, scroll int, filter, search, contextMode string, currentMatchLine int) string {
	boxW := max(screenWidth*70/100, 50)
	// Mirror HelpVisibleLines so outer height stays in sync with the
	// inner row budget — lipgloss pads short content to this height,
	// stopping the box from shrinking when filter narrows results or
	// from growing when long lines wrap.
	boxH := max(screenHeight*80/100, 20)

	contentW := boxW - 6 // account for border + padding

	title := OverlayTitleStyle.Render("Keybindings")

	// Build structural specs once, then render each row with the
	// search highlight pre-spliced into the plain segments before the
	// outer style is applied. Highlighting on plain text keeps the
	// match-finder away from ANSI escape bytes — the previous
	// "highlight on already-styled, already-truncated line" path could
	// match a digit query inside an SGR like \x1b[33;1m, which broke
	// the sequence and printed "[33;" / ";1m" as visible text.
	specs := buildHelpSpecs(filter, contextMode)
	// Truncate each line to the inner-panel content width so one entry
	// in `lines` always renders as exactly one row. Lipgloss's
	// auto-wrap behavior would otherwise silently expand long
	// descriptions to two rows, the rendered row count would diverge
	// from len(lines), and the outer box height would drift — making
	// a filter that narrows results visibly shrink the window.
	innerW := max(contentW-2, 10)
	lines := make([]string, len(specs))
	for i, s := range specs {
		lines[i] = Truncate(helpSpecStyled(s, search, i == currentMatchLine), innerW)
	}
	totalLines := len(lines)

	// Calculate visible area via shared helper so app-layer clamps see
	// the same maxScroll the renderer enforces.
	visibleLines := HelpVisibleLines(screenHeight)

	// Clamp scroll.
	maxScroll := max(totalLines-visibleLines, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	// Determine scroll indicators.
	hasAbove := scroll > 0
	hasBelow := scroll+visibleLines < totalLines

	// Slice visible portion.
	end := min(scroll+visibleLines, totalLines)
	visible := lines[scroll:end]

	// Pad the visible window to exactly visibleLines rows so a filter
	// that narrows results doesn't shrink the box. Without this the
	// outer overlay box collapses to fit the short content and the user
	// sees the window resize on every keystroke.
	for len(visible) < visibleLines {
		visible = append(visible, "")
	}

	// Build final lines with indicators.
	var displayLines []string
	// Always include indicator lines (empty when not scrollable) to keep height stable.
	if hasAbove {
		displayLines = append(displayLines, OverlayDimStyle.Render("  \u2191 more above"))
	} else {
		displayLines = append(displayLines, "")
	}
	displayLines = append(displayLines, visible...)
	if hasBelow {
		displayLines = append(displayLines, OverlayDimStyle.Render("  \u2193 more below"))
	} else {
		displayLines = append(displayLines, "")
	}

	content := strings.Join(displayLines, "\n")
	content = FillLinesBg(content, contentW-2, SurfaceBg) // -2 for innerPanelStyle padding
	innerPanel := innerPanelStyle.
		Width(contentW).
		Render(content)

	body := title + "\n" + innerPanel
	body = FillLinesBg(body, boxW-4, SurfaceBg) // -4 for OverlayStyle padding(1,2)

	return OverlayStyle.
		Width(boxW).
		Height(boxH).
		Render(body)
}
