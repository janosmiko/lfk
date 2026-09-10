package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/k8s"
)

func TestConstraintsBanner_DistinguishesDeniedFromFailed(t *testing.T) {
	out := stripANSI(constraintsBanner([]string{"poddisruptionbudgets"}, []string{"nodes"}, 120))

	if !strings.Contains(out, "denied") || !strings.Contains(out, "poddisruptionbudgets") {
		t.Errorf("expected the denied group to name poddisruptionbudgets, got %q", out)
	}
	if !strings.Contains(out, "failed") || !strings.Contains(out, "nodes") {
		t.Errorf("expected the failed group to name nodes, got %q", out)
	}
}

func TestFormatConstraintRow_FitsTheAvailableWidth(t *testing.T) {
	row := k8s.ConstraintRow{
		Source:    "Quota",
		Kind:      "ResourceQuota",
		Namespace: "a-rather-long-namespace-name",
		Name:      "compute-quota-with-a-long-name",
		Headroom:  "1500m",
		Detail:    "requests 500m cpu (hard 4, used 3) and then some more text to overflow",
	}

	for _, width := range []int{20, 40, 60, 80, 120, 200} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			rows := []k8s.ConstraintRow{row}
			line := stripANSI(formatConstraintRow(row, true, constraintsColumns(width, rows)))
			if got := lipgloss.Width(line); got > width {
				t.Errorf("row width = %d, want at most %d: %q", got, width, line)
			}
		})
	}
}

// Reproduces the UAT screenshot: node names differing only past the old
// fixed 28-char floor rendered identically until truncated.
func TestConstraintsColumns_SizesToWidestCellAtWideWidth(t *testing.T) {
	rows := []k8s.ConstraintRow{
		{
			Source: "Node", Kind: "Node", Name: "dev-envs-control-plane-fsn1-aaaaaa",
			Headroom: "not tolerated",
			Detail:   "taint node-role.kubernetes.io/control-plane=:NoSchedule not tolerated",
		},
		{
			Source: "Node", Kind: "Node", Name: "dev-envs-control-plane-fsn1-bbbbbb",
			Headroom: "not tolerated",
			Detail:   "taint node-role.kubernetes.io/control-plane=:NoSchedule not tolerated",
		},
	}

	cols := constraintsColumns(236, rows)

	wantNameWidth := lipgloss.Width("dev-envs-control-plane-fsn1-aaaaaa")
	if cols.name < wantNameWidth {
		t.Errorf("name column width = %d, want at least %d so the full node name renders", cols.name, wantNameWidth)
	}
	wantHeadroomWidth := lipgloss.Width("not tolerated")
	if cols.headroom < wantHeadroomWidth {
		t.Errorf("headroom column width = %d, want at least %d", cols.headroom, wantHeadroomWidth)
	}
	if got := cols.source + cols.kind + cols.name + cols.headroom + cols.detail + 5; got > cols.line+1 {
		t.Errorf("column sum = %d, want at most line width %d", got, cols.line+1)
	}

	for _, row := range rows {
		line := stripANSI(formatConstraintRow(row, false, cols))
		if !strings.Contains(line, row.Name) {
			t.Errorf("at width 236, expected the full node name %q to render untruncated, got %q", row.Name, line)
		}
	}
}

// The other end: at 80 columns the row must still fit and Detail gives way first.
func TestConstraintsColumns_NarrowWidthStillFitsAndTruncatesDetailLast(t *testing.T) {
	row := k8s.ConstraintRow{
		Source: "Webhook", Kind: "ValidatingWebhookConfiguration",
		Namespace: "default", Name: "policy-check",
		Headroom: "n/a",
		Detail:   "rule create/update on pods matches, service policy-checker.default.svc:443, failurePolicy Fail",
	}
	cols := constraintsColumns(80, []k8s.ConstraintRow{row})
	line := stripANSI(formatConstraintRow(row, false, cols))

	if got := lipgloss.Width(line); got > 80 {
		t.Errorf("row width = %d, want at most 80: %q", got, line)
	}
	if !strings.Contains(line, "Webhook") {
		t.Errorf("expected the narrow Source column to still show its content, got %q", line)
	}
}

func TestConstraintsHeaderLine_MatchesColumnWidthsAndIsNotSelectable(t *testing.T) {
	rows := []k8s.ConstraintRow{
		{Source: "Quota", Kind: "ResourceQuota", Namespace: "default", Name: "compute-quota", Headroom: "1", Detail: "requests 500m cpu"},
	}
	cols := constraintsColumns(120, rows)
	header := stripANSI(constraintsHeaderLine(cols))

	for _, label := range []string{"SOURCE", "KIND", "NAME", "HEADROOM", "DETAIL"} {
		if !strings.Contains(header, label) {
			t.Errorf("expected header to contain %q, got %q", label, header)
		}
	}
	if got := lipgloss.Width(header); got > cols.line+1 {
		t.Errorf("header width = %d, want at most %d", got, cols.line+1)
	}
}

