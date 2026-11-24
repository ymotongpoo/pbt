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

// leapSeconds are the UTC days that ended with an inserted leap second,
// 23:59:60, from IERS Bulletin C: all 27 from 1972 to 2016, after which
// none has been inserted. RFC 3339 Appendix D lists those up to 1998.
var leapSeconds = func() []time.Time {
	days := []string{
		"1972-06-30", "1972-12-31", "1973-12-31", "1974-12-31", "1975-12-31", "1976-12-31",
		"1977-12-31", "1978-12-31", "1979-12-31", "1981-06-30", "1982-06-30", "1983-06-30",
		"1985-06-30", "1987-12-31", "1989-12-31", "1990-12-31", "1992-06-30", "1993-06-30",
		"1994-06-30", "1995-12-31", "1997-06-30", "1998-12-31", "2005-12-31", "2008-12-31",
		"2012-06-30", "2015-06-30", "2016-12-31",
	}
	ts := make([]time.Time, len(days))
	for i, d := range days {
		v, err := time.Parse(time.DateOnly, d)
		if err != nil {
			panic(err)
		}
		ts[i] = v.Add(23*time.Hour + 59*time.Minute) // the leap second is 23:59:60 of the day
	}
	return ts
}()

var (
	times       = pbt.Time()
	zoneIndex   = pbt.IntRange(-48, 56)
	leapIndex   = pbt.IntRange(0, len(leapSeconds)-1)
	hours       = pbt.IntRange(0, 23)
	minutes     = pbt.IntRange(0, 59)
	seconds     = pbt.IntRange(0, 59)
	months      = pbt.IntRange(1, 12)
	days31      = pbt.IntRange(1, 31)
	sign        = pbt.SampledFrom([]string{"+", "-"})
	fracDigits  = stringIn(digits, 1, 12)
	letterT     = literalCase("T", 9, 1)
	letterZ     = literalCase("Z", 4, 1)
	dateMode    = choice(27, 1, 1, 1) // as drawn, February 29, the first year, the last year
	years0to9   = pbt.IntRangeToward(0, 9999, 1970)
	quarterHour = pbt.IntRange(-48, 56)
)

