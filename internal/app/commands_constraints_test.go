package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
)

func TestLoadConstraints_SchedulesOneCall(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.scheduler.StopWorkers()
	m.constraints.target = k8s.ConstraintTarget{Namespace: "default"}

	assertSchedulesOne(t, m, m.loadConstraints("pod-1"))
}

func TestUpdateConstraintsLoaded_RejectsAnotherTabsResult(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.tabs = []TabState{m.cloneCurrentTab()}
	m.activeTab = 0
	m.constraints.loading = true

	msg := constraintsLoadedMsg{
		gen:    m.constraints.gen,
		tabUID: m.currentTabUID() + 1,
		report: k8s.ConstraintReport{
			Rows: []k8s.ConstraintRow{{Source: "Quota", Name: "from-other-tab"}},
		},
	}

	result, _ := m.updateConstraintsLoaded(msg)
	updated, ok := result.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", result)
	}
	if len(updated.constraints.report.Rows) != 0 {
		t.Errorf("a same-generation result from another tab must not apply, got %+v", updated.constraints.report.Rows)
	}
	if !updated.constraints.loading {
		t.Error("the pending load must stay pending when a foreign result is dropped")
	}
}

func TestUpdateConstraintsLoaded_ClampsScrollToTheRefreshedReport(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.constraints.scroll = 40
	m.constraints.cursor = 40

	msg := constraintsLoadedMsg{
		gen:    m.constraints.gen,
		tabUID: m.currentTabUID(),
		report: k8s.ConstraintReport{
			Rows: []k8s.ConstraintRow{{Source: "Quota", Name: "only-row"}},
		},
	}

	result, _ := m.updateConstraintsLoaded(msg)
	updated, ok := result.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", result)
	}
	if updated.constraints.scroll != 0 {
		t.Errorf("scroll = %d, want 0 for a one-row report", updated.constraints.scroll)
	}
	// A scroll past the last row makes the renderer's row count negative.
	if out := stripANSI(updated.View().Content); !strings.Contains(out, "only-row") {
		t.Errorf("expected the surviving row to render, got:\n%s", out)
	}
}

func TestUpdateConstraintsLoaded_PartialReportShowsBannerAndRows(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.constraints.loading = true
	m.constraints.title = "web-1"
	msg := constraintsLoadedMsg{
		gen:    m.constraints.gen,
		tabUID: m.currentTabUID(),
		report: k8s.ConstraintReport{
			Rows: []k8s.ConstraintRow{{
				Source: "Quota", Kind: "ResourceQuota", Namespace: "default", Name: "compute-quota",
				Detail: "requests 500m cpu", Headroom: "1",
			}},
			Skipped: []string{"poddisruptionbudgets"},
		},
	}

	result, _ := m.updateConstraintsLoaded(msg)
	updated, ok := result.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", result)
	}

	out := stripANSI(updated.View().Content)
	if !strings.Contains(out, "poddisruptionbudgets") {
		t.Errorf("expected the banner to name the skipped source, got:\n%s", out)
	}
	if !strings.Contains(out, "compute-quota") {
		t.Errorf("expected the surviving row to still render, got:\n%s", out)
	}
}
