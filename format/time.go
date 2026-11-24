// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.ymotongpoo.com/pbt"
)

// unnamedZones holds a location per quarter hour from -12:00 to +14:00
// without a name, like the locations time.Parse creates for numeric
// offsets, with time.UTC for the zero offset.
var unnamedZones = func() []*time.Location {
	zones := make([]*time.Location, 0, 105)
	for q := -48; q <= 56; q++ {
		if q == 0 {
			zones = append(zones, time.UTC)
			continue
		}
		zones = append(zones, time.FixedZone("", q*15*60))
	}
	return zones
}()

// Time generates the strings that time.Time.Format writes for layout, for
// instants of pbt.Time in UTC and in fixed offsets from -12:00 to +14:00
// in quarter hours. The offsets carry no zone name, like those time.Parse
// creates for numeric offsets, so a layout with MST writes the offset, as
// in "+0900", and a layout with Z07:00 writes "Z" for UTC. Pass the layout
// constants of package time, such as time.RFC1123 or time.Kitchen, or any
// other layout. Values shrink toward the Unix epoch in UTC.
//
// Time generates what Go writes, not what a standard allows: for the whole
// syntax of RFC 3339 use RFC3339.
func Time(layout string) pbt.Gen[string] {
	times, zones := pbt.Time(), pbt.IntRange(-48, 56)
	return pbt.Custom(fmt.Sprintf("Time(%q)", layout), func(t *pbt.T) string {
		z := zones.Draw(t, "")
		v := times.Draw(t, "")
		// Near the ends of the range an offset can move the local date
		// out of years 1 to 9999, which no four-digit layout writes;
		// such instants stay in UTC.
		if local := v.In(unnamedZones[z+48]); local.Year() >= 1 && local.Year() <= 9999 {
			v = local
		}
		return v.Format(layout)
	})
}