// Time generates the strings that time.Time.Format writes for layout, for
// instants of pbt.Time in UTC and in fixed offsets from -12:00 to +14:00
// in quarter hours. The offsets carry no zone name, like those time.Parse
// creates for numeric offsets, so a layout with MST writes the offset, as
// in "+0900", and a layout with Z07:00 writes "Z" for UTC. Near the ends
// of the range, where an offset would move the local date out of years 1
// to 9999, the instant stays in UTC. Pass the layout constants of package
// time, such as time.RFC1123 or time.Kitchen, or any other layout. Values
// shrink toward the Unix epoch in UTC.
//
// Time generates what Go writes, not what a standard allows: for the whole
// syntax of RFC 3339 use RFC3339.
func Time(layout string) pbt.Gen[string] {
	return pbt.Custom(fmt.Sprintf("Time(%q)", layout), func(t *pbt.T) string {
		z := zoneIndex.Draw(t, "")
		v := times.Draw(t, "")
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

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// calendarDate draws a date in years lo to hi from years, which must stay
// in that range, shrinking toward its center and January 1. The day is
// drawn from 1 to 31 and clamped to the month, so the last day of a month
// is common, and about one date in ten is February 29 or lies in the first
// or the last year. It always takes the same number of choices.
func calendarDate(t *pbt.T, years pbt.Gen[int], lo, hi int) (y, m, d int) {
	mode := dateMode.Draw(t, "")
	y = years.Draw(t, "")
	m = months.Draw(t, "")
	d = days31.Draw(t, "")
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

// offsetDuration converts a numeric offset such as "+09:00" or "-0530";
// anything else is UTC.
func offsetDuration(off string) time.Duration {
	off = strings.ReplaceAll(off, ":", "")
	if len(off) != 5 || (off[0] != '+' && off[0] != '-') {
		return 0
	}
	h, _ := strconv.Atoi(off[1:3])
	m, _ := strconv.Atoi(off[3:5])
	d := time.Duration(h)*time.Hour + time.Duration(m)*time.Minute
	if off[0] == '-' {
		d = -d
	}
	return d
}

// rfc3339Offset generates a time-offset of RFC 3339 §5.6: Z or z, a
// quarter-hour offset in current use, -00:00 (§4.3), or any hour from 00
// to 23 and minute from 00 to 59.
var rfc3339Offset = pbt.Frequency(
	pbt.W(5, letterZ),
	pbt.W(4, pbt.Custom("offset", func(t *pbt.T) string {
		q := quarterHour.Draw(t, "")
		s := "+"
		if q < 0 {
			s, q = "-", -q
		}
		return fmt.Sprintf("%s%02d:%02d", s, q/4, q%4*15)
	})),
	pbt.W(1, pbt.Just("-00:00")),
	pbt.W(1, pbt.Custom("offset", func(t *pbt.T) string {
		return fmt.Sprintf("%s%02d:%02d", sign.Draw(t, ""), hours.Draw(t, ""), minutes.Draw(t, ""))
	})),
)

// secfrac generates an optional time-secfrac of 1 to 12 digits.
var secfrac = optional(3, 2, pbt.Map(fracDigits, func(s string) string { return "." + s }))

// leapLocal draws a leap second of the table and an offset, and returns the
// local date and time of 23:59:60 UTC under that offset: the second stays
// 60 while the date, hour and minute move with the offset.
func leapLocal(t *pbt.T, offset string) time.Time {
	return leapSeconds[leapIndex.Draw(t, "")].Add(offsetDuration(offset))
}

// RFC3339 generates timestamps in the date-time syntax of RFC 3339 §5.6:
// full-date "T" full-time, such as "1985-04-12T23:20:50.52Z". It covers
// years 0000 to 9999, days up to the last of each month including
// February 29, fractions of a second of 1 to 12 digits, the letters T and
// Z in lower case as §5.6 permits, the offset -00:00 of §4.3, offsets of
// any hour from 00 to 23 and minute from 00 to 59, and in one value in
// twenty a leap second: one of the 27 that were inserted, 23:59:60 UTC at
// the end of a day listed in IERS Bulletin C, written in the local time of
// its offset as §5.7 requires. It does not use the space that §5.6 lets
// applications put in place of T, which is outside the ABNF.
//
// time.Parse with time.RFC3339 (Go 1.26) rejects the lower-case t and z
// and the leap second, and accepts the rest, including offsets up to
// +24:00. Values shrink toward "1970-01-01T00:00:00Z", and a leap second
// toward "1972-06-30T23:59:60Z".
func RFC3339() pbt.Gen[string] {
	return named(pbt.Frequency(
		pbt.W(19, pbt.Custom("date-time", func(t *pbt.T) string {
			y, m, d := calendarDate(t, years0to9, 0, 9999)
			return fmt.Sprintf("%04d-%02d-%02d%s%02d:%02d:%02d%s%s", y, m, d, letterT.Draw(t, ""),
				hours.Draw(t, ""), minutes.Draw(t, ""), seconds.Draw(t, ""), secfrac.Draw(t, ""), rfc3339Offset.Draw(t, ""))
		})),
		pbt.W(1, pbt.Custom("leap second", func(t *pbt.T) string {
			off := rfc3339Offset.Draw(t, "")
			v := leapLocal(t, off)
			return fmt.Sprintf("%s%s%02d:%02d:60%s%s", v.Format(time.DateOnly), letterT.Draw(t, ""),
				v.Hour(), v.Minute(), secfrac.Draw(t, ""), off)
		})),
	), "RFC3339()")
}

// named renames a generator of this package for reports.
func named[V any](g pbt.Gen[V], name string) pbt.Gen[V] {
	return pbt.Custom(name, func(t *pbt.T) V { return g.Draw(t, "") })
}

// RFC3339FullDate generates dates in the full-date syntax of RFC 3339 §5.6,
// such as "1985-04-12", for years 0000 to 9999 with the last day of each
// month and February 29 common. Values shrink toward "1970-01-01".
func RFC3339FullDate() pbt.Gen[string] {
	return pbt.Custom("RFC3339FullDate()", func(t *pbt.T) string {
		y, m, d := calendarDate(t, years0to9, 0, 9999)
		return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	})
}

// RFC3339FullTime generates times of day in the full-time syntax of RFC
// 3339 §5.6, partial-time followed by an offset, such as "23:20:50.52Z" or
// "16:39:57-08:00", with the fractions, offsets and letter cases of
// RFC3339 and, in one value in twenty, the local time of a leap second,
// 23:59:60 UTC under its offset. Values shrink toward "00:00:00Z".
func RFC3339FullTime() pbt.Gen[string] {
	return named(pbt.Frequency(
		pbt.W(19, pbt.Custom("full-time", func(t *pbt.T) string {
			return fmt.Sprintf("%02d:%02d:%02d%s%s", hours.Draw(t, ""), minutes.Draw(t, ""), seconds.Draw(t, ""),
				secfrac.Draw(t, ""), rfc3339Offset.Draw(t, ""))
		})),
		pbt.W(1, pbt.Custom("leap second", func(t *pbt.T) string {
			off := rfc3339Offset.Draw(t, "")
			v := leapLocal(t, off)
			return fmt.Sprintf("%02d:%02d:60%s%s", v.Hour(), v.Minute(), secfrac.Draw(t, ""), off)
		})),
	), "RFC3339FullTime()")
}

var (
	durationValue = pbt.Int64Range(0, 1<<40)
	durationMask  = pbt.IntRange(1, 63)
	durationFrac  = optional(4, 2, pbt.Custom("fraction", func(t *pbt.T) string {
		return string(".,"[pbt.IntRange(0, 1).Draw(t, "")]) + stringIn(digits, 1, 6).Draw(t, "")
	}))
)

// durationOrder lists the designators of ISO 8601 durations from the
// highest order to the lowest, with the index of the mask bit that selects
// each: bit 0 selects days.
var durationOrder = [...]struct {
	bit  int
	unit string
	time bool
}{{2, "Y", false}, {1, "M", false}, {0, "D", false}, {5, "H", true}, {4, "M", true}, {3, "S", true}}

// ISO8601Duration generates durations in the format with designators of
// ISO 8601-1:2019 §5.5.2, such as "P3Y6M4DT12H30M5S", "PT0.5S" and "P2W":
// any non-empty selection of years, months, days, hours, minutes and
// seconds in that order, with T before the time components, or a number of
// weeks, each with a non-negative integer value up to 2^40 and in a third
// of the cases a decimal fraction on the lowest-order component written
// with a full stop or a comma. Values shrink toward "P0W", the form with
// the fewest parts.
func ISO8601Duration() pbt.Gen[string] {
	return named(pbt.Frequency(
		pbt.W(1, pbt.Custom("weeks", func(t *pbt.T) string {
			return fmt.Sprintf("P%d%sW", durationValue.Draw(t, ""), durationFrac.Draw(t, ""))
		})),
		pbt.W(5, pbt.Custom("designators", func(t *pbt.T) string {
			var b strings.Builder
			b.WriteByte('P')
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
					b.WriteString(durationFrac.Draw(t, ""))
				}
				b.WriteString(c.unit)
			}
			return b.String()
		})),
	), "ISO8601Duration()")
}

var (
	durDigits = stringIn(digits, 1, 12)
	unitRun   = pbt.IntRange(0, 5)
	// unitRuns are the runs of consecutive components writeRun picks
	// from, as start and length, the shortest first.
	unitRuns  = [...][2]int{{0, 1}, {1, 1}, {2, 1}, {0, 2}, {1, 2}, {0, 3}}
	durLetter = map[byte]pbt.Gen[string]{}
)

func init() {
	for _, c := range "PTWYMDHS" {
		durLetter[byte(c)] = literalCase(string(c), 9, 1)
	}
}

// run generates a non-empty run of consecutive components of units, such
// as "1Y2M" of "YMD", each a number of 1 to 12 digits with leading zeros
// allowed and a designator in either case.
func run(units string) pbt.Gen[string] {
	return pbt.Custom("run", func(t *pbt.T) string {
		r := unitRuns[unitRun.Draw(t, "")]
		var b strings.Builder
		for _, u := range []byte(units[r[0] : r[0]+r[1]]) {
			b.WriteString(durDigits.Draw(t, "") + durLetter[u].Draw(t, ""))
		}
		return b.String()
	})
}

// RFC3339Duration generates durations in the grammar of RFC 3339 Appendix
// A, which JSON Schema uses for its duration format: "P" followed by weeks
// ("P2W"), by consecutive date components with optional consecutive time
// components ("P1Y2M", "P3DT4H5M"), or by time components alone ("PT36H").
// Components cannot be skipped, so "P1Y3D" is not in the grammar, and
// values have no fractions. Numbers are 1*DIGIT, so leading zeros occur, as
// in "P01D", and the designators, which ABNF matches without regard to
// case (RFC 5234 §2.3), are in lower case in one case in ten. Numbers have
// at most 12 digits. Values shrink toward "P0W", the form with the fewest
// parts.
func RFC3339Duration() pbt.Gen[string] {
	timePart := concat(durLetter['T'], run("HMS"))
	return named(concat(durLetter['P'], pbt.Frequency(
		pbt.W(1, concat(durDigits, durLetter['W'])),
		pbt.W(4, concat(run("YMD"), optional(1, 1, timePart))),
		pbt.W(2, timePart),
	)), "RFC3339Duration()")
}

// IMF-fixdate of RFC 9110 §5.6.7 and its obsolete forms, as Go layouts on
// UTC instants.
const (
	imfFixdate = "Mon, 02 Jan 2006 15:04:05 GMT"
	rfc850Date = "Monday, 02-Jan-06 15:04:05 GMT"
	asctime    = "Mon Jan _2 15:04:05 2006"
)

// HTTP dates use the year semantics of RFC 5322 §3.3, which RFC 9110
// §5.6.7 adopts: a year is 1900 or later.
var (
	httpTimes  = pbt.TimeRange(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC))
	rfc850Days = pbt.TimeRange(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2049, 12, 31, 23, 59, 59, 0, time.UTC))
)

