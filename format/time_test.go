// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/mail"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

// leapDays lists the UTC days that ended with 23:59:60, from IERS
// Bulletin C, written out again for the tests.
var leapDays = strings.Fields(`
	1972-06-30 1972-12-31 1973-12-31 1974-12-31 1975-12-31 1976-12-31 1977-12-31 1978-12-31 1979-12-31
	1981-06-30 1982-06-30 1983-06-30 1985-06-30 1987-12-31 1989-12-31 1990-12-31 1992-06-30 1993-06-30
	1994-06-30 1995-12-31 1997-06-30 1998-12-31 2005-12-31 2008-12-31 2012-06-30 2015-06-30 2016-12-31`)

// isLeapSecond reports whether the local time y-m-d hh:mm:60 at the given
// offset in minutes is 23:59:60 UTC on a day of leapDays.
func isLeapSecond(y, m, d, hh, mm, off int) bool {
	utc := time.Date(y, time.Month(m), d, hh, mm, 0, 0, time.UTC).Add(-time.Duration(off) * time.Minute)
	return utc.Hour() == 23 && utc.Minute() == 59 && slices.Contains(leapDays, utc.Format(time.DateOnly))
}

// rfc3339Re is the date-time production of RFC 3339 §5.6 with the
// alternatives of §5.6's note on letter case.
var rfc3339Re = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})[Tt](\d{2}):(\d{2}):(\d{2})(\.\d+)?([Zz]|([+-])(\d{2}):(\d{2}))$`)

// checkRFC3339 checks the syntax and the restrictions of §5.7.
func checkRFC3339(s string) string {
	m := rfc3339Re.FindStringSubmatch(s)
	if m == nil {
		return "not an RFC 3339 date-time"
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	y, mo, d, hh, mm, ss := n(1), n(2), n(3), n(4), n(5), n(6)
	switch {
	case mo < 1 || mo > 12:
		return "month out of range"
	case d < 1 || d > time.Date(y, time.Month(mo)+1, 0, 0, 0, 0, 0, time.UTC).Day():
		return "day out of range"
	case hh > 23 || mm > 59 || ss > 60:
		return "time out of range"
	case m[9] != "" && (n(10) > 23 || n(11) > 59):
		return "offset out of range"
	}
	if ss == 60 {
		off := 0
		if m[9] != "" {
			off = n(10)*60 + n(11)
			if m[9] == "-" {
				off = -off
			}
		}
		if !isLeapSecond(y, mo, d, hh, mm, off) {
			return "second 60 where no leap second was inserted"
		}
	}
	return ""
}

func TestRFC3339(t *testing.T) {
	checkValid(t, RFC3339(), checkRFC3339)
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC3339().Draw(pt, "s")
		m := rfc3339Re.FindStringSubmatch(s)
		pt.Cover(strings.ContainsAny(s, "tz"), 3, "lower-case letter")
		pt.Cover(m[6] == "60", 2, "leap second")
		pt.Cover(m[6] == "60" && m[8] != "Z" && m[8] != "z" && m[8] != "-00:00" && m[8] != "+00:00", 1, "leap second in local time")
		pt.Cover(m[7] != "", 20, "fraction")
		pt.Cover(len(m[7]) > 10, 1, "fraction beyond nanoseconds")
		pt.Cover(strings.HasSuffix(s, "-00:00"), 3, "unknown local offset")
		pt.Cover(m[2] == "02" && m[3] == "29", 1, "February 29")
		pt.Cover(m[1] == "0000", 1, "year 0000")
		pt.Cover(m[1] == "9999", 1, "year 9999")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC3339(), always, "1970-01-01T00:00:00Z", "1972-06-30T23:59:60Z")
}

// TestGoRFC3339Divergences pins what RFC3339's godoc says about
// time.Parse, so that a change in Go shows up here.
func TestGoRFC3339Divergences(t *testing.T) {
	for _, s := range []string{"1985-04-12t23:20:50Z", "1985-04-12T23:20:50z", "1990-12-31T23:59:60Z"} {
		if checkRFC3339(s) != "" {
			t.Fatalf("%s is not valid RFC 3339", s)
		}
		if _, err := time.Parse(time.RFC3339, s); err == nil {
			t.Errorf("time.Parse now accepts %s; update the godoc of RFC3339", s)
		}
	}
	for _, s := range []string{"0000-01-01T00:00:00Z", "1985-04-12T23:20:50.123456789012Z", "1985-04-12T23:20:50-00:00", "1985-04-12T23:20:50+23:59", "1970-01-01T00:00:00+24:00"} {
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			t.Errorf("time.Parse now rejects %s: %v; update the godoc of RFC3339", s, err)
		}
	}
	if _, err := time.Parse(time.RFC3339, "1970-01-01T00:00:00+25:00"); err == nil {
		t.Error("time.Parse now accepts +25:00; update the godoc of RFC3339")
	}
}

func TestRFC3339FullDateAndTime(t *testing.T) {
	dateRe := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
	checkValid(t, RFC3339FullDate(), func(s string) string {
		if !dateRe.MatchString(s) {
			return "not a full-date"
		}
		return checkRFC3339(s + "T00:00:00Z")
	})
	timeRe := regexp.MustCompile(`^(\d{2}):(\d{2}):(\d{2})(\.\d+)?([Zz]|([+-])(\d{2}):(\d{2}))$`)
	checkValid(t, RFC3339FullTime(), func(s string) string {
		m := timeRe.FindStringSubmatch(s)
		if m == nil {
			return "not a full-time"
		}
		if m[3] != "60" {
			return checkRFC3339("2000-01-01T" + s)
		}
		// A leap second: some leap day must make it 23:59:60 UTC.
		hh, _ := strconv.Atoi(m[1])
		mm, _ := strconv.Atoi(m[2])
		off := 0
		if m[6] != "" {
			h, _ := strconv.Atoi(m[7])
			mi, _ := strconv.Atoi(m[8])
			off = h*60 + mi
			if m[6] == "-" {
				off = -off
			}
		}
		if (hh*60+mm-off+24*60)%(24*60) != 23*60+59 {
			return "second 60 that is not 23:59:60 UTC"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		pt.Cover(strings.Contains(RFC3339FullTime().Draw(pt, "s"), ":60"), 2, "leap second")
	}, pbt.Seed(1), pbt.Checks(1000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC3339FullDate(), always, "1970-01-01")
	shrinksTo(t, RFC3339FullTime(), always, "00:00:00Z", "23:59:60Z")
}

func TestTimeUsesGoLayouts(t *testing.T) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC1123Z, time.RFC822Z, time.Kitchen, time.DateOnly} {
		t.Run(layout, func(t *testing.T) {
			checkValid(t, Time(layout), func(s string) string {
				if _, err := time.Parse(layout, s); err != nil {
					return err.Error()
				}
				return ""
			})
		})
	}
	pbt.Check(t, func(pt *pbt.T) {
		s := Time(time.RFC3339).Draw(pt, "s")
		pt.Cover(strings.HasSuffix(s, "Z"), 10, "UTC")
		pt.Cover(!strings.HasSuffix(s, "Z"), 50, "offset")
	}, pbt.Seed(1), pbt.Checks(1000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, Time(time.RFC1123), always, "Thu, 01 Jan 1970 00:00:00 UTC")
}

var (
	isoDurationRe     = regexp.MustCompile(`^P(?:\d+(?:[.,]\d+)?W|(?:\d+(?:[.,]\d+)?Y)?(?:\d+(?:[.,]\d+)?M)?(?:\d+(?:[.,]\d+)?D)?(?:T(?:\d+(?:[.,]\d+)?H)?(?:\d+(?:[.,]\d+)?M)?(?:\d+(?:[.,]\d+)?S)?)?)$`)
	rfc3339DurTime    = `[Tt](?:\d+[Hh](?:\d+[Mm](?:\d+[Ss])?)?|\d+[Mm](?:\d+[Ss])?|\d+[Ss])`
	rfc3339DurationRe = regexp.MustCompile(`^[Pp](?:(?:\d+[Dd]|\d+[Mm](?:\d+[Dd])?|\d+[Yy](?:\d+[Mm](?:\d+[Dd])?)?)(?:` + rfc3339DurTime + `)?|` + rfc3339DurTime + `|\d+[Ww])$`)
	fractionRe        = regexp.MustCompile(`[.,]\d+`)
)

func TestISO8601Duration(t *testing.T) {
	checkValid(t, ISO8601Duration(), func(s string) string {
		switch {
		case !isoDurationRe.MatchString(s):
			return "not an ISO 8601 duration"
		case s == "P" || strings.HasSuffix(s, "T"):
			return "no component"
		}
		if loc := fractionRe.FindAllStringIndex(s, -1); len(loc) > 1 || (len(loc) == 1 && loc[0][1] != len(s)-1) {
			return "fraction not on the lowest-order component"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := ISO8601Duration().Draw(pt, "s")
		pt.Cover(strings.HasSuffix(s, "W"), 5, "weeks")
		pt.Cover(strings.Contains(s, "T"), 30, "time part")
		pt.Cover(strings.Contains(s, ","), 3, "decimal comma")
		pt.Cover(strings.Count(s, "M") == 2, 1, "months and minutes")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, ISO8601Duration(), always, "P0W", "P0D")
}

func TestRFC3339Duration(t *testing.T) {
	checkValid(t, RFC3339Duration(), func(s string) string {
		if !rfc3339DurationRe.MatchString(s) {
			return "not in the grammar of RFC 3339 Appendix A"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC3339Duration().Draw(pt, "s")
		pt.Cover(strings.HasPrefix(strings.ToUpper(s), "PT"), 10, "time only")
		pt.Cover(strings.ContainsAny(s, "Yy") && strings.ContainsAny(s, "Dd"), 3, "years to days")
		pt.Cover(strings.ContainsAny(s, "Hh") && strings.ContainsAny(s, "Ss"), 3, "hours to seconds")
		pt.Cover(regexp.MustCompile(`(^|[^0-9])0[0-9]`).MatchString(s), 5, "leading zero")
		pt.Cover(strings.ContainsAny(s, "ptwymdhs"), 20, "lower-case designator")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC3339Duration(), always, "P0W", "P0Y", "PT0H")
}

// The three forms of RFC 9110 §5.6.7, written from its ABNF.
var (
	imfRe    = regexp.MustCompile(`^(Mon|Tue|Wed|Thu|Fri|Sat|Sun), (\d{2}) (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) (\d{4}) (\d{2}):(\d{2}):(\d{2}) GMT$`)
	rfc850Re = regexp.MustCompile(`^(Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday), (\d{2})-(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)-(\d{2}) (\d{2}):(\d{2}):(\d{2}) GMT$`)
	ascRe    = regexp.MustCompile(`^(Mon|Tue|Wed|Thu|Fri|Sat|Sun) (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) ([ \d]\d) (\d{2}):(\d{2}):(\d{2}) (\d{4})$`)
	months3  = "JanFebMarAprMayJunJulAugSepOctNovDec"
)

// checkHTTPDate checks an HTTP-date and the semantics of §5.6.7: the day
// of the week of the date, years from 1900, the two-digit years of RFC 850
// read as 2000 to 2049, and second 60 only at a leap second.
func checkHTTPDate(s string) string {
	var wd string
	var y, mo, d, hh, mm, ss int
	n := func(v string) int { i, _ := strconv.Atoi(strings.TrimSpace(v)); return i }
	switch m := imfRe.FindStringSubmatch(s); {
	case m != nil:
		wd, d, mo, y, hh, mm, ss = m[1], n(m[2]), strings.Index(months3, m[3])/3+1, n(m[4]), n(m[5]), n(m[6]), n(m[7])
	default:
		if m = rfc850Re.FindStringSubmatch(s); m != nil {
			wd, d, mo, y, hh, mm, ss = m[1][:3], n(m[2]), strings.Index(months3, m[3])/3+1, 2000+n(m[4]), n(m[5]), n(m[6]), n(m[7])
			if y > 2049 {
				return "RFC 850 year outside 2000-2049"
			}
		} else if m = ascRe.FindStringSubmatch(s); m != nil {
			wd, mo, d, hh, mm, ss, y = m[1], strings.Index(months3, m[2])/3+1, n(m[3]), n(m[4]), n(m[5]), n(m[6]), n(m[7])
		} else {
			return "not an HTTP-date"
		}
	}
	date := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	switch {
	case y < 1900:
		return "year before 1900"
	case date.Day() != d:
		return "no such day"
	case date.Weekday().String()[:3] != wd:
		return "the day of the week is not the date's"
	case hh > 23 || mm > 59 || ss > 60:
		return "time out of range"
	case ss == 60 && !isLeapSecond(y, mo, d, hh, mm, 0):
		return "second 60 where no leap second was inserted"
	}
	return ""
}

func TestHTTPDate(t *testing.T) {
	checkValid(t, HTTPDate(), func(s string) string {
		if !imfRe.MatchString(s) {
			return "not an IMF-fixdate"
		}
		return checkHTTPDate(s)
	})
	checkValid(t, RFC9110HTTPDate(), checkHTTPDate)
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC9110HTTPDate().Draw(pt, "s")
		pt.Cover(rfc850Re.MatchString(s), 15, "RFC 850")
		pt.Cover(ascRe.MatchString(s), 15, "asctime")
		pt.Cover(strings.Contains(s, ":60 "), 2, "leap second")
	}, pbt.Seed(1), pbt.Checks(1000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, HTTPDate(), always, "Thu, 01 Jan 1970 00:00:00 GMT")
	shrinksTo(t, RFC9110HTTPDate(), always, "Thu, 01 Jan 1970 00:00:00 GMT", "Saturday, 01-Jan-00 00:00:00 GMT",
		"Thu Jan  1 00:00:00 1970", "Fri, 30 Jun 1972 23:59:60 GMT")
}

// checkRFC5322DateTime parses date-time of RFC 5322 §3.3 and checks its
// semantics: the day of the week, the day of the month, years from 1900,
// zones up to 99 hours 59 minutes, and second 60 only at a leap second.
func checkRFC5322DateTime(s string) string {
	p := &parser5322{s: s}
	word := func(n int) string {
		if p.i+n > len(s) {
			p.bad = true
			return ""
		}
		w := s[p.i : p.i+n]
		p.i += n
		return w
	}
	num := func(minDigits, maxDigits int) int {
		start := p.i
		for p.i < len(s) && p.i-start < maxDigits && s[p.i] >= '0' && s[p.i] <= '9' {
			p.i++
		}
		if p.i-start < minDigits {
			p.bad = true
		}
		v, _ := strconv.Atoi(s[start:p.i])
		return v
	}
	mustFWS := func() {
		if !p.fws() {
			p.bad = true
		}
	}
	p.fws()
	var dow string
	if p.i+3 < len(s) && s[p.i] > '9' {
		dow = word(3)
		if !p.in(",") {
			return "no comma after the day of the week"
		}
		p.fws()
	}
	d := num(1, 2)
	mustFWS()
	mo := strings.Index(strings.ToLower(months3), strings.ToLower(word(3)))/3 + 1
	mustFWS()
	y := num(4, 9)
	mustFWS()
	hh := num(2, 2)
	p.in(":")
	mm := num(2, 2)
	ss := 0
	if p.in(":") {
		ss = num(2, 2)
	}
	mustFWS()
	signs := word(1)
	zh, zm := num(2, 2), num(2, 2)
	p.cfws()
	if p.bad || p.i != len(s) || mo == 0 || (signs != "+" && signs != "-") {
		return "not an RFC 5322 date-time"
	}
	date := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	off := zh*60 + zm
	if signs == "-" {
		off = -off
	}
	switch {
	case y < 1900:
		return "year before 1900"
	case date.Day() != d:
		return "no such day"
	case dow != "" && !strings.EqualFold(date.Weekday().String()[:3], dow):
		return "the day of the week is not the date's"
	case hh > 23 || mm > 59 || ss > 60 || zm > 59:
		return "out of range"
	case ss == 60 && !isLeapSecond(y, mo, d, hh, mm, off):
		return "second 60 where no leap second was inserted"
	}
	return ""
}

func TestRFC5322DateTime(t *testing.T) {
	checkValid(t, RFC5322DateTime(), checkRFC5322DateTime)
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5322DateTime().Draw(pt, "s")
		pt.Cover(!strings.Contains(s, ","), 10, "no day of the week")
		pt.Cover(strings.Contains(s, "\t"), 3, "tab")
		pt.Cover(strings.Contains(s, "\r\n"), 3, "folded line")
		pt.Cover(strings.Contains(s, "("), 5, "comment")
		pt.Cover(strings.Contains(s, ":60"), 2, "leap second")
		pt.Cover(strings.Contains(s, "-0000"), 3, "-0000")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, RFC5322DateTime(), always, "1 Jan 1970 00:00 +0000", "30 Jun 1972 23:59:60 +0000")
}

// TestGoMailDateDivergences pins what RFC5322DateTime's godoc says about
// net/mail.ParseDate.
func TestGoMailDateDivergences(t *testing.T) {
	for _, s := range []string{
		"Fri,\t21 Nov 1997 09:55:06 -0600",
		"31 Dec 1998 23:59:60 +0000",
		"1 Jan 12345 00:00 +0000",
		"1 Jan 1970 00:00 +9959",
	} {
		if msg := checkRFC5322DateTime(s); msg != "" {
			t.Fatalf("%q is not valid RFC 5322: %s", s, msg)
		}
		if _, err := mail.ParseDate(s); err == nil {
			t.Errorf("net/mail.ParseDate now accepts %q; update the godoc of RFC5322DateTime", s)
		}
	}
	for _, s := range []string{"1 Jan 1970 00:00 +2400", "Fri, 21 Nov 1997 09:55:06\r\n -0600", "1 Jan 1970 00:00 +0000 (UTC)", "fri, 21 nov 1997 09:55:06 -0600"} {
		if _, err := mail.ParseDate(s); err != nil {
			t.Errorf("net/mail.ParseDate now rejects %q: %v; update the godoc of RFC5322DateTime", s, err)
		}
	}
}
