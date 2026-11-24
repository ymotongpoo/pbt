// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"strings"
	"time"
)

// The range of Time: the years RFC 3339 and ISO 8601 write with four
// digits. Its lower end is the zero time.Time, for which IsZero reports true.
var (
	minTime = time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC)
	maxTime = time.Date(9999, time.December, 31, 23, 59, 59, 999_999_999, time.UTC)
)

// Time generates instants between the zero time, 0001-01-01T00:00:00Z, and
// 9999-12-31T23:59:59.999999999Z, in UTC and without a monotonic clock
// reading. It is TimeRange over that span; see TimeRange for the
// distribution. Values shrink toward the Unix epoch.
func Time() Gen[time.Time] {
	return named(TimeRange(minTime, maxTime), "Time()")
}

// timeSnap names the calendar instants that TimeRange moves a drawn time to
// with some probability, because date and time code breaks there.
type timeSnap int

const (
	snapNone timeSnap = iota
	snapDayStart
	snapDayEnd
	snapMonthStart
	snapMonthEnd
	snapYearStart
	snapYearEnd
	snapLeapDay
	snapTransition
	snapLo
	snapHi
)

// timeSnapWeights gives the share of each snap. Most values keep the instant
// drawn; the rest move to the start or the last nanosecond of a day, month
// or year, to February 29 of a leap year, next to a change of the zone's
// offset, or to a bound of the range.
var timeSnapWeights = [...]int{
	snapNone:       16,
	snapDayStart:   2,
	snapDayEnd:     2,
	snapMonthStart: 1,
	snapMonthEnd:   2,
	snapYearStart:  1,
	snapYearEnd:    1,
	snapLeapDay:    2,
	snapTransition: 3,
	snapLo:         1,
	snapHi:         1,
}

// timePrecisionWeights and timePrecisions round the fraction of a second:
// most stored timestamps carry whole seconds, milliseconds or microseconds,
// and code that rounds or truncates them is where precision bugs live.
var (
	timePrecisions       = [...]time.Duration{1, time.Microsecond, time.Millisecond, time.Second}
	timePrecisionWeights = [...]int{4, 1, 2, 3}
)

// TimeRange generates instants in [lo, hi], in the location of lo and
// without a monotonic clock reading. It panics when lo is after hi.
//
// An instant is drawn as seconds and nanoseconds since the Unix epoch, and
// the nanoseconds are rounded to whole microseconds, milliseconds or seconds
// in a share of the cases. About a third of the values are then moved to an
// instant where calendar code tends to break, as long as that instant lies
// in the range: the first or last nanosecond of a day, a month or a year in
// the location, February 29 of the nearest leap year, and, in a location
// with daylight saving time or other changes of offset, the moment the
// offset changes, the nanosecond before it, and the half hours around it,
// whose wall-clock times repeat or do not exist. Either bound of the range is drawn as
// well, so Time generates the zero time.Time.
//
// Values shrink toward the Unix epoch, or toward the bound nearest to it
// when the epoch is outside the range.
func TimeRange(lo, hi time.Time) Gen[time.Time] {
	if lo.After(hi) {
		panic(fmt.Sprintf("pbt: TimeRange(%s, %s): lo is after hi", lo.Format(time.RFC3339Nano), hi.Format(time.RFC3339Nano)))
	}
	loc := lo.Location()
	lo, hi = lo.Round(0), hi.Round(0) // drop monotonic readings so that comparisons use the wall clock alone
	loSec, hiSec := lo.Unix(), hi.Unix()
	center := min(max(0, loSec), hiSec)
	secs := integerRange(loSec, hiSec, center, "")

	snapWeights := timeSnapWeights
	if !hasTransitions(loc, lo, hi) {
		snapWeights[snapTransition] = 0
	}
	snapTotal := 0
	for _, w := range snapWeights {
		snapTotal += w
	}
	precTotal := 0
	for _, w := range timePrecisionWeights {
		precTotal += w
	}

	name := fmt.Sprintf("TimeRange(%s, %s)", lo.Format(time.RFC3339Nano), hi.Format(time.RFC3339Nano))
	return Gen[time.Time]{name: name, draw: func(t *T) time.Time {
		// The snap is drawn first: the shrinker prefers sequences that are
		// smaller from the front, so an instant reached by moving a small
		// draw to a bound must not look simpler than the drawn instant.
		snap := timeSnap(t.drawIndex(snapWeights[:], snapTotal))
		variant := 0
		if snap == snapTransition {
			variant = int(t.draw(3, plainMeta, modeUniform))
		}
		sec := secs.draw(t)
		nsLo, nsHi := int64(0), int64(999_999_999)
		if sec == loSec {
			nsLo = int64(lo.Nanosecond())
		}
		if sec == hiSec {
			nsHi = int64(hi.Nanosecond())
		}
		ns := nsLo + int64(t.draw(uint64(nsHi-nsLo), plainMeta, modeBiased))
		v := time.Unix(sec, ns).In(loc)
		if p := timePrecisions[t.drawIndex(timePrecisionWeights[:], precTotal)]; p > 1 {
			if r := v.Truncate(p); !r.Before(lo) {
				v = r
			}
		}
		switch snap {
		case snapLo:
			v = lo
		case snapHi:
			v = hi
		default:
			if s, ok := snapTime(v, snap, variant, lo); ok && !s.Before(lo) && !s.After(hi) {
				v = s
			}
		}
		return v
	}}
}

