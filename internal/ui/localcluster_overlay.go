// Renders the local-cluster manager overlay. Three layouts:
//   - "no providers installed" — shows brew install hints
//   - "empty list"             — shows the empty hint
//   - "populated list"         — shows the table
//
// Wizard sub-screens and delete confirmation are rendered by separate
// helpers. Keymap hints live in overlayHintBarMisc (overlay_hintbar.go);
// do not add inline hint rows here. Caller wraps the returned content
// in OverlayStyle.Width(w).Height(h).Render(...) — consistent with the
// other standard overlays routed through renderOverlayContent.
package ui

import (
	"fmt"
	"strings"
)

// LocalClusterOverlayState is the renderer's input. Built by the app's
// view function from Model.localClusterState.
type LocalClusterOverlayState struct {
	ProvidersInstalled []string
	Clusters           []LocalClusterRowView
	Cursor             int
	Loading            bool
	Info               string // transient banner ("Created kind/dev"); rendered above the table
	GlobalErr          string
	Width              int
	Height             int
}

// LocalClusterRowView is one row in the manager table. Mutating is
// non-empty when an op is in flight on this row (e.g. "stopping...").
type LocalClusterRowView struct {
	Provider   string
	Name       string
	Status     string
	K8sVersion string
	Nodes      int
	Age        string
	Mutating   string
	ListError  string
}

// RenderLocalClusterOverlay produces the overlay content. The caller
// wraps the result in OverlayStyle.Width(w).Height(h).Render(...).
//
// Pending Create rows are merged into Clusters by the caller, so an
// in-flight first-ever Create lands in the populated-list path even
// when the real cluster list is still empty.
func RenderLocalClusterOverlay(s LocalClusterOverlayState) string {
	switch {
	case s.Loading:
		// Show the loading state before checking ProvidersInstalled
		// or Clusters. Otherwise an in-flight detect/refresh briefly
		// flashes the install-hint or empty-list panes when the
		// previous result was empty.
		return renderLocalClusterLoading(s)
	case len(s.ProvidersInstalled) == 0:
		return renderLocalClusterInstallHint(s)
	case len(s.Clusters) == 0:
		return renderLocalClusterEmpty(s)
	default:
		return renderLocalClusterList(s)
	}
}

func renderLocalClusterLoading(s LocalClusterOverlayState) string {
	innerH := max(s.Height-2, 1)
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Local Clusters"))
	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("Detecting local clusters..."))
	return PadToHeight(b.String(), innerH)
}

func renderLocalClusterInstallHint(s LocalClusterOverlayState) string {
	innerH := max(s.Height-2, 1)
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Local Clusters"))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("No local-cluster provider installed."))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("  brew install kind     # https://kind.sigs.k8s.io"))
	b.WriteString("\n")
	b.WriteString(OverlayNormalStyle.Render("  brew install k3d      # https://k3d.io"))
	b.WriteString("\n")
	b.WriteString(OverlayNormalStyle.Render("  brew install minikube # https://minikube.sigs.k8s.io"))
	return PadToHeight(b.String(), innerH)
}

func renderLocalClusterEmpty(s LocalClusterOverlayState) string {
	innerH := max(s.Height-2, 1)
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Local Clusters"))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("No local clusters yet."))
	return PadToHeight(b.String(), innerH)
}