func TestRenderConstraintsRows_HeaderPrecedesRowsAndIsNotARow(t *testing.T) {
	m := basePush80Model()
	m.width = 120
	m.height = 40
	m.constraints.report = k8s.ConstraintReport{
		Rows: []k8s.ConstraintRow{
			{Source: "Quota", Kind: "ResourceQuota", Namespace: "default", Name: "compute-quota", Headroom: "1", Detail: "requests 500m cpu"},
			{Source: "PDB", Kind: "PodDisruptionBudget", Namespace: "default", Name: "web-pdb", Headroom: "0", Detail: "minAvailable 2"},
		},
	}

	out := m.renderConstraintsRows()
	if len(out) != len(m.constraints.report.Rows)+1 {
		t.Fatalf("got %d lines, want %d (header + rows)", len(out), len(m.constraints.report.Rows)+1)
	}
	header := stripANSI(out[0])
	if !strings.Contains(header, "SOURCE") {
		t.Errorf("expected the first line to be the header, got %q", header)
	}
	if !strings.Contains(stripANSI(out[1]), "compute-quota") {
		t.Errorf("expected the first row after the header, got %q", stripANSI(out[1]))
	}

	// The cursor indexes report.Rows, not the rendered lines, so it stays
	// in range even though the header adds an extra line.
	m.constraints.cursor = 1
	out = m.renderConstraintsRows()
	if !strings.Contains(stripANSI(out[2]), "web-pdb") {
		t.Errorf("expected cursor row 1 to render web-pdb, got %q", stripANSI(out[2]))
	}
}

func TestViewConstraints_At240Columns_DoesNotTruncateColumnsThatFit(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.width = 240
	m.height = 40
	m.constraints.title = "web-1"
	m.constraints.report = k8s.ConstraintReport{
		Rows: []k8s.ConstraintRow{
			{
				Source: "Node", Kind: "Node", Name: "dev-envs-control-plane-fsn1-aaaaaaaaaa",
				Headroom: "not tolerated",
				Detail:   "taint node-role.kubernetes.io/control-plane=:NoSchedule not tolerated",
			},
			{
				Source: "Node", Kind: "Node", Name: "dev-envs-control-plane-fsn1-bbbbbbbbbb",
				Headroom: "not tolerated",
				Detail:   "taint node-role.kubernetes.io/control-plane=:NoSchedule not tolerated",
			},
		},
	}

	out := stripANSI(m.View().Content)
	for _, row := range m.constraints.report.Rows {
		if !strings.Contains(out, row.Name) {
			t.Errorf("expected full node name %q to render at 240 columns, got:\n%s", row.Name, out)
		}
		if !strings.Contains(out, row.Detail) {
			t.Errorf("expected full detail %q to render at 240 columns, got:\n%s", row.Detail, out)
		}
	}
}

func TestViewConstraints_At80Columns_RowsFitWithinWidth(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.width = 80
	m.height = 30
	m.constraints.title = "web-1"
	m.constraints.report = k8s.ConstraintReport{
		Rows: []k8s.ConstraintRow{{
			Source: "Webhook", Kind: "ValidatingWebhookConfiguration",
			Namespace: "default", Name: "policy-check",
			Headroom: "n/a",
			Detail:   "rule create/update on pods matches, service policy-checker.default.svc:443, failurePolicy Fail",
		}},
	}

	out := stripANSI(m.View().Content)
	for line := range strings.SplitSeq(out, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line width = %d, want at most 80: %q", w, line)
		}
	}
}

func TestConstraintsBanner_TruncatesToOneLineSoDataRowsAreNotClipped(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.width = 60
	m.height = 20
	rows := make([]k8s.ConstraintRow, 30)
	for i := range rows {
		// Short, so the row survives column truncation at 60 columns — the
		// assertion below is about the row being scrolled into view, not
		// about its cell width (that's covered by the column-width tests).
		rows[i] = k8s.ConstraintRow{Source: "Quota", Kind: "RQ", Name: fmt.Sprintf("q%02d", i)}
	}
	m.constraints.report = k8s.ConstraintReport{
		Rows: rows,
		Skipped: []string{
			"a-very-long-denied-resource-name-one",
			"a-very-long-denied-resource-name-two",
			"a-very-long-denied-resource-name-three",
		},
		Failed: []string{
			"a-very-long-failed-resource-name-one",
			"a-very-long-failed-resource-name-two",
			"a-very-long-failed-resource-name-three",
		},
	}

	result, _ := m.handleConstraintsKey(tea.KeyPressMsg{Code: 'G', Text: "G"})
	updated, ok := result.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", result)
	}

	content := stripANSI(updated.viewConstraints())
	bannerLines := 0
	for line := range strings.SplitSeq(content, "\n") {
		if strings.Contains(line, "skipped (") {
			bannerLines++
		}
	}
	if bannerLines != 1 {
		t.Errorf("banner rendered as %d lines, want exactly 1: %q", bannerLines, content)
	}

	lastRow := rows[len(rows)-1]
	if !strings.Contains(content, lastRow.Name) {
		t.Errorf("expected the cursor row %q to still render, got:\n%s", lastRow.Name, content)
	}
}

func TestConstraintsBanner_EmptyWhenNothingSkippedOrFailed(t *testing.T) {
	if out := constraintsBanner(nil, nil, 120); out != "" {
		t.Errorf("expected an empty banner, got %q", out)
	}
}