func formatted(name string, ts pbt.Gen[time.Time], layout string) pbt.Gen[string] {
	return pbt.Custom(name, func(t *pbt.T) string { return ts.Draw(t, "").Truncate(time.Second).Format(layout) })
}

// HTTPDate generates dates in the IMF-fixdate syntax of RFC 9110 §5.6.7,
// such as "Sun, 06 Nov 1994 08:49:37 GMT", the form a sender must write,
// for instants from 1900, the first year RFC 5322 §3.3 allows, to 9999.
// Values shrink toward "Thu, 01 Jan 1970 00:00:00 GMT".
func HTTPDate() pbt.Gen[string] {
	return formatted("HTTPDate()", httpTimes, imfFixdate)
}

// RFC9110HTTPDate generates dates in the three forms a recipient of RFC
// 9110 §5.6.7 must accept: IMF-fixdate, the obsolete RFC 850 form
// ("Sunday, 06-Nov-94 08:49:37 GMT") and the asctime form ("Sun Nov  6
// 08:49:37 1994"), and in one value in seventeen an IMF-fixdate at one of
// the 27 leap seconds, 23:59:60, which time-of-day allows. RFC 850 dates
// fall in 2000 to 2049: §5.6.7 has a recipient read a two-digit year that
// looks more than 50 years ahead as a year of the past century, so every
// recipient since 2000 reads those years the same way, and the day of the
// week is the day of that date. The other forms have years from 1900 to
// 9999. Go's time package cannot write second 60, and time.Parse (Go
// 1.26) rejects it. Values shrink toward the IMF-fixdate of the Unix epoch.
func RFC9110HTTPDate() pbt.Gen[string] {
	return named(pbt.Frequency(
		pbt.W(8, formatted("IMF-fixdate", httpTimes, imfFixdate)),
		pbt.W(4, formatted("rfc850-date", rfc850Days, rfc850Date)),
		pbt.W(4, formatted("asctime-date", httpTimes, asctime)),
		pbt.W(1, pbt.Custom("leap second", func(t *pbt.T) string {
			v := leapSeconds[leapIndex.Draw(t, "")]
			return strings.Replace(v.Format(imfFixdate), ":59:00 GMT", ":59:60 GMT", 1)
		})),
	), "RFC9110HTTPDate()")
}