func renderLocalClusterList(s LocalClusterOverlayState) string {
	innerH := max(s.Height-2, 1)
	innerW := max(s.Width-6, 40)
	var b strings.Builder

	running, stopped := 0, 0
	for _, c := range s.Clusters {
		switch c.Status {
		case "running":
			running++
		case "stopped":
			stopped++
		}
	}
	header := fmt.Sprintf("Local Clusters: %d clusters, %d running, %d stopped",
		len(s.Clusters), running, stopped)
	b.WriteString(OverlayTitleStyle.Render(header))
	b.WriteString("\n")

	if s.Info != "" {
		b.WriteString(OverlayDimStyle.Render(s.Info))
		b.WriteString("\n")
	}
	if s.GlobalErr != "" {
		b.WriteString(OverlayWarningStyle.Render(s.GlobalErr))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Compute the Name column width to flex with the overlay. All other
	// columns stay fixed: provider 9, status 12, k8s 10, nodes 5, age 8;
	// plus 5 inter-column gaps of 2 cols = 54 cols of fixed budget. The
	// Name column claims the rest, with a 15-col floor so narrow overlays
	// still show enough of the name to be useful.
	const fixedCols = 9 + 12 + 10 + 5 + 8 + 5*2
	nameW := max(innerW-fixedCols, 15)

	b.WriteString(OverlayDimStyle.Render(formatLocalClusterRow(
		nameW, "PROVIDER", "NAME", "STATUS", "K8S", "NODES", "AGE",
	)))
	b.WriteString("\n")

	for i, c := range s.Clusters {
		status := c.Status
		if c.Mutating != "" {
			status = c.Mutating
		}
		nodes := ""
		if c.Nodes > 0 {
			nodes = fmt.Sprintf("%d", c.Nodes)
		}
		row := formatLocalClusterRow(nameW, c.Provider, c.Name, status, c.K8sVersion, nodes, c.Age)
		if i == s.Cursor {
			b.WriteString(OverlaySelectedStyle.Render(row))
		} else {
			b.WriteString(OverlayNormalStyle.Render(row))
		}
		b.WriteString("\n")
		if c.ListError != "" {
			b.WriteString(OverlayDimStyle.Render("    " + c.ListError))
			b.WriteString("\n")
		}
	}
	return PadToHeight(b.String(), innerH)
}

// formatLocalClusterRow lays out one table row. nameW is the width of
// the Name column (flex with overlay width); other columns are fixed
// (provider 9, status 12, k8s 10, nodes 5, age 8). Values are
// truncated to fit; provider/status/k8s/nodes are short by
// construction so truncation only really matters for Name.
func formatLocalClusterRow(nameW int, prov, name, status, ver, nodes, age string) string {
	return fmt.Sprintf("%-9s  %-*s  %-12s  %-10s  %-5s  %-8s",
		Truncate(prov, 9),
		nameW, Truncate(name, nameW),
		Truncate(status, 12),
		Truncate(ver, 10),
		Truncate(nodes, 5),
		Truncate(age, 8),
	)
}

// LocalClusterDeleteConfirmView is the render input for the
// "type DELETE to confirm" sub-screen.
type LocalClusterDeleteConfirmView struct {
	Provider    string
	Name        string
	ContextName string
	Buffer      string
}

// RenderLocalClusterDeleteConfirm renders the delete-confirm sub-screen
// using the same shape as the project's Force Delete confirmation
// (RenderConfirmTypeOverlay) so destructive actions feel uniform across
// the app. The caller wraps the result in
// OverlayStyle.Width(w).Height(h).Render(...).
func RenderLocalClusterDeleteConfirm(s LocalClusterOverlayState, v LocalClusterDeleteConfirmView) string {
	innerH := max(s.Height-2, 1)
	question := fmt.Sprintf("Delete cluster %s? This destroys the cluster and removes its kubeconfig context. This cannot be undone.",
		v.ContextName)
	return PadToHeight(
		RenderConfirmTypeOverlay("Confirm Delete Local Cluster", question, v.Buffer),
		innerH,
	)
}

// LocalClusterWizardView is the render input for any wizard step.
// Step is a 1-based index into the breadcrumb (1..5).
type LocalClusterWizardView struct {
	Step               int
	Provider           string
	ProviderCursor     int
	InstalledProviders []string
	Name               string
	NameErr            string
	Version            string
	VersionErr         string
	Nodes              string
	NodesErr           string
}

// RenderLocalClusterWizard renders the wizard sub-screen body. The
// caller wraps the result in OverlayStyle.Width(w).Height(h).Render(...)
// — same as the list view.
func RenderLocalClusterWizard(s LocalClusterOverlayState, w LocalClusterWizardView) string {
	innerH := max(s.Height-2, 1)
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Create local cluster"))
	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render(fmt.Sprintf("(%d/5) Provider > Name > Version > Nodes > Confirm", w.Step)))
	b.WriteString("\n\n")
	switch w.Step {
	case 1:
		if len(w.InstalledProviders) == 0 {
			b.WriteString(OverlayNormalStyle.Render("No installed providers - press esc to go back."))
		} else {
			for i, p := range w.InstalledProviders {
				prefix := "  "
				line := p
				if i == w.ProviderCursor {
					prefix = "> "
					line = OverlaySelectedStyle.Render(p)
				}
				b.WriteString(prefix + line + "\n")
			}
		}
	case 2:
		b.WriteString(OverlayNormalStyle.Render("Name (lowercase letters, digits, dashes; <= 30 chars):"))
		b.WriteString("\n\n")
		b.WriteString(OverlayNormalStyle.Render("  > " + w.Name + "_"))
		b.WriteString("\n")
		if w.NameErr != "" {
			b.WriteString("\n  ")
			b.WriteString(OverlayWarningStyle.Render(w.NameErr))
		}
	case 3:
		b.WriteString(OverlayNormalStyle.Render("Kubernetes version (empty = provider default; e.g. v1.30.0):"))
		b.WriteString("\n\n")
		b.WriteString(OverlayNormalStyle.Render("  > " + w.Version + "_"))
		b.WriteString("\n")
		if w.VersionErr != "" {
			b.WriteString("\n  ")
			b.WriteString(OverlayWarningStyle.Render(w.VersionErr))
		}
	case 4:
		b.WriteString(OverlayNormalStyle.Render("Nodes (1-9):"))
		b.WriteString("\n\n")
		b.WriteString(OverlayNormalStyle.Render("  > " + w.Nodes + "_"))
		b.WriteString("\n")
		if w.NodesErr != "" {
			b.WriteString("\n  ")
			b.WriteString(OverlayWarningStyle.Render(w.NodesErr))
		}
	case 5:
		b.WriteString(OverlayNormalStyle.Render("Confirm:"))
		b.WriteString("\n\n")
		b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Provider:    %s", w.Provider)))
		b.WriteString("\n")
		b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Name:        %s", w.Name)))
		b.WriteString("\n")
		v := w.Version
		if v == "" {
			v = "(default)"
		}
		b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  K8s version: %s", v)))
		b.WriteString("\n")
		b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Nodes:       %s", w.Nodes)))
		b.WriteString("\n")
	}
	return PadToHeight(b.String(), innerH)
}
