package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func makeFixture(source string, containers []model.ContainerRec) *model.Rightsizing {
	return &model.Rightsizing{Source: source, PodCount: 2, Containers: containers}
}

func TestRenderRightsizingOverlay_VPASource(t *testing.T) {
	data := makeFixture("VPA", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{Usage: "80m", CurrentRequest: "100m", CurrentLimit: "500m", RecommendedRequest: "60m", RecommendedLimit: "300m", LowerBound: "50m", UpperBound: "250m"},
		Mem:  model.ResourceRec{Usage: "180Mi", CurrentRequest: "256Mi", CurrentLimit: "512Mi", RecommendedRequest: "200Mi", RecommendedLimit: "400Mi"},
	}})
	// 200x40 reflects realistic terminal sizes — at narrower widths the
	// 2-row grouped header (11 sub-cells for VPA) would have to either
	// abbreviate sub-labels or drop the BOUNDS group; 200 leaves room
	// for the full "CURRENT" / "SUGGESTION" / "Δ" labels.
	out := RenderRightsizingOverlay(data, false, nil, 0, 200, 40)
	stripped := ansi.Strip(out)
	assert.Contains(t, out, "Strategy: VPA")
	assert.Contains(t, out, "Pods aggregated: 2")
	assert.Contains(t, out, "app")
	// Group headers (top row) and sub-headers (bottom row) coexist:
	// REQUEST/LIMIT/BOUNDS span CURRENT/SUGGESTION/Δ sub-columns. The
	// previous "inline arrow" cell shape was replaced by a real
	// 2-level grouped layout.
	assert.Contains(t, stripped, "REQUEST", "REQUEST group header")
	assert.Contains(t, stripped, "LIMIT", "LIMIT group header")
	assert.Contains(t, stripped, "BOUNDS", "BOUNDS group header")
	assert.Contains(t, stripped, "USAGE", "USAGE sub-header")
	assert.Contains(t, stripped, "CURRENT", "CURRENT sub-header")
	assert.Contains(t, stripped, "SUGGESTION", "SUGGESTION sub-header")
	assert.Contains(t, stripped, "Δ", "Δ sub-header")
	assert.Contains(t, stripped, "80m", "actual CPU usage value rendered in USAGE cell")
	assert.Contains(t, stripped, "180Mi", "actual memory usage value rendered in USAGE cell")
	assert.Contains(t, stripped, "100m", "current request value")
	assert.Contains(t, stripped, "60m", "recommended request value")
	// VPA bounds appear as separate LOWER / UPPER cells now (not the
	// old "[low, high]" string), so check for both raw values.
	assert.Contains(t, stripped, "50m", "VPA lower bound rendered")
	assert.Contains(t, stripped, "250m", "VPA upper bound rendered")
}

func TestRenderRightsizingOverlay_TransitionCellsCollapseWhenEqual(t *testing.T) {
	// When current equals recommended, the Δ column shows "=" instead
	// of a percentage so the no-change case reads as a quiet
	// confirmation. Both CURRENT and SUGGESTION still render the value
	// — the user sees "100m │ 100m │ =" — minimal noise for the common
	// case where VPA confirms the spec is already right-sized.
	data := makeFixture("VPA", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{Usage: "80m", CurrentRequest: "100m", CurrentLimit: "500m", RecommendedRequest: "100m", RecommendedLimit: "500m"},
	}})
	out := RenderRightsizingOverlay(data, false, nil, 0, 200, 40)
	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "100m")
	assert.Contains(t, stripped, "500m")
	// The Δ column collapses to "=" when current == recommended.
	assert.Contains(t, stripped, "=", "no-change Δ collapses to '='")
	// And the old inline-arrow shape is gone — there's no "→" anywhere
	// since CURRENT/SUGGESTION are now separate cells.
	assert.NotContains(t, stripped, "→", "no inline arrow in 2-row grouped layout")
}

