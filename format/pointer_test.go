// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
)

func checkPointer(s string) string {
	if !utf8.ValidString(s) {
		return "UTF-8"
	}
	if s == "" {
		return ""
	}
	if s[0] != '/' {
		return "initial slash"
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '~' {
			i++
			if i == len(s) || s[i] != '0' && s[i] != '1' {
				return "escape"
			}
		}
	}
	return ""
}
func TestJSONPointers(t *testing.T) {
	for _, g := range []pbt.Gen[string]{JSONPointer(), JSONPointerFragment()} {
		t.Run(g.String(), func(t *testing.T) {
			fragment := g.String() == "JSONPointerFragment()"
			checkValid(t, g, func(s string) string {
				if fragment {
					if !regexp.MustCompile(`^#(?:[a-zA-Z0-9._~!$&'()*+,;=:@/?-]|%[0-9A-F]{2})*$`).MatchString(s) {
						return "fragment syntax"
					}
					u, e := url.Parse(s)
					if e != nil {
						return e.Error()
					}
					s = u.Fragment
				}
				return checkPointer(s)
			})
			pbt.Check(t, func(pt *pbt.T) {
				s := g.Draw(pt, "")
				pt.Cover(strings.Contains(s, "~0"), 25, "tilde")
				pt.Cover(strings.Contains(s, "~1"), 25, "slash")
				pt.Cover(strings.Contains(s, "//"), 1, "empty token")
				if fragment {
					pt.Cover(strings.Contains(s, "%"), 25, "percent encoding")
				}
			}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
			want := ""
			if fragment {
				want = "#"
			}
			checkMinimumSeeds(t, g, want)
			if s := minimalOf(t, g, always); s != want {
				t.Errorf("minimum %q", s)
			}
			if s := minimalOf(t, g, func(s string) bool { return len(s) > len(want) }); s != want+"/" {
				t.Errorf("nonempty minimum %q", s)
			}
		})
	}
}
