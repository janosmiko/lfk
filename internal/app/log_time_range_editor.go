package app

import (
	"time"
)

// logTimeRangeMode selects which sub-mode a Start / End editor panel is
// rendering. "Now" is the zero/inactive endpoint; "Relative" builds a
// negative offset from three d/h/m spinners; "Absolute" (Phase 3) shows
// a 6-field datetime editor.
type logTimeRangeMode int

const (
	logTimeRangeModeNow logTimeRangeMode = iota
	logTimeRangeModeRelative
	logTimeRangeModeAbsolute
)

// logTimeRangeRelField identifies which of the three spinner fields
// (Days, Hours, Minutes) is currently focused for keyboard input.
type logTimeRangeRelField int

const (
	logTimeRangeRelFieldDays logTimeRangeRelField = iota
	logTimeRangeRelFieldHours
	logTimeRangeRelFieldMinutes
)

// logTimeRangeEditor captures the editable state of one endpoint in the
// picker. It is kept small by design — the panels are cheap enough to
// re-render each frame, so this struct never stores derived display
// text.
//
// Days/Hours/Minutes are non-negative. The editor always interprets the
// resulting duration as negative (i.e. "Days days ago"). A positive
// offset would rarely be useful for an End bound and the UX prompt says
// "ago" under each field, so positive relative offsets are intentionally
// unreachable from the editor.
type logTimeRangeEditor struct {
	Mode    logTimeRangeMode
	RelDays int
	RelHrs  int
	RelMin  int
	// RelField tracks which spinner field has the column cursor.
	RelField logTimeRangeRelField
	// AbsTime is the working value for absolute mode (Phase 3).
	AbsTime time.Time
	// AbsField tracks which of the 6 datetime fields has the column
	// cursor in absolute mode (Phase 3).
	AbsField logTimeRangeAbsField
}

// logTimeRangeAbsField names the 6 two-to-four-digit cells in the
// absolute-datetime editor. Phase 3 wires this up; Phase 2 leaves
// AbsField pinned to its zero value.
type logTimeRangeAbsField int

// Phase 3 wires these in; declared here so the editor struct's
// AbsField carries a typed value in Phase 2's zero state.
const (
	logTimeRangeAbsFieldYear   logTimeRangeAbsField = iota //nolint:unused // Phase 3
	logTimeRangeAbsFieldMonth                              //nolint:unused // Phase 3
	logTimeRangeAbsFieldDay                                //nolint:unused // Phase 3
	logTimeRangeAbsFieldHour                               //nolint:unused // Phase 3
	logTimeRangeAbsFieldMinute                             //nolint:unused // Phase 3
	logTimeRangeAbsFieldSecond                             //nolint:unused // Phase 3
)

// toPoint collapses an editor into the corresponding LogTimePoint so
// the overlay can preview / commit without leaking editor state into
// the range model.
func (e logTimeRangeEditor) toPoint() LogTimePoint {
	switch e.Mode {
	case logTimeRangeModeNow:
		return LogTimePoint{}
	case logTimeRangeModeRelative:
		d := time.Duration(e.RelDays)*24*time.Hour +
			time.Duration(e.RelHrs)*time.Hour +
			time.Duration(e.RelMin)*time.Minute
		if d == 0 {
			// Zero offset = "now". Preserve the Mode so the editor
			// panel still reads as "Relative" while being semantically
			// equivalent to the zero endpoint.
			return LogTimePoint{}
		}
		return LogTimePoint{Kind: TimeRelative, Relative: -d}
	case logTimeRangeModeAbsolute:
		if e.AbsTime.IsZero() {
			return LogTimePoint{}
		}
		return LogTimePoint{Kind: TimeAbsolute, Absolute: e.AbsTime}
	}
	return LogTimePoint{}
}

// editorFromPoint reverses toPoint so opening the editor with an
// already-set endpoint re-hydrates the spinners/fields with the
// existing value. Defaults (when the point is zero) produce a
// sensible "24h ago" starting position in Relative mode so a fresh
// Custom… flow isn't locked at all-zeros.
func editorFromPoint(p LogTimePoint, defaultAbs time.Time) logTimeRangeEditor {
	if p.IsZero() {
		return logTimeRangeEditor{
			Mode:    logTimeRangeModeNow,
			RelHrs:  24, // a sensible preset when the user switches Mode=Relative
			AbsTime: defaultAbs,
		}
	}
	if p.Kind == TimeAbsolute {
		return logTimeRangeEditor{
			Mode:    logTimeRangeModeAbsolute,
			AbsTime: p.Absolute,
		}
	}
	// Relative. Split the magnitude into days/hours/minutes.
	d := p.Relative
	if d < 0 {
		d = -d
	}
	days := int(d / (24 * time.Hour))
	d -= time.Duration(days) * 24 * time.Hour
	hrs := int(d / time.Hour)
	d -= time.Duration(hrs) * time.Hour
	mins := int(d / time.Minute)
	return logTimeRangeEditor{
		Mode:    logTimeRangeModeRelative,
		RelDays: days,
		RelHrs:  hrs,
		RelMin:  mins,
		AbsTime: defaultAbs,
	}
}

// relFieldLeft / relFieldRight cycle between the three spinner fields.
// Movement clamps at the ends; it does not wrap, which mirrors how
// most terminal editors work and avoids the user accidentally jumping
// from Minutes back to Days on a single right-arrow.
func (e *logTimeRangeEditor) relFieldLeft() {
	if e.RelField > logTimeRangeRelFieldDays {
		e.RelField--
	}
}

func (e *logTimeRangeEditor) relFieldRight() {
	if e.RelField < logTimeRangeRelFieldMinutes {
		e.RelField++
	}
}

// relFieldAdjust nudges the active spinner by delta. Negative values
// never go below zero (see toPoint — negative values would flip the
// duration's sign, which the editor semantically forbids). Upper
// clamps: 999 days, 23h, 59m. 999d is a soft ceiling to keep the
// spinner column visually bounded; nobody actually wants a 3-year
// log window.
func (e *logTimeRangeEditor) relFieldAdjust(delta int) {
	switch e.RelField {
	case logTimeRangeRelFieldDays:
		e.RelDays = clampInt(e.RelDays+delta, 0, 999)
	case logTimeRangeRelFieldHours:
		e.RelHrs = clampInt(e.RelHrs+delta, 0, 23)
	case logTimeRangeRelFieldMinutes:
		e.RelMin = clampInt(e.RelMin+delta, 0, 59)
	}
}

// relFieldSetDigit replaces the active spinner value with the single
// digit typed by the user. This matches the spec: "typing digits
// overwrites". We use a single-digit overwrite rather than a
// multi-keystroke accumulator because the user can always follow up
// with j/k (or +/-) to fine-tune.
func (e *logTimeRangeEditor) relFieldSetDigit(d int) {
	if d < 0 || d > 9 {
		return
	}
	switch e.RelField {
	case logTimeRangeRelFieldDays:
		e.RelDays = d
	case logTimeRangeRelFieldHours:
		e.RelHrs = d
	case logTimeRangeRelFieldMinutes:
		e.RelMin = d
	}
}

// clampInt pins v into [lo, hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