func TestRenderRightsizingOverlay_EstimatedSourceHidesBounds(t *testing.T) {
	data := makeFixture("estimated", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "120m"},
	}})
	out := RenderRightsizingOverlay(data, false, nil, 0, 120, 30)
	stripped := ansi.Strip(out)
	assert.Contains(t, out, "Strategy: estimated")
	// estimated source has no LowerBound/UpperBound for any container,
	// so the BOUNDS group is dropped to free up width for the variable
	// REQUEST and LIMIT columns.
	assert.NotContains(t, stripped, "BOUNDS", "BOUNDS group header dropped when source != VPA")
	assert.NotContains(t, stripped, "LOWER", "LOWER sub-header absent without BOUNDS")
	assert.NotContains(t, stripped, "UPPER", "UPPER sub-header absent without BOUNDS")
}

func TestRenderRightsizingOverlay_EmptyUsageRendersDash(t *testing.T) {
	// Regression: when a container had no live metrics, the USAGE cell
	// rendered blank (instead of a dim em-dash) AND broke row alignment
	// because `orDash` returned an ANSI-styled `—` whose escape bytes
	// counted as runes — `rsTruncate` cut mid-escape and the visible `—`
	// never reached the screen. Now the empty-cell handling lives at the
	// cell-renderer level (`valueCellOrDash`) so content stays plain.
	data := makeFixture("estimated", []model.ContainerRec{{
		Name: "split-brain-fix",
		CPU:  model.ResourceRec{}, // no usage at all
		Mem:  model.ResourceRec{Usage: "2Mi", RecommendedRequest: "2Mi"},
	}})
	stripped := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 240, 30))
	cpuLine := ""
	for line := range strings.SplitSeq(stripped, "\n") {
		if strings.Contains(line, "split-brain-fix") && strings.Contains(line, "cpu") {
			cpuLine = line
			break
		}
	}
	assert.NotEmpty(t, cpuLine, "expected the cpu data row for split-brain-fix")
	// The USAGE cell sits between RES (`cpu`) and the REQUEST group's
	// first `│`. Verify a `—` lives in there.
	iCpu := strings.Index(cpuLine, "cpu")
	assert.Greater(t, iCpu, 0)
	tail := cpuLine[iCpu:]
	iSep := strings.Index(tail, "│")
	require.Greater(t, iSep, 0, "expected a `│` after the cpu cell")
	tail = tail[iSep+len("│"):] // skip the cpu→USAGE separator
	iSep = strings.Index(tail, "│")
	require.Greater(t, iSep, 0, "expected a `│` after the USAGE cell")
	usage := tail[:iSep]
	assert.Contains(t, usage, "—", "empty USAGE should render as a dim em-dash, not blank")
}

func TestRenderRightsizingOverlay_TableFillsPanelWidth(t *testing.T) {
	// The table previously sized to its content, leaving a sparse band
	// of whitespace on the right. The grouped 2-row layout distributes
	// the panel width across columns explicitly, so the sub-header row
	// (the most reliable width signal — pure chrome with `│` between
	// every cell) should fill most of the inner-panel content area.
	data := makeFixture("VPA", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{Usage: "80m", CurrentRequest: "100m", CurrentLimit: "500m", RecommendedRequest: "60m", RecommendedLimit: "300m", LowerBound: "50m", UpperBound: "250m"},
		Mem:  model.ResourceRec{Usage: "180Mi", CurrentRequest: "256Mi", CurrentLimit: "512Mi", RecommendedRequest: "200Mi", RecommendedLimit: "400Mi"},
	}})
	out := RenderRightsizingOverlay(data, false, nil, 0, 200, 40)
	stripped := ansi.Strip(out)
	// Use the mid-rule row ("─┼─") as the width signal — pure table
	// chrome, no trailing-whitespace ambiguity. At screenW=200 the
	// inner-panel content area is ~140 wide; the mid-rule should fill
	// most of it.
	tableWidth := 0
	for line := range strings.SplitSeq(stripped, "\n") {
		if strings.Contains(line, "─┼─") {
			trimmed := strings.Trim(line, " │")
			w := len([]rune(trimmed))
			if w > tableWidth {
				tableWidth = w
			}
		}
	}
	assert.GreaterOrEqual(t, tableWidth, 130,
		"table should fill most of the panel width (got %d)", tableWidth)
}