var (
	monthNames = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	dayNames   = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	nameCase   = choice(18, 1, 1) // as written, lower case, upper case
	twoDigits  = choice(1, 1)     // the day with one digit when it has one, with two
	years5322  = pbt.IntRangeToward(1900, 9999, 1970)
	years5     = pbt.IntRange(10000, 99999)
	years1900  = pbt.Frequency(pbt.W(19, years5322), pbt.W(1, years5)) // four digits, five
	present    = pbt.Frequency(pbt.W(1, pbt.Just(false)), pbt.W(3, pbt.Just(true)))
)

// wsp generates one to three spaces and tabs.
var wsp = stringIn(" \t", 1, 3)

// fws generates folding white space of RFC 5322 §3.2.2: one space in most
// cases, a tab, a run of spaces and tabs, or a line break folded onto the
// next line, CRLF followed by white space.
var fws = pbt.Frequency(
	pbt.W(12, pbt.Just(" ")),
	pbt.W(1, pbt.Just("\t")),
	pbt.W(1, wsp),
	pbt.W(1, pbt.Map(wsp, func(s string) string { return "\r\n" + s })),
)

// ctext of RFC 5322 §3.2.2: printable ASCII without the parentheses and the
// backslash.
const ctext = alnum + "!\"#$%&'*+,-./:;<=>?@[]^_`{|}~"

