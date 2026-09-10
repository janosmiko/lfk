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

func TestUpdateConstraintsLoaded_PartialReportShowsBannerAndRows(t *testing.T) {
	m := basePush80Model()
	m.mode = modeConstraints
	m.constraints.loading = true
	m.constraints.title = "web-1"
	msg := constraintsLoadedMsg{
		gen: m.constraints.gen,
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