func TestRenderRightsizingOverlay_HeaderShowsMethodology(t *testing.T) {
	// VPA path — header should hint at history-based VPA recommender.
	vpa := makeFixture("VPA", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "60m"},
	}})
	outVPA := ansi.Strip(RenderRightsizingOverlay(vpa, false, nil, 0, 120, 30))
	assert.Contains(t, strings.ToLower(outVPA), "history",
		"VPA methodology hint not present in header (looking for 'history')")

	// estimated path — header should hint at headroom factor.
	est := makeFixture("estimated", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "120m"},
	}})
	outEst := ansi.Strip(RenderRightsizingOverlay(est, false, nil, 0, 120, 30))
	assert.Contains(t, strings.ToLower(outEst), "headroom",
		"estimated methodology hint not present in header (looking for 'headroom')")
}

func TestRenderRightsizingOverlay_HeaderShowsWindowWhenSet(t *testing.T) {
	// When metrics-server window is plumbed through, header includes it
	// so the user knows the snapshot duration backing the recommendation.
	data := makeFixture("estimated", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{Usage: "10m", CurrentRequest: "100m", RecommendedRequest: "12m"},
	}})
	data.Window = "30s"
	out := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 120, 30))
	assert.Contains(t, out, "30s", "window value should appear in header when set")
}

func TestRenderRightsizingOverlay_GroupHeadersCenter(t *testing.T) {
	// REQUEST and LIMIT (and BOUNDS for VPA) appear in the top header
	// row centered above their respective sub-column spans. Verify by
	// (a) presence and (b) that they sit on the SAME line (a single
	// group-header row, not interleaved with sub-headers or data).
	data := makeFixture("VPA", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{Usage: "80m", CurrentRequest: "100m", CurrentLimit: "500m", RecommendedRequest: "60m", RecommendedLimit: "300m", LowerBound: "50m", UpperBound: "250m"},
	}})
	stripped := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 200, 40))
	groupLine := ""
	for line := range strings.SplitSeq(stripped, "\n") {
		if strings.Contains(line, "REQUEST") && strings.Contains(line, "LIMIT") {
			groupLine = line
			break
		}
	}
	assert.NotEmpty(t, groupLine, "REQUEST and LIMIT should share one group-header line")
	assert.Contains(t, groupLine, "BOUNDS", "BOUNDS should sit on the same group line for VPA")
	// Group-header line must NOT contain the sub-column verticals — the
	// whole point of the top row is that spans render without internal
	// `│` separators. Trim panel chrome (` `, `│`, `╭`, `─` etc) before
	// asserting so the surrounding box doesn't false-positive.
	inner := strings.Trim(groupLine, " │╭╮")
	assert.NotContains(t, inner, "│", "group-header row has no internal `│` separators (spans are seamless)")

	// Corner brackets demarcate each group span. The first group opens
	// with `┌`, adjacent groups are separated by `┬`. With 3 groups
	// (REQUEST, LIMIT, BOUNDS) we expect exactly 1 `┌` and 2 `┬`.
	assert.Contains(t, groupLine, "┌", "first group opens with `┌` corner")
	tee := strings.Count(groupLine, "┬")
	assert.Equal(t, 2, tee, "two `┬` separators between REQUEST/LIMIT and LIMIT/BOUNDS")
	assert.Contains(t, groupLine, "─", "group spans use `─` filler around the name")
}

func TestRenderRightsizingOverlay_DeltaColumnIsSeparate(t *testing.T) {
	// The Δ percentage is its own cell, separated from SUGGESTION by
	// `│`. Verify by finding the data row and asserting that "60m"
	// (the suggestion) and "-40%" (the delta) are NOT adjacent — there
	// is at least one `│` between them.
	data := makeFixture("VPA", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "60m"},
	}})
	stripped := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 200, 40))
	dataLine := ""
	for line := range strings.SplitSeq(stripped, "\n") {
		if strings.Contains(line, "app") && strings.Contains(line, "cpu") {
			dataLine = line
			break
		}
	}
	assert.NotEmpty(t, dataLine, "expected an 'app cpu' data row in the rendered table")
	// Find the suggestion's position and the delta's position; confirm
	// at least one `│` sits between them.
	iSugg := strings.Index(dataLine, "60m")
	iDelta := strings.Index(dataLine, "-40%")
	assert.Greater(t, iDelta, iSugg, "delta should follow suggestion in the same row")
	between := dataLine[iSugg:iDelta]
	assert.Contains(t, between, "│",
		"a `│` separator must sit between the SUGGESTION and Δ cells (got: %q)", between)
}

