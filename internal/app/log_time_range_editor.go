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
// absolute-datetime editor.
type logTimeRangeAbsField int

const (
	logTimeRangeAbsFieldYear logTimeRangeAbsField = iota
	logTimeRangeAbsFieldMonth
	logTimeRangeAbsFieldDay
	logTimeRangeAbsFieldHour
	logTimeRangeAbsFieldMinute
	logTimeRangeAbsFieldSecond
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

// absFieldLeft / absFieldRight cycle between the six datetime cells.
// Movement clamps rather than wraps — matching the Relative editor's
// h/l behaviour.
func (e *logTimeRangeEditor) absFieldLeft() {
	if e.AbsField > logTimeRangeAbsFieldYear {
		e.AbsField--
	}
}

func (e *logTimeRangeEditor) absFieldRight() {
	if e.AbsField < logTimeRangeAbsFieldSecond {
		e.AbsField++
	}
}

// absFieldAdjust nudges the focused datetime cell by delta. Different
// fields use different semantics to match the spec:
//
//   - Date fields (Y/M/D) clamp in-range and re-clamp Day when
//     Year/Month change shrinks the month (e.g. switching from March
//     to February while Day==30 pins Day to the month's last day —
//     this is the "Feb 30 → Feb 28" case called out in the spec).
//   - Time fields (H/M/S) use wrap-with-carry via time.Date so "25:00"
//     rolls into the next day, mirroring the spec's "25:00 → 00:00
//     next day".
//
// The second return value is a short status string describing the
// clamp/carry event, or "" when the edit landed cleanly. Callers pipe
// this into setStatusMessage so the user knows their input was
// adjusted.
func (e *logTimeRangeEditor) absFieldAdjust(delta int) string {
	y, mo, d := e.AbsTime.Year(), int(e.AbsTime.Month()), e.AbsTime.Day()
	h, mn, s := e.AbsTime.Hour(), e.AbsTime.Minute(), e.AbsTime.Second()
	loc := e.AbsTime.Location()
	if loc == nil {
		loc = time.Local
	}
	switch e.AbsField {
	case logTimeRangeAbsFieldYear:
		newY := clampInt(y+delta, 1970, 9999)
		d, _ = clampDayInMonth(newY, mo, d)
		e.AbsTime = time.Date(newY, time.Month(mo), d, h, mn, s, 0, loc)
		return ""
	case logTimeRangeAbsFieldMonth:
		newMo := clampInt(mo+delta, 1, 12)
		var status string
		d, status = clampDayInMonth(y, newMo, d)
		e.AbsTime = time.Date(y, time.Month(newMo), d, h, mn, s, 0, loc)
		return status
	case logTimeRangeAbsFieldDay:
		newD := d + delta
		var status string
		newD, status = clampDayInMonth(y, mo, newD)
		e.AbsTime = time.Date(y, time.Month(mo), newD, h, mn, s, 0, loc)
		return status
	case logTimeRangeAbsFieldHour:
		// Wrap-with-carry: time.Date normalizes hour overflow into
		// the next day automatically.
		wrapped := time.Date(y, time.Month(mo), d, h+delta, mn, s, 0, loc)
		e.AbsTime = wrapped
		if wrapped.Hour() != h+delta && (h+delta >= 24 || h+delta < 0) {
			return "hour wrapped"
		}
		return ""
	case logTimeRangeAbsFieldMinute:
		wrapped := time.Date(y, time.Month(mo), d, h, mn+delta, s, 0, loc)
		e.AbsTime = wrapped
		if wrapped.Minute() != mn+delta && (mn+delta >= 60 || mn+delta < 0) {
			return "minute wrapped"
		}
		return ""
	case logTimeRangeAbsFieldSecond:
		wrapped := time.Date(y, time.Month(mo), d, h, mn, s+delta, 0, loc)
		e.AbsTime = wrapped
		if wrapped.Second() != s+delta && (s+delta >= 60 || s+delta < 0) {
			return "second wrapped"
		}
		return ""
	}
	return ""
}

// absFieldOverwriteDigit replaces the focused field with the digit the
// user typed. For Y/M/D/h/mn/s we accumulate into a multi-digit slot
// over multiple keystrokes is complex and error-prone; the editor
// opts for "typing a digit replaces the last digit", identical to the
// Relative editor's policy. Callers follow up with +/- for precise
// moves. Returns a clamp status string, as absFieldAdjust does.
func (e *logTimeRangeEditor) absFieldOverwriteDigit(d int) string {
	if d < 0 || d > 9 {
		return ""
	}
	y, mo, day := e.AbsTime.Year(), int(e.AbsTime.Month()), e.AbsTime.Day()
	h, mn, s := e.AbsTime.Hour(), e.AbsTime.Minute(), e.AbsTime.Second()
	loc := e.AbsTime.Location()
	if loc == nil {
		loc = time.Local
	}
	switch e.AbsField {
	case logTimeRangeAbsFieldYear:
		// Year has 4 digits; digit-overwrite shifts left to preserve
		// the most-significant digits, so typing "2" onto 2026 gives
		// 0262 — a surprising result. Instead, we multiply + mod: the
		// typed digit becomes the new units place. Callers who want
		// a full jump use +/-.
		newY := (y/1000)*1000 + (y%1000/100)*100 + (y%100/10)*10 + d
		if newY < 1970 {
			newY = 1970
		}
		day, _ = clampDayInMonth(newY, mo, day)
		e.AbsTime = time.Date(newY, time.Month(mo), day, h, mn, s, 0, loc)
		return ""
	case logTimeRangeAbsFieldMonth:
		newMo := clampInt(d, 1, 12)
		if d == 0 {
			newMo = 1
		}
		var status string
		day, status = clampDayInMonth(y, newMo, day)
		e.AbsTime = time.Date(y, time.Month(newMo), day, h, mn, s, 0, loc)
		return status
	case logTimeRangeAbsFieldDay:
		newD := d
		if newD < 1 {
			newD = 1
		}
		var status string
		newD, status = clampDayInMonth(y, mo, newD)
		e.AbsTime = time.Date(y, time.Month(mo), newD, h, mn, s, 0, loc)
		return status
	case logTimeRangeAbsFieldHour:
		newH := clampInt(d, 0, 23)
		e.AbsTime = time.Date(y, time.Month(mo), day, newH, mn, s, 0, loc)
		return ""
	case logTimeRangeAbsFieldMinute:
		newMn := clampInt(d, 0, 59)
		e.AbsTime = time.Date(y, time.Month(mo), day, h, newMn, s, 0, loc)
		return ""
	case logTimeRangeAbsFieldSecond:
		newS := clampInt(d, 0, 59)
		e.AbsTime = time.Date(y, time.Month(mo), day, h, mn, newS, 0, loc)
		return ""
	}
	return ""
}

// clampDayInMonth pins day into [1, daysInMonth(year, month)]. Returns
// the clamped day and a status string describing the adjustment ("" on
// a clean value).
func clampDayInMonth(year, month, day int) (int, string) {
	mo := time.Month(month)
	if mo < time.January {
		mo = time.January
	}
	if mo > time.December {
		mo = time.December
	}
	days := daysInMonth(year, mo)
	if day < 1 {
		return 1, "day clamped"
	}
	if day > days {
		return days, "day clamped to last of month"
	}
	return day, ""
}

// daysInMonth returns the number of days in the given month/year
// (handles Feb 29 on leap years).
func daysInMonth(year int, month time.Month) int {
	// First day of next month minus 1 day = last day of this month.
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	return last.Day()
}
