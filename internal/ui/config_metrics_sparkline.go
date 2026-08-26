package ui

import (
	"fmt"
	"time"
)

// Sparkline column bounds. A sparkline narrower than MinSparklineWidth reads
// as noise rather than a trend, and the CPU/MEM columns cap at 40 characters
// in fullscreen anyway (see fitExtraColumns), so a wider one cannot be drawn.
const (
	MinSparklineWidth = 4
	MaxSparklineWidth = 40
	// DefaultSparklineWidth leaves room for the value beside the sparkline
	// inside the 20-character non-fullscreen column cap.
	DefaultSparklineWidth = 12
)

// Sparkline range-fetch bounds. A range query costs more than an instant one
// because Prometheus reads a whole window per series, so the floor is higher
// than MinMetricsInterval.
const (
	DefaultSparklineInterval = 30 * time.Second
	MinSparklineInterval     = 10 * time.Second
)

// DefaultSparklineWindows is the hotkey cycle order.
var DefaultSparklineWindows = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour}

// Sparkline runtime config, set from YAML by applySparklineConfig.
var (
	// ConfigSparklineWindows is the ordered list the cycle hotkey walks.
	ConfigSparklineWindows = DefaultSparklineWindows
	// ConfigSparklineWidth is the preferred glyph count. The rendered width
	// is still bounded by the space the column has left after the value.
	ConfigSparklineWidth = DefaultSparklineWidth
	// ConfigSparklineInterval is the minimum gap between two range fetches
	// on a watch-tick refresh. 0 disables the throttle.
	ConfigSparklineInterval = DefaultSparklineInterval
)

// MetricsDisplayMode selects how the CPU and MEM columns render.
type MetricsDisplayMode int

const (
	// MetricsDisplayNumeric is the default: the value alone, with the trend
	// arrow the metrics refresh may prepend.
	MetricsDisplayNumeric MetricsDisplayMode = iota
	// MetricsDisplaySpark renders a sparkline over Window() followed by the
	// current value.
	MetricsDisplaySpark
)

// MetricsSparkState is the CPU/MEM column display state. It is a mode plus an
// index into ConfigSparklineWindows rather than one constant per window, so
// the cycle length follows the user's configured list.
type MetricsSparkState struct {
	Mode      MetricsDisplayMode
	WindowIdx int
}

// Window returns the range this state queries, or 0 in numeric mode and for
// an index that no longer exists, which happens when a restored session
// outlives a shortened window list.
func (s MetricsSparkState) Window() time.Duration {
	if s.Mode != MetricsDisplaySpark {
		return 0
	}
	if s.WindowIdx < 0 || s.WindowIdx >= len(ConfigSparklineWindows) {
		return 0
	}
	return ConfigSparklineWindows[s.WindowIdx]
}

// Label names the state for the status bar and the hint bar.
func (s MetricsSparkState) Label() string {
	w := s.Window()
	if w == 0 {
		return "numeric"
	}
	return fmt.Sprintf("sparkline %s", formatSparklineWindow(w))
}

// Next advances the cycle: numeric, then each configured window in order,
// then numeric again. An empty window list keeps the state numeric so the
// hotkey cannot select a mode with no window to query.
func (s MetricsSparkState) Next() MetricsSparkState {
	if len(ConfigSparklineWindows) == 0 {
		return MetricsSparkState{}
	}
	if s.Mode != MetricsDisplaySpark {
		return MetricsSparkState{Mode: MetricsDisplaySpark, WindowIdx: 0}
	}
	if s.WindowIdx+1 >= len(ConfigSparklineWindows) {
		return MetricsSparkState{}
	}
	return MetricsSparkState{Mode: MetricsDisplaySpark, WindowIdx: s.WindowIdx + 1}
}

// formatSparklineWindow prints a duration the way a user wrote it in config:
// "5m" and "1h" rather than Go's "5m0s" and "1h0m0s".
func formatSparklineWindow(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return d.String()
	}
}

// ClampSparklineWidth restricts n to [MinSparklineWidth, MaxSparklineWidth].
func ClampSparklineWidth(n int) int {
	if n < MinSparklineWidth {
		return MinSparklineWidth
	}
	if n > MaxSparklineWidth {
		return MaxSparklineWidth
	}
	return n
}

// ClampSparklineInterval restricts d to [MinSparklineInterval,
// MaxWatchInterval]. A non-positive d means "no throttle" and passes through
// as 0, matching ClampMetricsInterval.
func ClampSparklineInterval(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	if d < MinSparklineInterval {
		return MinSparklineInterval
	}
	if d > MaxWatchInterval {
		return MaxWatchInterval
	}
	return d
}

// applySparklineConfig wires the three YAML keys into their runtime globals.
// Each global resets to its default first, so a reload that removes or
// corrupts a key falls back instead of keeping the previous value.
func applySparklineConfig(windows []string, width *int, interval string) {
	ConfigSparklineWindows = DefaultSparklineWindows
	ConfigSparklineWidth = DefaultSparklineWidth
	ConfigSparklineInterval = DefaultSparklineInterval

	if len(windows) > 0 {
		parsed := make([]time.Duration, 0, len(windows))
		for _, raw := range windows {
			d, err := time.ParseDuration(raw)
			if err != nil || d <= 0 {
				continue // one typo must not disable the whole cycle
			}
			parsed = append(parsed, d)
		}
		// An all-unparseable list keeps the defaults: an empty list would
		// make the hotkey silently do nothing.
		if len(parsed) > 0 {
			ConfigSparklineWindows = parsed
		}
	}
	if width != nil {
		ConfigSparklineWidth = ClampSparklineWidth(*width)
	}
	if interval != "" {
		if d, err := time.ParseDuration(interval); err == nil {
			ConfigSparklineInterval = ClampSparklineInterval(d)
		}
	}
}

// MetricsSparkStartupState is the display mode a new tab opens with. It is the
// seed ApplyViewerPrefs writes and NewModel reads, so a remembered keypress
// survives a restart without becoming a user-authored config key.
var MetricsSparkStartupState MetricsSparkState

// ResolveMetricsSparkState turns a persisted window duration back into a state.
// It matches against the CONFIGURED windows rather than trusting a stored
// index, so shortening metrics_sparkline_windows cannot silently select a
// different window than the user chose. Anything unrecognised, including an
// empty string, resolves to numeric.
func ResolveMetricsSparkState(window string) MetricsSparkState {
	if window == "" {
		return MetricsSparkState{}
	}
	d, err := time.ParseDuration(window)
	if err != nil {
		return MetricsSparkState{}
	}
	for i, w := range ConfigSparklineWindows {
		if w == d {
			return MetricsSparkState{Mode: MetricsDisplaySpark, WindowIdx: i}
		}
	}
	return MetricsSparkState{}
}

// PersistedWindow renders the state for the prefs file: the chosen window's
// duration, or the empty string for numeric. Empty is EXPLICIT numeric, which
// the caller must keep distinct from an absent field meaning never chosen.
func (s MetricsSparkState) PersistedWindow() string {
	if w := s.Window(); w > 0 {
		return w.String()
	}
	return ""
}