func TestRenderRightsizingOverlay_LoadingState(t *testing.T) {
	out := RenderRightsizingOverlay(nil, true, nil, 0, 120, 30)
	assert.Contains(t, out, "Computing right-sizing")
}

func TestRenderRightsizingOverlay_LoadingWithStaleDataShowsTable(t *testing.T) {
	// loading=true + data != nil is the "fetching new strategy" state.
	// The renderer keeps the existing table on screen (visual
	// continuity) and adds a subtle "Loading" hint in the header
	// instead of wiping to the centered "Computing right-sizing…".
	data := makeFixture("VPA", []model.ContainerRec{{
		Name: "frontend",
		CPU:  model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "60m"},
	}})
	out := RenderRightsizingOverlay(data, true, nil, 0, 200, 40)
	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "frontend", "container row from stale data must still render")
	assert.NotContains(t, stripped, "Computing right-sizing",
		"loading-with-stale-data must NOT show the centered cold-load message")
	assert.Contains(t, strings.ToLower(stripped), "loading",
		"header must signal a fetch-in-progress when loading=true && data != nil")
}

func TestRenderRightsizingOverlay_LoadingWithoutDataShowsCenteredMessage(t *testing.T) {
	// loading=true + data == nil keeps the original cold-load behavior:
	// centered "Computing right-sizing…" message takes over the body.
	out := RenderRightsizingOverlay(nil, true, nil, 0, 120, 30)
	assert.Contains(t, out, "Computing right-sizing",
		"cold load (no data yet) still shows the centered placeholder")
}

func TestRenderRightsizingOverlay_ErrorState(t *testing.T) {
	out := RenderRightsizingOverlay(nil, false, assertErr("boom"), 0, 120, 30)
	assert.Contains(t, out, "boom")
}

func TestRenderRightsizingOverlay_EmptyState(t *testing.T) {
	data := makeFixture("estimated", nil)
	data.PodCount = 0
	out := RenderRightsizingOverlay(data, false, nil, 0, 120, 30)
	assert.Contains(t, strings.ToLower(out), "no")
}

func TestRenderRightsizingOverlay_NoRecommendationShowsCurrentOnly(t *testing.T) {
	// Container has spec values but no live metrics + no VPA target →
	// CURRENT cell shows the current value, SUGGESTION shows "—", Δ
	// shows "—" (no comparison possible). The USAGE column also shows
	// "—" since there's no live data.
	data := makeFixture("VPA", []model.ContainerRec{{
		Name: "app",
		CPU:  model.ResourceRec{CurrentRequest: "100m"}, // no RecommendedRequest, no Usage
		Mem:  model.ResourceRec{CurrentRequest: "256Mi"},
	}})
	out := RenderRightsizingOverlay(data, false, nil, 0, 200, 40)
	stripped := ansi.Strip(out)
	assert.Contains(t, stripped, "100m", "current value still surfaced even without recommendation")
	assert.Contains(t, stripped, "256Mi")
	// SUGGESTION cell is "—" when no recommendation; the row must
	// contain at least one em-dash placeholder for the missing data.
	assert.Contains(t, stripped, "—", "missing suggestion → em-dash placeholder")
	// And the old inline-arrow shape is gone.
	assert.NotContains(t, stripped, "→", "no inline arrow in 2-row grouped layout")
}

// --- Strategy in header ---

func TestRenderRightsizingOverlay_StrategyInHeader(t *testing.T) {
	// New header layout shows the active strategy + position in the
	// available list + the methodology hint so the user knows which
	// algorithm produced the numbers they're staring at.
	data := &model.Rightsizing{
		Source:   "1d-max",
		Strategy: model.StrategyPromMax1D,
		AvailableStrategies: []model.RightsizingStrategy{
			model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot,
		},
		PodCount: 3,
		Window:   "1d",
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "60m"},
		}},
	}
	out := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 200, 40))
	assert.Contains(t, out, "1d-max", "header must show the active strategy's human label")
	assert.Contains(t, out, "[2/3]", "header must show position 2 of 3 available strategies")
	assert.Contains(t, out, "Pods aggregated: 3")
	// The methodology hint should mention the 1d window + max aggregation.
	assert.Contains(t, out, "1d")
}

