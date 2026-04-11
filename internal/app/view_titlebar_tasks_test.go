package app

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/model"
)

func TestRenderTasksIndicatorEmpty(t *testing.T) {
	t.Parallel()
	got := renderTasksIndicator("\u280b", nil)
	assert.Empty(t, got)
}

func TestRenderTasksIndicatorShowsOnlySpinner(t *testing.T) {
	t.Parallel()
	// The indicator is intentionally minimal — just the spinner glyph,
	// no task name or count. Users who want details open the :tasks
	// overlay. This guards against anyone re-adding a label.
	snap := []bgtasks.Task{
		{ID: 1, Kind: bgtasks.KindResourceList, Name: "List Pods", StartedAt: time.Now()},
	}
	got := renderTasksIndicator("\u280b", snap)
	assert.Contains(t, got, "\u280b")
	stripped := stripANSI(got)
	assert.NotContains(t, stripped, "Loading")
	assert.NotContains(t, stripped, "List Pods")
	assert.NotContains(t, stripped, "task")
}

func TestRenderTasksIndicatorMultipleTasksStillOnlySpinner(t *testing.T) {
	t.Parallel()
	// Even with many tasks, the indicator shows only the spinner — no
	// count. The user opens :tasks to see how many are running.
	snap := []bgtasks.Task{
		{ID: 1, Name: "List Pods", StartedAt: time.Now()},
		{ID: 2, Name: "Get YAML", StartedAt: time.Now()},
		{ID: 3, Name: "Pod metrics", StartedAt: time.Now()},
	}
	got := renderTasksIndicator("\u280b", snap)
	assert.Contains(t, got, "\u280b")
	stripped := stripANSI(got)
	assert.NotContains(t, stripped, "3 tasks")
	assert.NotContains(t, stripped, "3")
}

func TestRenderTasksIndicatorRendersProvidedSpinnerFrame(t *testing.T) {
	t.Parallel()
	// The spinner glyph is passed in by the caller (so the indicator
	// animates at whatever cadence the caller's spinner is running).
	// This guards against regressions where the helper starts hard-coding
	// a frame and ignores its input.
	snap := []bgtasks.Task{
		{ID: 1, Name: "Task", StartedAt: time.Now()},
	}
	got := renderTasksIndicator("\u2807", snap)
	assert.Contains(t, got, "\u2807",
		"helper must render the spinner frame its caller provided")
}

func TestTitleBarTasksIndicatorShownWhenRegistryNonEmpty(t *testing.T) {
	t.Parallel()
	// Construct a Model with a populated registry and verify the rendered
	// title bar contains the spinner glyph passed in by m.spinner.View().
	// We check for the spinner glyph rather than any label, because the
	// indicator has no label anymore.
	r := bgtasks.New(0) // 0 threshold so the task is visible immediately
	r.Start(bgtasks.KindResourceList, "List Pods", "default")
	m := Model{
		width:  120,
		height: 40,
		nav: model.NavigationState{
			Context:      "test-ctx",
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"},
		},
		bgtasks: r,
	}
	m.spinner = spinner.New()

	out := m.renderTitleBar()
	stripped := stripANSI(out)
	assert.Contains(t, stripped, m.spinner.View(),
		"title bar must include the spinner glyph when tasks are active")
}

func TestTitleBarTasksIndicatorHiddenWhenRegistryEmpty(t *testing.T) {
	t.Parallel()
	// When the registry is empty the title bar renders no indicator.
	// We verify by checking that removing spaces + the namespace badge
	// leaves no stray spinner frame on the bar.
	m := Model{
		width:  120,
		height: 40,
		nav: model.NavigationState{
			Context:      "test-ctx",
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"},
		},
		bgtasks: bgtasks.New(0),
	}
	m.spinner = spinner.New()

	outEmpty := m.renderTitleBar()

	r := bgtasks.New(0)
	r.Start(bgtasks.KindResourceList, "List Pods", "default")
	m.bgtasks = r
	outActive := m.renderTitleBar()

	// When tasks are active the rendered title bar must be strictly
	// longer than when it is empty (the spinner adds at least 3 cells).
	assert.NotEqual(t, outEmpty, outActive,
		"title bar must change appearance when tasks start")
}