// comment generates the comments of RFC 5322 §3.2.2, nested up to two
// levels, with ctext, quoted pairs and folding white space.
var comment = commentOf(2)

func commentOf(depth int) pbt.Gen[string] {
	items := []pbt.Weighted[string]{
		pbt.W(8, pbt.Map(charIn(ctext), func(c byte) string { return string(c) })),
		pbt.W(1, pbt.Map(charIn(vchar+" \t"), func(c byte) string { return `\` + string(c) })),
		pbt.W(1, fws),
	}
	if depth > 0 {
		items = append(items, pbt.W(1, commentOf(depth-1)))
	}
	content := pbt.SliceOfN(pbt.Frequency(items...), 0, 8)
	return pbt.Custom("comment", func(t *pbt.T) string {
		return "(" + joinWS(content.Draw(t, "")) + ")"
	})
}

var (
	maybeFWS = optional(1, 1, fws)
	comments = pbt.SliceOfN(concat(maybeFWS, comment), 1, 3)
)

// cfws generates CFWS of RFC 5322 §3.2.2: comments with optional folding
// white space between them, or folding white space alone.
var cfws = pbt.Frequency(
	pbt.W(2, pbt.Custom("cfws", func(t *pbt.T) string {
		return strings.Join(comments.Draw(t, ""), "") + maybeFWS.Draw(t, "")
	})),
	pbt.W(1, fws),
)

// rfc5322Zone generates the zone of RFC 5322 §3.3: a quarter-hour offset in
// current use, +0000, -0000, or any hour from 00 to 99 and minute from 00
// to 59, the range §3.3 allows.
var rfc5322Zone = pbt.Frequency(
	pbt.W(4, pbt.Custom("zone", func(t *pbt.T) string {
		q := quarterHour.Draw(t, "")
		s := "+"
		if q < 0 {
			s, q = "-", -q
		}
		return fmt.Sprintf("%s%02d%02d", s, q/4, q%4*15)
	})),
	pbt.W(2, pbt.Just("+0000")),
	pbt.W(1, pbt.Just("-0000")),
	pbt.W(1, pbt.Custom("zone", func(t *pbt.T) string {
		return fmt.Sprintf("%s%02d%02d", sign.Draw(t, ""), pbt.IntRange(0, 99).Draw(t, ""), minutes.Draw(t, ""))
	})),
)

// dateTime5322 is a date and time drawn for RFC5322DateTime.
type dateTime5322 struct {
	y, m, d, hh, mm, ss int
	zone                string
	leap                bool
}

var dateTimes5322 = pbt.Frequency(
	pbt.W(19, pbt.Custom("date-time", func(t *pbt.T) dateTime5322 {
		var v dateTime5322
		v.y, v.m, v.d = calendarDate(t, years1900, 1900, 99999)
		v.hh, v.mm, v.ss = hours.Draw(t, ""), minutes.Draw(t, ""), seconds.Draw(t, "")
		v.zone = rfc5322Zone.Draw(t, "")
		return v
	})),
	pbt.W(1, pbt.Custom("leap second", func(t *pbt.T) dateTime5322 {
		zone := rfc5322Zone.Draw(t, "")
		l := leapLocal(t, zone)
		return dateTime5322{l.Year(), int(l.Month()), l.Day(), l.Hour(), l.Minute(), 60, zone, true}
	})),
)

// RFC5322DateTime generates dates in the date-time syntax of RFC 5322
// §3.3, as e-mail writes them in the Date header, such as "Fri, 21 Nov
// 1997 09:55:06 -0600". It covers the optional day of the week, which is
// the day of the date; days of one or two digits; years from 1900, the
// first §3.3 allows, with four digits and in one case in twenty five;
// optional seconds; in one value in twenty a leap second, 23:59:60 UTC of
// one of the 27 that were inserted, shown in the local time of its zone;
// the zones +0000 and -0000 and offsets of any hour from 00 to 99 and
// minute from 00 to 59; folding white space of spaces, tabs and folded
// lines between the parts; a trailing CFWS with comments in one value in
// five; and day and month names in any letter case, since ABNF strings are
// case-insensitive. The obsolete syntax of §4, which §4 forbids generators
// to produce, is left out. Values shrink toward "1 Jan 1970 00:00 +0000",
// and a leap second toward "30 Jun 1972 23:59:60 +0000".
//
// net/mail.ParseDate (Go 1.26) rejects some of these: a tab after the
// comma, the leap second, five-digit years and zones beyond +2400 such as
// +9959. It accepts folded lines before the zone and trailing comments.
func RFC5322DateTime() pbt.Gen[string] {
	trailing := optional(4, 1, cfws)
	return pbt.Custom("RFC5322DateTime()", func(t *pbt.T) string {
		withDay := present.Draw(t, "")
		withSeconds := present.Draw(t, "")
		v := dateTimes5322.Draw(t, "")
		var b strings.Builder
		if withDay {
			b.WriteString(caseOf(t, dayNames[time.Date(v.y, time.Month(v.m), v.d, 0, 0, 0, 0, time.UTC).Weekday()]))
			b.WriteString("," + fws.Draw(t, ""))
		}
		if twoDigits.Draw(t, "") == 1 {
			fmt.Fprintf(&b, "%02d", v.d)
		} else {
			fmt.Fprintf(&b, "%d", v.d)
		}
		b.WriteString(fws.Draw(t, "") + caseOf(t, monthNames[v.m-1]) + fws.Draw(t, ""))
		fmt.Fprintf(&b, "%04d%s%02d:%02d", v.y, fws.Draw(t, ""), v.hh, v.mm)
		if withSeconds || v.leap {
			fmt.Fprintf(&b, ":%02d", v.ss)
		}
		b.WriteString(fws.Draw(t, "") + v.zone + trailing.Draw(t, ""))
		return b.String()
	})
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
