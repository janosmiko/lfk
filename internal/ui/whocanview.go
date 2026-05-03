package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// WhoCanRow is the renderer-facing representation of a single result
// row in the Who-Can table. Mirrors k8s.WhoCanSubject but kept here so
// the ui package doesn't import the k8s package.
type WhoCanRow struct {
	Kind      string // "User" / "Group" / "ServiceAccount"
	Name      string
	Namespace string // ServiceAccount namespace; empty for User/Group
	Via       string // "ClusterRoleBinding/foo → ClusterRole/bar"
}

// WhoCanVerbs is the canonical verb chip list rendered above the
// subjects table. The trailing "*" matches any verb (rule.Verbs == ["*"]
// in RBAC). Order is `kubectl who-can`'s order so muscle memory carries.
var WhoCanVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete", "*"}

// RenderWhoCanView builds the inner content of the reverse-RBAC view.
// Layout (top → bottom):
//
//   - title
//   - verb chip row with cursor highlight on verbCursor
//   - resource filter line (input or current resource)
//   - subjects table (header + scrollable rows)
//
// Returns raw content; the caller wraps it in OverlayStyle the same way
// the forward Can-I view is wrapped (see view_overlays.go's
// renderCanIOverlay path).
func RenderWhoCanView(
	verbCursor int,
	resource string,
	resourceFilter string,
	resourceFilterActive bool,
	namespaceLabel string,
	rows []WhoCanRow,
	scroll int,
	loading bool,
	width, height int,
) string {
	title := TitleStyle.Render("Reverse RBAC: Who can …") + "  " + BarDimStyle.Render(namespaceLabel)

	verbsLine := renderWhoCanVerbs(verbCursor)
	resourceLine := renderWhoCanResourceLine(resource, resourceFilter, resourceFilterActive)

	// Reserve 4 lines for title + verbs + resource + spacing.
	tableHeight := max(height-5, 3)
	table := renderWhoCanTable(rows, scroll, loading, width, tableHeight)

	return strings.Join([]string{title, "", verbsLine, resourceLine, "", table}, "\n")
}

// renderWhoCanVerbs builds the verb-chip row. Each chip is a label
// padded with a space; the cursor chip uses OverlaySelectedStyle so
// the highlight covers it cleanly across themes.
func renderWhoCanVerbs(cursor int) string {
	var b strings.Builder
	b.WriteString(DimStyle.Render("Verb: "))
	for i, v := range WhoCanVerbs {
		chip := " " + v + " "
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(chip))
		} else {
			b.WriteString(OverlayNormalStyle.Render(chip))
		}
		if i < len(WhoCanVerbs)-1 {
			b.WriteString(" ")
		}
	}
	return b.String()
}

// renderWhoCanResourceLine prints the resource picker. The line either
// shows the live filter input (cursor visible while typing) or the
// committed resource name plus a hint to start filtering. Empty
// resource shows a placeholder so the user knows what to type.
func renderWhoCanResourceLine(resource, filter string, active bool) string {
	prefix := DimStyle.Render("Resource: ")
	switch {
	case active:
		return prefix + OverlayFilterStyle.Render("/ "+filter+"█")
	case resource != "":
		return prefix + OverlayNormalStyle.Render(resource) + "  " + DimStyle.Render("(/  to filter)")
	default:
		return prefix + DimStyle.Render("/  to filter (e.g. pods, secrets, deployments)")
	}
}

// renderWhoCanTable prints the subjects header + scrollable rows. The
// height contract: caller passes how many lines we may use, including
// the header. Empty rows produce an explanatory placeholder; loading
// state shows a spinner-like marker so users know the fetch is live.
func renderWhoCanTable(rows []WhoCanRow, scroll int, loading bool, width, height int) string {
	const (
		kindW = 16
		nsW   = 18
	)
	nameW := max(20, width/3)
	viaW := max(20, width-(kindW+nsW+nameW+6)) // -6 for column separators

	header := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s",
		nameW, "SUBJECT", kindW, "KIND", nsW, "NAMESPACE", viaW, "VIA")
	headerLine := DimStyle.Bold(true).Render(header)

	if loading {
		return headerLine + "\n  " + DimStyle.Render("Loading…")
	}
	if len(rows) == 0 {
		return headerLine + "\n  " + DimStyle.Render("No subject found with this permission")
	}

	maxLines := max(height-1, 1) // -1 for the header
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(rows)-maxLines {
		scroll = max(len(rows)-maxLines, 0)
	}
	end := min(scroll+maxLines, len(rows))

	body := make([]string, 0, end-scroll)
	for _, r := range rows[scroll:end] {
		body = append(body, renderWhoCanRow(r, nameW, kindW, nsW, viaW))
	}
	return headerLine + "\n" + strings.Join(body, "\n")
}

// renderWhoCanRow formats a single subject line, dimming the namespace
// for User/Group rows (which don't have one) so the column stays
// visually consistent. ANSI-aware truncation isn't necessary here —
// every value is plain text.
func renderWhoCanRow(r WhoCanRow, nameW, kindW, nsW, viaW int) string {
	ns := r.Namespace
	if ns == "" {
		ns = "—"
	}
	nameCell := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Render(whoCanTruncate(r.Name, nameW))
	kindCell := DimStyle.Render(whoCanTruncate(r.Kind, kindW))
	nsCell := DimStyle.Render(whoCanTruncate(ns, nsW))
	viaCell := DimStyle.Render(whoCanTruncate(r.Via, viaW))
	return fmt.Sprintf("  %s  %s  %s  %s",
		whoCanPadRight(nameCell, nameW),
		whoCanPadRight(kindCell, kindW),
		whoCanPadRight(nsCell, nsW),
		whoCanPadRight(viaCell, viaW))
}

// whoCanTruncate trims a plain (non-ANSI) string to maxW columns,
// appending "…" when cut. Kept private to this file under a unique
// name so it doesn't collide with the existing padRight in
// explorer_format.go (which is ANSI-aware in different ways).
func whoCanTruncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return string(runes[:maxW-1]) + "…"
}

// whoCanPadRight extends a styled cell to width by appending plain
// spaces. lipgloss.Width is ANSI-aware so this measures the visible
// width of the styled string before padding.
func whoCanPadRight(cell string, width int) string {
	gap := width - lipgloss.Width(cell)
	if gap <= 0 {
		return cell
	}
	return cell + strings.Repeat(" ", gap)
}
