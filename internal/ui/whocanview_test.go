package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// TestRenderWhoCanView_ContainsExpectedText is a smoke test: the
// renderer surfaces the title, verb chips, the resource picker, and
// the subjects column. The user needs to see at minimum what verb
// they're querying, what resources are available to pick from, and
// the subject result so the layout reads as a complete picture.
func TestRenderWhoCanView_ContainsExpectedText(t *testing.T) {
	rows := []WhoCanRow{
		{Kind: "User", Name: "alice", Namespace: "", Via: "ClusterRoleBinding/admins → ClusterRole/cluster-admin"},
		{Kind: "ServiceAccount", Name: "default", Namespace: "kube-system", Via: "RoleBinding/foo → Role/bar"},
	}
	out := RenderWhoCanView(WhoCanViewParams{
		VerbCursor:     0,
		Resources:      []string{"pods", "secrets", "deployments"},
		ResourceCursor: 0,
		NamespaceLabel: "ns: default",
		Subjects:       rows,
		Width:          160, Height: 30,
	})

	for _, want := range []string{
		"RBAC Explorer: Who-Can?",
		"Verb:",
		"Resources",
		"pods", "secrets", "deployments",
		"SUBJECT", // column header in the subjects panel
		"alice",
		"default",
		"kube-system",
	} {
		assert.Contains(t, out, want, "rendered overlay must surface %q", want)
	}
}

// TestRenderWhoCanView_LoadingPlaceholder shows a Loading… line in
// place of rows while a fetch is in flight. Without this users see
// an empty subject panel and assume "no permissions" — a worse
// default than "still loading".
func TestRenderWhoCanView_LoadingPlaceholder(t *testing.T) {
	out := RenderWhoCanView(WhoCanViewParams{
		Resources:      []string{"pods"},
		ResourceCursor: 0,
		NamespaceLabel: "ns: default",
		Loading:        true,
		Width:          120, Height: 20,
	})
	assert.Contains(t, out, "Loading")
}

// TestRenderWhoCanView_EmptyResultMessage tells users that a verb is
// genuinely unbound rather than leaving a blank panel that looks
// broken. Required state: a resource is selected (otherwise the
// panel says "pick a resource" instead of "no subject").
func TestRenderWhoCanView_EmptyResultMessage(t *testing.T) {
	out := RenderWhoCanView(WhoCanViewParams{
		Resources:      []string{"pods"},
		ResourceCursor: 0,
		NamespaceLabel: "ns: default",
		Subjects:       []WhoCanRow{},
		Width:          120, Height: 20,
	})
	assert.Contains(t, out, "No subject has this permission")
}

// TestRenderWhoCanView_NoResourceSelectedShowsHint covers the
// initial-empty case: Can-I had nothing highlighted, the picker is
// empty, and the right pane should tell the user what to do next
// instead of looking silently broken.
func TestRenderWhoCanView_NoResourceSelectedShowsHint(t *testing.T) {
	out := RenderWhoCanView(WhoCanViewParams{
		Resources:      nil,
		NamespaceLabel: "all-namespaces",
		Width:          120, Height: 20,
	})
	assert.Contains(t, out, "Pick a resource")
}

// TestRenderWhoCanView_FooterBarRenderedAtBottom confirms that the
// caller-supplied footer string lands in the overlay's bottom row
// (the slot that mirrors Can-I's search bar location). The caller —
// renderWhoCanOverlay — puts the filter input there so its position
// matches when the user pivots between modes with Tab.
func TestRenderWhoCanView_FooterBarRenderedAtBottom(t *testing.T) {
	out := RenderWhoCanView(WhoCanViewParams{
		Resources:      []string{"pods/exec"},
		NamespaceLabel: "all-namespaces",
		FooterBar:      "FILTER-MARKER-/exe█",
		Width:          120, Height: 20,
	})
	assert.Contains(t, out, "FILTER-MARKER-/exe█", "footer bar must appear in the rendered overlay")
}

