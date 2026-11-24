// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

// rfc3339Re is the date-time production of RFC 3339 §5.6 with the
// alternatives of §5.6's note on letter case.
var rfc3339Re = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})[Tt](\d{2}):(\d{2}):(\d{2})(\.\d+)?([Zz]|[+-](\d{2}):(\d{2}))$`)

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
	case d < 1 || d > daysIn(y, mo):
		return "day out of range"
	case hh > 23 || mm > 59 || ss > 60:
		return "time out of range"
	case m[9] != "" && (n(9) > 23 || n(10) > 59):
		return "offset out of range"
	}
	if ss == 60 {
		// The UTC equivalent must be 23:59:60 on June 30 or December 31.
		off := 0
		if m[9] != "" {
			off = n(9)*60 + n(10)
			if m[8][0] == '-' {
				off = -off
			}
		}
		utc := time.Date(y, time.Month(mo), d, hh, mm, 59, 0, time.UTC).Add(-time.Duration(off) * time.Minute)
		if utc.Hour() != 23 || utc.Minute() != 59 || !((utc.Month() == 6 && utc.Day() == 30) || (utc.Month() == 12 && utc.Day() == 31)) {
			return "leap second at " + utc.String()
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
		pt.Cover(m[6] == "60", 1, "leap second")
		pt.Cover(m[7] != "", 30, "fraction")
		pt.Cover(len(m[7]) > 10, 1, "fraction beyond nanoseconds")
		pt.Cover(strings.HasSuffix(s, "-00:00"), 3, "unknown local offset")
		pt.Cover(m[2] == "02" && m[3] == "29", 1, "February 29")
		pt.Cover(m[1] == "0000", 1, "year 0000")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, RFC3339(), always); got != "1970-01-01T00:00:00Z" {
		t.Errorf("shrinks to %q", got)
	}
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
	for _, s := range []string{"0000-01-01T00:00:00Z", "1985-04-12T23:20:50.123456789012Z", "1985-04-12T23:20:50-00:00", "1985-04-12T23:20:50+23:59"} {
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			t.Errorf("time.Parse now rejects %s: %v; update the godoc of RFC3339", s, err)
		}
	}
}

func TestRFC3339FullDateAndTime(t *testing.T) {
	dateRe := regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
	checkValid(t, RFC3339FullDate(), func(s string) string {
		m := dateRe.FindStringSubmatch(s)
		if m == nil {
			return "not a full-date"
		}
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		if mo < 1 || mo > 12 || d < 1 || d > daysIn(y, mo) {
			return "out of range"
		}
		return ""
	})
	checkValid(t, RFC3339FullTime(), func(s string) string {
		return checkRFC3339("2000-01-01T" + s)
	})
	if got := minimalOf(t, RFC3339FullDate(), always); got != "1970-01-01" {
		t.Errorf("full-date shrinks to %q", got)
	}
	if got := minimalOf(t, RFC3339FullTime(), always); got != "00:00:00Z" {
		t.Errorf("full-time shrinks to %q", got)
	}
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
	if got := minimalOf(t, Time(time.RFC1123), always); got != "Thu, 01 Jan 1970 00:00:00 UTC" {
		t.Errorf("shrinks to %q", got)
	}
}

var (
	isoDurationRe     = regexp.MustCompile(`^P(?:\d+(?:[.,]\d+)?W|(?:\d+(?:[.,]\d+)?Y)?(?:\d+(?:[.,]\d+)?M)?(?:\d+(?:[.,]\d+)?D)?(?:T(?:\d+(?:[.,]\d+)?H)?(?:\d+(?:[.,]\d+)?M)?(?:\d+(?:[.,]\d+)?S)?)?)$`)
	rfc3339DurTime    = `T(?:\d+H(?:\d+M(?:\d+S)?)?|\d+M(?:\d+S)?|\d+S)`
	rfc3339DurationRe = regexp.MustCompile(`^P(?:(?:\d+D|\d+M(?:\d+D)?|\d+Y(?:\d+M(?:\d+D)?)?)(?:` + rfc3339DurTime + `)?|` + rfc3339DurTime + `|\d+W)$`)
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
		pt.Cover(s == strings.Join([]string{"P", "Y", "M", "D", "T", "H", "M", "S"}, "") || strings.Count(s, "M") == 2, 1, "months and minutes")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, ISO8601Duration(), always); got != "P0W" {
		t.Errorf("shrinks to %q", got)
	}
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
		pt.Cover(strings.HasPrefix(s, "PT"), 10, "time only")
		pt.Cover(strings.Contains(s, "Y") && strings.Contains(s, "D"), 3, "years to days")
		pt.Cover(strings.Contains(s, "H") && strings.Contains(s, "S"), 3, "hours to seconds")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, RFC3339Duration(), always); got != "P0W" {
		t.Errorf("shrinks to %q", got)
	}
}

func TestHTTPDate(t *testing.T) {
	checkValid(t, HTTPDate(), func(s string) string {
		v, err := time.Parse(imfFixdate, s)
		if err != nil || v.Format(imfFixdate) != s {
			return "not an IMF-fixdate"
		}
		return ""
	})
	forms := regexp.MustCompile(`^(?:(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun), \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT|(?:Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday), \d{2}-[A-Z][a-z]{2}-\d{2} \d{2}:\d{2}:\d{2} GMT|(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun) [A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2} \d{4})$`)
	checkValid(t, RFC9110HTTPDate(), func(s string) string {
		if !forms.MatchString(s) {
			return "not one of the three forms"
		}
		for _, layout := range []string{imfFixdate, rfc850Date, asctime} {
			if _, err := time.Parse(layout, s); err == nil {
				return ""
			}
		}
		return "no layout parses it"
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC9110HTTPDate().Draw(pt, "s")
		pt.Cover(strings.Contains(s, "-"), 15, "RFC 850")
		pt.Cover(!strings.Contains(s, ","), 15, "asctime")
	}, pbt.Seed(1), pbt.Checks(1000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, HTTPDate(), always); got != "Thu, 01 Jan 1970 00:00:00 GMT" {
		t.Errorf("shrinks to %q", got)
	}
}

var rfc5322DateRe = regexp.MustCompile(`^(?:(?i:Mon|Tue|Wed|Thu|Fri|Sat|Sun),[ \t]+)?(\d{1,2})[ \t]+(?i:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[ \t]+(\d{4,})[ \t]+(\d{2}):(\d{2})(?::(\d{2}))?[ \t]+[+-](\d{2})(\d{2})$`)

func TestRFC5322DateTime(t *testing.T) {
	checkValid(t, RFC5322DateTime(), func(s string) string {
		m := rfc5322DateRe.FindStringSubmatch(s)
		if m == nil {
			return "not an RFC 5322 date-time"
		}
		n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
		if n(2) < 1900 || n(3) > 23 || n(4) > 59 || n(7) > 59 {
			return "out of range"
		}
		if m[5] == "60" && (m[3] != "23" || m[4] != "59") {
			return "leap second outside 23:59"
		}
		if n(5) > 60 {
			return "second out of range"
		}
		if comma := strings.Index(s, ","); comma > 0 {
			year, _ := strconv.Atoi(m[2])
			mo := strings.Index("janfebmaraprmayjunjulaugsepoctnovdec", strings.ToLower(strings.Fields(s[comma+1:])[1]))/3 + 1
			if wd := time.Date(year, time.Month(mo), n(1), 0, 0, 0, 0, time.UTC).Weekday().String()[:3]; !strings.EqualFold(wd, s[:3]) {
				return "the day of the week is not the date's"
			}
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5322DateTime().Draw(pt, "s")
		pt.Cover(!strings.Contains(s, ","), 10, "no day of the week")
		pt.Cover(strings.Contains(s, "\t"), 3, "tab")
		pt.Cover(strings.Contains(s, ":60 "), 1, "leap second")
		pt.Cover(strings.HasSuffix(s, "-0000"), 3, "-0000")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, RFC5322DateTime(), always); got != "1 Jan 1970 00:00 +0000" {
		t.Errorf("shrinks to %q", got)
	}
}

// TestGoMailDateDivergences pins what RFC5322DateTime's godoc says about
// net/mail.ParseDate.
func TestGoMailDateDivergences(t *testing.T) {
	for _, s := range []string{"Fri,\t21 Nov 1997 09:55:06 -0600", "31 Dec 1998 23:59:60 +0000", "1 Jan 12345 00:00 +0000", "1 Jan 1970 00:00 +9959"} {
		if rfc5322DateRe.FindStringSubmatch(s) == nil {
			t.Fatalf("%q is not valid RFC 5322", s)
		}
		if _, err := mail.ParseDate(s); err == nil {
			t.Errorf("net/mail.ParseDate now accepts %q; update the godoc of RFC5322DateTime", s)
		}
	}
}
