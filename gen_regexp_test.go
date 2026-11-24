// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

var matchingPatterns = []string{
	`[a-z]{2,5}`,
	`\d+`,
	`(foo|bar)+baz`,
	`(?i)hello, world`,
	`\p{Greek}{3}`,
	`[^\n]*`,
	`a?b*c+`,
	`(ab){2,}`,
	`^\w+$`,
	`x{0}y`,
	`(?s).{3}`,
	`(?m)^a$\n^b$`,
	`[\p{Han}\p{Hiragana}]+`,
	`[[:alpha:][:digit:]_-]{1,8}@[a-z]+\.(com|org|jp)`,
	`\bfoo\b`,
	`(a|[^\x00-\x{10FFFF}])z`,
	`[\x{D7FF}-\x{E000}]`,
	`(?:x*)*y`,
	``,
}

func TestStringMatchingMatchesForArbitraryChoices(t *testing.T) {
	for _, p := range matchingPatterns {
		t.Run(p, func(t *testing.T) {
			whole := regexp.MustCompile(`\A(?:` + p + `)\z`)
			forArbitraryChoices(t, StringMatching(p), 2000, func(s string) string {
				switch {
				case !utf8.ValidString(s):
					return "invalid UTF-8"
				case !whole.MatchString(s):
					return "does not match"
				}
				return ""
			})
		})
	}
}

func TestStringMatchingShrinks(t *testing.T) {
	cases := []struct {
		pattern string
		fails   func(string) bool
		want    string
	}{
		{`[a-z]+`, func(string) bool { return true }, "a"},
		{`\d{3}`, func(string) bool { return true }, "000"},
		{`(foo|bar)baz`, func(string) bool { return true }, "foobaz"},
		{`(foo|bar)baz`, func(s string) bool { return strings.HasPrefix(s, "bar") }, "barbaz"},
		{`(?i)abc`, func(string) bool { return true }, "abc"},
		{`[^0-9]{2}`, func(string) bool { return true }, "aa"},
		{`\p{Han}`, func(string) bool { return true }, "⺀"},
		{`[a-z]*`, func(s string) bool { return len(s) >= 3 }, "aaa"},
		{`[a-z]*`, func(s string) bool { return strings.Contains(s, "q") }, "q"},
	}
	for _, c := range cases {
		if got := minimalOf(t, StringMatching(c.pattern), c.fails); got != c.want {
			t.Errorf("%s: got %q, want %q", c.pattern, got, c.want)
		}
	}
}

func TestStringMatchingCovers(t *testing.T) {
	Check(t, func(pt *T) {
		s := StringMatching(`[a-z]{1,10}`).Draw(pt, "s")
		pt.Cover(len(s) == 1, 5, "shortest")
		pt.Cover(len(s) == 10, 5, "longest")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
	Check(t, func(pt *T) {
		s := StringMatching(`.+`).Draw(pt, "s")
		pt.Cover(strings.IndexFunc(s, func(r rune) bool { return r > 0x7f }) >= 0, 20, "non-ASCII")
		pt.Cover(strings.IndexFunc(s, func(r rune) bool { return r > 0x7f }) < 0, 10, "ASCII only")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
	Check(t, func(pt *T) {
		s := StringMatching(`(?i)ab`).Draw(pt, "s")
		pt.Cover(s == "AB", 5, "upper")
		pt.Cover(s == "ab", 5, "lower")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
}

func TestStringMatchingPanics(t *testing.T) {
	for _, c := range []struct{ pattern, want string }{
		{`(`, "missing closing )"},
		{`[^\x00-\x{10FFFF}]`, "matches no string"},
		{`a[^\x00-\x{10FFFF}]+`, "matches no string"},
	} {
		t.Run(c.pattern, func(t *testing.T) {
			defer func() {
				if msg, _ := recover().(string); !strings.Contains(msg, c.want) {
					t.Errorf("panic %q", msg)
				}
			}()
			StringMatching(c.pattern)
		})
	}
}

func TestStringMatchingDiscardsUnsatisfiableAssertions(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), Checks(10), NoSave()})
	r := &runner{tb: nopTB{}, prop: func(pt *T) { StringMatching(`a\bb`).Draw(pt, "s") }, cfg: cfg}
	if _, _, err := r.search(); err == nil || !strings.Contains(err.Error(), "satisfied the assertions") {
		t.Errorf("err %v", err)
	}
}

func TestStringMatchingFoldsAlongTheOrbitOfTheParsedRune(t *testing.T) {
	for _, p := range []string{`(?i)İ`, `(?i)K`, `(?i)ſ`, `(?i)σ`, `(?i)ς`, `(?i)Σ`, `(?i)k`} {
		whole := regexp.MustCompile(`\A(?:` + p + `)\z`)
		forArbitraryChoices(t, StringMatching(p), 300, func(s string) string {
			if !whole.MatchString(s) {
				return "does not match " + p
			}
			return ""
		})
	}
}