// TestRenderWhoCanView_CursorHighlight pins the visual contract that
// the resource under the cursor is rendered with the selection style
// (the user must see which resource the right pane is for).
func TestRenderWhoCanView_CursorHighlight(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(originalProfile)
		ApplyTheme(DefaultTheme())
	})
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ApplyTheme(DefaultTheme())
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	out := RenderWhoCanView(WhoCanViewParams{
		Resources:      []string{"pods", "secrets", "services"},
		ResourceCursor: 1, // "secrets"
		NamespaceLabel: "ns: default",
		Width:          120, Height: 20,
	})
	// The selected row gets the "> " prefix; non-selected rows get "  ".
	assert.Contains(t, out, "> secrets", "cursor row must carry the selection prefix")
}

// TestRenderWhoCanRow_NamespaceDashWhenEmpty prevents a regression
// where the namespace column was blank for User/Group rows (no ns at
// all) and made the row look like a render glitch. The em dash makes
// it obvious the value is intentionally absent.
func TestRenderWhoCanRow_NamespaceDashWhenEmpty(t *testing.T) {
	row := renderWhoCanRow(WhoCanRow{Kind: "User", Name: "alice", Namespace: ""}, 20, 16, 18, 30)
	assert.Contains(t, row, "—", "empty namespace must render as em dash")
}

// TestRenderWhoCanSubjects_RowsFitAvailableWidth pins the table
// layout math: the row format prepends 2 leading spaces and 3 × 2-
// space separators between cells (8 cells of overhead). The viaW
// formula must reserve those 8 cells, otherwise rows are wider than
// the column's inner area and lipgloss wraps each row onto a second
// line.
func TestRenderWhoCanSubjects_RowsFitAvailableWidth(t *testing.T) {
	rows := []WhoCanRow{
		{Kind: "User", Name: "alice", Namespace: "default", Via: "ClusterRoleBinding/admins → ClusterRole/cluster-admin"},
	}
	// Include narrow widths to guard against the off-by-cap regression
	// where hard-coded minimum widths exceeded the available area.
	for _, width := range []int{40, 60, 80, 120, 160} {
		t.Run("width="+itoa(width), func(t *testing.T) {
			out := renderWhoCanSubjects(rows, 0, false, "pods", width, 20)
			for i, line := range strings.Split(out, "\n") {
				visible := lipgloss.Width(line)
				assert.LessOrEqualf(t, visible, width,
					"line %d (%q) is %d visible cols but the column only has %d available — row will wrap",
					i, line, visible, width)
			}
		})
	}
}

// TestRenderWhoCanRow_CellsBindBaseBackground locks in the fix for
// the "background swap" the user reported. Subject rows live inside
// the InactiveColumnStyle box (baseBg). Cell text rendered with fg-
// only escapes shows the terminal default bg in place of baseBg, while
// the unstyled padding spaces inherit baseBg from the column wrap —
// producing a visible alternating band along the row. Every cell style
// must therefore set Background(BaseBg) explicitly.
func TestRenderWhoCanRow_CellsBindBaseBackground(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ConfigNoColor
	originalTransparent := ConfigTransparentBg
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(originalProfile)
		ConfigNoColor = originalNoColor
		ConfigTransparentBg = originalTransparent
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ConfigTransparentBg = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ApplyTheme(DefaultTheme())
	// ApplyTheme restores originalColorProfile (theme.go:109-110), so
	// re-force TrueColor here for the SGR-counting assertion to be
	// observable at all.
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	row := renderWhoCanRow(WhoCanRow{
		Kind: "User", Name: "alice", Namespace: "default",
		Via: "ClusterRoleBinding/admins → ClusterRole/cluster-admin",
	}, 20, 14, 16, 30)

	// 256-color bg = "48;5;", truecolor bg = "48;2;". Either is enough
	// to know a span has its bg set.
	bgMarkers := strings.Count(row, "48;5;") + strings.Count(row, "48;2;")
	assert.GreaterOrEqual(t, bgMarkers, 4,
		"row has 4 styled cells; each should emit a bg-setting SGR so the row's bg matches the column's baseBg (got %d, row=%q)", bgMarkers, row)
}

// itoa is a tiny helper to avoid importing strconv just for this file.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
