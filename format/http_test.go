// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

func checkCookiePair(s string) string {
	name, v, ok := strings.Cut(s, "=")
	if !ok || name == "" || takeHTTPToken(name) != name {
		return "cookie name"
	}
	if strings.HasPrefix(v, "\"") {
		if len(v) < 2 || v[len(v)-1] != '"' {
			return "cookie quote"
		}
		v = v[1 : len(v)-1]
	}
	for i := range len(v) {
		c := v[i]
		if !(c == 0x21 || c >= 0x23 && c <= 0x2b || c >= 0x2d && c <= 0x3a || c >= 0x3c && c <= 0x5b || c >= 0x5d && c <= 0x7e) {
			return "cookie octet"
		}
	}
	return ""
}
func checkCookie(s string, set bool) string {
	parts := strings.Split(s, "; ")
	if msg := checkCookiePair(parts[0]); msg != "" {
		return msg
	}
	for _, part := range parts[1:] {
		if !set {
			if msg := checkCookiePair(part); msg != "" {
				return msg
			}
			continue
		}
		for i := range len(part) {
			if part[i] < 32 || part[i] > 126 || part[i] == ';' {
				return "cookie-av octet"
			}
		}
	}
	return ""
}
func TestHTTPFields(t *testing.T) {
	for _, g := range []pbt.Gen[string]{HTTPToken(), HTTPFieldName()} {
		checkMinimumSeeds(t, g, "a")
		pbt.Check(t, func(pt *pbt.T) {
			s := g.Draw(pt, "")
			for _, c := range "!#$%&' *+-.^_`|~" {
				if c == ' ' {
					continue
				}
				pt.Cover(strings.ContainsRune(s, c), 1, string(c))
			}
			pt.Cover(strings.ContainsAny(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZ"), 10, "upper case")
		}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
		checkValid(t, g, func(s string) string {
			if s == "" || takeHTTPToken(s) != s {
				return "not token"
			}
			return ""
		})
		if s := minimalOf(t, g, always); s != "a" {
			t.Errorf("minimum %q", s)
		}
	}
	for _, g := range []pbt.Gen[string]{HTTPFieldValue(), RFC9110FieldValue()} {
		checkMinimumSeeds(t, g, "")
		checkValid(t, g, func(s string) string {
			for i := range len(s) {
				c := s[i]
				if c == 32 || c == 9 {
					if i == 0 || i == len(s)-1 {
						return "edge whitespace"
					}
					continue
				}
				if c < 33 || c == 127 || g.String() == "HTTPFieldValue()" && c > 126 {
					return "field-vchar"
				}
			}
			return ""
		})
		if s := minimalOf(t, g, always); s != "" {
			t.Errorf("minimum %q", s)
		}
		pbt.Check(t, func(pt *pbt.T) {
			s := g.Draw(pt, "")
			obs := false
			for i := range len(s) {
				obs = obs || s[i] >= 128
			}
			pt.Cover(strings.ContainsAny(s, " \t"), 5, "interior whitespace")
			if g.String() == "RFC9110FieldValue()" {
				pt.Cover(obs, 60, "obs-text")
			}
		}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	}
}
func TestCookies(t *testing.T) {
	for _, g := range []pbt.Gen[string]{RFC6265Cookie(), RFC6265SetCookie()} {
		t.Run(g.String(), func(t *testing.T) {
			set := g.String() == "RFC6265SetCookie()"
			checkMinimumSeeds(t, g, "a=")
			checkValid(t, g, func(s string) string {
				if msg := checkCookie(s, set); msg != "" {
					return msg
				}
				if set {
					if _, e := http.ParseSetCookie(s); e != nil {
						return e.Error()
					}
				} else {
					if _, e := http.ParseCookie(s); e != nil {
						return e.Error()
					}
				}
				return ""
			})
			pbt.Check(t, func(pt *pbt.T) {
				s := g.Draw(pt, "")
				pt.Cover(strings.Contains(strings.Split(s, "; ")[0], "=\""), 30, "quoted pair")
				pt.Cover(strings.Contains(s, "; "), 50, "multiple parts")
				if set {
					lo := strings.ToLower(s)
					for _, a := range []string{"expires=", "max-age=", "domain=", "path=", "secure", "httponly"} {
						pt.Cover(strings.Contains(lo, "; "+a), 10, a)
					}
				}
			}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
			if s := minimalOf(t, g, always); s != "a=" {
				t.Errorf("minimum %q", s)
			}
		})
	}
	if s := minimalOf(t, RFC6265Cookie(), func(s string) bool { return strings.Contains(strings.Split(s, "; ")[0], "=\"") }); s != `a=""` {
		t.Errorf("quoted minimum %q", s)
	}
	checkValid(t, cookieDate, checkCookieDate)
	// Check structured attributes independently, before extension-av could
	// mask an invalid structured production in the union's grammar.
	checkValid(t, cookieMaxAge, func(s string) string {
		if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(s) {
			return "max-age"
		}
		return ""
	})
	checkValid(t, cookieDomain, func(s string) string {
		if len(s) > 253 {
			return "domain length"
		}
		for _, l := range strings.Split(s, ".") {
			if !regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`).MatchString(l) {
				return "domain label"
			}
		}
		return ""
	})
}
func TestCookieParserDifferences(t *testing.T) {
	for _, tc := range []struct {
		s        string
		max      int
		unparsed int
	}{
		{"a=b; Max-Age=0", -1, 0}, {"a=b; Max-Age=-1", -1, 0},
		{"a=b; Max-Age=99999999999999999999", 0, 1},
		{`a=b; Path=a\b`, 0, 1}, {`a=b; Path=a"b`, 0, 1},
	} {
		c, e := http.ParseSetCookie(tc.s)
		if e != nil {
			t.Fatal(e)
		}
		if c.MaxAge != tc.max || len(c.Unparsed) != tc.unparsed {
			t.Errorf("%q: %#v", tc.s, c)
		}
	}
	for _, s := range []string{`a="b"`, "a=b; a=c", "a=", "a=b; c=d"} {
		if _, e := http.ParseCookie(s); e != nil {
			t.Errorf("%q: %v", s, e)
		}
	}
}

func checkCookieDate(s string) string {
	// rfc1123-date of RFC 2616 §3.3.1, which is case-sensitive.
	re := regexp.MustCompile(`^(Mon|Tue|Wed|Thu|Fri|Sat|Sun), ([0-9]{2}) (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) ([0-9]{4}) ([0-9]{2}):([0-9]{2}):([0-9]{2}) GMT$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return "sane-cookie-date syntax"
	}
	d, _ := strconv.Atoi(m[2])
	year, _ := strconv.Atoi(m[4])
	hh, _ := strconv.Atoi(m[5])
	mm, _ := strconv.Atoi(m[6])
	ss, _ := strconv.Atoi(m[7])
	month := time.Month(strings.Index("janfebmaraprmayjunjulaugsepoctnovdec", strings.ToLower(m[3]))/3 + 1)
	date := time.Date(year, month, d, hh, mm, ss, 0, time.UTC)
	if date.Day() != d || date.Month() != month || hh > 23 || mm > 59 || ss > 59 || !strings.EqualFold(date.Weekday().String()[:3], m[1]) {
		return "calendar/time"
	}
	return ""
}
