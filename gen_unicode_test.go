// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestRuneInStaysInItsTables(t *testing.T) {
	cases := []struct {
		name   string
		tables []*unicode.RangeTable
	}{
		{"Han, Hiragana, Katakana", []*unicode.RangeTable{unicode.Han, unicode.Hiragana, unicode.Katakana}},
		{"Nd", []*unicode.RangeTable{unicode.Nd}},
		{"L", []*unicode.RangeTable{unicode.L}},
		{"White_Space", []*unicode.RangeTable{unicode.White_Space}},
		{"C, with surrogates", []*unicode.RangeTable{unicode.C}},
		{"strided", []*unicode.RangeTable{{R16: []unicode.Range16{{Lo: 0xD7F0, Hi: 0xE010, Stride: 7}}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			forArbitraryChoices(t, RuneIn(c.tables...), 3000, func(r rune) string {
				if !utf8.ValidRune(r) {
					return "invalid rune"
				}
				if !unicode.In(r, c.tables...) {
					return "outside the tables"
				}
				return ""
			})
		})
	}
}

func TestRuneInNamesAndShrinks(t *testing.T) {
	g := RuneIn(unicode.Han, unicode.Hiragana, unicode.Nd, unicode.White_Space)
	if g.String() != "RuneIn(Han, Hiragana, Nd, White_Space)" {
		t.Errorf("name %q", g)
	}
	if got := minimalOf(t, RuneIn(unicode.Hiragana, unicode.Latin), func(rune) bool { return true }); got != rune(unicode.Hiragana.R16[0].Lo) {
		t.Errorf("shrinks to %U", got)
	}
	if got := minimalOf(t, RuneIn(unicode.Latin), func(r rune) bool { return r > 'z' }); got != 'ª' {
		t.Errorf("above z shrinks to %U", got)
	}
}

func TestRuneInCoversSmallScripts(t *testing.T) {
	Check(t, func(pt *T) {
		r := RuneIn(unicode.Han, unicode.Hiragana, unicode.Katakana).Draw(pt, "r")
		pt.Cover(unicode.Is(unicode.Hiragana, r), 5, "hiragana")
		pt.Cover(unicode.Is(unicode.Katakana, r), 5, "katakana")
		pt.Cover(r > 0xFFFF, 3, "outside the BMP")
	}, Seed(1), Checks(1000), NoSave(), CheckCoverage())
}

func TestRuneInPanics(t *testing.T) {
	for _, c := range []struct {
		name string
		f    func()
		want string
	}{
		{"no tables", func() { RuneIn() }, "at least one table"},
		{"nil", func() { RuneIn(nil) }, "nil table"},
		{"surrogates only", func() { RuneIn(unicode.Cs) }, "no valid runes"},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if msg, _ := recover().(string); !strings.Contains(msg, c.want) {
					t.Errorf("panic %q", msg)
				}
			}()
			c.f()
		})
	}
}