func TestRenderRightsizingOverlay_HeaderHidesPickerChipWhenSingleStrategy(t *testing.T) {
	// When only one strategy is available, the [N/M] chip is noise —
	// `[/]` keys won't do anything. Drop the chip in that case.
	data := &model.Rightsizing{
		Source:              "snapshot",
		Strategy:            model.StrategySnapshot,
		AvailableStrategies: []model.RightsizingStrategy{model.StrategySnapshot},
		PodCount:            1,
		Containers:          []model.ContainerRec{{Name: "app", CPU: model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "120m"}}},
	}
	out := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 200, 40))
	assert.Contains(t, out, "snapshot")
	assert.NotContains(t, out, "[1/1]", "single-strategy header should not advertise a picker chip")
}

// --- Headroom in header ---

func TestRenderRightsizingOverlay_HeaderShowsHeadroom(t *testing.T) {
	// New header layout shows the active headroom + position in the
	// preset list ([N/M]) so the user knows what multiplier produced
	// the displayed numbers and where they are in the </> cycle.
	data := &model.Rightsizing{
		Source:              "1d-max",
		Strategy:            model.StrategyPromMax1D,
		AvailableStrategies: []model.RightsizingStrategy{model.StrategyVPA, model.StrategyPromMax1D, model.StrategySnapshot},
		Headroom:            1.25,
		PodCount:            3,
		Window:              "1d",
		Containers: []model.ContainerRec{{
			Name: "app",
			CPU:  model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "60m"},
		}},
	}
	out := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 200, 40))
	assert.Contains(t, out, "Headroom:", "header must label the headroom field")
	assert.Contains(t, out, "1.25x", "header must show the active headroom value with x suffix")
	assert.Contains(t, out, "[3/6]", "header must show position 3 of 6 preset headrooms (1.25 is index 2 → 3/6)")
	// The methodology line should now embed the headroom value.
	assert.Contains(t, out, "x 1.25 headroom", "methodology hint must append the active headroom multiplier")
}

func TestRenderRightsizingOverlay_HeaderHeadroomChipAlwaysShown(t *testing.T) {
	// Unlike the strategy chip (hidden when single-strategy), the
	// headroom chip is always shown — model.RightsizingHeadrooms always
	// has 6 entries, so </> always has somewhere to go.
	data := &model.Rightsizing{
		Source:              "snapshot",
		Strategy:            model.StrategySnapshot,
		AvailableStrategies: []model.RightsizingStrategy{model.StrategySnapshot},
		Headroom:            2.0,
		PodCount:            1,
		Containers:          []model.ContainerRec{{Name: "app", CPU: model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "120m"}}},
	}
	out := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 200, 40))
	assert.Contains(t, out, "Headroom:")
	assert.Contains(t, out, "2x", "%g formatting renders 2.0 as '2'")
	assert.Contains(t, out, "[6/6]", "2.0 is the last preset → 6/6")
}

func TestRenderRightsizingOverlay_VPAMethodologyAt1RawNote(t *testing.T) {
	// When VPA is the active strategy AND headroom == 1.0, the
	// methodology hint should signal "raw" so the user understands the
	// numbers are the recommender's untouched output (no padding).
	data := &model.Rightsizing{
		Source:              "VPA",
		Strategy:            model.StrategyVPA,
		AvailableStrategies: []model.RightsizingStrategy{model.StrategyVPA, model.StrategySnapshot},
		Headroom:            1.0,
		PodCount:            1,
		Containers:          []model.ContainerRec{{Name: "app", CPU: model.ResourceRec{CurrentRequest: "100m", RecommendedRequest: "60m"}}},
	}
	out := ansi.Strip(RenderRightsizingOverlay(data, false, nil, 0, 200, 40))
	assert.Contains(t, strings.ToLower(out), "raw",
		"VPA at headroom 1.0 should mention 'raw' in the methodology hint")
}

type assertErr string

func (a assertErr) Error() string { return string(a) }