// snapTime moves v to the calendar instant snap names, in v's location. A
// transition snap uses the change of offset that starts v's zone period,
// or the one that ends it when the start lies before lo, and moves to that
// moment, the nanosecond before it, or half an hour after or before it.
func snapTime(v time.Time, snap timeSnap, variant int, lo time.Time) (time.Time, bool) {
	loc := v.Location()
	y, m, d := v.Date()
	switch snap {
	case snapDayStart:
		return time.Date(y, m, d, 0, 0, 0, 0, loc), true
	case snapDayEnd:
		return time.Date(y, m, d+1, 0, 0, 0, 0, loc).Add(-1), true
	case snapMonthStart:
		return time.Date(y, m, 1, 0, 0, 0, 0, loc), true
	case snapMonthEnd:
		return time.Date(y, m+1, 1, 0, 0, 0, 0, loc).Add(-1), true
	case snapYearStart:
		return time.Date(y, time.January, 1, 0, 0, 0, 0, loc), true
	case snapYearEnd:
		return time.Date(y+1, time.January, 1, 0, 0, 0, 0, loc).Add(-1), true
	case snapLeapDay:
		ly := y
		for !isLeap(ly) {
			ly--
		}
		hh, mm, ss := v.Clock()
		return time.Date(ly, time.February, 29, hh, mm, ss, v.Nanosecond(), loc), true
	case snapTransition:
		at, end := v.ZoneBounds()
		if at.IsZero() || at.Before(lo) {
			at = end
		}
		if at.IsZero() {
			return v, false
		}
		return at.Add([...]time.Duration{0, -1, 30 * time.Minute, -30 * time.Minute}[variant]), true
	}
	return v, false
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// hasTransitions reports whether the offset of loc changes within [lo, hi]:
// the zone period that contains lo starts at lo or ends before hi.
func hasTransitions(loc *time.Location, lo, hi time.Time) bool {
	start, end := lo.In(loc).ZoneBounds()
	return (!start.IsZero() && start.Equal(lo)) || (!end.IsZero() && !end.After(hi))
}

// durationUnits are the multiples DurationRange rounds to in a share of the
// cases; durationUnitWeights gives the share of each, the first entry being
// the duration as drawn.
var (
	durationUnits       = [...]time.Duration{1, time.Microsecond, time.Millisecond, time.Second, time.Minute, time.Hour}
	durationUnitWeights = [...]int{6, 1, 2, 2, 1, 1}
)

// Duration generates every time.Duration, from about -292 years to +292
// years. See DurationRange for the distribution. Values shrink toward 0.
func Duration() Gen[time.Duration] {
	return named(DurationRange(-1<<63, 1<<63-1), "Duration()")
}

// DurationRange generates durations in [lo, hi]. It panics when lo is
// above hi. Small durations and the bounds are over-represented, as with
// the integer generators, and about half of the values are truncated to a
// whole number of microseconds, milliseconds, seconds, minutes or hours
// when that stays in the range. Values shrink toward 0, or toward the bound
// nearest to 0.
func DurationRange(lo, hi time.Duration) Gen[time.Duration] {
	if lo > hi {
		panic(fmt.Sprintf("pbt: DurationRange(%v, %v): lo is above hi", lo, hi))
	}
	g := integerRange(lo, hi, min(max(0, lo), hi), "")
	total := 0
	for _, w := range durationUnitWeights {
		total += w
	}
	return Gen[time.Duration]{name: fmt.Sprintf("DurationRange(%v, %v)", lo, hi), draw: func(t *T) time.Duration {
		d := g.draw(t)
		if u := durationUnits[t.drawIndex(durationUnitWeights[:], total)]; u > 1 {
			if r := d.Truncate(u); r >= lo && r <= hi {
				d = r
			}
		}
		return d
	}}
}

// fixedZones holds one location per quarter hour from -12:00 to +14:00,
// the span of the offsets in use, with time.UTC for the zero offset. Each
// is named by its offset, which is how time.Time's GoString and the MST
// layout show it.
var fixedZones = func() []*time.Location {
	zones := make([]*time.Location, 0, 105)
	for q := -48; q <= 56; q++ {
		if q == 0 {
			zones = append(zones, time.UTC)
			continue
		}
		sign, a := '+', q
		if q < 0 {
			sign, a = '-', -q
		}
		zones = append(zones, time.FixedZone(fmt.Sprintf("%c%02d:%02d", sign, a/4, a%4*15), q*15*60))
	}
	return zones
}()

// FixedZone generates locations with a fixed offset from UTC, in quarter
// hours from -12:00 to +14:00, which covers every offset in current use,
// such as +05:45 and +12:45. The zero offset is time.UTC itself, and every
// other location is named by its offset, for example "+09:00". The same
// offset always yields the same *time.Location. Values shrink toward
// time.UTC.
func FixedZone() Gen[*time.Location] {
	quarters := integerRange(-48, 56, 0, "")
	return Gen[*time.Location]{name: "FixedZone()", draw: func(t *T) *time.Location {
		return fixedZones[quarters.draw(t)+48]
	}}
}

// defaultZones are the IANA time zones Zone picks from when it is given no
// names. Each has a rule that date and time code gets wrong.
var defaultZones = []string{
	"UTC",
	"America/New_York",    // daylight saving time in the northern hemisphere
	"America/Sao_Paulo",   // southern hemisphere; daylight saving time until 2019, with transitions at midnight
	"America/St_Johns",    // offset of -03:30 with daylight saving time
	"Europe/London",       // offset of 0 that is not UTC half of the year
	"Europe/Dublin",       // negative daylight saving time in the tz database
	"Africa/Casablanca",   // daylight saving time suspended during Ramadan
	"Asia/Kolkata",        // offset of +05:30
	"Asia/Kathmandu",      // offset of +05:45
	"Asia/Tokyo",          // no daylight saving time since 1951
	"Australia/Lord_Howe", // daylight saving time of 30 minutes
	"Pacific/Chatham",     // offset of +12:45 with daylight saving time
	"Pacific/Kiritimati",  // offset of +14:00
	"Pacific/Apia",        // skipped 2011-12-30 when it crossed the date line
	"Antarctica/Troll",    // daylight saving time of two hours
}

// Zone generates the IANA time zones with the given names, loaded with
// time.LoadLocation when Zone is called; it panics when a name does not
// load. Without names it picks from a list of zones whose rules tend to
// break date and time code: UTC, daylight saving time in both hemispheres,
// offsets of 30 and 45 minutes, +14:00, a daylight saving time of 30
// minutes and one of two hours, a zone that suspends daylight saving time,
// and one that skipped a day. Values shrink toward the first zone, which in
// the default list is UTC.
//
// Loading needs the time zone database of the system or of the time/tzdata
// package; a test that must run where the system has none can import
// time/tzdata.
func Zone(names ...string) Gen[*time.Location] {
	name := fmt.Sprintf("Zone(%s)", strings.Join(names, ", "))
	if len(names) == 0 {
		names, name = defaultZones, "Zone()"
	}
	locs := make([]*time.Location, len(names))
	for i, n := range names {
		loc, err := time.LoadLocation(n)
		if err != nil {
			panic(fmt.Sprintf("pbt: Zone: %v; import time/tzdata if the system has no time zone database", err))
		}
		locs[i] = loc
	}
	return Gen[*time.Location]{name: name, draw: func(t *T) *time.Location {
		return locs[t.draw(uint64(len(locs)-1), plainMeta, modeUniform)]
	}}
}
