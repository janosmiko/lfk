package ui

// LogMaxLines clamps for the live log-viewer buffer. The default of 50000 is
// generous for inspecting a busy stream while bounding per-tab memory; the
// floor stops a typo from making the viewer near-useless; the ceiling caps a
// single tab's retained log lines. Oldest lines are dropped past the cap.
const (
	LogMaxLinesDefault = 50_000
	LogMaxLinesMin     = 1_000
	LogMaxLinesMax     = 1_000_000
)

// ConfigLogMaxLines bounds the number of streamed log lines retained per tab
// in the log viewer; once exceeded, the oldest lines are dropped so a
// long-running follow does not grow memory without bound. Set via the
// `log_viewer.max_lines:` config key.
var ConfigLogMaxLines = LogMaxLinesDefault