// daysIn returns the number of days of month m in year y of the proleptic
// Gregorian calendar.
func daysIn(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// dateMode picks a date as drawn, February 29 of a leap year, or a date in
// the first or the last year of the range.
var dateMode = choice(27, 1, 1, 1)

// calendarDate draws a date in years lo to hi, shrinking toward 1970-01-01
// when that is in range. The day is drawn from 1 to 31 and clamped to the
// month, so the last day of a month is common, and about one date in ten
// is February 29 or lies in the first or the last year.
func calendarDate(t *pbt.T, lo, hi int) (y, m, d int) {
	mode := dateMode.Draw(t, "")
	y = pbt.IntRangeToward(lo, hi, min(max(1970, lo), hi)).Draw(t, "")
	m = pbt.IntRange(1, 12).Draw(t, "")
	d = pbt.IntRange(1, 31).Draw(t, "")
	switch mode {
	case 1:
		if ly := leapYearNear(y, lo, hi); ly >= 0 {
			y, m, d = ly, 2, 29
		}
	case 2:
		y = lo
	case 3:
		y = hi
	}
	return y, m, min(d, daysIn(y, m))
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// leapYearNear returns a leap year in [lo, hi] at or before y, or after it
// when there is none before, or -1.
func leapYearNear(y, lo, hi int) int {
	for c := y; c >= lo && c > y-8; c-- {
		if isLeap(c) {
			return c
		}
	}
	for c := y + 1; c <= hi && c < y+9; c++ {
		if isLeap(c) {
			return c
		}
	}
	return -1
}

var (
	rfc3339Mode   = choice(19, 1) // a regular time or a leap second
	rfc3339Offset = choice(4, 1, 4, 1, 1)
	letterCase    = choice(9, 1) // the letter as the RFC writes it, or in lower case
	fracDigits    = pbt.IntRange(0, 12)
	digit         = pbt.IntRange(0, 9)
	quarterHours  = pbt.IntRange(-48, 56)
)

// RFC3339 generates timestamps in the date-time syntax of RFC 3339 §5.6:
// full-date "T" full-time, such as "1985-04-12T23:20:50.52Z". It covers
// the whole production: years 0000 to 9999, days up to the last of each
// month including February 29, fractions of a second of 1 to 12 digits,
// the letters T and Z in lower case as §5.6 permits, the offset -00:00 of
// §4.3, offsets of any hour and minute, and the leap second 23:59:60 UTC at
// the end of June and December that §5.7 allows, written in the local time
// of its offset. It does not use the space that §5.6 lets applications put
// in place of T, which is outside the ABNF.
//
// time.Parse with time.RFC3339 rejects some of these: the leap second
// (second 60), lower-case t and z in some Go versions, and offsets of 24
// hours or more, which this generator does not produce. Values shrink
// toward "1970-01-01T00:00:00Z".
func RFC3339() pbt.Gen[string] {
	return pbt.Custom("RFC3339()", func(t *pbt.T) string {
		return rfc3339DateTime(t, true, true)
	})
}

// RFC3339FullDate generates dates in the full-date syntax of RFC 3339 §5.6,
// such as "1985-04-12", for years 0000 to 9999 with the last day of each
// month common. Values shrink toward "1970-01-01".
func RFC3339FullDate() pbt.Gen[string] {
	return pbt.Custom("RFC3339FullDate()", func(t *pbt.T) string {
		y, m, d := calendarDate(t, 0, 9999)
		return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	})
}

// RFC3339FullTime generates times of day in the full-time syntax of RFC
// 3339 §5.6, partial-time followed by an offset, such as "23:20:50.52Z" or
// "16:39:57-08:00", with the same fractions, offsets and letter cases as
// RFC3339 and no leap seconds. Values shrink toward "00:00:00Z".
func RFC3339FullTime() pbt.Gen[string] {
	return pbt.Custom("RFC3339FullTime()", func(t *pbt.T) string {
		return rfc3339DateTime(t, false, false)
	})
}

func rfc3339DateTime(t *pbt.T, withDate, leap bool) string {
	leapSecond := leap && rfc3339Mode.Draw(t, "") == 1
	offset := rfc3339OffsetText(t)
	var b strings.Builder
	var y, m, d, hh, mm, ss int
	if leapSecond {
		// 23:59:60 UTC on the last day of June or December, shown in the
		// local time of the offset: the second stays 60 while the date,
		// hour and minute move with the offset.
		y = pbt.IntRangeToward(1, 9998, 1972).Draw(t, "")
		m = 6 + 6*pbt.IntRange(0, 1).Draw(t, "")
		local := time.Date(y, time.Month(m), daysIn(y, m), 23, 59, 0, 0, time.UTC).Add(offsetDuration(offset))
		y, m, d = local.Year(), int(local.Month()), local.Day()
		hh, mm, ss = local.Hour(), local.Minute(), 60
	} else {
		if withDate {
			y, m, d = calendarDate(t, 0, 9999)
		}
		hh = pbt.IntRange(0, 23).Draw(t, "")
		mm = pbt.IntRange(0, 59).Draw(t, "")
		ss = pbt.IntRange(0, 59).Draw(t, "")
	}
	if withDate {
		sep := "T"
		if letterCase.Draw(t, "") == 1 {
			sep = "t"
		}
		fmt.Fprintf(&b, "%04d-%02d-%02d%s", y, m, d, sep)
	}
	fmt.Fprintf(&b, "%02d:%02d:%02d", hh, mm, ss)
	if n := fracDigits.Draw(t, ""); n > 0 {
		b.WriteByte('.')
		for range n {
			b.WriteByte(byte('0' + digit.Draw(t, "")))
		}
	}
	b.WriteString(offset)
	return b.String()
}

// rfc3339OffsetText draws a time-offset: Z, z, a whole quarter-hour offset
// in current use, -00:00, or any hour from 00 to 23 and minute from 00 to
// 59.
func rfc3339OffsetText(t *pbt.T) string {
	switch rfc3339Offset.Draw(t, "") {
	case 0:
		return "Z"
	case 1:
		return "z"
	case 2:
		q := quarterHours.Draw(t, "")
		sign := '+'
		if q < 0 {
			sign, q = '-', -q
		}
		return fmt.Sprintf("%c%02d:%02d", sign, q/4, q%4*15)
	case 3:
		return "-00:00"
	default:
		sign := "+-"[pbt.IntRange(0, 1).Draw(t, "")]
		return fmt.Sprintf("%c%02d:%02d", sign, pbt.IntRange(0, 23).Draw(t, ""), pbt.IntRange(0, 59).Draw(t, ""))
	}
}

// offsetDuration converts an offset written by rfc3339OffsetText.
func offsetDuration(off string) time.Duration {
	if len(off) != 6 {
		return 0
	}
	h, _ := strconv.Atoi(off[1:3])
	m, _ := strconv.Atoi(off[4:6])
	d := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
	if off[0] == '-' {
		d = -d
	}
	return d
}

var (
	durationForm  = choice(1, 5) // weeks, the other designators
	durationMask  = pbt.IntRange(1, 63)
	durationValue = pbt.Int64Range(0, 1<<40)
	durationFrac  = choice(4, 1, 1) // no fraction, a decimal point, a decimal comma
	fracLen       = pbt.IntRange(1, 6)
)

// durationOrder lists the designators of ISO 8601 durations from the
// highest order to the lowest, with the index of the mask bit that selects
// each: bit 0 selects days, so the smallest mask, which is where shrinking
// ends, selects days alone.
var durationOrder = [...]struct {
	bit  int
	unit string
	time bool
}{{2, "Y", false}, {1, "M", false}, {0, "D", false}, {5, "H", true}, {4, "M", true}, {3, "S", true}}

// ISO8601Duration generates durations in the format with designators of
// ISO 8601-1:2019 §5.5.2, such as "P3Y6M4DT12H30M5S", "PT0.5S" and "P2W":
// any non-empty selection of years, months, days, hours, minutes and
// seconds in that order, with T before the time components, or a number of
// weeks, each with a non-negative integer value, and in a share of the
// cases a decimal fraction on the lowest-order component written with a
// full stop or a comma. Values shrink toward "P0W", the form with the
// fewest parts.
func ISO8601Duration() pbt.Gen[string] {
	return pbt.Custom("ISO8601Duration()", func(t *pbt.T) string {
		var b strings.Builder
		b.WriteByte('P')
		if durationForm.Draw(t, "") == 0 {
			fmt.Fprintf(&b, "%d", durationValue.Draw(t, ""))
			writeFraction(t, &b)
			b.WriteByte('W')
			return b.String()
		}
		mask := durationMask.Draw(t, "")
		last := 0
		for i, c := range durationOrder {
			if mask&(1<<c.bit) != 0 {
				last = i
			}
		}
		inTime := false
		for i, c := range durationOrder {
			if mask&(1<<c.bit) == 0 {
				continue
			}
			if c.time && !inTime {
				b.WriteByte('T')
				inTime = true
			}
			fmt.Fprintf(&b, "%d", durationValue.Draw(t, ""))
			if i == last {
				writeFraction(t, &b)
			}
			b.WriteString(c.unit)
		}
		return b.String()
	})
}

func writeFraction(t *pbt.T, b *strings.Builder) {
	if k := durationFrac.Draw(t, ""); k > 0 {
		b.WriteByte(".,"[k-1])
		for range fracLen.Draw(t, "") {
			b.WriteByte(byte('0' + digit.Draw(t, "")))
		}
	}
}

var (
	rfc3339DurForm = choice(1, 4, 2) // weeks, a date part, a time part only
	unitRun        = pbt.IntRange(0, 5)
)

// RFC3339Duration generates durations in the stricter grammar of RFC 3339
// Appendix A, which JSON Schema uses for its duration format: "P" followed
// by weeks ("P2W"), or by consecutive date components with optional
// consecutive time components ("P1Y2M", "P3DT4H5M"), or by time components
// alone ("PT36H"). Components cannot be skipped, so "P1Y3D" is not in the
// grammar, and values have no fractions. Values shrink toward "P0W", the
// form with the fewest parts.
func RFC3339Duration() pbt.Gen[string] {
	return pbt.Custom("RFC3339Duration()", func(t *pbt.T) string {
		var b strings.Builder
		b.WriteByte('P')
		switch rfc3339DurForm.Draw(t, "") {
		case 0:
			fmt.Fprintf(&b, "%dW", durationValue.Draw(t, ""))
			return b.String()
		case 1:
			writeRun(t, &b, "YMD")
			if pbt.Bool().Draw(t, "") {
				b.WriteByte('T')
				writeRun(t, &b, "HMS")
			}
		case 2:
			b.WriteByte('T')
			writeRun(t, &b, "HMS")
		}
		return b.String()
	})
}

// unitRuns are the runs of consecutive components writeRun picks from, as
// start and length, the shortest first.
var unitRuns = [...][2]int{{0, 1}, {1, 1}, {2, 1}, {0, 2}, {1, 2}, {0, 3}}

// writeRun writes a non-empty run of consecutive components of units.
func writeRun(t *pbt.T, b *strings.Builder, units string) {
	r := unitRuns[unitRun.Draw(t, "")]
	for _, u := range units[r[0] : r[0]+r[1]] {
		fmt.Fprintf(b, "%d%c", durationValue.Draw(t, ""), u)
	}
}

// IMF-fixdate of RFC 9110 §5.6.7 and its obsolete forms, as Go layouts on
// UTC instants.
const (
	imfFixdate = "Mon, 02 Jan 2006 15:04:05 GMT"
	rfc850Date = "Monday, 02-Jan-06 15:04:05 GMT"
	asctime    = "Mon Jan _2 15:04:05 2006"
)

var httpDateForm = choice(2, 1, 1)

// HTTPDate generates dates in the IMF-fixdate syntax of RFC 9110 §5.6.7,
// such as "Sun, 06 Nov 1994 08:49:37 GMT", the form a sender must write,
// for instants of pbt.Time. Values shrink toward "Thu, 01 Jan 1970
// 00:00:00 GMT".
func HTTPDate() pbt.Gen[string] {
	times := pbt.Time()
	return pbt.Custom("HTTPDate()", func(t *pbt.T) string {
		return times.Draw(t, "").Format(imfFixdate)
	})
}

// RFC9110HTTPDate generates dates in the three forms a recipient of RFC
// 9110 §5.6.7 must accept: IMF-fixdate in half of the cases, and the
// obsolete RFC 850 form ("Sunday, 06-Nov-94 08:49:37 GMT") and the asctime
// form ("Sun Nov  6 08:49:37 1994") in a quarter each. Values shrink toward
// the IMF-fixdate of the Unix epoch.
func RFC9110HTTPDate() pbt.Gen[string] {
	times := pbt.Time()
	return pbt.Custom("RFC9110HTTPDate()", func(t *pbt.T) string {
		layout := [...]string{imfFixdate, rfc850Date, asctime}[httpDateForm.Draw(t, "")]
		return times.Draw(t, "").Format(layout)
	})
}

var (
	monthNames = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	dayNames   = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	fws        = choice(12, 1, 1)   // one space, a tab, several spaces and tabs
	nameCase   = choice(18, 1, 1)   // as written, lower case, upper case
	zoneForm   = choice(4, 2, 1, 1) // a quarter-hour offset, +0000, -0000, any hour 00-99 and minute
	yearDigits = choice(19, 1)      // four digits, five
)

// RFC5322DateTime generates dates in the date-time syntax of RFC 5322
// §3.3, as e-mail writes them in the Date header, such as "Fri, 21 Nov
// 1997 09:55:06 -0600". It covers the optional day of the week, which is
// the day of the date, days of one or two digits, years from 1900 with
// four digits and in a share of the cases five, optional seconds, the leap
// second 23:59:60, the zones +0000 and -0000 and offsets of any hour and
// minute up to +9959, white space of spaces and tabs between the parts,
// and day and month names in any letter case, since ABNF strings are
// case-insensitive. It leaves out comments, line folding and the obsolete
// syntax of §4.3. Values shrink toward "1 Jan 1970 00:00 +0000".
func RFC5322DateTime() pbt.Gen[string] {
	return pbt.Custom("RFC5322DateTime()", func(t *pbt.T) string {
		withDay := pbt.Frequency(pbt.W(1, pbt.Just(false)), pbt.W(3, pbt.Just(true))).Draw(t, "")
		withSeconds := pbt.Frequency(pbt.W(1, pbt.Just(false)), pbt.W(3, pbt.Just(true))).Draw(t, "")
		leap := withSeconds && rfc3339Mode.Draw(t, "") == 1
		var y int
		if yearDigits.Draw(t, "") == 1 {
			y = pbt.IntRange(10000, 99999).Draw(t, "")
		} else {
			y = pbt.IntRangeToward(1900, 9999, 1970).Draw(t, "")
		}
		m := pbt.IntRange(1, 12).Draw(t, "")
		d := min(pbt.IntRange(1, 31).Draw(t, ""), daysIn(y, m))
		hh, mm, ss := 23, 59, 60
		if !leap {
			hh = pbt.IntRange(0, 23).Draw(t, "")
			mm = pbt.IntRange(0, 59).Draw(t, "")
			ss = pbt.IntRange(0, 59).Draw(t, "")
		}
		var b strings.Builder
		if withDay {
			b.WriteString(caseOf(t, dayNames[time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC).Weekday()]))
			b.WriteString(",")
			b.WriteString(fwsText(t))
		}
		if pbt.Bool().Draw(t, "") {
			fmt.Fprintf(&b, "%02d", d)
		} else {
			fmt.Fprintf(&b, "%d", d)
		}
		b.WriteString(fwsText(t))
		b.WriteString(caseOf(t, monthNames[m-1]))
		b.WriteString(fwsText(t))
		fmt.Fprintf(&b, "%04d", y)
		b.WriteString(fwsText(t))
		fmt.Fprintf(&b, "%02d:%02d", hh, mm)
		if withSeconds {
			fmt.Fprintf(&b, ":%02d", ss)
		}
		b.WriteString(fwsText(t))
		b.WriteString(rfc5322Zone(t))
		return b.String()
	})
}

func fwsText(t *pbt.T) string {
	switch fws.Draw(t, "") {
	case 0:
		return " "
	case 1:
		return "\t"
	default:
		return stringIn(" \t", 1, 4).Draw(t, "")
	}
}

func caseOf(t *pbt.T, s string) string {
	switch nameCase.Draw(t, "") {
	case 1:
		return strings.ToLower(s)
	case 2:
		return strings.ToUpper(s)
	}
	return s
}

func rfc5322Zone(t *pbt.T) string {
	switch zoneForm.Draw(t, "") {
	case 1:
		return "+0000"
	case 2:
		return "-0000"
	case 3:
		sign := "+-"[pbt.IntRange(0, 1).Draw(t, "")]
		return fmt.Sprintf("%c%02d%02d", sign, pbt.IntRange(0, 99).Draw(t, ""), pbt.IntRange(0, 59).Draw(t, ""))
	}
	q := quarterHours.Draw(t, "")
	sign := '+'
	if q < 0 {
		sign, q = '-', -q
	}
	return fmt.Sprintf("%c%02d%02d", sign, q/4, q%4*15)
}
